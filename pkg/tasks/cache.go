package tasks

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// The server re-parses the vault on every request. That is fine for a folder of
// task notes, but a vault also holds big files with no tasks in them (a
// handwriting-export note with embedded images can be 10MB+), and reading
// hundreds of megabytes per request made every call take seconds. Parsing is a
// pure function of a file's content, so each file's result is cached and only
// re-read when the file's modification time or size changes.
//
// A change is detected by mtime + size. That alone is not enough: file
// timestamps are coarse (a few ms), so a rewrite that keeps the size the same
// (ticking `[ ]` to `[x]`) right after a read can carry an identical mtime and
// look unchanged. Git's index has the same "racy" problem and the same answer,
// used here: an entry is only trusted once the file's mtime is at least
// racyWindow older than the moment it was cached. A file modified moments ago is
// simply re-read until it has settled, which costs nothing for the big old files
// this cache exists for.

type cacheKey struct {
	abs string
	rel string // Task.FilePath is derived from it, so it is part of the identity
}

type cacheEntry struct {
	mtimeNano int64
	size      int64
	cachedAt  time.Time
	tasks     []Task
	err       error
}

// racyWindow: see the comment above. It must exceed the filesystem's timestamp
// granularity (ext4/btrfs ~4ms; FAT/HFS+/some network mounts 1-2s).
const racyWindow = 3 * time.Second

// maxCacheEntries bounds memory if a vault churns through file names: past it
// the whole cache is dropped and refills on demand.
const maxCacheEntries = 100000

var (
	parseCacheMu sync.Mutex
	parseCache   = map[cacheKey]*cacheEntry{}

	// parseMisses counts real parses (cache misses); tests use it to prove a
	// second scan didn't re-read anything.
	parseMisses atomic.Int64
)

// parseFileCached is parseFile with the per-file cache. The returned slice is
// the caller's to modify.
func parseFileCached(absPath, relPath string) ([]Task, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	key := cacheKey{abs: absPath, rel: relPath}
	mtime, size := info.ModTime().UnixNano(), info.Size()

	parseCacheMu.Lock()
	e := parseCache[key]
	parseCacheMu.Unlock()
	if e != nil && e.mtimeNano == mtime && e.size == size && e.settled() {
		return cloneTasks(e.tasks), e.err
	}

	parseMisses.Add(1)
	// Take the timestamp before reading: an edit landing during the read then
	// makes the entry racy (or mismatched), never wrongly settled.
	cachedAt := time.Now()
	tasks, perr := parseFile(absPath, relPath)

	parseCacheMu.Lock()
	if len(parseCache) >= maxCacheEntries {
		parseCache = map[cacheKey]*cacheEntry{}
	}
	parseCache[key] = &cacheEntry{mtimeNano: mtime, size: size, cachedAt: cachedAt, tasks: cloneTasks(tasks), err: perr}
	parseCacheMu.Unlock()
	return tasks, perr
}

// settled reports whether the file was already old, relative to when it was
// cached, so an unnoticed same-size edit in the same timestamp tick is impossible.
// A modification time in the future never settles.
func (e *cacheEntry) settled() bool {
	return e.cachedAt.Sub(time.Unix(0, e.mtimeNano)) >= racyWindow
}

// cloneTasks copies tasks deeply enough that mutating the copy (setting Type,
// appending a tag, ...) can't reach the cached originals.
func cloneTasks(in []Task) []Task {
	if in == nil {
		return nil
	}
	out := make([]Task, len(in))
	copy(out, in)
	for i := range out {
		if out[i].Tags != nil {
			out[i].Tags = append([]string(nil), out[i].Tags...)
		}
		if out[i].Subtasks != nil {
			out[i].Subtasks = append([]Subtask(nil), out[i].Subtasks...)
		}
	}
	return out
}

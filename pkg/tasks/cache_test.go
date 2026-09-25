package tasks

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newVault(t *testing.T) string {
	t.Helper()
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Tasks"), 0755)
	return dir
}

// age backdates a file's mtime so the cache treats it as settled (see racyWindow).
func age(t *testing.T, p string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
}

func titles(ts []Task) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Title)
	}
	return out
}

func TestUnchangedFilesAreNotReparsed(t *testing.T) {
	dir := newVault(t)
	os.WriteFile(filepath.Join(dir, "Tasks", "A.md"), []byte("# A\n- [ ] one #ToDo\n- [ ] two\n"), 0644)
	os.WriteFile(filepath.Join(dir, "Tasks", "B.md"), []byte("- [ ] three\n"), 0644)
	age(t, filepath.Join(dir, "Tasks", "A.md"))
	age(t, filepath.Join(dir, "Tasks", "B.md"))

	before := parseMisses.Load()
	first, err := ParseFolders(dir, []string{"Tasks"})
	if err != nil || len(first) != 3 {
		t.Fatalf("first scan: %v %v", titles(first), err)
	}
	if got := parseMisses.Load() - before; got != 2 {
		t.Fatalf("first scan should parse both files, parsed %d", got)
	}

	mid := parseMisses.Load()
	second, _ := ParseFolders(dir, []string{"Tasks"})
	if parseMisses.Load() != mid {
		t.Fatalf("second scan re-parsed %d unchanged files", parseMisses.Load()-mid)
	}
	if len(second) != 3 || second[0].Title != first[0].Title {
		t.Fatalf("cached scan differs: %v vs %v", titles(second), titles(first))
	}
}

func TestEditsAreSeenWhetherOrNotTheSizeChanges(t *testing.T) {
	dir := newVault(t)
	p := filepath.Join(dir, "Tasks", "A.md")
	os.WriteFile(p, []byte("- [ ] alpha\n"), 0644)
	age(t, p)
	ParseFolders(dir, []string{"Tasks"})

	// Different size.
	os.WriteFile(p, []byte("- [ ] alpha\n- [ ] beta\n"), 0644)
	age(t, p)
	ts, _ := ParseFolders(dir, []string{"Tasks"})
	if len(ts) != 2 {
		t.Fatalf("size-changing edit not seen: %v", titles(ts))
	}

	// Same size, later mtime (e.g. a task renamed to a same-length title).
	os.WriteFile(p, []byte("- [ ] alpha\n- [ ] gamma\n"), 0644)
	older := time.Now().Add(-30 * time.Minute)
	os.Chtimes(p, older, older)
	ts, _ = ParseFolders(dir, []string{"Tasks"})
	if got := titles(ts); len(got) != 2 || got[1] != "gamma" {
		t.Fatalf("same-size edit not seen: %v", got)
	}

	// A file added or removed between scans.
	os.WriteFile(filepath.Join(dir, "Tasks", "New.md"), []byte("- [ ] fresh\n"), 0644)
	ts, _ = ParseFolders(dir, []string{"Tasks"})
	if len(ts) != 3 {
		t.Fatalf("new file not seen: %v", titles(ts))
	}
	os.Remove(p)
	ts, _ = ParseFolders(dir, []string{"Tasks"})
	if len(ts) != 1 || ts[0].Title != "fresh" {
		t.Fatalf("removed file still returned: %v", titles(ts))
	}
}

func TestOurOwnWritesInvalidateTheCache(t *testing.T) {
	dir := newVault(t)
	p := filepath.Join(dir, "Tasks", "A.md")
	os.WriteFile(p, []byte("- [ ] alpha #ToDo\n"), 0644)
	age(t, p)
	ParseFolders(dir, []string{"Tasks"}) // warm: file is old, so the entry is trusted

	if err := ToggleStatus(p, 1, StatusCompleted); err != nil {
		t.Fatal(err)
	}
	ts, _ := ParseFolders(dir, []string{"Tasks"})
	if len(ts) != 1 || ts[0].Status != StatusCompleted {
		t.Fatalf("toggle not visible through the cache: %+v", ts)
	}
	if _, err := AddTask(p, NewTask{Title: "added"}); err != nil {
		t.Fatal(err)
	}
	if ts, _ = ParseFolders(dir, []string{"Tasks"}); len(ts) != 2 {
		t.Fatalf("AddTask not visible through the cache: %v", titles(ts))
	}
}

func TestCallersCannotCorruptCachedResults(t *testing.T) {
	dir := newVault(t)
	os.WriteFile(filepath.Join(dir, "Tasks", "A.md"), []byte("- [ ] alpha #ToDo #x\n"), 0644)
	age(t, filepath.Join(dir, "Tasks", "A.md"))

	first, _ := ParseFolders(dir, []string{"Tasks"})
	first[0].Type = "event"
	first[0].Title = "mutated"
	first[0].Tags[0] = "MUTATED"
	first[0].Subtasks = append(first[0].Subtasks, Subtask{Title: "junk"})

	again, _ := ParseFolders(dir, []string{"Tasks"})
	if again[0].Type != "task" || again[0].Title != "alpha" || again[0].Tags[0] != "ToDo" || len(again[0].Subtasks) != 0 {
		t.Fatalf("cache was corrupted by a caller's mutation: %+v", again[0])
	}
}

func TestParseCacheIsSafeUnderConcurrency(t *testing.T) {
	dir := newVault(t)
	for _, n := range []string{"A", "B", "C"} {
		os.WriteFile(filepath.Join(dir, "Tasks", n+".md"), []byte("- [ ] "+n+" #ToDo\n"), 0644)
		age(t, filepath.Join(dir, "Tasks", n+".md"))
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				ts, _ := ParseFolders(dir, []string{"Tasks"})
				if len(ts) != 3 {
					t.Errorf("got %d tasks", len(ts))
					return
				}
				ts[0].Type = "event" // must only touch our copy
			}
		}()
	}
	wg.Wait()
}

// The flaw this guards against: a same-size rewrite in the same timestamp tick
// as the read that filled the cache. A just-modified file must never be served
// stale, however quickly the edits follow each other.
func TestRecentlyModifiedFilesAreNeverServedStale(t *testing.T) {
	dir := newVault(t)
	p := filepath.Join(dir, "Tasks", "A.md")
	os.WriteFile(p, []byte("- [ ] alpha #ToDo\n"), 0644)

	for i := 0; i < 200; i++ {
		want := StatusCompleted
		if i%2 == 1 {
			want = StatusTodo
		}
		if err := ToggleStatus(p, 1, want); err != nil { // same size every time
			t.Fatal(err)
		}
		ts, _ := ParseFolders(dir, []string{"Tasks"})
		if len(ts) != 1 || ts[0].Status != want {
			t.Fatalf("iteration %d: stale result, got %v want %v", i, ts[0].Status, want)
		}
	}
}

func TestRecentFilesAreReparsedOldFilesAreNot(t *testing.T) {
	dir := newVault(t)
	fresh := filepath.Join(dir, "Tasks", "Fresh.md")
	old := filepath.Join(dir, "Tasks", "Old.md")
	os.WriteFile(fresh, []byte("- [ ] f\n"), 0644)
	os.WriteFile(old, []byte("- [ ] o\n"), 0644)
	age(t, old)
	ParseFolders(dir, []string{"Tasks"})

	before := parseMisses.Load()
	ParseFolders(dir, []string{"Tasks"})
	if got := parseMisses.Load() - before; got != 1 {
		t.Fatalf("expected exactly the fresh file to be re-read, got %d parses", got)
	}
}

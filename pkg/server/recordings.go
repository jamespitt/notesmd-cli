package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

// Audio recordings pushed from the AIREC Android app. They are large binary
// files, so they are kept outside the vaults (which are git-synced) in one
// folder shared by every vault: $NOTESMD_RECORDINGS_DIR, else
// "recordings_folder" in preferences.json, else ~/airec_recordings.
const (
	defaultRecordingsFolder = "airec_recordings"
	maxRecordingBytes       = 2 << 30 // 2 GiB
)

// recordingNameRe keeps an upload inside the recordings folder: a bare file
// name, no separators, no leading dot.
var recordingNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

var recordingExts = map[string]bool{
	".wav": true, ".bin": true, ".opus": true, ".ogg": true,
	".mp3": true, ".m4a": true, ".txt": true,
}

func recordingsDir() (string, error) {
	dir := os.Getenv("NOTESMD_RECORDINGS_DIR")
	if dir == "" {
		if _, cliConfigFile, err := obsidian.CliConfigPath(); err == nil {
			if content, err := os.ReadFile(cliConfigFile); err == nil {
				cliConfig := obsidian.CliConfig{}
				if json.Unmarshal(content, &cliConfig) == nil {
					dir = cliConfig.RecordingsFolder
				}
			}
		}
	}
	if dir == "" {
		dir = "~/" + defaultRecordingsFolder
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("recordings folder %q must be an absolute path", dir)
	}
	return dir, nil
}

func validRecordingName(name string) bool {
	return recordingNameRe.MatchString(name) && recordingExts[strings.ToLower(filepath.Ext(name))]
}

type recordingInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// GET /api/recordings
func (s *Server) listRecordings(w http.ResponseWriter, r *http.Request) {
	dir, err := recordingsDir()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordings := []recordingInfo{}
	for _, e := range entries {
		if e.IsDir() || !validRecordingName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		recordings = append(recordings, recordingInfo{
			Name:     e.Name(),
			Size:     info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(recordings, func(i, j int) bool { return recordings[i].Name < recordings[j].Name })
	jsonOK(w, map[string]any{"recordings": recordings})
}

// PUT /api/recordings/{name}
// Body: the raw file. Replaces any existing recording of the same name.
func (s *Server) putRecording(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validRecordingName(name) {
		jsonError(w, http.StatusBadRequest, "invalid recording name")
		return
	}
	dir, err := recordingsDir()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Write to a temp file and rename, so a dropped connection never leaves a
	// truncated recording under its real name.
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck

	size, err := io.Copy(tmp, http.MaxBytesReader(w, r.Body, maxRecordingBytes))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, "upload failed: "+err.Error())
		return
	}
	if size == 0 {
		jsonError(w, http.StatusBadRequest, "empty upload")
		return
	}
	if r.ContentLength > 0 && size != r.ContentLength {
		jsonError(w, http.StatusBadRequest, "upload truncated")
		return
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonCreated(w, map[string]any{"name": name, "size": size})
}

// Resumable upload: the client sends a recording as a series of chunks, each
// appended to a hidden ".part-<name>" file, and can ask how much has arrived
// so far - so a dropped connection costs one chunk, not the whole file.

// partialLocks serialises the chunk writes of one recording; a second writer
// for the same name is refused rather than queued behind a stalled connection.
var (
	partialLocksMu sync.Mutex
	partialLocks   = map[string]*sync.Mutex{}
)

func partialLock(name string) *sync.Mutex {
	partialLocksMu.Lock()
	defer partialLocksMu.Unlock()
	mu := partialLocks[name]
	if mu == nil {
		mu = &sync.Mutex{}
		partialLocks[name] = mu
	}
	return mu
}

func partialPath(dir, name string) string {
	return filepath.Join(dir, ".part-"+name)
}

func queryInt(r *http.Request, key string) (int64, bool) {
	v, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return v, err == nil
}

// GET /api/recordings/{name}/partial?total=N
// Where an upload of this recording should (re)start from.
func (s *Server) getRecordingPartial(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validRecordingName(name) {
		jsonError(w, http.StatusBadRequest, "invalid recording name")
		return
	}
	dir, err := recordingsDir()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if total, ok := queryInt(r, "total"); ok && total > 0 {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.Size() == total {
			jsonOK(w, map[string]any{"offset": total, "complete": true})
			return
		}
	}
	var offset int64
	if info, err := os.Stat(partialPath(dir, name)); err == nil {
		offset = info.Size()
	}
	jsonOK(w, map[string]any{"offset": offset, "complete": false})
}

// PUT /api/recordings/{name}/partial?offset=N&total=T[&sha256=hex]
// Body: the bytes of the file from offset. offset=0 starts over.
func (s *Server) putRecordingPartial(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validRecordingName(name) {
		jsonError(w, http.StatusBadRequest, "invalid recording name")
		return
	}
	offset, okOffset := queryInt(r, "offset")
	total, okTotal := queryInt(r, "total")
	if !okOffset || !okTotal || total <= 0 || total > maxRecordingBytes || offset < 0 || offset >= total {
		jsonError(w, http.StatusBadRequest, "offset and total are required, with 0 <= offset < total")
		return
	}
	dir, err := recordingsDir()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	mu := partialLock(name)
	if !mu.TryLock() {
		jsonError(w, http.StatusConflict, "another upload of this recording is in progress")
		return
	}
	defer mu.Unlock()

	part := partialPath(dir, name)
	f, err := os.OpenFile(part, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if offset != 0 && info.Size() != offset {
		// The client is out of step (e.g. it never saw the reply to its last
		// chunk): tell it where to carry on from.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]any{"error": "offset mismatch", "offset": info.Size()}) //nolint:errcheck
		return
	}
	if err := f.Truncate(offset); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	n, err := io.Copy(f, http.MaxBytesReader(w, r.Body, total-offset))
	if err == nil && r.ContentLength > 0 && n != r.ContentLength {
		err = io.ErrUnexpectedEOF
	}
	if err != nil || n == 0 {
		// Keep only whole chunks, so the stored offset is always one the client sent.
		f.Truncate(offset) //nolint:errcheck
		jsonError(w, http.StatusBadRequest, "chunk upload failed")
		return
	}
	if offset+n < total {
		jsonOK(w, map[string]any{"offset": offset + n, "complete": false})
		return
	}

	if want := strings.ToLower(r.URL.Query().Get("sha256")); want != "" {
		h := sha256.New()
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			_, err = io.Copy(h, f)
		}
		if err != nil || hex.EncodeToString(h.Sum(nil)) != want {
			os.Remove(part) //nolint:errcheck
			jsonError(w, http.StatusUnprocessableEntity, "checksum mismatch; upload discarded, start again from offset 0")
			return
		}
	}
	if err := f.Close(); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.Rename(part, filepath.Join(dir, name)); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonCreated(w, map[string]any{"name": name, "size": total, "offset": total, "complete": true})
}

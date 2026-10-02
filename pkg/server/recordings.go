package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

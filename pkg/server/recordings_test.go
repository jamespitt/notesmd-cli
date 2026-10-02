package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPutAndListRecordings(t *testing.T) {
	h, _, _ := twoVaultServer(t)
	dir := filepath.Join(t.TempDir(), "recordings")
	t.Setenv("NOTESMD_RECORDINGS_DIR", dir)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/recordings", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"recordings":[]}`, rec.Body.String())

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/recordings/20261002151000.wav?vault=work", strings.NewReader("RIFFdata")))
	assert.Equal(t, http.StatusCreated, rec.Code)

	saved, err := os.ReadFile(filepath.Join(dir, "20261002151000.wav"))
	assert.NoError(t, err)
	assert.Equal(t, "RIFFdata", string(saved))

	// Uploading again replaces the file and leaves no temp files behind.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/recordings/20261002151000.wav", strings.NewReader("RIFFlonger")))
	assert.Equal(t, http.StatusCreated, rec.Code)
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/recordings", nil))
	var body struct {
		Recordings []recordingInfo
	}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Recordings, 1)
	assert.Equal(t, "20261002151000.wav", body.Recordings[0].Name)
	assert.Equal(t, int64(10), body.Recordings[0].Size)
}

func TestPutRecordingRejectsBadUploads(t *testing.T) {
	h, _, _ := twoVaultServer(t)
	dir := t.TempDir()
	t.Setenv("NOTESMD_RECORDINGS_DIR", dir)

	for _, name := range []string{".hidden.wav", "notes.md", "a..%2F..%2Fx.wav", "noext"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/recordings/"+name, strings.NewReader("x")))
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/recordings/empty.wav", strings.NewReader("")))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

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

func TestResumableRecordingUpload(t *testing.T) {
	h, _, _ := twoVaultServer(t)
	dir := t.TempDir()
	t.Setenv("NOTESMD_RECORDINGS_DIR", dir)

	do := func(method, url, body string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, url, strings.NewReader(body)))
		var out map[string]any
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		return rec.Code, out
	}
	const base = "/api/recordings/a.wav/partial"
	// sha256("helloworld")
	const sum = "936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"

	code, out := do("GET", base+"?total=10", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), out["offset"])

	code, out = do("PUT", base+"?offset=0&total=10", "hello")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(5), out["offset"])

	// The partial file is not a recording yet.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/recordings", nil))
	assert.JSONEq(t, `{"recordings":[]}`, rec.Body.String())

	// A chunk at the wrong offset is refused and reports where to resume.
	code, out = do("PUT", base+"?offset=3&total=10", "xx")
	assert.Equal(t, http.StatusConflict, code)
	assert.Equal(t, float64(5), out["offset"])

	// A chunk running past the declared total is dropped, keeping the offset.
	code, _ = do("PUT", base+"?offset=5&total=10", "worldEXTRA")
	assert.Equal(t, http.StatusBadRequest, code)
	code, out = do("GET", base+"?total=10", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(5), out["offset"])

	code, out = do("PUT", base+"?offset=5&total=10&sha256="+sum, "world")
	assert.Equal(t, http.StatusCreated, code)
	assert.Equal(t, true, out["complete"])

	saved, err := os.ReadFile(filepath.Join(dir, "a.wav"))
	assert.NoError(t, err)
	assert.Equal(t, "helloworld", string(saved))
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	code, out = do("GET", base+"?total=10", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, out["complete"])

	// A bad checksum discards the upload.
	code, _ = do("PUT", "/api/recordings/b.wav/partial?offset=0&total=5&sha256="+sum, "hello")
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	_, err = os.Stat(filepath.Join(dir, ".part-b.wav"))
	assert.True(t, os.IsNotExist(err))
}

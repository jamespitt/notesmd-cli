package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yakitrak/notesmd-cli/mocks"
	"github.com/stretchr/testify/assert"
)

func testServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	vaultDir := t.TempDir()
	s := New(&mocks.MockVaultOperator{PathValue: vaultDir, Name: "v"}, &mocks.MockNoteManager{})
	return s.Handler(), vaultDir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestListProjectPagesHandler(t *testing.T) {
	h, vault := testServer(t)
	proj := filepath.Join(vault, "Projects", "Trip")
	write(t, filepath.Join(proj, "Trip.md"), "---\ntags: Project\n---\n# Trip\n")
	write(t, filepath.Join(proj, "Notes", "Budget.md"), "# Budget\n")

	req := httptest.NewRequest("GET", "/api/projects/Trip/pages", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Pages []struct{ Path, Name, Rel string }
	}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Pages, 2)
	assert.Equal(t, "Projects/Trip/Notes/Budget.md", body.Pages[0].Path)
}

func TestPostProjectDiaryHandler(t *testing.T) {
	h, vault := testServer(t)
	write(t, filepath.Join(vault, "Projects", "Trip", "Trip.md"),
		"---\ntags: Project\ntitle: Big Trip\n---\n# Trip\n")

	req := httptest.NewRequest("POST", "/api/projects/Trip/diary",
		strings.NewReader(`{"text":"Booked the ferry."}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	onDisk, err := os.ReadFile(filepath.Join(vault, "Projects", "Trip", "Diary.md"))
	assert.NoError(t, err)
	assert.Contains(t, string(onDisk), "Booked the ferry.")
	assert.Contains(t, string(onDisk), "# Big Trip Diary")

	// GET returns the same content.
	greq := httptest.NewRequest("GET", "/api/projects/Trip/diary", nil)
	grec := httptest.NewRecorder()
	h.ServeHTTP(grec, greq)
	assert.Equal(t, http.StatusOK, grec.Code)
	var gbody struct {
		Content string
		Exists  bool
	}
	assert.NoError(t, json.Unmarshal(grec.Body.Bytes(), &gbody))
	assert.True(t, gbody.Exists)
	assert.Equal(t, string(onDisk), gbody.Content)
}

func TestPostProjectDiaryRejectsEmpty(t *testing.T) {
	h, _ := testServer(t)
	req := httptest.NewRequest("POST", "/api/projects/Trip/diary", strings.NewReader(`{"text":"  "}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

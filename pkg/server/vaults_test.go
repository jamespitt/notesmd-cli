package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Yakitrak/notesmd-cli/mocks"
	"github.com/stretchr/testify/assert"
)

// twoVaultServer serves a "personal" (default) and a "work" vault, each with
// one task file, and returns the handler plus both vault directories.
func twoVaultServer(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	personalDir := t.TempDir()
	workDir := t.TempDir()

	write(t, filepath.Join(personalDir, "Tasks", "Home.md"), "- [ ] Buy milk\n")
	write(t, filepath.Join(workDir, "Action Items.md"), "- [ ] Write the PRD\n")

	s := NewMulti([]Vault{
		{ID: "personal", Label: "Personal", Manager: &mocks.MockVaultOperator{PathValue: personalDir, Name: "personal"}},
		{ID: "work", Label: "Work", Manager: &mocks.MockVaultOperator{PathValue: workDir, Name: "work"}},
	}, &mocks.MockNoteManager{})

	return s.Handler(), personalDir, workDir
}

func getTaskTitles(t *testing.T, h http.Handler, url string) (int, []string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))

	var body struct {
		Tasks []struct{ Title string }
	}
	if rec.Code == http.StatusOK {
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	}
	titles := make([]string, 0, len(body.Tasks))
	for _, task := range body.Tasks {
		titles = append(titles, task.Title)
	}
	return rec.Code, titles
}

func TestTasksUseRequestedVault(t *testing.T) {
	h, _, _ := twoVaultServer(t)

	code, titles := getTaskTitles(t, h, "/api/tasks?vault=work")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"Write the PRD"}, titles)

	code, titles = getTaskTitles(t, h, "/api/tasks?vault=personal")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"Buy milk"}, titles)
}

func TestTasksWithoutVaultParamUseFirstVault(t *testing.T) {
	h, _, _ := twoVaultServer(t)

	code, titles := getTaskTitles(t, h, "/api/tasks")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"Buy milk"}, titles)
}

func TestVaultCanBeSelectedByHeader(t *testing.T) {
	h, _, _ := twoVaultServer(t)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	req.Header.Set("X-Vault", "work")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Write the PRD")
}

// An unknown id must fail rather than silently fall back to the default vault -
// a mistyped ?vault= writing into the wrong vault is the failure that matters.
func TestUnknownVaultIsRejected(t *testing.T) {
	h, _, _ := twoVaultServer(t)

	code, _ := getTaskTitles(t, h, "/api/tasks?vault=nope")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestListVaults(t *testing.T) {
	h, _, _ := twoVaultServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/vaults?vault=work", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Vaults []struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			Default bool   `json:"default"`
		}
		Active string `json:"active"`
	}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Vaults, 2)
	assert.Equal(t, "personal", body.Vaults[0].ID)
	assert.True(t, body.Vaults[0].Default)
	assert.Equal(t, "Work", body.Vaults[1].Label)
	assert.False(t, body.Vaults[1].Default)
	assert.Equal(t, "work", body.Active)
}

// A single-vault server (New) keeps answering requests that name no vault, and
// also answers to the "default" id.
func TestSingleVaultServerBackCompat(t *testing.T) {
	vaultDir := t.TempDir()
	write(t, filepath.Join(vaultDir, "Tasks", "Home.md"), "- [ ] Buy milk\n")
	s := New(&mocks.MockVaultOperator{PathValue: vaultDir, Name: "v"}, &mocks.MockNoteManager{})
	h := s.Handler()

	code, titles := getTaskTitles(t, h, "/api/tasks")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"Buy milk"}, titles)

	code, titles = getTaskTitles(t, h, "/api/tasks?vault=default")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"Buy milk"}, titles)
}

// Writes must land in the vault the request named, not the default one.
func TestAddTaskWritesToRequestedVault(t *testing.T) {
	h, personalDir, workDir := twoVaultServer(t)

	req := httptest.NewRequest("POST", "/api/tasks/list/Action%20Items?vault=work",
		jsonBody(`{"title":"Book the room"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)

	assert.FileExists(t, filepath.Join(workDir, "Action Items.md"))
	assert.Contains(t, readFile(t, filepath.Join(workDir, "Action Items.md")), "Book the room")
	assert.NotContains(t, readFile(t, filepath.Join(personalDir, "Tasks", "Home.md")), "Book the room")
}

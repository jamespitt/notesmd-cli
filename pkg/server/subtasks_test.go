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

func subtaskServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Tasks", "Work.md"),
		"# Work\n- [ ] Epic #InProgress\n    - [ ] Child #ToDo\n- [ ] Loose #ToDo\n- [ ] Other\n")
	write(t, filepath.Join(dir, "Tasks", "Home.md"), "# Home\n- [ ] Chores\n")
	s := New(&mocks.MockVaultOperator{PathValue: dir, Name: "v"}, &mocks.MockNoteManager{})
	return s.Handler(), dir
}

func patch(t *testing.T, h http.Handler, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("PATCH", path, jsonBody(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestKanbanEndpointFoldsSubtasksIntoParentCard(t *testing.T) {
	h, _ := subtaskServer(t)
	req := httptest.NewRequest("GET", "/api/tasks/kanban", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Tasks []struct {
			Title    string `json:"title"`
			Subtasks []struct {
				Title string `json:"title"`
				Level int    `json:"level"`
			} `json:"subtasks"`
		} `json:"tasks"`
	}
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	var titles []string
	for _, tk := range body.Tasks {
		titles = append(titles, tk.Title)
	}
	assert.Equal(t, []string{"Epic", "Loose"}, titles, "Child is inside Epic, not a card of its own")
	assert.Len(t, body.Tasks[0].Subtasks, 1)
	assert.Equal(t, "Child", body.Tasks[0].Subtasks[0].Title)
	assert.Equal(t, 1, body.Tasks[0].Subtasks[0].Level)
}

func TestSetParentActionSameFile(t *testing.T) {
	h, dir := subtaskServer(t)
	rec, out := patch(t, h, "/api/tasks/Tasks/Work.md", `{"action":"set-parent","line":4,"parent_line":2}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "Tasks/Work.md", out["path"])
	assert.EqualValues(t, 4, out["line"])
	assert.Equal(t, "# Work\n- [ ] Epic #InProgress\n    - [ ] Child #ToDo\n    - [ ] Loose #ToDo\n- [ ] Other\n",
		readFile(t, filepath.Join(dir, "Tasks", "Work.md")))
}

func TestSetParentActionAcrossFilesReportsTheNewLocation(t *testing.T) {
	h, dir := subtaskServer(t)
	rec, out := patch(t, h, "/api/tasks/Tasks/Work.md", `{"action":"set-parent","line":5,"parent_line":2,"parent_path":"Tasks/Home"}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "Tasks/Home.md", out["path"])
	assert.EqualValues(t, 3, out["line"])
	assert.Equal(t, "# Home\n- [ ] Chores\n    - [ ] Other\n", readFile(t, filepath.Join(dir, "Tasks", "Home.md")))
	assert.NotContains(t, readFile(t, filepath.Join(dir, "Tasks", "Work.md")), "Other")
}

func TestSetParentActionPromote(t *testing.T) {
	h, dir := subtaskServer(t)
	rec, out := patch(t, h, "/api/tasks/Tasks/Work.md", `{"action":"set-parent","line":3,"parent_line":0}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.EqualValues(t, 3, out["line"])
	assert.Equal(t, "# Work\n- [ ] Epic #InProgress\n- [ ] Child #ToDo\n- [ ] Loose #ToDo\n- [ ] Other\n",
		readFile(t, filepath.Join(dir, "Tasks", "Work.md")))
}

func TestSetParentActionRejectsBadRequests(t *testing.T) {
	h, dir := subtaskServer(t)
	before := readFile(t, filepath.Join(dir, "Tasks", "Work.md"))

	for name, body := range map[string]string{
		"self parent":       `{"action":"set-parent","line":2,"parent_line":2}`,
		"own descendant":    `{"action":"set-parent","line":2,"parent_line":3}`,
		"no parent_line":    `{"action":"set-parent","line":3}`,
		"negative parent":   `{"action":"set-parent","line":3,"parent_line":-1}`,
		"parent not a task": `{"action":"set-parent","line":3,"parent_line":1}`,
	} {
		rec, _ := patch(t, h, "/api/tasks/Tasks/Work.md", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
	rec, _ := patch(t, h, "/api/tasks/Tasks/Work.md", `{"action":"set-parent","line":3,"parent_line":2,"parent_path":"../../etc/passwd"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "path outside the vault")
	rec, _ = patch(t, h, "/api/tasks/Tasks/Work.md", `{"action":"set-parent","line":3,"parent_line":2,"parent_path":"Tasks/Nope"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, "missing parent file")

	assert.Equal(t, before, readFile(t, filepath.Join(dir, "Tasks", "Work.md")), "rejected requests must not touch the file")
}

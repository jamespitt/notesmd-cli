package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
	"github.com/stretchr/testify/assert"
)

type mockVaultManager struct {
	path        string
	taskFolders []string
}

func (m *mockVaultManager) DefaultName() (string, error)    { return "test", nil }
func (m *mockVaultManager) SetDefaultName(string) error      { return nil }
func (m *mockVaultManager) Path() (string, error)            { return m.path, nil }
func (m *mockVaultManager) DefaultOpenType() (string, error) { return "", nil }
func (m *mockVaultManager) TaskFolders() ([]string, error)   { return m.taskFolders, nil }
func (m *mockVaultManager) ProjectsFolder() (string, error)  { return "Projects", nil }
func (m *mockVaultManager) CalendarFolder() (string, error)  { return "Journal/Calendar", nil }

func TestLocalClient(t *testing.T) {
	dir := t.TempDir()
	taskDir := filepath.Join(dir, "Tasks")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `# Tasks
- [ ] Task 1 #ToDo [id::taskid1111] [google_id::goog123]
- [ ] Task 2 #InProgress [id::taskid2222]
`
	listPath := filepath.Join(taskDir, "Obsidian.md")
	if err := os.WriteFile(listPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	vm := &mockVaultManager{
		path:        dir,
		taskFolders: []string{"Tasks"},
	}

	cli, err := NewClient(Config{
		ForceLocal:   true,
		VaultManager: vm,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	t.Run("FindTask by google_id", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "goog123")
		assert.NoError(t, err)
		assert.Equal(t, "Task 1", task.Title)
		assert.Equal(t, "goog123", task.GoogleID)
		assert.Equal(t, "taskid1111", task.ID)
	})

	t.Run("FindTask by task id", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "taskid2222")
		assert.NoError(t, err)
		assert.Equal(t, "Task 2", task.Title)
		assert.Equal(t, "taskid2222", task.ID)
	})

	t.Run("ListTasks", func(t *testing.T) {
		all, err := cli.ListTasks(ctx, "all")
		assert.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("SetStatusTag and ToggleStatus", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "taskid1111")
		assert.NoError(t, err)
		err = cli.SetStatusTag(ctx, task, "Done", nil)
		assert.NoError(t, err)

		updated, err := cli.FindTask(ctx, "taskid1111")
		assert.NoError(t, err)
		assert.Equal(t, tasks.StatusCompleted, updated.Status)
	})

	t.Run("RenameTask", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "taskid2222")
		assert.NoError(t, err)
		err = cli.RenameTask(ctx, task, "Renamed Task 2")
		assert.NoError(t, err)

		updated, err := cli.FindTask(ctx, "taskid2222")
		assert.NoError(t, err)
		assert.Equal(t, "Renamed Task 2", updated.Title)
	})
}

func TestHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/tasks":
			tasksList := []tasks.Task{
				{FilePath: "Tasks/Obsidian.md", LineNum: 2, Title: "HTTP Task 1", ID: "httpid1111", GoogleID: "g1"},
				{FilePath: "Tasks/Obsidian.md", LineNum: 3, Title: "HTTP Task 2", ID: "httpid2222"},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tasks": tasksList})
		case r.Method == "PATCH" && r.URL.Path == "/api/tasks/Tasks/Obsidian.md":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == "POST" && r.URL.Path == "/api/tasks/list/Obsidian":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "created"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cli, err := NewClient(Config{
		ServerURL: server.URL,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	t.Run("FindTask by google_id", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "g1")
		assert.NoError(t, err)
		assert.Equal(t, "HTTP Task 1", task.Title)
	})

	t.Run("FindTask by id", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "httpid2222")
		assert.NoError(t, err)
		assert.Equal(t, "HTTP Task 2", task.Title)
	})

	t.Run("SetStatusTag", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "httpid1111")
		assert.NoError(t, err)
		err = cli.SetStatusTag(ctx, task, "InProgress", nil)
		assert.NoError(t, err)
	})

	t.Run("AddTask", func(t *testing.T) {
		err := cli.AddTask(ctx, "Obsidian", "New Task", false)
		assert.NoError(t, err)
	})
}

package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks/client"
	"github.com/stretchr/testify/assert"
)

type mockVaultMgr struct {
	path        string
	taskFolders []string
}

func (m *mockVaultMgr) DefaultName() (string, error)    { return "test", nil }
func (m *mockVaultMgr) SetDefaultName(string) error      { return nil }
func (m *mockVaultMgr) Path() (string, error)            { return m.path, nil }
func (m *mockVaultMgr) DefaultOpenType() (string, error) { return "", nil }
func (m *mockVaultMgr) TaskFolders() ([]string, error)   { return m.taskFolders, nil }
func (m *mockVaultMgr) ProjectsFolder() (string, error)  { return "Projects", nil }
func (m *mockVaultMgr) CalendarFolder() (string, error)  { return "Journal/Calendar", nil }

func setupTestVault(t *testing.T) (*mockVaultMgr, string) {
	dir := t.TempDir()
	taskDir := filepath.Join(dir, "Tasks")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `# Tasks
- [ ] First Task #ToDo [id::taskid0001] [google_id::g001] [due::2026-10-10]
- [ ] Second Task #InProgress [id::taskid0002]
`
	listPath := filepath.Join(taskDir, "Obsidian.md")
	if err := os.WriteFile(listPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	return &mockVaultMgr{path: dir, taskFolders: []string{"Tasks"}}, dir
}

func TestTasksCommandsDirect(t *testing.T) {
	vm, vaultPath := setupTestVault(t)
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))

	cli, err := client.NewClient(client.Config{
		ForceLocal:   true,
		VaultManager: vm,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	t.Run("Find by Google ID and Move", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "g001")
		assert.NoError(t, err)
		assert.Equal(t, "First Task", task.Title)

		err = cli.SetStatusTag(ctx, task, "Done", nil)
		assert.NoError(t, err)

		updated, err := cli.FindTask(ctx, "g001")
		assert.NoError(t, err)
		assert.Equal(t, tasks.StatusCompleted, updated.Status)
	})

	t.Run("Find by Task ID and Rename", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "taskid0002")
		assert.NoError(t, err)
		assert.Equal(t, "Second Task", task.Title)

		err = cli.RenameTask(ctx, task, "Renamed Second")
		assert.NoError(t, err)

		updated, err := cli.FindTask(ctx, "taskid0002")
		assert.NoError(t, err)
		assert.Equal(t, "Renamed Second", updated.Title)
	})

	t.Run("Edit Task", func(t *testing.T) {
		task, err := cli.FindTask(ctx, "taskid0002")
		assert.NoError(t, err)

		prio := "high"
		err = cli.EditTask(ctx, task, tasks.TaskEdit{Priority: &prio})
		assert.NoError(t, err)

		updated, err := cli.FindTask(ctx, "taskid0002")
		assert.NoError(t, err)
		assert.Equal(t, "high", updated.Priority)
	})

	t.Run("Add Subtask and Set Parent", func(t *testing.T) {
		parent, err := cli.FindTask(ctx, "g001")
		assert.NoError(t, err)

		err = cli.AddSubtask(ctx, parent, "A Subtask")
		assert.NoError(t, err)

		// Verify child exists in file
		fileBytes, _ := os.ReadFile(filepath.Join(vaultPath, "Tasks", "Obsidian.md"))
		assert.Contains(t, string(fileBytes), "A Subtask")
	})

	t.Run("Add Task to List", func(t *testing.T) {
		err := cli.AddTask(ctx, "Obsidian", "Brand New Task", false)
		assert.NoError(t, err)

		fileBytes, _ := os.ReadFile(filepath.Join(vaultPath, "Tasks", "Obsidian.md"))
		assert.Contains(t, string(fileBytes), "Brand New Task")
	})

	t.Run("List Kanban Cards", func(t *testing.T) {
		cards, err := cli.ListKanban(ctx, []string{"ToDo", "InProgress", "Done"})
		assert.NoError(t, err)
		assert.NotEmpty(t, cards)
	})
}

func TestTasksCommandsHTTP(t *testing.T) {
	var receivedAction string
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/tasks":
			tasksList := []tasks.Task{
				{FilePath: "Tasks/Obsidian.md", LineNum: 2, Title: "Remote Task 1", ID: "r001", GoogleID: "g_remote"},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tasks": tasksList})
		case r.Method == "PATCH" && r.URL.Path == "/api/tasks/Tasks/Obsidian.md":
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			receivedAction, _ = receivedBody["action"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cli, err := client.NewClient(client.Config{
		ServerURL: server.URL,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	task, err := cli.FindTask(ctx, "g_remote")
	assert.NoError(t, err)
	assert.Equal(t, "Remote Task 1", task.Title)

	err = cli.SetStatusTag(ctx, task, "Done", []string{"ToDo", "InProgress", "Done"})
	assert.NoError(t, err)
	assert.Equal(t, "set-status-tag", receivedAction)
	assert.Equal(t, "Done", receivedBody["kanban_status"])
}

func TestPerVaultServerConfig(t *testing.T) {
	serverWork := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tasksList := []tasks.Task{
			{FilePath: "Action Items.md", LineNum: 1, Title: "Work Task", ID: "w001"},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": tasksList})
	}))
	defer serverWork.Close()

	cli, err := client.NewClient(client.Config{
		ServerURL: serverWork.URL,
		VaultID:   "work",
	})
	assert.NoError(t, err)

	task, err := cli.FindTask(context.Background(), "w001")
	assert.NoError(t, err)
	assert.Equal(t, "Work Task", task.Title)
}

func TestFormatTaskIdentifier(t *testing.T) {
	t1 := tasks.Task{GoogleID: "g1", ID: "id1"}
	assert.Equal(t, "[g1]", formatTaskIdentifier(t1))

	t2 := tasks.Task{ID: "id2"}
	assert.Equal(t, "[id2]", formatTaskIdentifier(t2))

	t3 := tasks.Task{}
	assert.Equal(t, "", formatTaskIdentifier(t3))
}

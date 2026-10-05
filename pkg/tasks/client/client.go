package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
)

// TaskClient defines the abstract interface for task operations, implemented by both
// LocalClient (direct vault access) and HTTPClient (notesmd-cli serve HTTP API).
type TaskClient interface {
	FindTask(ctx context.Context, id string) (*tasks.Task, error)
	ListTasks(ctx context.Context, filter string) ([]tasks.Task, error)
	ListKanban(ctx context.Context, columns []string) ([]tasks.Task, error)
	SetStatusTag(ctx context.Context, task *tasks.Task, status string, columns []string) error
	RenameTask(ctx context.Context, task *tasks.Task, newTitle string) error
	EditTask(ctx context.Context, task *tasks.Task, edit tasks.TaskEdit) error
	SetParent(ctx context.Context, task *tasks.Task, parent *tasks.Task) error
	AddSubtask(ctx context.Context, parent *tasks.Task, title string) error
	AddTask(ctx context.Context, listName string, title string, completed bool) error
}

// Config specifies connection and vault configuration for NewClient.
type Config struct {
	ForceLocal     bool
	ServerURL      string
	ServerUsername string
	ServerPassword string
	VaultID        string
	VaultManager   obsidian.VaultManager
}

// NewClient returns a TaskClient according to the configured preferences and options:
// - If ForceLocal is true or ServerURL is empty, returns a LocalClient.
// - Otherwise returns an HTTPClient.
func NewClient(cfg Config) (TaskClient, error) {
	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = os.Getenv("NOTESMD_SERVER_URL")
	}
	serverUsername := cfg.ServerUsername
	if serverUsername == "" {
		serverUsername = os.Getenv("NOTESMD_SERVER_USERNAME")
	}
	serverPassword := cfg.ServerPassword
	if serverPassword == "" {
		serverPassword = os.Getenv("NOTESMD_SERVER_PASSWORD")
	}

	if cfg.ForceLocal || strings.TrimSpace(serverURL) == "" {
		if cfg.VaultManager == nil {
			vm, err := ResolveVaultManager(cfg.VaultID)
			if err != nil {
				return nil, err
			}
			cfg.VaultManager = vm
		}
		return &LocalClient{VaultManager: cfg.VaultManager}, nil
	}

	return &HTTPClient{
		BaseURL:    strings.TrimRight(serverURL, "/"),
		Username:   serverUsername,
		Password:   serverPassword,
		VaultID:    cfg.VaultID,
		HTTPClient: http.DefaultClient,
	}, nil
}

// ResolveVaultManager finds the appropriate VaultManager given an optional vaultID.
func ResolveVaultManager(vaultID string) (obsidian.VaultManager, error) {
	if vaultID != "" {
		if cv, ok := obsidian.FindConfiguredVault(vaultID); ok {
			return obsidian.NewConfiguredVault(cv), nil
		}
		return &obsidian.Vault{Name: vaultID}, nil
	}
	configured, err := obsidian.ConfiguredVaults()
	if err == nil && len(configured) > 0 {
		return obsidian.NewConfiguredVault(configured[0]), nil
	}
	return &obsidian.Vault{}, nil
}

// LocalClient directly operates on local Markdown files within an Obsidian vault.
type LocalClient struct {
	VaultManager obsidian.VaultManager
}

func (c *LocalClient) getVaultPathAndFolders() (string, []string, error) {
	if c.VaultManager == nil {
		return "", nil, fmt.Errorf("vault manager not configured")
	}
	vaultPath, err := c.VaultManager.Path()
	if err != nil {
		return "", nil, err
	}
	folders, err := c.VaultManager.TaskFolders()
	if err != nil {
		return "", nil, err
	}
	return vaultPath, folders, nil
}

func (c *LocalClient) FindTask(_ context.Context, id string) (*tasks.Task, error) {
	vaultPath, folders, err := c.getVaultPathAndFolders()
	if err != nil {
		return nil, err
	}
	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		return nil, err
	}

	// 1. Match Google ID first
	for _, t := range all {
		if t.GoogleID != "" && t.GoogleID == id {
			res := t
			return &res, nil
		}
	}

	// 2. Otherwise match task Crockford ID
	for _, t := range all {
		if t.ID != "" && t.ID == id {
			res := t
			return &res, nil
		}
	}

	return nil, fmt.Errorf("task with id %q not found", id)
}

func (c *LocalClient) ListTasks(_ context.Context, filter string) ([]tasks.Task, error) {
	vaultPath, folders, err := c.getVaultPathAndFolders()
	if err != nil {
		return nil, err
	}
	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(filter) {
	case "today":
		return tasks.FilterToday(all), nil
	case "tomorrow":
		return tasks.FilterTomorrow(all), nil
	case "overdue":
		return tasks.FilterOverdue(all), nil
	case "kanban":
		return tasks.FilterKanban(all), nil
	case "", "all":
		return all, nil
	default:
		return all, nil
	}
}

func (c *LocalClient) ListKanban(_ context.Context, columns []string) ([]tasks.Task, error) {
	vaultPath, folders, err := c.getVaultPathAndFolders()
	if err != nil {
		return nil, err
	}
	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		columns = tasks.KanbanTags
	}
	return tasks.KanbanCardsIn(all, columns), nil
}

func (c *LocalClient) SetStatusTag(_ context.Context, task *tasks.Task, status string, columns []string) error {
	vaultPath, _, err := c.getVaultPathAndFolders()
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		columns = tasks.KanbanTags
	}
	absPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(task.FilePath))

	if err := tasks.SetStatusTagIn(absPath, task.LineNum, status, columns); err != nil {
		return err
	}

	newStatus := tasks.StatusTodo
	if strings.EqualFold(status, "Done") {
		newStatus = tasks.StatusCompleted
	}
	return tasks.ToggleStatus(absPath, task.LineNum, newStatus)
}

func (c *LocalClient) RenameTask(_ context.Context, task *tasks.Task, newTitle string) error {
	vaultPath, _, err := c.getVaultPathAndFolders()
	if err != nil {
		return err
	}
	absPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(task.FilePath))
	return tasks.RenameTask(absPath, task.LineNum, newTitle)
}

func (c *LocalClient) EditTask(_ context.Context, task *tasks.Task, edit tasks.TaskEdit) error {
	vaultPath, _, err := c.getVaultPathAndFolders()
	if err != nil {
		return err
	}
	absPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(task.FilePath))
	return tasks.EditTask(absPath, task.LineNum, edit)
}

func (c *LocalClient) SetParent(_ context.Context, task *tasks.Task, parent *tasks.Task) error {
	vaultPath, _, err := c.getVaultPathAndFolders()
	if err != nil {
		return err
	}
	srcPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(task.FilePath))
	if parent == nil {
		_, err := tasks.SetParent(srcPath, task.LineNum, srcPath, 0)
		return err
	}
	dstPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(parent.FilePath))
	_, err = tasks.SetParent(srcPath, task.LineNum, dstPath, parent.LineNum)
	return err
}

func (c *LocalClient) AddSubtask(_ context.Context, parent *tasks.Task, title string) error {
	vaultPath, _, err := c.getVaultPathAndFolders()
	if err != nil {
		return err
	}
	absPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(parent.FilePath))
	return tasks.AppendSubtask(absPath, parent.LineNum, title)
}

func (c *LocalClient) AddTask(_ context.Context, listName string, title string, completed bool) error {
	vaultPath, folders, err := c.getVaultPathAndFolders()
	if err != nil {
		return err
	}
	absPath, err := tasks.FindListFile(vaultPath, folders, listName)
	if err != nil {
		return fmt.Errorf("list %q not found in task folders %v: %w", listName, folders, err)
	}
	status := tasks.StatusTodo
	if completed {
		status = tasks.StatusCompleted
	}
	return tasks.AppendTaskWithStatus(absPath, title, status)
}

// HTTPClient interacts with the notesmd-cli serve HTTP API.
type HTTPClient struct {
	BaseURL    string
	Username   string
	Password   string
	VaultID    string
	HTTPClient *http.Client
}

func (c *HTTPClient) urlWithVault(path string) string {
	u := c.BaseURL + path
	if c.VaultID != "" {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "vault=" + url.QueryEscape(c.VaultID)
	}
	return u
}

func (c *HTTPClient) doRequest(ctx context.Context, method, path string, bodyData any) ([]byte, error) {
	var bodyReader io.Reader
	if bodyData != nil {
		b, err := json.Marshal(bodyData)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	reqURL := c.urlWithVault(path)
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	if bodyData != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(c.Username + ":" + c.Password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Error != "" {
			return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

func (c *HTTPClient) FindTask(ctx context.Context, id string) (*tasks.Task, error) {
	all, err := c.ListTasks(ctx, "all")
	if err != nil {
		return nil, err
	}

	// 1. Google ID first
	for _, t := range all {
		if t.GoogleID != "" && t.GoogleID == id {
			res := t
			return &res, nil
		}
	}

	// 2. Otherwise task Crockford ID
	for _, t := range all {
		if t.ID != "" && t.ID == id {
			res := t
			return &res, nil
		}
	}

	return nil, fmt.Errorf("task with id %q not found", id)
}

func (c *HTTPClient) ListTasks(ctx context.Context, filter string) ([]tasks.Task, error) {
	path := "/api/tasks"
	switch strings.ToLower(filter) {
	case "today":
		path = "/api/tasks/today"
	case "tomorrow":
		path = "/api/tasks/tomorrow"
	case "overdue":
		path = "/api/tasks/overdue"
	case "kanban":
		path = "/api/tasks/kanban"
	}

	respBytes, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tasks []tasks.Task `json:"tasks"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	return resp.Tasks, nil
}

func (c *HTTPClient) ListKanban(ctx context.Context, columns []string) ([]tasks.Task, error) {
	path := "/api/tasks/kanban"
	if len(columns) > 0 {
		path += "?columns=" + url.QueryEscape(strings.Join(columns, ","))
	}
	respBytes, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tasks []tasks.Task `json:"tasks"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	return resp.Tasks, nil
}

func (c *HTTPClient) SetStatusTag(ctx context.Context, task *tasks.Task, status string, columns []string) error {
	path := "/api/tasks/" + strings.TrimPrefix(task.FilePath, "/")
	body := map[string]any{
		"action":        "set-status-tag",
		"line":          task.LineNum,
		"kanban_status": status,
	}
	if len(columns) > 0 {
		body["kanban_columns"] = columns
	}
	_, err := c.doRequest(ctx, "PATCH", path, body)
	return err
}

func (c *HTTPClient) RenameTask(ctx context.Context, task *tasks.Task, newTitle string) error {
	path := "/api/tasks/" + strings.TrimPrefix(task.FilePath, "/")
	body := map[string]any{
		"action": "rename",
		"line":   task.LineNum,
		"title":  newTitle,
	}
	_, err := c.doRequest(ctx, "PATCH", path, body)
	return err
}

func (c *HTTPClient) EditTask(ctx context.Context, task *tasks.Task, edit tasks.TaskEdit) error {
	path := "/api/tasks/" + strings.TrimPrefix(task.FilePath, "/")
	body := map[string]any{
		"action": "edit",
		"line":   task.LineNum,
	}
	if edit.Title != nil {
		body["title"] = *edit.Title
	}
	if edit.Due != nil {
		body["due"] = *edit.Due
	}
	if edit.Scheduled != nil {
		body["scheduled"] = *edit.Scheduled
	}
	if edit.Priority != nil {
		body["priority"] = *edit.Priority
	}
	if edit.Repeat != nil {
		body["repeat"] = *edit.Repeat
	}
	if edit.Tags != nil {
		body["tags"] = *edit.Tags
	}
	_, err := c.doRequest(ctx, "PATCH", path, body)
	return err
}

func (c *HTTPClient) SetParent(ctx context.Context, task *tasks.Task, parent *tasks.Task) error {
	path := "/api/tasks/" + strings.TrimPrefix(task.FilePath, "/")
	body := map[string]any{
		"action":      "set-parent",
		"line":        task.LineNum,
		"parent_line": 0,
	}
	if parent != nil {
		body["parent_line"] = parent.LineNum
		if parent.FilePath != task.FilePath {
			body["parent_path"] = parent.FilePath
		}
	}
	_, err := c.doRequest(ctx, "PATCH", path, body)
	return err
}

func (c *HTTPClient) AddSubtask(ctx context.Context, parent *tasks.Task, title string) error {
	path := "/api/tasks/" + strings.TrimPrefix(parent.FilePath, "/")
	body := map[string]any{
		"action": "add-subtask",
		"line":   parent.LineNum,
		"title":  title,
	}
	_, err := c.doRequest(ctx, "PATCH", path, body)
	return err
}

func (c *HTTPClient) AddTask(ctx context.Context, listName string, title string, completed bool) error {
	path := "/api/tasks/list/" + url.PathEscape(listName)
	status := "todo"
	if completed {
		status = "completed"
	}
	body := map[string]any{
		"title":  title,
		"status": status,
	}
	_, err := c.doRequest(ctx, "POST", path, body)
	return err
}

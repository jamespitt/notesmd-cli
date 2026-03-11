package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/Yakitrak/notesmd-cli/pkg/actions"
	"github.com/Yakitrak/notesmd-cli/pkg/frontmatter"
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
)

// Server holds the dependencies for the HTTP handlers.
type Server struct {
	vault obsidian.VaultManager
	note  obsidian.NoteManager
}

func New(vault obsidian.VaultManager, note obsidian.NoteManager) *Server {
	return &Server{vault: vault, note: note}
}

// Handler returns the HTTP mux with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/notes", s.listNotes)
	mux.HandleFunc("GET /api/notes/{path...}", s.getNote)
	mux.HandleFunc("POST /api/notes/{path...}", s.createNote)
	mux.HandleFunc("PATCH /api/notes/{path...}", s.patchNote)
	mux.HandleFunc("DELETE /api/notes/{path...}", s.deleteNote)
	mux.HandleFunc("GET /api/search", s.searchNotes)

	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("GET /api/tasks/today", s.listTasksToday)
	mux.HandleFunc("GET /api/tasks/overdue", s.listTasksOverdue)
	mux.HandleFunc("PATCH /api/tasks/{path...}", s.patchTask)

	return withCORS(mux)
}

// withCORS adds permissive CORS headers for local network use.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

func jsonCreated(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

// GET /api/notes
func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := actions.ListEntries(s.vault, actions.ListParams{})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if notes == nil {
		notes = []string{}
	}
	jsonOK(w, map[string]any{"notes": notes})
}

// GET /api/notes/{path...}
func (s *Server) getNote(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	_, err := s.vault.DefaultName()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	vaultPath, err := s.vault.Path()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	content, err := s.note.GetContents(vaultPath, path)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	fm, body, _ := frontmatter.Parse(content)

	jsonOK(w, map[string]any{
		"path":        path,
		"content":     content,
		"body":        body,
		"frontmatter": fm,
	})
}

// POST /api/notes/{path...}
// Body: { "content": "...", "overwrite": bool, "append": bool }
func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	var body struct {
		Content   string `json:"content"`
		Overwrite bool   `json:"overwrite"`
		Append    bool   `json:"append"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := actions.CreateNote(s.vault, &obsidian.Uri{}, actions.CreateParams{
		NoteName:        path,
		Content:         body.Content,
		ShouldOverwrite: body.Overwrite,
		ShouldAppend:    body.Append,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonCreated(w, map[string]string{"path": path})
}

// PATCH /api/notes/{path...}
// For move:         { "action": "move", "newPath": "..." }
// For frontmatter:  { "action": "set",    "key": "...", "value": "..." }
//
//	{ "action": "delete", "key": "..." }
func (s *Server) patchNote(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	var body struct {
		Action  string `json:"action"`
		NewPath string `json:"newPath"`
		Key     string `json:"key"`
		Value   string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch body.Action {
	case "move":
		if body.NewPath == "" {
			jsonError(w, http.StatusBadRequest, "newPath is required")
			return
		}
		err := actions.MoveNote(s.vault, s.note, &obsidian.Uri{}, actions.MoveParams{
			CurrentNoteName: path,
			NewNoteName:     body.NewPath,
		})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]string{"path": body.NewPath})

	case "set":
		if body.Key == "" {
			jsonError(w, http.StatusBadRequest, "key is required")
			return
		}
		if err := s.updateFrontmatter(w, path, func(content string) (string, error) {
			return frontmatter.SetKey(content, body.Key, body.Value)
		}); err != nil {
			return
		}

	case "delete":
		if body.Key == "" {
			jsonError(w, http.StatusBadRequest, "key is required")
			return
		}
		if err := s.updateFrontmatter(w, path, func(content string) (string, error) {
			return frontmatter.DeleteKey(content, body.Key)
		}); err != nil {
			return
		}

	default:
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown action %q", body.Action))
	}
}

func (s *Server) updateFrontmatter(w http.ResponseWriter, path string, transform func(string) (string, error)) error {
	_, err := s.vault.DefaultName()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return err
	}

	vaultPath, err := s.vault.Path()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return err
	}

	content, err := s.note.GetContents(vaultPath, path)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return err
	}

	updated, err := transform(content)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return err
	}

	if err := s.note.SetContents(vaultPath, path, updated); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return err
	}

	fm, _, _ := frontmatter.Parse(updated)
	jsonOK(w, map[string]any{"path": path, "frontmatter": fm})
	return nil
}

// DELETE /api/notes/{path...}
func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	err := actions.DeleteNote(s.vault, s.note, actions.DeleteParams{NotePath: path})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]string{"deleted": path})
}

// GET /api/search?q=term
func (s *Server) searchNotes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	_, err := s.vault.DefaultName()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	vaultPath, err := s.vault.Path()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	matches, err := s.note.SearchNotesWithSnippets(vaultPath, q)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type result struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Snippet string `json:"snippet"`
	}
	results := make([]result, len(matches))
	for i, m := range matches {
		results[i] = result{Path: m.FilePath, Line: m.LineNumber, Snippet: m.MatchLine}
	}

	jsonOK(w, map[string]any{"results": results})
}

// getVaultPath is a helper to resolve and return the vault path.
func (s *Server) getVaultPath(w http.ResponseWriter) (string, error) {
	_, err := s.vault.DefaultName()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return "", err
	}
	vaultPath, err := s.vault.Path()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return "", err
	}
	return vaultPath, nil
}

// getTaskFolders returns the configured task folders, or nil for the whole vault.
func (s *Server) getTaskFolders(w http.ResponseWriter) ([]string, error) {
	folders, err := s.vault.TaskFolders()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, err
	}
	return folders, nil
}

// GET /api/tasks
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	folders, err := s.getTaskFolders(w)
	if err != nil {
		return
	}

	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if all == nil {
		all = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": all})
}

// GET /api/tasks/today
func (s *Server) listTasksToday(w http.ResponseWriter, r *http.Request) {
	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	folders, err := s.getTaskFolders(w)
	if err != nil {
		return
	}

	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := tasks.FilterToday(all)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// GET /api/tasks/overdue
func (s *Server) listTasksOverdue(w http.ResponseWriter, r *http.Request) {
	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	folders, err := s.getTaskFolders(w)
	if err != nil {
		return
	}

	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := tasks.FilterOverdue(all)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// PATCH /api/tasks/{path...}
// Body: { "line": 42, "status": "completed" | "todo" }
func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")

	var body struct {
		Line   int    `json:"line"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Line < 1 {
		jsonError(w, http.StatusBadRequest, "line must be >= 1")
		return
	}

	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}

	absPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(notePath))

	var newStatus tasks.Status
	switch body.Status {
	case "completed":
		newStatus = tasks.StatusCompleted
	case "todo":
		newStatus = tasks.StatusTodo
	default:
		jsonError(w, http.StatusBadRequest, "status must be 'completed' or 'todo'")
		return
	}

	if err := tasks.ToggleStatus(absPath, body.Line, newStatus); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]any{"path": notePath, "line": body.Line, "status": body.Status})
}

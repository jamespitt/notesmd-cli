package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/actions"
	"github.com/Yakitrak/notesmd-cli/pkg/frontmatter"
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/projects"
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
	mux.HandleFunc("GET /api/tasks/tomorrow", s.listTasksTomorrow)
	mux.HandleFunc("GET /api/tasks/overdue", s.listTasksOverdue)
	mux.HandleFunc("GET /api/tasks/timeline", s.listTasksTimeline)
	mux.HandleFunc("GET /api/tasks/now", s.getTasksNow)
	mux.HandleFunc("GET /api/tasks/lists", s.listTaskLists)
	mux.HandleFunc("GET /api/tasks/list/{name}", s.listTasksByList)
	mux.HandleFunc("POST /api/tasks/list/{name}", s.addTask)
	mux.HandleFunc("PATCH /api/tasks/{path...}", s.patchTask)
	mux.HandleFunc("DELETE /api/tasks/{path...}", s.deleteTask)

	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("GET /api/projects/{name}", s.getProject)
	mux.HandleFunc("POST /api/projects/{name}/tasks", s.addProjectTask)

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

// parseTasks is a shared helper that parses tasks from the configured vault/folders.
func (s *Server) parseTasks(w http.ResponseWriter) ([]tasks.Task, string, []string, error) {
	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return nil, "", nil, err
	}
	folders, err := s.getTaskFolders(w)
	if err != nil {
		return nil, "", nil, err
	}
	all, err := tasks.ParseFolders(vaultPath, folders)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, "", nil, err
	}
	return all, vaultPath, folders, nil
}

// GET /api/tasks
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	if all == nil {
		all = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": all})
}

// GET /api/tasks/today
func (s *Server) listTasksToday(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	result := tasks.FilterToday(all)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// GET /api/tasks/tomorrow
func (s *Server) listTasksTomorrow(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	result := tasks.FilterTomorrow(all)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// GET /api/tasks/overdue
func (s *Server) listTasksOverdue(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	result := tasks.FilterOverdue(all)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// GET /api/tasks/timeline
func (s *Server) listTasksTimeline(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	// FilterTimeline only returns today's tasks with start+end time
	result := tasks.FilterTimeline(all)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// nowContext holds the contextual task set for the "now" view.
type nowContext struct {
	Last       *tasks.Task `json:"last,omitempty"`
	Current    *tasks.Task `json:"current,omitempty"`
	Next       *tasks.Task `json:"next,omitempty"`
	SecondNext *tasks.Task `json:"second_next,omitempty"`
}

// timeToMinutes parses "HH:MM" and returns total minutes, or -1 on error.
func timeToMinutes(t string) int {
	var h, m int
	if _, err := fmt.Sscanf(t, "%d:%d", &h, &m); err != nil {
		return -1
	}
	return h*60 + m
}

// GET /api/tasks/now
func (s *Server) getTasksNow(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}

	todayTasks := tasks.FilterToday(all)

	// Build list of timed tasks with parsed start/end minutes
	type timedTask struct {
		task      tasks.Task
		startMins int
		endMins   int
	}

	var timed []timedTask
	titleTimeRe := regexp.MustCompile(`^\s*(\d{1,2}:\d{2})(?:-(\d{1,2}:\d{2}))?`)
	scheduledTimeRe := regexp.MustCompile(`(\d{1,2}:\d{2})`)

	for _, t := range todayTasks {
		startMins := -1
		endMins := -1

		// 1. From title prefix
		if m := titleTimeRe.FindStringSubmatch(t.Title); m != nil {
			startMins = timeToMinutes(m[1])
			if m[2] != "" {
				endMins = timeToMinutes(m[2])
			} else {
				endMins = startMins + 60
			}
		}

		// 2. From scheduled field
		if startMins == -1 && t.Scheduled != "" && len(t.Scheduled) > 10 {
			if m := scheduledTimeRe.FindStringSubmatch(t.Scheduled[10:]); m != nil {
				startMins = timeToMinutes(m[1])
			}
		}

		if startMins == -1 {
			continue
		}
		timed = append(timed, timedTask{task: t, startMins: startMins, endMins: endMins})
	}

	// Sort by start time
	sort.Slice(timed, func(i, j int) bool {
		return timed[i].startMins < timed[j].startMins
	})

	now := time.Now()
	nowMins := now.Hour()*60 + now.Minute()

	var ctx nowContext
	currentIdx := -1

	// Find current task (one whose window contains now)
	for i, tt := range timed {
		if tt.endMins > 0 && tt.startMins <= nowMins && nowMins < tt.endMins {
			task := tt.task
			ctx.Current = &task
			currentIdx = i
			break
		}
	}

	if currentIdx >= 0 {
		if currentIdx > 0 {
			t := timed[currentIdx-1].task
			ctx.Last = &t
		}
		if currentIdx+1 < len(timed) {
			t := timed[currentIdx+1].task
			ctx.Next = &t
		}
		if currentIdx+2 < len(timed) {
			t := timed[currentIdx+2].task
			ctx.SecondNext = &t
		}
	} else {
		// No current task — find the next upcoming one
		nextIdx := -1
		for i, tt := range timed {
			if tt.startMins > nowMins {
				nextIdx = i
				break
			}
		}
		if nextIdx >= 0 {
			t := timed[nextIdx].task
			ctx.Next = &t
			if nextIdx > 0 {
				t2 := timed[nextIdx-1].task
				ctx.Last = &t2
			}
			if nextIdx+1 < len(timed) {
				t2 := timed[nextIdx+1].task
				ctx.SecondNext = &t2
			}
		} else if len(timed) > 0 {
			// All tasks are past
			t := timed[len(timed)-1].task
			ctx.Last = &t
		}
	}

	jsonOK(w, ctx)
}

// GET /api/tasks/lists
func (s *Server) listTaskLists(w http.ResponseWriter, r *http.Request) {
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	lists := tasks.GetLists(all)
	if lists == nil {
		lists = []string{}
	}
	jsonOK(w, map[string]any{"lists": lists})
}

// GET /api/tasks/list/{name}
func (s *Server) listTasksByList(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	all, _, _, err := s.parseTasks(w)
	if err != nil {
		return
	}
	result := tasks.FilterByList(all, name)
	if result == nil {
		result = []tasks.Task{}
	}
	jsonOK(w, map[string]any{"tasks": result})
}

// POST /api/tasks/list/{name}
// Body: { "title": "..." }
func (s *Server) addTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}

	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	folders, err := s.getTaskFolders(w)
	if err != nil {
		return
	}

	absPath, err := tasks.FindListFile(vaultPath, folders, name)
	if err != nil {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("list %q not found", name))
		return
	}

	if err := tasks.AppendTask(absPath, body.Title); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonCreated(w, map[string]string{"list": name, "title": body.Title})
}

// PATCH /api/tasks/{path...}
// Existing: { "line": 42, "status": "completed" | "todo" }
// New schedule: { "action": "schedule", "line": 42, "scheduled": "2026-03-11T14:00" }
// New move:     { "action": "move", "line": 42, "new_list": "Work" }
func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")

	var body struct {
		Action    string `json:"action"`
		Line      int    `json:"line"`
		Status    string `json:"status"`
		Scheduled string `json:"scheduled"`
		NewList   string `json:"new_list"`
		Title     string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}

	absPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(notePath))

	switch body.Action {
	case "rename":
		if body.Line < 1 {
			jsonError(w, http.StatusBadRequest, "line must be >= 1")
			return
		}
		if body.Title == "" {
			jsonError(w, http.StatusBadRequest, "title is required")
			return
		}
		if err := tasks.RenameTask(absPath, body.Line, body.Title); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]any{"path": notePath, "line": body.Line, "title": body.Title})

	case "schedule":
		if body.Line < 1 {
			jsonError(w, http.StatusBadRequest, "line must be >= 1")
			return
		}
		if body.Scheduled == "" {
			jsonError(w, http.StatusBadRequest, "scheduled is required")
			return
		}
		if err := tasks.SetScheduled(absPath, body.Line, body.Scheduled); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]any{"path": notePath, "line": body.Line, "scheduled": body.Scheduled})

	case "move":
		if body.Line < 1 {
			jsonError(w, http.StatusBadRequest, "line must be >= 1")
			return
		}
		if body.NewList == "" {
			jsonError(w, http.StatusBadRequest, "new_list is required")
			return
		}
		folders, err := s.getTaskFolders(w)
		if err != nil {
			return
		}
		dstPath, err := tasks.FindListFile(vaultPath, folders, body.NewList)
		if err != nil {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("list %q not found", body.NewList))
			return
		}
		if err := tasks.MoveTask(absPath, body.Line, dstPath); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]any{"path": notePath, "line": body.Line, "new_list": body.NewList})

	default:
		// Original toggle-status behaviour
		if body.Line < 1 {
			jsonError(w, http.StatusBadRequest, "line must be >= 1")
			return
		}
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
}

// DELETE /api/tasks/{path...}
// Body: { "line": 42 }
func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")

	var body struct {
		Line int `json:"line"`
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

	if err := tasks.DeleteTask(absPath, body.Line); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]any{"path": notePath, "line": body.Line})
}

// getProjectsFolder returns the configured projects folder (e.g. "Projects").
func (s *Server) getProjectsFolder(w http.ResponseWriter) (string, error) {
	folder, err := s.vault.ProjectsFolder()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return "", err
	}
	return folder, nil
}

// GET /api/projects
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	projectsFolder, err := s.getProjectsFolder(w)
	if err != nil {
		return
	}

	list, err := projects.ParseProjects(vaultPath, projectsFolder)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []projects.Project{}
	}
	jsonOK(w, map[string]any{"projects": list})
}

// GET /api/projects/{name}
func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	projectsFolder, err := s.getProjectsFolder(w)
	if err != nil {
		return
	}
	taskFolders, err := s.getTaskFolders(w)
	if err != nil {
		return
	}

	// Parse project metadata
	list, err := projects.ParseProjects(vaultPath, projectsFolder)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var proj *projects.Project
	for i := range list {
		if list[i].Name == name {
			proj = &list[i]
			break
		}
	}
	if proj == nil {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("project %q not found", name))
		return
	}

	// Gather tasks
	projectTasks, err := projects.GetProjectTasks(vaultPath, projectsFolder, name, taskFolders)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projectTasks == nil {
		projectTasks = []tasks.Task{}
	}

	jsonOK(w, map[string]any{
		"project": proj,
		"tasks":   projectTasks,
	})
}

// POST /api/projects/{name}/tasks
// Body: { "title": "..." }
// Appends a new task to the project's main .md file.
func (s *Server) addProjectTask(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Title == "" {
		jsonError(w, http.StatusBadRequest, "title is required")
		return
	}

	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}
	projectsFolder, err := s.getProjectsFolder(w)
	if err != nil {
		return
	}

	// The main project file has the same name as the directory
	mainFile := filepath.Join(vaultPath, projectsFolder, name, name+".md")
	if _, err := os.Stat(mainFile); err != nil {
		jsonError(w, http.StatusNotFound, fmt.Sprintf("project file not found for %q", name))
		return
	}

	if err := tasks.AppendTask(mainFile, body.Title); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonCreated(w, map[string]string{"project": name, "title": body.Title})
}

// sortTaskLists sorts a list of task names, keeping well-known names first.
// This mirrors the ordering logic in the TUI.
func sortTaskLists(lists []string) []string {
	sort.Strings(lists)
	return lists
}

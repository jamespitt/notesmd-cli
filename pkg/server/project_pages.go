package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/projects"
)

// GET /api/projects/{name}/pages
// Lists every .md file inside the project's directory (recursively).
func (s *Server) listProjectPages(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	vaultPath, err := s.getVaultPath(w, r)
	if err != nil {
		return
	}
	projectsFolder, err := s.getProjectsFolder(w, r)
	if err != nil {
		return
	}

	pages, err := projects.ListPages(vaultPath, projectsFolder, name)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	jsonOK(w, map[string]any{"pages": pages})
}

// GET /api/projects/{name}/diary
// Returns the raw markdown of the project's Diary.md (empty if not created yet).
func (s *Server) getProjectDiary(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	vaultPath, err := s.getVaultPath(w, r)
	if err != nil {
		return
	}
	projectsFolder, err := s.getProjectsFolder(w, r)
	if err != nil {
		return
	}

	content, exists, err := projects.ReadDiary(vaultPath, projectsFolder, name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{
		"path":    projects.DiaryPath(projectsFolder, name),
		"content": content,
		"exists":  exists,
	})
}

// POST /api/projects/{name}/diary
// Body: { "text": "..." }
// Appends a dated entry to the project's Diary.md (see projects.AppendDiaryEntry).
func (s *Server) postProjectDiary(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		jsonError(w, http.StatusBadRequest, "text is required")
		return
	}

	vaultPath, err := s.getVaultPath(w, r)
	if err != nil {
		return
	}
	projectsFolder, err := s.getProjectsFolder(w, r)
	if err != nil {
		return
	}

	title := name
	if list, err := projects.ParseProjects(vaultPath, projectsFolder); err == nil {
		for i := range list {
			if list[i].Name == name {
				title = list[i].Title
				break
			}
		}
	}

	content, err := projects.AppendDiaryEntry(vaultPath, projectsFolder, name, title, body.Text, time.Now())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{
		"path":    projects.DiaryPath(projectsFolder, name),
		"content": content,
	})
}

package server

import (
	"encoding/json"
	"net/http"

	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
)

// GET /api/tasks/hidden
func (s *Server) getHiddenEvents(w http.ResponseWriter, r *http.Request) {
	events, err := tasks.LoadHiddenEvents()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []tasks.HiddenEvent{}
	}
	jsonOK(w, map[string]any{"events": events})
}

// POST /api/tasks/hidden
// Body: { "event_id": "...", "title": "..." }
func (s *Server) hideEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EventID string `json:"event_id"`
		Title   string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.EventID == "" {
		jsonError(w, http.StatusBadRequest, "event_id is required")
		return
	}
	if err := tasks.HideEvent(body.EventID, body.Title); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	events, err := tasks.LoadHiddenEvents()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"events": events})
}

// DELETE /api/tasks/hidden/{event_id}
func (s *Server) unhideEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("event_id")
	if eventID == "" {
		jsonError(w, http.StatusBadRequest, "event_id is required")
		return
	}
	if err := tasks.UnhideEvent(eventID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	events, err := tasks.LoadHiddenEvents()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"events": events})
}

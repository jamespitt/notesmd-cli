package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/config"
)

// HiddenEvent records a calendar event that has been hidden from all task views.
type HiddenEvent struct {
	EventID  string `json:"event_id"`
	Title    string `json:"title"`
	HiddenAt string `json:"hidden_at"`
}

func hiddenEventsPath() (string, error) {
	dir, _, err := config.CliPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hidden_events.json"), nil
}

// LoadHiddenEvents reads the persisted hidden events list.
func LoadHiddenEvents() ([]HiddenEvent, error) {
	path, err := hiddenEventsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []HiddenEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	var events []HiddenEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func saveHiddenEvents(events []HiddenEvent) error {
	path, err := hiddenEventsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// HideEvent adds an event to the hidden list (idempotent).
func HideEvent(eventID, title string) error {
	events, err := LoadHiddenEvents()
	if err != nil {
		return err
	}
	for _, e := range events {
		if e.EventID == eventID {
			return nil
		}
	}
	events = append(events, HiddenEvent{
		EventID:  eventID,
		Title:    title,
		HiddenAt: time.Now().Format(time.RFC3339),
	})
	return saveHiddenEvents(events)
}

// UnhideEvent removes an event from the hidden list.
func UnhideEvent(eventID string) error {
	events, err := LoadHiddenEvents()
	if err != nil {
		return err
	}
	filtered := events[:0]
	for _, e := range events {
		if e.EventID != eventID {
			filtered = append(filtered, e)
		}
	}
	return saveHiddenEvents(filtered)
}

// FilterHiddenEvents removes hidden calendar events from the list.
// Fails open: if the hidden list can't be read, the original list is returned unchanged.
func FilterHiddenEvents(taskList []Task) []Task {
	hidden, err := LoadHiddenEvents()
	if err != nil || len(hidden) == 0 {
		return taskList
	}
	hiddenSet := make(map[string]struct{}, len(hidden))
	for _, e := range hidden {
		hiddenSet[e.EventID] = struct{}{}
	}
	result := make([]Task, 0, len(taskList))
	for _, t := range taskList {
		if t.EventID != "" {
			if _, ok := hiddenSet[t.EventID]; ok {
				continue
			}
		}
		result = append(result, t)
	}
	return result
}

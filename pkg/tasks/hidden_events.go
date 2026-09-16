package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/config"
)

// HiddenEvent records a calendar event that has been hidden from all task views.
type HiddenEvent struct {
	EventID  string `json:"event_id"`
	Title    string `json:"title"`
	HiddenAt string `json:"hidden_at"`
}

// hiddenEventsPath returns the state file for one vault. vaultKey namespaces
// it: empty (the default vault) keeps the original filename, any other key
// gets its own file - event ids only mean anything within the vault they came
// from, so hiding an event in one vault must not hide anything in another.
// The key is sanitised into a filename since it comes from config.
func hiddenEventsPath(vaultKey string) (string, error) {
	dir, _, err := config.CliPath()
	if err != nil {
		return "", err
	}
	name := "hidden_events.json"
	if vaultKey != "" {
		name = "hidden_events-" + sanitizeKey(vaultKey) + ".json"
	}
	return filepath.Join(dir, name), nil
}

// sanitizeKey reduces a vault id to characters that are safe in a filename.
func sanitizeKey(key string) string {
	var b strings.Builder
	for _, c := range key {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			b.WriteRune(c)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// LoadHiddenEvents reads the persisted hidden events list for a vault.
func LoadHiddenEvents(vaultKey string) ([]HiddenEvent, error) {
	path, err := hiddenEventsPath(vaultKey)
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

func saveHiddenEvents(vaultKey string, events []HiddenEvent) error {
	path, err := hiddenEventsPath(vaultKey)
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
func HideEvent(vaultKey, eventID, title string) error {
	events, err := LoadHiddenEvents(vaultKey)
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
	return saveHiddenEvents(vaultKey, events)
}

// UnhideEvent removes an event from the hidden list.
func UnhideEvent(vaultKey, eventID string) error {
	events, err := LoadHiddenEvents(vaultKey)
	if err != nil {
		return err
	}
	filtered := events[:0]
	for _, e := range events {
		if e.EventID != eventID {
			filtered = append(filtered, e)
		}
	}
	return saveHiddenEvents(vaultKey, filtered)
}

// FilterHiddenEvents removes hidden calendar events from the list.
// Fails open: if the hidden list can't be read, the original list is returned unchanged.
func FilterHiddenEvents(vaultKey string, taskList []Task) []Task {
	hidden, err := LoadHiddenEvents(vaultKey)
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

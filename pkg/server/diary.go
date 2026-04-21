package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

var diaryHeadingRe = regexp.MustCompile(`(?i)^####\s+Diary Notes\s*$`)

// todayJournalPath returns the absolute path to today's daily note.
func (s *Server) todayJournalPath(vaultPath string) string {
	config := obsidian.ReadDailyNotesConfig(vaultPath)
	format := config.Format
	if format == "" {
		format = "YYYY-MM-DD"
	}
	name := time.Now().Format(obsidian.MomentToGoFormat(format)) + ".md"
	if config.Folder != "" {
		return filepath.Join(vaultPath, config.Folder, name)
	}
	return filepath.Join(vaultPath, name)
}

// parseDiaryEntries reads the file at path and returns the text of every
// "- ..." bullet line found under the "#### Diary Notes" heading.
func parseDiaryEntries(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var entries []string
	inSection := false

	for _, line := range lines {
		if diaryHeadingRe.MatchString(line) {
			inSection = true
			continue
		}
		if inSection {
			// Stop at the next heading of any level
			if regexp.MustCompile(`^#{1,6}\s`).MatchString(line) {
				break
			}
			trimmed := strings.TrimPrefix(line, "- ")
			if trimmed != line && strings.TrimSpace(trimmed) != "" {
				entries = append(entries, trimmed)
			}
		}
	}

	if entries == nil {
		entries = []string{}
	}
	return entries, nil
}

// GET /api/journal/today/diary
func (s *Server) getTodayDiary(w http.ResponseWriter, r *http.Request) {
	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}

	entries, err := parseDiaryEntries(s.todayJournalPath(vaultPath))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]any{"entries": entries})
}

// POST /api/journal/today/diary
// Body: { "text": "new diary entry" }
func (s *Server) postTodayDiary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		jsonError(w, http.StatusBadRequest, "text is required")
		return
	}

	vaultPath, err := s.getVaultPath(w)
	if err != nil {
		return
	}

	journalPath := s.todayJournalPath(vaultPath)

	var lines []string
	if data, err := os.ReadFile(journalPath); err == nil {
		lines = strings.Split(string(data), "\n")
	} else {
		// Create a minimal daily note
		today := time.Now().Format("2006-01-02")
		lines = []string{
			"---",
			"date-created: " + today,
			"date-modified: " + today,
			"title: " + today,
			"tags: DailyNote",
			"---",
			"## " + today,
			"",
			"#### Diary Notes",
		}
	}

	// Find the Diary Notes heading
	diaryIdx := -1
	for i, line := range lines {
		if diaryHeadingRe.MatchString(line) {
			diaryIdx = i
			break
		}
	}

	if diaryIdx == -1 {
		// Append section at end
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "#### Diary Notes")
		diaryIdx = len(lines) - 1
	}

	// Insert the new bullet after the heading and any existing bullets
	insertAt := diaryIdx + 1
	for insertAt < len(lines) && strings.HasPrefix(lines[insertAt], "- ") {
		insertAt++
	}

	newLine := "- " + text
	lines = append(lines[:insertAt], append([]string{newLine}, lines[insertAt:]...)...)

	if err := os.MkdirAll(filepath.Dir(journalPath), 0755); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(journalPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries, err := parseDiaryEntries(journalPath)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]any{"entries": entries})
}

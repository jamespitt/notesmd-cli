// Package tasks provides parsing and querying of Obsidian markdown checkbox tasks.
package tasks

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Status represents a task's completion state.
type Status string

const (
	StatusTodo      Status = "todo"
	StatusCompleted Status = "completed"
)

// Task represents a parsed task from an Obsidian markdown file.
type Task struct {
	FilePath  string   `json:"file_path"`
	LineNum   int      `json:"line_num"`
	Title     string   `json:"title"`
	Status    Status   `json:"status"`
	Due       string   `json:"due,omitempty"`
	Scheduled string   `json:"scheduled,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	Repeat    string   `json:"repeat,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Level     int      `json:"level"`
	ListName  string   `json:"list_name,omitempty"`
	StartTime string   `json:"start_time,omitempty"`
	EndTime   string   `json:"end_time,omitempty"`
}

var (
	taskLineRe  = regexp.MustCompile(`^(\s*)-\s*\[([xX ])\]\s+(.*)`)
	dataviewRe  = regexp.MustCompile(`\[([^\]]+?)::([^\]]*)\]`)
	tagRe       = regexp.MustCompile(`#([\w/]+)`)
	legacyDueRe = regexp.MustCompile(`📅\s*(\d{4}-\d{2}-\d{2})`)
	// Matches "09:30-10:00" or "09:30" at the start of a title (with optional leading space)
	titleTimeRe = regexp.MustCompile(`^\s*(\d{1,2}:\d{2})(?:-(\d{1,2}:\d{2}))?`)
)

// ParseVault walks the vault and returns all tasks found in .md files.
func ParseVault(vaultPath string) ([]Task, error) {
	return ParseFolders(vaultPath, nil)
}

// ParseFolders walks the given folders within the vault and returns all tasks.
// If folders is empty, the entire vault is walked.
func ParseFolders(vaultPath string, folders []string) ([]Task, error) {
	roots := []string{vaultPath}
	if len(folders) > 0 {
		roots = make([]string, len(folders))
		for i, f := range folders {
			roots[i] = filepath.Join(vaultPath, f)
		}
	}

	var tasks []Task
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			relPath, err := filepath.Rel(vaultPath, path)
			if err != nil {
				return nil //nolint:nilerr
			}

			fileTasks, err := parseFile(path, relPath)
			if err != nil {
				return nil //nolint:nilerr
			}
			tasks = append(tasks, fileTasks...)
			return nil
		})
		if err != nil {
			return tasks, err
		}
	}

	return tasks, nil
}

func parseFile(absPath, relPath string) ([]Task, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Derive list name from file stem (e.g. "Tasks/Work.md" -> "Work")
	listName := strings.TrimSuffix(filepath.Base(relPath), ".md")

	var tasks []Task
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		task := parseLine(line, relPath, lineNum)
		if task != nil {
			task.ListName = listName
			tasks = append(tasks, *task)
		}
	}

	return tasks, scanner.Err()
}

func parseLine(line, filePath string, lineNum int) *Task {
	m := taskLineRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	indent := m[1]
	statusChar := strings.ToLower(m[2])
	raw := m[3]

	// Determine nesting level (4 spaces = 1 level)
	level := len(indent) / 4

	status := StatusTodo
	if statusChar == "x" {
		status = StatusCompleted
	}

	// Extract dataview fields
	fields := make(map[string]string)
	for _, match := range dataviewRe.FindAllStringSubmatch(raw, -1) {
		fields[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
	}

	// Determine due date
	due := fields["due"]
	if due == "" {
		if m2 := legacyDueRe.FindStringSubmatch(raw); m2 != nil {
			due = m2[1]
		}
	}

	// Extract tags
	var tags []string
	for _, tm := range tagRe.FindAllStringSubmatch(raw, -1) {
		tags = append(tags, tm[1])
	}

	// Clean title: remove dataview fields, tags, and legacy due emoji
	title := dataviewRe.ReplaceAllString(raw, "")
	title = tagRe.ReplaceAllString(title, "")
	title = legacyDueRe.ReplaceAllString(title, "")
	title = strings.TrimSpace(title)

	// Parse start/end time from title prefix (e.g. "09:30-10:00 Standup")
	var startTime, endTime string
	if tm := titleTimeRe.FindStringSubmatch(title); tm != nil {
		startTime = tm[1]
		endTime = tm[2] // may be empty if no range given
	}

	return &Task{
		FilePath:  filePath,
		LineNum:   lineNum,
		Title:     title,
		Status:    status,
		Due:       due,
		Scheduled: fields["scheduled"],
		Priority:  fields["priority"],
		Repeat:    fields["repeat"],
		Tags:      tags,
		Level:     level,
		StartTime: startTime,
		EndTime:   endTime,
	}
}

// today returns today's date in YYYY-MM-DD format.
func today() string {
	return time.Now().Format("2006-01-02")
}

// tomorrow returns tomorrow's date in YYYY-MM-DD format.
func tomorrow() string {
	return time.Now().AddDate(0, 0, 1).Format("2006-01-02")
}

// FilterToday returns tasks due or scheduled today or earlier that are not completed.
func FilterToday(tasks []Task) []Task {
	td := today()
	var result []Task
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			continue
		}
		if (t.Due != "" && t.Due[:10] <= td) || (t.Scheduled != "" && t.Scheduled[:10] <= td) {
			result = append(result, t)
		}
	}
	return result
}

// FilterOverdue returns incomplete tasks with a due date strictly before today.
func FilterOverdue(tasks []Task) []Task {
	td := today()
	var result []Task
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			continue
		}
		if t.Due != "" && t.Due[:10] < td {
			result = append(result, t)
		}
	}
	return result
}

// FilterTomorrow returns incomplete tasks due or scheduled on tomorrow's date.
func FilterTomorrow(tasks []Task) []Task {
	tm := tomorrow()
	var result []Task
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			continue
		}
		dueMatch := t.Due != "" && t.Due[:10] == tm
		schedMatch := t.Scheduled != "" && t.Scheduled[:10] == tm
		if dueMatch || schedMatch {
			result = append(result, t)
		}
	}
	return result
}

// FilterTimeline returns today's incomplete tasks that have both a start and end time
// parsed from the title, sorted chronologically by start time.
func FilterTimeline(tasks []Task) []Task {
	today := today()
	var result []Task
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			continue
		}
		if t.StartTime == "" || t.EndTime == "" {
			continue
		}
		// Only include tasks that are for today (scheduled or due today, or have no date filter)
		isToday := (t.Due != "" && t.Due[:10] == today) ||
			(t.Scheduled != "" && t.Scheduled[:10] == today)
		if isToday {
			result = append(result, t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime < result[j].StartTime
	})
	return result
}

// FilterByList returns tasks whose ListName matches the given name.
func FilterByList(tasks []Task, listName string) []Task {
	var result []Task
	for _, t := range tasks {
		if t.ListName == listName {
			result = append(result, t)
		}
	}
	return result
}

// GetLists returns a sorted slice of unique list names from the given tasks.
func GetLists(tasks []Task) []string {
	seen := make(map[string]struct{})
	for _, t := range tasks {
		if t.ListName != "" {
			seen[t.ListName] = struct{}{}
		}
	}
	lists := make([]string, 0, len(seen))
	for name := range seen {
		lists = append(lists, name)
	}
	sort.Strings(lists)
	return lists
}

// ToggleStatus toggles a task line between complete and incomplete in a file.
// It rewrites the line at lineNum (1-indexed) in the file at absPath.
func ToggleStatus(absPath string, lineNum int, newStatus Status) error {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return nil
	}

	idx := lineNum - 1
	line := lines[idx]
	m := taskLineRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	newChar := " "
	if newStatus == StatusCompleted {
		newChar = "x"
	}

	lines[idx] = m[1] + "- [" + newChar + "] " + m[3]
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
}

// DeleteTask removes the task line at lineNum (1-indexed) from the file at absPath.
func DeleteTask(absPath string, lineNum int) error {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return nil
	}

	lines = append(lines[:lineNum-1], lines[lineNum:]...)
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
}

// AppendTask appends a new incomplete task with the given title to the file at absPath.
func AppendTask(absPath string, title string) error {
	f, err := os.OpenFile(absPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n- [ ] " + title)
	return err
}

// SetScheduled sets or replaces the [scheduled::value] field on the task line at lineNum.
func SetScheduled(absPath string, lineNum int, scheduled string) error {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return nil
	}

	idx := lineNum - 1
	line := lines[idx]
	m := taskLineRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	raw := m[3]

	// Remove existing [scheduled::...] if present
	raw = regexp.MustCompile(`\[scheduled::[^\]]*\]`).ReplaceAllString(raw, "")
	raw = strings.TrimSpace(raw)

	// Append new scheduled field
	raw = raw + " [scheduled::" + scheduled + "]"

	lines[idx] = m[1] + "- [" + strings.ToLower(m[2]) + "] " + strings.TrimSpace(raw)
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
}

// FindListFile searches task folders within vaultPath for a file named {listName}.md.
// Returns the absolute path of the file if found, or an error.
func FindListFile(vaultPath string, folders []string, listName string) (string, error) {
	roots := []string{vaultPath}
	if len(folders) > 0 {
		roots = make([]string, len(folders))
		for i, f := range folders {
			roots[i] = filepath.Join(vaultPath, f)
		}
	}

	target := listName + ".md"
	for _, root := range roots {
		candidate := filepath.Join(root, target)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

// MoveTask removes the task at lineNum from srcPath and appends it to dstPath.
func MoveTask(srcPath string, lineNum int, dstPath string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return nil
	}

	taskLine := lines[lineNum-1]

	// Remove the line from source
	lines = append(lines[:lineNum-1], lines[lineNum:]...)
	if err := os.WriteFile(srcPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}

	// Append to destination
	f, err := os.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + taskLine)
	return err
}

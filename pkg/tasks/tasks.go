// Package tasks provides parsing and querying of Obsidian markdown checkbox tasks.
package tasks

import (
	"bufio"
	"fmt"
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
//
// A task has no persistent, stable identity of its own in the markdown -
// FilePath+LineNum is the only thing that addresses it, and every API
// action that targets a task (PATCH/DELETE) already does so by line number.
// ParentID reuses that same "file_path:line_num" scheme (see parentID) so a
// child's ParentID can be matched directly against another task's
// FilePath+LineNum without a separate id concept. It's recomputed fresh on
// every parse, same as LineNum itself - both go stale the instant a line
// above them is added or removed, until the next parse.
type Task struct {
	FilePath  string   `json:"file_path"`
	LineNum   int      `json:"line_num"`
	Title     string   `json:"title"`
	Status    Status   `json:"status"`
	Type      string   `json:"type"` // "task" or "event"
	Due       string   `json:"due,omitempty"`
	Scheduled string   `json:"scheduled,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	Repeat    string   `json:"repeat,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Level     int      `json:"level"`
	ParentID  string   `json:"parent_id,omitempty"`
	ListName  string   `json:"list_name,omitempty"`
	StartTime string   `json:"start_time,omitempty"`
	EndTime   string   `json:"end_time,omitempty"`
	GoogleID  string   `json:"google_id,omitempty"`
	EventID   string   `json:"event_id,omitempty"`
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
	// Stack of open ancestors by level, shallowest first. Kept as plain
	// values (not pointers into `tasks`) since append() can reallocate the
	// backing array and invalidate any pointer taken into it.
	var ancestors []struct {
		level int
		id    string
	}
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		task := parseLine(line, relPath, lineNum)
		if task != nil {
			task.ListName = listName

			for len(ancestors) > 0 && ancestors[len(ancestors)-1].level >= task.Level {
				ancestors = ancestors[:len(ancestors)-1]
			}
			if len(ancestors) > 0 {
				task.ParentID = ancestors[len(ancestors)-1].id
			}

			tasks = append(tasks, *task)
			ancestors = append(ancestors, struct {
				level int
				id    string
			}{level: task.Level, id: parentID(relPath, lineNum)})
		}
	}

	return tasks, scanner.Err()
}

// parentID builds the "file_path:line_num" identifier a child's ParentID
// points at.
func parentID(filePath string, lineNum int) string {
	return fmt.Sprintf("%s:%d", filePath, lineNum)
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
		Type:      "task",
		Due:       due,
		Scheduled: fields["scheduled"],
		Priority:  fields["priority"],
		Repeat:    fields["repeat"],
		Tags:      tags,
		Level:     level,
		StartTime: startTime,
		EndTime:   endTime,
		GoogleID:  fields["google_id"],
		EventID:   fields["event_id"],
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

// containsTagCI returns true if tags contains the given tag (case-insensitive).
func containsTagCI(tags []string, tag string) bool {
	tagLower := strings.ToLower(tag)
	for _, t := range tags {
		if strings.ToLower(t) == tagLower {
			return true
		}
	}
	return false
}

// KanbanTags are the mutually-exclusive status tags that place a task on the
// Kanban board. A task carries at most one of these at a time.
var KanbanTags = []string{"ToDo", "InProgress", "Done"}

// KanbanStatus returns the task's current Kanban status ("ToDo", "InProgress",
// or "Done"), or "" if it carries none of the KanbanTags and so isn't on the
// board.
func KanbanStatus(t Task) string {
	for _, kt := range KanbanTags {
		if containsTagCI(t.Tags, kt) {
			return kt
		}
	}
	return ""
}

// FilterKanban returns tasks that carry one of the KanbanTags, regardless of
// completion status (a "Done" card is typically also completed, but the
// board is driven by the tag, not the checkbox).
func FilterKanban(tasks []Task) []Task {
	var result []Task
	for _, t := range tasks {
		if KanbanStatus(t) != "" {
			result = append(result, t)
		}
	}
	return result
}

// ParseDir walks a specific absolute directory path and returns all tasks,
// using vaultPath to produce relative file paths.
func ParseDir(vaultPath, absDir string) ([]Task, error) {
	var result []Task
	err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
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
		result = append(result, fileTasks...)
		return nil
	})
	return result, err
}

// FilterToday returns incomplete tasks due or scheduled exactly today, or tagged #Today.
func FilterToday(tasks []Task) []Task {
	td := today()
	var result []Task
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			continue
		}
		dueToday := t.Due != "" && t.Due[:10] == td
		scheduledToday := t.Scheduled != "" && t.Scheduled[:10] == td
		taggedToday := containsTagCI(t.Tags, "today")
		if dueToday || scheduledToday || taggedToday {
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
	td := today()
	var result []Task
	for _, t := range tasks {
		if t.Status == StatusCompleted {
			continue
		}
		if t.StartTime == "" || t.EndTime == "" {
			continue
		}
		dueToday := t.Due != "" && t.Due[:10] == td
		scheduledToday := t.Scheduled != "" && t.Scheduled[:10] == td
		taggedToday := containsTagCI(t.Tags, "today")
		inTodayFile := strings.Contains(filepath.Base(t.FilePath), td)
		if dueToday || scheduledToday || taggedToday || inTodayFile {
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

// FindLineByGoogleID scans absPath and returns the 1-based line number of the task
// whose [google_id::...] field matches googleID. Returns 0 if not found.
func FindLineByGoogleID(absPath, googleID string) (int, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return 0, err
	}
	googleIDRe := regexp.MustCompile(`\[google_id::([^\]]+)\]`)
	for i, line := range strings.Split(string(content), "\n") {
		if taskLineRe.MatchString(line) {
			if m := googleIDRe.FindStringSubmatch(line); m != nil && strings.TrimSpace(m[1]) == googleID {
				return i + 1, nil
			}
		}
	}
	return 0, nil
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

// SetDue sets or replaces the [due::value] field on the task line at lineNum.
func SetDue(absPath string, lineNum int, due string) error {
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

	// Remove existing [due::...] and legacy emoji due date if present
	raw = regexp.MustCompile(`\[due::[^\]]*\]`).ReplaceAllString(raw, "")
	raw = legacyDueRe.ReplaceAllString(raw, "")
	raw = strings.TrimSpace(raw)

	raw = raw + " [due::" + due + "]"

	lines[idx] = m[1] + "- [" + strings.ToLower(m[2]) + "] " + strings.TrimSpace(raw)
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
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

// SetStatusTag sets the task's Kanban status tag on the task line at lineNum,
// replacing any existing tag from KanbanTags while leaving every other tag
// and field on the line untouched. Pass status="" to take the task off the
// board (remove any KanbanTags tag without adding a new one). status must be
// one of KanbanTags or "".
func SetStatusTag(absPath string, lineNum int, status string) error {
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

	// Remove any existing Kanban status tag; every other tag is untouched.
	for _, kt := range KanbanTags {
		raw = regexp.MustCompile(`(?i)#`+kt+`\b`).ReplaceAllString(raw, "")
	}
	raw = strings.TrimSpace(raw)

	if status != "" {
		raw = strings.TrimSpace(raw + " #" + status)
	}

	lines[idx] = m[1] + "- [" + strings.ToLower(m[2]) + "] " + raw
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
}

// SetTags replaces every tag on the task line at lineNum with the given
// tags, in order. The title and every [key::value] field are left
// untouched. Pass an empty slice to strip all tags from the task.
func SetTags(absPath string, lineNum int, tags []string) error {
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

	raw := strings.TrimSpace(tagRe.ReplaceAllString(m[3], ""))

	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		raw = strings.TrimSpace(raw + " #" + t)
	}

	lines[idx] = m[1] + "- [" + strings.ToLower(m[2]) + "] " + raw
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

// RenameTask replaces the title portion of a task line, preserving all metadata and tags.
func RenameTask(absPath string, lineNum int, newTitle string) error {
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

	// Collect metadata [key::value] parts and #tags to preserve them
	var metaParts []string
	for _, match := range dataviewRe.FindAllString(raw, -1) {
		metaParts = append(metaParts, match)
	}
	var tagParts []string
	for _, match := range tagRe.FindAllString(raw, -1) {
		tagParts = append(tagParts, match)
	}

	newRaw := strings.TrimSpace(newTitle)
	if len(metaParts) > 0 {
		newRaw += " " + strings.Join(metaParts, " ")
	}
	if len(tagParts) > 0 {
		newRaw += " " + strings.Join(tagParts, " ")
	}

	lines[idx] = m[1] + "- [" + strings.ToLower(m[2]) + "] " + newRaw
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
}

// TaskEdit is a set of field updates for EditTask. Each field is a pointer:
// nil means "leave unchanged"; non-nil means "set to this value" (an empty
// string clears the field, e.g. removing a due date). Tags, when non-nil,
// replaces the entire tag set (an empty slice removes all tags).
type TaskEdit struct {
	Title     *string
	Due       *string
	Scheduled *string
	Priority  *string
	Repeat    *string
	Tags      *[]string
}

// EditTask applies a set of field updates to the task line at lineNum in a
// single rewrite. Any [key::value] field not touched by the edit (including
// ones EditTask doesn't know about, like google_id or custom fields) is
// preserved as-is; only its position within the line may change.
func EditTask(absPath string, lineNum int, edit TaskEdit) error {
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

	// Collect existing [key::value] fields, preserving first-seen order.
	type kv struct{ key, value string }
	var fields []kv
	indexOf := map[string]int{}
	for _, match := range dataviewRe.FindAllStringSubmatch(raw, -1) {
		key := strings.TrimSpace(match[1])
		val := strings.TrimSpace(match[2])
		if i, ok := indexOf[strings.ToLower(key)]; ok {
			fields[i].value = val
		} else {
			indexOf[strings.ToLower(key)] = len(fields)
			fields = append(fields, kv{key, val})
		}
	}

	setField := func(key string, value *string) {
		if value == nil {
			return
		}
		lk := strings.ToLower(key)
		if i, ok := indexOf[lk]; ok {
			if *value == "" {
				fields = append(fields[:i], fields[i+1:]...)
				delete(indexOf, lk)
				for k, v := range indexOf {
					if v > i {
						indexOf[k] = v - 1
					}
				}
			} else {
				fields[i].value = *value
			}
		} else if *value != "" {
			indexOf[lk] = len(fields)
			fields = append(fields, kv{key, *value})
		}
	}

	setField("due", edit.Due)
	setField("scheduled", edit.Scheduled)
	setField("priority", edit.Priority)
	setField("repeat", edit.Repeat)

	// Title
	title := strings.TrimSpace(tagRe.ReplaceAllString(dataviewRe.ReplaceAllString(raw, ""), ""))
	if edit.Title != nil {
		title = strings.TrimSpace(*edit.Title)
	}

	// Tags
	var tags []string
	for _, tm := range tagRe.FindAllStringSubmatch(raw, -1) {
		tags = append(tags, tm[1])
	}
	if edit.Tags != nil {
		tags = *edit.Tags
	}

	newRaw := title
	for _, f := range fields {
		newRaw += " [" + f.key + "::" + f.value + "]"
	}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			newRaw += " #" + t
		}
	}

	lines[idx] = m[1] + "- [" + strings.ToLower(m[2]) + "] " + strings.TrimSpace(newRaw)
	return os.WriteFile(absPath, []byte(strings.Join(lines, "\n")), 0644)
}

// AppendSubtask inserts a new incomplete task one indentation level deeper
// than the parent at parentLine, positioned after any of the parent's
// existing children (so it becomes the last child), in the same file.
// Returns an error only on I/O failure; a missing/non-task parentLine is a
// silent no-op, matching the other line-targeted functions in this file.
func AppendSubtask(absPath string, parentLine int, title string) error {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if parentLine < 1 || parentLine > len(lines) {
		return nil
	}

	parentIdx := parentLine - 1
	pm := taskLineRe.FindStringSubmatch(lines[parentIdx])
	if pm == nil {
		return nil
	}
	parentIndent := pm[1]
	childIndent := parentIndent + "    "

	// Walk past the parent's existing children (lines indented deeper than
	// the parent) to insert after the last one.
	insertAt := parentIdx + 1
	for insertAt < len(lines) {
		cm := taskLineRe.FindStringSubmatch(lines[insertAt])
		if cm == nil || len(cm[1]) <= len(parentIndent) {
			break
		}
		insertAt++
	}

	newLine := childIndent + "- [ ] " + strings.TrimSpace(title)
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertAt]...)
	newLines = append(newLines, newLine)
	newLines = append(newLines, lines[insertAt:]...)

	return os.WriteFile(absPath, []byte(strings.Join(newLines, "\n")), 0644)
}

// MoveTask removes the task at lineNum from srcPath - along with any of its
// existing children (contiguous lines indented deeper than it, same as
// AppendSubtask's notion of "children") - and appends the whole block to
// dstPath, preserving each line's indentation relative to the others. This
// keeps the parent/child structure intact (and therefore ParentID correct
// on the next parse) instead of leaving children orphaned in the old file.
func MoveTask(srcPath string, lineNum int, dstPath string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return nil
	}

	idx := lineNum - 1
	m := taskLineRe.FindStringSubmatch(lines[idx])
	if m == nil {
		return nil
	}
	taskIndent := m[1]

	// Find the end of this task's block: itself plus any contiguous lines
	// indented deeper than it.
	end := idx + 1
	for end < len(lines) {
		cm := taskLineRe.FindStringSubmatch(lines[end])
		if cm == nil || len(cm[1]) <= len(taskIndent) {
			break
		}
		end++
	}

	block := append([]string(nil), lines[idx:end]...)

	// Remove the block from source.
	remaining := make([]string, 0, len(lines)-(end-idx))
	remaining = append(remaining, lines[:idx]...)
	remaining = append(remaining, lines[end:]...)
	if err := os.WriteFile(srcPath, []byte(strings.Join(remaining, "\n")), 0644); err != nil {
		return err
	}

	// Append the block to destination.
	f, err := os.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + strings.Join(block, "\n"))
	return err
}

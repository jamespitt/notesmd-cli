// Package tasks provides parsing and querying of Obsidian markdown checkbox tasks.
package tasks

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	FilePath  string  `json:"file_path"`
	LineNum   int     `json:"line_num"`
	Title     string  `json:"title"`
	Status    Status  `json:"status"`
	Due       string  `json:"due,omitempty"`
	Scheduled string  `json:"scheduled,omitempty"`
	Priority  string  `json:"priority,omitempty"`
	Repeat    string  `json:"repeat,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Level     int     `json:"level"`
}

var (
	taskLineRe    = regexp.MustCompile(`^(\s*)-\s*\[([xX ])\]\s+(.*)`)
	dataviewRe    = regexp.MustCompile(`\[([^\]]+?)::([^\]]*)\]`)
	tagRe         = regexp.MustCompile(`#([\w/]+)`)
	legacyDueRe   = regexp.MustCompile(`📅\s*(\d{4}-\d{2}-\d{2})`)
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

	var tasks []Task
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		task := parseLine(line, relPath, lineNum)
		if task != nil {
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

	// Clean title: remove dataview fields and tags
	title := dataviewRe.ReplaceAllString(raw, "")
	title = tagRe.ReplaceAllString(title, "")
	title = legacyDueRe.ReplaceAllString(title, "")
	title = strings.TrimSpace(title)

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
	}
}

// today returns today's date in YYYY-MM-DD format.
func today() string {
	return time.Now().Format("2006-01-02")
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

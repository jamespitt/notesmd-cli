package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	diaryDayHeadingRe = regexp.MustCompile(`^###\s+(\d{4}-\d{2}-\d{2})`)
	anyHeadingRe      = regexp.MustCompile(`^#{1,6}\s`)
	h1HeadingRe       = regexp.MustCompile(`^#\s`)
)

// DiaryPath returns the vault-relative path to a project's diary file.
func DiaryPath(projectsFolder, projectName string) string {
	return filepath.ToSlash(filepath.Join(projectsFolder, projectName, "Diary.md"))
}

// ReadDiary returns the raw markdown of a project's Diary.md. The bool is false
// (with an empty string, no error) when the file doesn't exist yet.
func ReadDiary(vaultPath, projectsFolder, projectName string) (string, bool, error) {
	abs := filepath.Join(vaultPath, DiaryPath(projectsFolder, projectName))
	data, err := os.ReadFile(abs)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}

// AppendDiaryEntry adds text to a project's Diary.md under a heading for the
// given day. If a "### YYYY-MM-DD" heading for that day already exists, the text
// is appended as a new paragraph at the end of that day's section; otherwise a
// new day section is inserted directly below the "# ... Diary" title, above any
// older day sections. The file (with frontmatter + title) is created if missing.
// Returns the full updated file content.
func AppendDiaryEntry(vaultPath, projectsFolder, projectName, projectTitle, text string, day time.Time) (string, error) {
	text = strings.Trim(text, "\n \t")
	if text == "" {
		return "", fmt.Errorf("diary entry text is empty")
	}
	dayStr := day.Format("2006-01-02")
	abs := filepath.Join(vaultPath, DiaryPath(projectsFolder, projectName))

	var content string
	if data, err := os.ReadFile(abs); err == nil {
		content = string(data)
	} else if os.IsNotExist(err) {
		if strings.TrimSpace(projectTitle) == "" {
			projectTitle = projectName
		}
		content = fmt.Sprintf(
			"---\ndate-created: %s\ntitle: %s - Diary\ntags: ProjectDiary\n---\n\n# %s Diary\n",
			dayStr, projectTitle, projectTitle,
		)
	} else {
		return "", err
	}

	updated := strings.Join(insertDiaryText(strings.Split(content, "\n"), dayStr, text), "\n")

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return updated, nil
}

func insertDiaryText(lines []string, dayStr, text string) []string {
	entry := strings.Split(text, "\n")

	// Case 1: a heading for this day already exists — append under it.
	for i, line := range lines {
		m := diaryDayHeadingRe.FindStringSubmatch(line)
		if m == nil || m[1] != dayStr {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if anyHeadingRe.MatchString(lines[j]) {
				end = j
				break
			}
		}
		for end > i+1 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		block := append([]string{""}, entry...)
		return splice(lines, end, block)
	}

	// Case 2: insert a new day section below the H1 title (or after frontmatter).
	section := append([]string{"", "### " + dayStr, ""}, entry...)
	insertAt := frontmatterEnd(lines)
	for i := insertAt; i < len(lines); i++ {
		if h1HeadingRe.MatchString(lines[i]) {
			insertAt = i + 1
			break
		}
	}
	return splice(lines, insertAt, section)
}

// splice inserts block into lines at index i.
func splice(lines []string, i int, block []string) []string {
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:i]...)
	out = append(out, block...)
	out = append(out, lines[i:]...)
	return out
}

// frontmatterEnd returns the index of the first line after a leading `---` YAML
// frontmatter block, or 0 if there isn't one.
func frontmatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1
		}
	}
	return 0
}

package tasks

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	atTimeRe = regexp.MustCompile(`\bat\s+(\d{1,2}:\d{2})\b`)
	everyNRe = regexp.MustCompile(`^every\s+(\d+)\s+(day|week|month)s?$`)
)

// NextDueDate returns the next occurrence after baseDate for the given repeat rule.
// Supported rules (case-insensitive):
//
//	daily / every day
//	weekly / every week
//	every N days / every N weeks / every N months
//	monthly / every month
//	when done  (returns today)
//	any of the above + " at HH:MM"
func NextDueDate(baseDate time.Time, rule string) (time.Time, error) {
	rule = strings.ToLower(strings.TrimSpace(rule))

	// Extract and strip optional "at HH:MM" suffix
	var timeOfDay *time.Time
	if m := atTimeRe.FindStringSubmatch(rule); m != nil {
		t, err := time.Parse("15:04", m[1])
		if err == nil {
			timeOfDay = &t
		}
		rule = strings.TrimSpace(atTimeRe.ReplaceAllString(rule, ""))
	}

	// Normalise common aliases
	switch rule {
	case "daily":
		rule = "every day"
	case "weekly":
		rule = "every week"
	case "monthly":
		rule = "every month"
	}

	var next time.Time

	switch rule {
	case "when done":
		t := time.Now()
		next = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	case "every day":
		next = baseDate.AddDate(0, 0, 1)

	case "every week":
		next = baseDate.AddDate(0, 0, 7)

	case "every month":
		next = baseDate.AddDate(0, 1, 0)

	default:
		if m := everyNRe.FindStringSubmatch(rule); m != nil {
			n, _ := strconv.Atoi(m[1])
			switch m[2] {
			case "day":
				next = baseDate.AddDate(0, 0, n)
			case "week":
				next = baseDate.AddDate(0, 0, n*7)
			case "month":
				next = baseDate.AddDate(0, n, 0)
			}
		} else {
			return time.Time{}, fmt.Errorf("unrecognised repeat rule: %q", rule)
		}
	}

	if timeOfDay != nil {
		next = time.Date(next.Year(), next.Month(), next.Day(),
			timeOfDay.Hour(), timeOfDay.Minute(), 0, 0, next.Location())
	}

	return next, nil
}

// RescheduleTask advances the repeating task at lineNum in absPath to its next due date.
// If the computed next date is in the past, the date portion is snapped to today while
// preserving any time component from the repeat rule.
// Returns the new due string that was written, e.g. "2026-05-01" or "2026-05-01T09:00".
func RescheduleTask(absPath string, lineNum int) (string, error) {
	content, err := readLines(absPath)
	if err != nil {
		return "", err
	}
	if lineNum < 1 || lineNum > len(content) {
		return "", fmt.Errorf("line %d out of range", lineNum)
	}

	task := parseLine(content[lineNum-1], absPath, lineNum)
	if task == nil {
		return "", fmt.Errorf("no task found at line %d", lineNum)
	}
	if task.Repeat == "" {
		return "", fmt.Errorf("task has no repeat rule")
	}
	if task.Due == "" {
		return "", fmt.Errorf("task has no due date")
	}

	due, err := parseTaskDate(task.Due)
	if err != nil {
		return "", fmt.Errorf("cannot parse due date %q: %w", task.Due, err)
	}

	next, err := NextDueDate(due, task.Repeat)
	if err != nil {
		return "", err
	}

	// Snap to today if the computed next date is already in the past
	if now := time.Now(); next.Before(now) {
		next = time.Date(now.Year(), now.Month(), now.Day(),
			next.Hour(), next.Minute(), 0, 0, next.Location())
	}

	var nextDueStr string
	if next.Hour() != 0 || next.Minute() != 0 {
		nextDueStr = next.Format("2006-01-02T15:04")
	} else {
		nextDueStr = next.Format("2006-01-02")
	}

	if err := SetDue(absPath, lineNum, nextDueStr); err != nil {
		return "", err
	}
	return nextDueStr, nil
}

// parseTaskDate parses a due/scheduled string in YYYY-MM-DD or YYYY-MM-DDTHH:MM format.
func parseTaskDate(s string) (time.Time, error) {
	if len(s) >= 16 {
		return time.Parse("2006-01-02T15:04", s[:16])
	}
	if len(s) >= 10 {
		return time.Parse("2006-01-02", s[:10])
	}
	return time.Time{}, fmt.Errorf("date string too short: %q", s)
}

// readLines reads a file and returns its lines split on "\n".
func readLines(absPath string) ([]string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

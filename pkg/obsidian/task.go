package obsidian

import (
	"regexp"
	"strings"
	"time"
)

var (
	taskLinePattern  = regexp.MustCompile(`^- \[([ xX])\] (.+)$`)
	scheduledPattern = regexp.MustCompile(`\[scheduled::([^\]]+)\]`)
	googleIDPattern  = regexp.MustCompile(`\[google_id::([^\]]+)\]`)
	tagPattern       = regexp.MustCompile(`#(\w+)`)
	metaPattern      = regexp.MustCompile(`\[[^\]]+::[^\]]+\]`)
)

type Task struct {
	FilePath   string
	LineNumber int
	RawText    string
	Text       string
	Tags       []string
	Scheduled  *time.Time
	GoogleID   string
	Completed  bool
}

type TaskFilters struct {
	Tags []string   // OR logic; empty = no tag filter
	Date *time.Time // exact date match (day only)
	From *time.Time // range start (inclusive)
	To   *time.Time // range end (inclusive)
}

var scheduledLayouts = []string{
	"2006-01-02T15:04",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// ParseTask parses a single line into a Task. Returns (nil, false) if the
// line is not a task checkbox.
func ParseTask(line string) (*Task, bool) {
	trimmed := strings.TrimSpace(line)
	m := taskLinePattern.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, false
	}

	completed := m[1] != " "
	body := m[2]

	task := &Task{
		RawText:   trimmed,
		Completed: completed,
	}

	// Extract [scheduled::...]
	if sm := scheduledPattern.FindStringSubmatch(body); sm != nil {
		for _, layout := range scheduledLayouts {
			if t, err := time.Parse(layout, strings.TrimSpace(sm[1])); err == nil {
				task.Scheduled = &t
				break
			}
		}
	}

	// Extract [google_id::...]
	if gm := googleIDPattern.FindStringSubmatch(body); gm != nil {
		task.GoogleID = gm[1]
	}

	// Extract #Tags
	for _, tm := range tagPattern.FindAllStringSubmatch(body, -1) {
		task.Tags = append(task.Tags, tm[1])
	}

	// Clean text: strip [key::value] and #Tag tokens
	text := metaPattern.ReplaceAllString(body, "")
	text = tagPattern.ReplaceAllString(text, "")
	task.Text = strings.TrimSpace(text)

	return task, true
}

func (t *Task) matchesTags(tags []string) bool {
	for _, filterTag := range tags {
		for _, taskTag := range t.Tags {
			if strings.EqualFold(taskTag, filterTag) {
				return true
			}
		}
	}
	return false
}

func (t *Task) matchesDateFilters(f TaskFilters) bool {
	if f.Date != nil {
		if t.Scheduled == nil {
			return false
		}
		sy, sm, sd := t.Scheduled.Date()
		fy, fm, fd := f.Date.Date()
		if sy != fy || sm != fm || sd != fd {
			return false
		}
	}

	if (f.From != nil || f.To != nil) && t.Scheduled == nil {
		return false
	}

	if f.From != nil && t.Scheduled != nil {
		fromDay := time.Date(f.From.Year(), f.From.Month(), f.From.Day(), 0, 0, 0, 0, time.UTC)
		schedDay := time.Date(t.Scheduled.Year(), t.Scheduled.Month(), t.Scheduled.Day(), 0, 0, 0, 0, time.UTC)
		if schedDay.Before(fromDay) {
			return false
		}
	}

	if f.To != nil && t.Scheduled != nil {
		toDay := time.Date(f.To.Year(), f.To.Month(), f.To.Day(), 0, 0, 0, 0, time.UTC)
		schedDay := time.Date(t.Scheduled.Year(), t.Scheduled.Month(), t.Scheduled.Day(), 0, 0, 0, 0, time.UTC)
		if schedDay.After(toDay) {
			return false
		}
	}

	return true
}

// MatchesFilters returns true if the task passes the active filters.
// When both tag and date filters are provided they use OR logic: a task
// matches if it satisfies the tag filter OR the date/range filter.
func (t *Task) MatchesFilters(f TaskFilters) bool {
	hasTagFilter := len(f.Tags) > 0
	hasDateFilter := f.Date != nil || f.From != nil || f.To != nil

	if !hasTagFilter && !hasDateFilter {
		return true
	}

	if hasTagFilter && hasDateFilter {
		return t.matchesTags(f.Tags) || t.matchesDateFilters(f)
	}

	if hasTagFilter {
		return t.matchesTags(f.Tags)
	}

	return t.matchesDateFilters(f)
}

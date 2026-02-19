package obsidian

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseTask_ValidUnchecked(t *testing.T) {
	line := "- [ ] Book car into service #Today [scheduled::2026-02-18T09:30] [google_id::UUdOdWVWUkVTX2I1SkJQVg]"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.NotNil(t, task)
	assert.False(t, task.Completed)
	assert.Equal(t, "Book car into service", task.Text)
	assert.Equal(t, []string{"Today"}, task.Tags)
	assert.Equal(t, "UUdOdWVWUkVTX2I1SkJQVg", task.GoogleID)
	assert.NotNil(t, task.Scheduled)
	assert.Equal(t, 2026, task.Scheduled.Year())
	assert.Equal(t, time.February, task.Scheduled.Month())
	assert.Equal(t, 18, task.Scheduled.Day())
	assert.Equal(t, 9, task.Scheduled.Hour())
	assert.Equal(t, 30, task.Scheduled.Minute())
}

func TestParseTask_ValidChecked(t *testing.T) {
	line := "- [x] Buy groceries #Today"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.True(t, task.Completed)
	assert.Equal(t, "Buy groceries", task.Text)
	assert.Equal(t, []string{"Today"}, task.Tags)
	assert.Nil(t, task.Scheduled)
}

func TestParseTask_CheckedUppercase(t *testing.T) {
	line := "- [X] Buy groceries"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.True(t, task.Completed)
}

func TestParseTask_MultipleTags(t *testing.T) {
	line := "- [ ] Fix bug #Today #Work #Urgent"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.Equal(t, []string{"Today", "Work", "Urgent"}, task.Tags)
}

func TestParseTask_NoScheduled(t *testing.T) {
	line := "- [ ] Some task #Later"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.Nil(t, task.Scheduled)
}

func TestParseTask_ScheduledWithSpaceAfterDoubleColon(t *testing.T) {
	line := "- [ ] Tori chat [scheduled:: 2026-02-18T09:05]"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.NotNil(t, task.Scheduled)
	assert.Equal(t, 2026, task.Scheduled.Year())
	assert.Equal(t, time.February, task.Scheduled.Month())
	assert.Equal(t, 18, task.Scheduled.Day())
	assert.Equal(t, 9, task.Scheduled.Hour())
	assert.Equal(t, 5, task.Scheduled.Minute())
}

func TestParseTask_DateOnlyScheduled(t *testing.T) {
	line := "- [ ] Task [scheduled::2026-03-01]"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.NotNil(t, task.Scheduled)
	assert.Equal(t, 2026, task.Scheduled.Year())
	assert.Equal(t, time.March, task.Scheduled.Month())
	assert.Equal(t, 1, task.Scheduled.Day())
}

func TestParseTask_NotATask(t *testing.T) {
	cases := []string{
		"# Heading",
		"Regular paragraph",
		"- Just a bullet",
		"  - [ incomplete bracket",
		"",
	}
	for _, line := range cases {
		task, ok := ParseTask(line)
		assert.False(t, ok, "expected not a task: %q", line)
		assert.Nil(t, task)
	}
}

func TestParseTask_WithIndentation(t *testing.T) {
	line := "   - [ ] Indented task #Today"
	task, ok := ParseTask(line)
	assert.True(t, ok)
	assert.Equal(t, "Indented task", task.Text)
}

func TestMatchesFilters_NoFilters(t *testing.T) {
	task := &Task{Tags: []string{"Today"}}
	assert.True(t, task.MatchesFilters(TaskFilters{}))
}

func TestMatchesFilters_TagMatch(t *testing.T) {
	task := &Task{Tags: []string{"Today", "Work"}}
	assert.True(t, task.MatchesFilters(TaskFilters{Tags: []string{"Today"}}))
	assert.True(t, task.MatchesFilters(TaskFilters{Tags: []string{"Work", "Other"}}))
	assert.False(t, task.MatchesFilters(TaskFilters{Tags: []string{"Tomorrow"}}))
}

func TestMatchesFilters_TagCaseInsensitive(t *testing.T) {
	task := &Task{Tags: []string{"Today"}}
	assert.True(t, task.MatchesFilters(TaskFilters{Tags: []string{"today"}}))
	assert.True(t, task.MatchesFilters(TaskFilters{Tags: []string{"TODAY"}}))
}

func TestMatchesFilters_ExactDate(t *testing.T) {
	sched := time.Date(2026, 2, 18, 9, 30, 0, 0, time.UTC)
	task := &Task{Scheduled: &sched}

	matchDate := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	assert.True(t, task.MatchesFilters(TaskFilters{Date: &matchDate}))

	noMatchDate := time.Date(2026, 2, 19, 0, 0, 0, 0, time.UTC)
	assert.False(t, task.MatchesFilters(TaskFilters{Date: &noMatchDate}))
}

func TestMatchesFilters_ExactDate_NoScheduled(t *testing.T) {
	task := &Task{}
	d := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	assert.False(t, task.MatchesFilters(TaskFilters{Date: &d}))
}

func TestMatchesFilters_DateRange(t *testing.T) {
	sched := time.Date(2026, 2, 18, 9, 30, 0, 0, time.UTC)
	task := &Task{Scheduled: &sched}

	from := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 19, 0, 0, 0, 0, time.UTC)
	assert.True(t, task.MatchesFilters(TaskFilters{From: &from, To: &to}))

	fromAfter := time.Date(2026, 2, 19, 0, 0, 0, 0, time.UTC)
	assert.False(t, task.MatchesFilters(TaskFilters{From: &fromAfter}))

	toBefore := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	assert.False(t, task.MatchesFilters(TaskFilters{To: &toBefore}))
}

func TestMatchesFilters_RangeWithNoScheduled(t *testing.T) {
	task := &Task{}
	from := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	assert.False(t, task.MatchesFilters(TaskFilters{From: &from}))
}

func TestMatchesFilters_TagAndDate(t *testing.T) {
	sched := time.Date(2026, 2, 18, 9, 30, 0, 0, time.UTC)
	matchDate := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	noMatchDate := time.Date(2026, 2, 19, 0, 0, 0, 0, time.UTC)

	// Both tag and date match → true
	taskBoth := &Task{Tags: []string{"Today"}, Scheduled: &sched}
	assert.True(t, taskBoth.MatchesFilters(TaskFilters{Tags: []string{"Today"}, Date: &matchDate}))

	// Tag matches, date doesn't → true (OR)
	assert.True(t, taskBoth.MatchesFilters(TaskFilters{Tags: []string{"Today"}, Date: &noMatchDate}))

	// Date matches, tag doesn't → true (OR)
	taskDateOnly := &Task{Scheduled: &sched}
	assert.True(t, taskDateOnly.MatchesFilters(TaskFilters{Tags: []string{"Tomorrow"}, Date: &matchDate}))

	// Neither matches → false
	taskNeither := &Task{Tags: []string{"Work"}, Scheduled: &sched}
	assert.False(t, taskNeither.MatchesFilters(TaskFilters{Tags: []string{"Tomorrow"}, Date: &noMatchDate}))
}

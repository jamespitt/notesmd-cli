package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A line as the calendar sync writes it: the time is on scheduled and the
// length in a single-colon "[duration: …]".
const calendarLine = "- [ ] MF [scheduled:: 2026-09-29T06:00] [duration: 1h15m] [event_id:: t22v_20260929T050000Z] [calendar_name:: x@gmail.com]"

func TestParseCalendarEventTimes(t *testing.T) {
	tk := parseLine(calendarLine, "Calendar_2026-09-29.md", 1)
	if tk == nil {
		t.Fatal("not parsed")
	}
	if tk.Title != "MF" || tk.StartTime != "06:00" || tk.EndTime != "07:15" || tk.Duration != "1h15m" {
		t.Fatalf("got title=%q start=%q end=%q duration=%q", tk.Title, tk.StartTime, tk.EndTime, tk.Duration)
	}
}

func TestParseTimes(t *testing.T) {
	cases := []struct {
		name, title, scheduled, due, duration string
		start, end                            string
	}{
		{"title range wins", "09:30-10:00 Standup", "2026-09-29T08:00", "", "1h", "09:30", "10:00"},
		{"title start plus duration", "09:30 Standup", "", "", "45m", "09:30", "10:15"},
		{"title start only", "09:30 Standup", "", "", "", "09:30", ""},
		{"scheduled time", "Standup", "2026-09-29T10:30", "", "30m", "10:30", "11:00"},
		{"due time", "Standup", "", "2026-09-29T10:30", "", "10:30", ""},
		{"date only", "Birthday", "2026-09-29", "", "", "", ""},
		{"dataview duration form", "Standup", "2026-09-29T10:30", "", "1h", "10:30", "11:30"},
		{"past midnight clamps", "Late", "2026-09-29T23:30", "", "2h", "23:30", "23:59"},
		{"bad duration ignored", "Standup", "2026-09-29T10:30", "", "soon", "10:30", ""},
	}
	for _, c := range cases {
		s, e := parseTimes(c.title, c.scheduled, c.due, c.duration)
		if s != c.start || e != c.end {
			t.Errorf("%s: got %q-%q, want %q-%q", c.name, s, e, c.start, c.end)
		}
	}
}

func TestDataviewDurationField(t *testing.T) {
	tk := parseLine("- [ ] Standup [scheduled:: 2026-09-29T10:30] [duration::30m]", "f.md", 1)
	if tk.StartTime != "10:30" || tk.EndTime != "11:00" || tk.Duration != "30m" {
		t.Fatalf("got %q-%q duration=%q", tk.StartTime, tk.EndTime, tk.Duration)
	}
}

func TestFilterTimelineIncludesCalendarEvents(t *testing.T) {
	td := today()
	all := []Task{
		{Title: "Event", Scheduled: td + "T10:00", StartTime: "10:00", EndTime: "10:30"},
		{Title: "No end", Scheduled: td + "T09:00", StartTime: "09:00"},
		{Title: "Untimed", Scheduled: td},
		{Title: "Done", Scheduled: td + "T08:00", StartTime: "08:00", Status: StatusCompleted},
	}
	got := FilterTimeline(all)
	if len(got) != 2 || got[0].Title != "No end" || got[1].Title != "Event" {
		t.Fatalf("got %+v", got)
	}
	if got[0].EndTime != "09:30" {
		t.Fatalf("default end = %q, want 09:30", got[0].EndTime)
	}
}

// Renaming or editing an event mustn't drop its "[duration: …]", which the
// client never sees in the title.
func TestRenameAndEditKeepDuration(t *testing.T) {
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "l"))
	file := filepath.Join(t.TempDir(), "Calendar_2026-09-29.md")
	os.WriteFile(file, []byte(calendarLine+"\n"), 0644)

	if err := RenameTask(file, 1, "Morning run"); err != nil {
		t.Fatal(err)
	}
	title := "Morning run!"
	if err := EditTask(file, 1, TaskEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(file)
	if strings.Count(string(got), "[duration: 1h15m]") != 1 {
		t.Fatalf("duration lost or duplicated:\n%s", got)
	}
	tk := parseLine(strings.TrimSpace(string(got)), file, 1)
	if tk.Title != "Morning run!" || tk.EndTime != "07:15" {
		t.Fatalf("round trip = %+v", tk)
	}
}

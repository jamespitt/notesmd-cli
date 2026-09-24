package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddTaskAppendsAndIsIdempotent(t *testing.T) {
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "l"))
	file := filepath.Join(t.TempDir(), "Obsidian.md")
	os.WriteFile(file, []byte("# Obsidian\n\n- [ ] Existing #Todo [google_id::abc]"), 0644)

	n := NewTask{Title: "Reschedule 1:1", Tag: "ToTriage", Created: "2026-09-24",
		Source: "wiki/x.md", Users: []string{"A", "B"}}
	added, err := AddTask(file, n)
	if err != nil || !added {
		t.Fatalf("first add: added=%v err=%v", added, err)
	}
	want := "- [ ] Reschedule 1:1 #ToTriage [created::2026-09-24] [source:: wiki/x.md] [user:: A, B]"
	got, _ := os.ReadFile(file)
	if !strings.Contains(string(got), "\n"+want+"\n") {
		t.Fatalf("line missing:\n%s", got)
	}

	// Same title again, different case and extra tag: skipped.
	n.Title = "RESCHEDULE 1:1"
	if added, _ := AddTask(file, n); added {
		t.Fatal("duplicate title should be skipped")
	}
	got2, _ := os.ReadFile(file)
	if string(got2) != string(got) {
		t.Fatal("file changed on duplicate add")
	}

	// Parses back as a task with the right title.
	tk := parseLine(want, file, 1)
	if tk == nil || tk.Title != "Reschedule 1:1" {
		t.Fatalf("round trip title = %+v", tk)
	}
}

func TestAddTaskCreatesMissingFile(t *testing.T) {
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "l"))
	file := filepath.Join(t.TempDir(), "New.md")
	if added, err := AddTask(file, NewTask{Title: "x"}); err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
}

func TestCancelledTasksAreHiddenButKeepSubtaskBlocksIntact(t *testing.T) {
	if tk := parseLine("- [-] Dropped #Todo [google_id::g]", "L.md", 1); tk != nil {
		t.Fatalf("cancelled task should not be listed, got %+v", tk)
	}
	if tk := parseLine("- [ ] Live", "L.md", 1); tk == nil {
		t.Fatal("open task should still parse")
	}
	if !taskLineRe.MatchString("    - [-] child") {
		t.Fatal("cancelled child must still count as a task line for block boundaries")
	}
}

package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Task 6r94865hm3 in tm_notes' Google Tasks/Obsidian.md lost its google_id in
// git on 2026-10-16 (see tasks/tests/test_id_loss.py). These check that the
// notesmd-cli writers never drop a synced line's [google_id::] or [id::]: a
// writer that did would produce exactly that loss.

const (
	keepID       = "6r94865hm3"
	keepGoogleID = "VTNUc3ZLa1p1YmFNNDI3eg"
)

const keepLine = "- [ ] Share internal master versioning RFC #ToTriage " +
	"[created::2026-10-15 00:00:00+00:00] [google_id::" + keepGoogleID + "] " +
	"[source:: 2026-10-15 Delivery Platform Standup.md] [user:: James Pitt] " +
	"[id::" + keepID + "] [updated::2026-10-15T11:17:15Z]"

// keptLine returns the line that carries the task's [id::] across files.
func keptLine(t *testing.T, paths ...string) string {
	t.Helper()
	for _, p := range paths {
		b, err := os.ReadFile(p)
		assert.NoError(t, err)
		for _, l := range strings.Split(string(b), "\n") {
			if strings.Contains(l, "[id::"+keepID+"]") {
				return l
			}
		}
	}
	return ""
}

func setupKeepFile(t *testing.T) (dir, file string) {
	t.Helper()
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))
	dir = t.TempDir()
	file = filepath.Join(dir, "Obsidian.md")
	assert.NoError(t, os.WriteFile(file, []byte("# Obsidian\n\n"+keepLine+"\n"), 0644))
	return dir, file
}

func assertKept(t *testing.T, line, op string) {
	t.Helper()
	assert.NotEmpty(t, line, "%s: task line disappeared", op)
	assert.Contains(t, line, "[google_id::"+keepGoogleID+"]", "%s dropped google_id: %s", op, line)
	assert.Contains(t, line, "[id::"+keepID+"]", "%s dropped id: %s", op, line)
}

func TestEditTaskKeepsGoogleIDAndID(t *testing.T) {
	_, file := setupKeepFile(t)
	assert.NoError(t, EditTask(file, 3, TaskEdit{Title: strPtr("Renamed"), Due: strPtr("2026-10-20")}))
	assertKept(t, keptLine(t, file), "EditTask")
}

func TestSetStatusTagKeepsGoogleIDAndID(t *testing.T) {
	_, file := setupKeepFile(t)
	assert.NoError(t, SetStatusTag(file, 3, "InProgress"))
	assertKept(t, keptLine(t, file), "SetStatusTag")
}

func TestSetTagsKeepsGoogleIDAndID(t *testing.T) {
	_, file := setupKeepFile(t)
	assert.NoError(t, SetTags(file, 3, []string{"Work"}))
	assertKept(t, keptLine(t, file), "SetTags")
}

func TestAppendSubtaskKeepsParentGoogleIDAndID(t *testing.T) {
	_, file := setupKeepFile(t)
	assert.NoError(t, AppendSubtask(file, 3, "child"))
	assertKept(t, keptLine(t, file), "AppendSubtask")
}

func TestCancelOrDeleteTaskKeepsGoogleIDAndID(t *testing.T) {
	// Cancelling a synced task must leave its line (the sync purges it later).
	_, file := setupKeepFile(t)
	cancelled, err := CancelOrDeleteTask(file, 3)
	assert.NoError(t, err)
	assert.True(t, cancelled)
	assertKept(t, keptLine(t, file), "CancelOrDeleteTask")
}

func TestMoveTaskKeepsGoogleIDAndID(t *testing.T) {
	dir, file := setupKeepFile(t)
	dst := filepath.Join(dir, "Other.md")
	assert.NoError(t, os.WriteFile(dst, []byte("# Other\n"), 0644))
	assert.NoError(t, MoveTask(file, 3, dst))
	assertKept(t, keptLine(t, dst, file), "MoveTask")
}

func TestAddTaskDoesNotDuplicateATitleAlreadyInTheFile(t *testing.T) {
	// The notes-side guard against the duplicate: adding a task whose title is
	// already on an id-less line in the list must not add a second line.
	_, file := setupKeepFile(t)
	stale := "- [ ] Share internal master versioning RFC #ToTriage [created::2026-10-15] [updated::2026-10-15T11:17:15Z]"
	b, _ := os.ReadFile(file)
	assert.NoError(t, os.WriteFile(file, []byte(string(b)+stale+"\n"), 0644))

	before, _ := os.ReadFile(file)
	added, err := AddTask(file, NewTask{Title: "Share internal master versioning RFC", Tag: "ToTriage"})
	assert.NoError(t, err)
	assert.False(t, added)
	after, _ := os.ReadFile(file)
	assert.Equal(t, string(before), string(after))
}

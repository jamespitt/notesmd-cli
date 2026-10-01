package tasks

import (
	"os"
	"regexp"
	"testing"
	"time"
)

// frozen is the instant every [updated::] stamp written during the tests
// carries, so assertions can compare whole lines.
var frozen = time.Now()

// stamp is the [updated::] value the code under test writes.
func stamp() string { return frozen.UTC().Format(updatedLayout) }

// testID is the [id::] every task created during the tests is given.
const testID = "testtask00"

func TestMain(m *testing.M) {
	clock = func() time.Time { return frozen }
	randomID := newTaskID
	newTaskID = func() string { return testID }
	code := m.Run()
	newTaskID = randomID
	os.Exit(code)
}

func TestNewTaskIDIsTenBase32CharactersAndRandom(t *testing.T) {
	pinned := newTaskID
	defer func() { newTaskID = pinned }()
	newTaskID = realNewTaskID

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id := newTaskID()
		if !regexp.MustCompile(`^[0-9a-hjkmnp-tv-z]{10}$`).MatchString(id) {
			t.Fatalf("id %q is not 10 Crockford base32 characters", id)
		}
		seen[id] = true
	}
	if len(seen) != 200 {
		t.Fatalf("expected 200 distinct ids, got %d", len(seen))
	}
}

func TestStampCreatedGivesAnIDOnce(t *testing.T) {
	if got, want := stampCreated("Buy milk #ToDo [created::2026-01-01]"), "Buy milk #ToDo [created::2026-01-01] [id::"+testID+"] [updated::"+stamp()+"]"; got != want {
		t.Fatalf("stampCreated = %q, want %q", got, want)
	}
	// A caller-supplied id (a line being re-created) is kept.
	if got, want := stampCreated("Buy milk #ToDo [created::2026-01-01] [id::keepme0000]"), "Buy milk #ToDo [created::2026-01-01] [id::keepme0000] [updated::"+stamp()+"]"; got != want {
		t.Fatalf("stampCreated = %q, want %q", got, want)
	}
	// A later write keeps the id and moves only the stamp.
	if got, want := touchUpdated("Buy milk #ToDo [id::keepme0000] [updated::2026-09-29]"), "Buy milk #ToDo [id::keepme0000] [updated::"+stamp()+"]"; got != want {
		t.Fatalf("touchUpdated = %q, want %q", got, want)
	}
}

func TestParseLineExposesID(t *testing.T) {
	task := parseLine("- [ ] Buy milk #ToDo [id::abcdefgh12] [updated::2026-10-01T11:56:10Z]", "L.md", 1)
	if task == nil || task.ID != "abcdefgh12" || task.Title != "Buy milk" {
		t.Fatalf("parsed %+v", task)
	}
}

func TestNowStampIsFullUTCTimestamp(t *testing.T) {
	clock = func() time.Time {
		return time.Date(2026, 10, 1, 12, 56, 10, 461093000, time.FixedZone("BST", 3600))
	}
	defer func() { clock = func() time.Time { return frozen } }()

	if got := nowStamp(); got != "2026-10-01T11:56:10Z" {
		t.Fatalf("nowStamp() = %q", got)
	}
	if got := touchUpdated("Buy milk #ToDo [updated::2026-09-29]"); got != "Buy milk #ToDo [updated::2026-10-01T11:56:10Z]" {
		t.Fatalf("touchUpdated replaced a date-only stamp with %q", got)
	}
}

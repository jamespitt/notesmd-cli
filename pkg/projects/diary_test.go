package projects

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testDay = time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)

func TestAppendDiaryEntryCreatesFile(t *testing.T) {
	vault := t.TempDir()

	out, err := AppendDiaryEntry(vault, "Projects", "Trip", "Big Trip", "First note.", testDay)
	assert.NoError(t, err)

	assert.Contains(t, out, "tags: ProjectDiary")
	assert.Contains(t, out, "# Big Trip Diary")
	assert.Contains(t, out, "### 2026-09-06")
	assert.Contains(t, out, "First note.")

	onDisk, err := os.ReadFile(filepath.Join(vault, "Projects", "Trip", "Diary.md"))
	assert.NoError(t, err)
	assert.Equal(t, out, string(onDisk))
}

func TestAppendDiaryEntrySameDayAppendsUnderHeading(t *testing.T) {
	vault := t.TempDir()
	_, err := AppendDiaryEntry(vault, "Projects", "Trip", "Trip", "Morning thought.", testDay)
	assert.NoError(t, err)
	out, err := AppendDiaryEntry(vault, "Projects", "Trip", "Trip", "Afternoon update.", testDay)
	assert.NoError(t, err)

	// Only one heading for the day, both entries under it, in order.
	assert.Equal(t, 1, countSubstr(out, "### 2026-09-06"))
	assert.Less(t, indexOf(out, "Morning thought."), indexOf(out, "Afternoon update."))
	assert.Less(t, indexOf(out, "### 2026-09-06"), indexOf(out, "Morning thought."))
}

func TestAppendDiaryEntryNewDayGoesAboveOlder(t *testing.T) {
	vault := t.TempDir()
	older := testDay.AddDate(0, 0, -2)
	_, err := AppendDiaryEntry(vault, "Projects", "Trip", "Trip", "Older entry.", older)
	assert.NoError(t, err)
	out, err := AppendDiaryEntry(vault, "Projects", "Trip", "Trip", "Newer entry.", testDay)
	assert.NoError(t, err)

	// Newest day section comes first, and both sit below the title.
	assert.Less(t, indexOf(out, "# Trip Diary"), indexOf(out, "### 2026-09-06"))
	assert.Less(t, indexOf(out, "### 2026-09-06"), indexOf(out, "### 2026-09-04"))
	assert.Less(t, indexOf(out, "Newer entry."), indexOf(out, "Older entry."))
}

func TestAppendDiaryEntryPreservesExistingContent(t *testing.T) {
	vault := t.TempDir()
	existing := "---\ntags: ProjectDiary\n---\n\n# Trip Diary\n\n### 2026-01-01\nHappy new year.\n"
	path := filepath.Join(vault, "Projects", "Trip", "Diary.md")
	writeFile(t, path, existing)

	out, err := AppendDiaryEntry(vault, "Projects", "Trip", "Trip", "Fresh entry.", testDay)
	assert.NoError(t, err)
	assert.Contains(t, out, "Happy new year.")
	assert.Contains(t, out, "### 2026-01-01")
	assert.Less(t, indexOf(out, "Fresh entry."), indexOf(out, "Happy new year."))
}

func TestReadDiaryMissing(t *testing.T) {
	vault := t.TempDir()
	content, exists, err := ReadDiary(vault, "Projects", "Trip")
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, content)
}

func TestAppendDiaryEntryRejectsEmpty(t *testing.T) {
	vault := t.TempDir()
	_, err := AppendDiaryEntry(vault, "Projects", "Trip", "Trip", "   \n  ", testDay)
	assert.Error(t, err)
}

func countSubstr(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

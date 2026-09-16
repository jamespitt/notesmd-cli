package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yakitrak/notesmd-cli/pkg/config"
	"github.com/stretchr/testify/assert"
)

// buildVault lays out a vault whose tasks live in a root file plus a folder,
// alongside a folder that must not be scanned.
func buildVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeVaultFile(t, filepath.Join(dir, "Action Items.md"), "- [ ] Write the PRD\n")
	writeVaultFile(t, filepath.Join(dir, "Projects", "Onsite", "Onsite.md"), "- [ ] Book the room\n")
	writeVaultFile(t, filepath.Join(dir, "wiki", "meetings", "Standup.md"), "- [ ] Meeting noise\n")

	return dir
}

func writeVaultFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func titlesOf(taskList []Task) []string {
	out := make([]string, 0, len(taskList))
	for _, task := range taskList {
		out = append(out, task.Title)
	}
	return out
}

func TestParseFoldersWithSingleFileEntry(t *testing.T) {
	dir := buildVault(t)

	parsed, err := ParseFolders(dir, []string{"Action Items.md", "Projects"})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"Write the PRD", "Book the room"}, titlesOf(parsed))
}

func TestParseFoldersIgnoresMissingEntries(t *testing.T) {
	dir := buildVault(t)

	parsed, err := ParseFolders(dir, []string{"Nope.md", "NoSuchFolder", "Action Items.md"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"Write the PRD"}, titlesOf(parsed))
}

func TestParseFoldersEmptyEntriesWalkWholeVault(t *testing.T) {
	dir := buildVault(t)

	parsed, err := ParseFolders(dir, nil)
	assert.NoError(t, err)
	assert.Len(t, parsed, 3)
}

// A file entry also has to resolve as a list, so tasks can be added to and
// moved into it.
func TestFindListFileMatchesFileEntry(t *testing.T) {
	dir := buildVault(t)
	folders := []string{"Action Items.md", "Projects"}

	path, err := FindListFile(dir, folders, "Action Items")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "Action Items.md"), path)

	_, err = FindListFile(dir, folders, "Standup")
	assert.Error(t, err)
}

// Hidden events are keyed per vault: hiding an event for one vault must leave
// the others untouched, since event ids only mean anything inside their vault.
func TestHiddenEventsAreNamespacedPerVault(t *testing.T) {
	original := config.UserConfigDirectory
	configDir := t.TempDir()
	config.UserConfigDirectory = func() (string, error) { return configDir, nil }
	t.Cleanup(func() { config.UserConfigDirectory = original })

	assert.NoError(t, HideEvent("", "evt-personal", "Dentist"))
	assert.NoError(t, HideEvent("work", "evt-work", "Standup"))

	personal, err := LoadHiddenEvents("")
	assert.NoError(t, err)
	assert.Len(t, personal, 1)
	assert.Equal(t, "evt-personal", personal[0].EventID)

	work, err := LoadHiddenEvents("work")
	assert.NoError(t, err)
	assert.Len(t, work, 1)
	assert.Equal(t, "evt-work", work[0].EventID)

	// Filtering a work task list must not consult the personal hidden list.
	taskList := []Task{{Title: "Dentist", EventID: "evt-personal"}, {Title: "Standup", EventID: "evt-work"}}
	assert.Equal(t, []string{"Dentist"}, titlesOf(FilterHiddenEvents("work", taskList)))
	assert.Equal(t, []string{"Standup"}, titlesOf(FilterHiddenEvents("", taskList)))

	assert.NoError(t, UnhideEvent("work", "evt-work"))
	work, err = LoadHiddenEvents("work")
	assert.NoError(t, err)
	assert.Empty(t, work)
	personal, err = LoadHiddenEvents("")
	assert.NoError(t, err)
	assert.Len(t, personal, 1)
}

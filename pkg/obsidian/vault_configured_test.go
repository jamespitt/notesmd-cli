package obsidian_test

import (
	"os"
	"testing"

	"github.com/Yakitrak/notesmd-cli/mocks"
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/stretchr/testify/assert"
)

// writeCliConfig points the CLI config at a temporary preferences.json holding
// the given JSON.
func writeCliConfig(t *testing.T, content string) {
	t.Helper()
	original := obsidian.CliConfigPath
	t.Cleanup(func() { obsidian.CliConfigPath = original })

	dir, file := mocks.CreateMockCliConfigDirectories(t)
	obsidian.CliConfigPath = func() (string, string, error) { return dir, file, nil }
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredVaults(t *testing.T) {
	t.Run("returns configured vaults in file order", func(t *testing.T) {
		writeCliConfig(t, `{
			"default_vault_name": "personal-notes",
			"vaults": [
				{"id": "personal", "label": "Personal", "path": "/vaults/personal"},
				{"id": "work", "label": "Work", "path": "/vaults/work",
				 "task_folders": ["Action Items.md"], "projects_folder": "Projects"}
			]
		}`)

		vaults, err := obsidian.ConfiguredVaults()
		assert.NoError(t, err)
		assert.Len(t, vaults, 2)
		assert.Equal(t, "personal", vaults[0].ID)
		assert.Equal(t, "Work", vaults[1].DisplayLabel())
		assert.Equal(t, []string{"Action Items.md"}, vaults[1].TaskFolders)
	})

	t.Run("skips entries missing an id or path", func(t *testing.T) {
		writeCliConfig(t, `{"vaults": [
			{"id": "", "path": "/vaults/nameless"},
			{"id": "no-path"},
			{"id": "work", "path": "/vaults/work"}
		]}`)

		vaults, err := obsidian.ConfiguredVaults()
		assert.NoError(t, err)
		assert.Len(t, vaults, 1)
		assert.Equal(t, "work", vaults[0].ID)
	})

	t.Run("no vaults key yields none", func(t *testing.T) {
		writeCliConfig(t, `{"default_vault_name": "personal-notes"}`)

		vaults, err := obsidian.ConfiguredVaults()
		assert.NoError(t, err)
		assert.Empty(t, vaults)
	})
}

func TestConfiguredVaultFolders(t *testing.T) {
	writeCliConfig(t, `{
		"default_task_folders": ["Tasks"],
		"default_projects_folder": "Areas",
		"default_calendar_folder": "Journal/Cal",
		"vaults": [{"id": "work", "path": "/vaults/work", "task_folders": ["Action Items.md"]}]
	}`)

	t.Run("uses its own folders where set", func(t *testing.T) {
		vault := obsidian.NewConfiguredVault(obsidian.VaultConfig{
			ID: "work", Path: "/vaults/work",
			TaskFolders:    []string{"Action Items.md", "Projects"},
			ProjectsFolder: "Projects",
			CalendarFolder: "Journal/Calendar",
		})

		folders, err := vault.TaskFolders()
		assert.NoError(t, err)
		assert.Equal(t, []string{"Action Items.md", "Projects"}, folders)

		projectsFolder, err := vault.ProjectsFolder()
		assert.NoError(t, err)
		assert.Equal(t, "Projects", projectsFolder)

		calendarFolder, err := vault.CalendarFolder()
		assert.NoError(t, err)
		assert.Equal(t, "Journal/Calendar", calendarFolder)
	})

	t.Run("falls back to the global defaults where unset", func(t *testing.T) {
		vault := obsidian.NewConfiguredVault(obsidian.VaultConfig{ID: "personal", Path: "/vaults/personal"})

		folders, err := vault.TaskFolders()
		assert.NoError(t, err)
		assert.Equal(t, []string{"Tasks"}, folders)

		projectsFolder, err := vault.ProjectsFolder()
		assert.NoError(t, err)
		assert.Equal(t, "Areas", projectsFolder)

		calendarFolder, err := vault.CalendarFolder()
		assert.NoError(t, err)
		assert.Equal(t, "Journal/Cal", calendarFolder)
	})
}

// An absolute path needs no Obsidian config, and its name is the directory's.
func TestConfiguredVaultPathAndName(t *testing.T) {
	vault := obsidian.NewConfiguredVault(obsidian.VaultConfig{ID: "work", Path: "/home/james/src/tm_notes"})

	path, err := vault.Path()
	assert.NoError(t, err)
	assert.Equal(t, "/home/james/src/tm_notes", path)

	name, err := vault.DefaultName()
	assert.NoError(t, err)
	assert.Equal(t, "tm_notes", name)
}

func TestFindConfiguredVault(t *testing.T) {
	writeCliConfig(t, `{"vaults": [{"id": "work", "path": "/vaults/work"}]}`)

	found, ok := obsidian.FindConfiguredVault("work")
	assert.True(t, ok)
	assert.Equal(t, "/vaults/work", found.Path)

	_, ok = obsidian.FindConfiguredVault("personal")
	assert.False(t, ok)
}

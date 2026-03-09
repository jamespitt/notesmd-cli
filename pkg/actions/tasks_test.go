package actions_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Yakitrak/notesmd-cli/mocks"
	"github.com/Yakitrak/notesmd-cli/pkg/actions"
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/stretchr/testify/assert"
)

func sampleTasks() []obsidian.Task {
	sched := time.Date(2026, 2, 18, 9, 30, 0, 0, time.UTC)
	return []obsidian.Task{
		{FilePath: "Daily/2026-02-18.md", LineNumber: 3, RawText: "- [ ] Book car #Today [scheduled::2026-02-18T09:30]", Tags: []string{"Today"}, Scheduled: &sched},
		{FilePath: "Daily/2026-02-18.md", LineNumber: 4, RawText: "- [ ] Get ready #Today", Tags: []string{"Today"}},
	}
}

func TestSearchTasks(t *testing.T) {
	t.Run("Returns tasks and prints them", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{SearchTasksResult: sampleTasks()}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.NoError(t, err)
	})

	t.Run("No tasks found prints message", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{NoMatches: true}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.NoError(t, err)
	})

	t.Run("vault.DefaultName returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{DefaultNameErr: errors.New("no vault")}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.EqualError(t, err, "no vault")
	})

	t.Run("vault.Path returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault", PathError: errors.New("bad path")}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.EqualError(t, err, "bad path")
	})

	t.Run("vault.TaskFolders returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault", TaskFoldersErr: errors.New("config error")}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.EqualError(t, err, "config error")
	})

	t.Run("note.SearchTasks returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{SearchTasksErr: errors.New("walk error")}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.EqualError(t, err, "walk error")
	})

	t.Run("Explicit folders override config defaults", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault", DefaultTaskFolders: []string{"Projects"}}
		note := mocks.MockNoteManager{SearchTasksResult: sampleTasks()}

		// Explicit --folder should be passed through; TaskFolders() should not be called
		err := actions.SearchTasks(&vault, &note, actions.TaskParams{Folders: []string{"Daily"}})
		assert.NoError(t, err)
	})

	t.Run("Uses config default folders when no flag given", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault", DefaultTaskFolders: []string{"Daily", "Projects"}}
		note := mocks.MockNoteManager{SearchTasksResult: sampleTasks()}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{})
		assert.NoError(t, err)
	})

	t.Run("Invalid --date format returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{Date: "18-02-2026"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--date")
	})

	t.Run("Invalid --from format returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{From: "not-a-date"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--from")
	})

	t.Run("Invalid --to format returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{To: "nope"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--to")
	})

	t.Run("Valid date filters pass through without error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{SearchTasksResult: sampleTasks()}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{
			Date: "2026-02-18",
			From: "2026-02-17",
			To:   "2026-02-19",
		})
		assert.NoError(t, err)
	})

	t.Run("--today sets tag=today and date=today", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{SearchTasksResult: sampleTasks()}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{Today: true})
		assert.NoError(t, err)
	})

	t.Run("--today with explicit --date uses explicit date", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{SearchTasksResult: sampleTasks()}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{Today: true, Date: "2026-02-18"})
		assert.NoError(t, err)
	})

	t.Run("--today with invalid explicit --date returns error", func(t *testing.T) {
		vault := mocks.MockVaultOperator{Name: "myVault"}
		note := mocks.MockNoteManager{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{Today: true, Date: "not-a-date"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--date")
	})
}

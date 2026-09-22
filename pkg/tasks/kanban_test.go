package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKanbanStatus(t *testing.T) {
	t.Run("returns the matching tag with original casing", func(t *testing.T) {
		task := Task{Tags: []string{"groceries", "InProgress", "urgent"}}
		assert.Equal(t, "InProgress", KanbanStatus(task))
	})

	t.Run("is case-insensitive", func(t *testing.T) {
		task := Task{Tags: []string{"done"}}
		assert.Equal(t, "Done", KanbanStatus(task))
	})

	t.Run("returns empty string when no kanban tag present", func(t *testing.T) {
		task := Task{Tags: []string{"groceries", "urgent"}}
		assert.Equal(t, "", KanbanStatus(task))
	})
}

func TestFilterKanban(t *testing.T) {
	tasks := []Task{
		{Title: "On the board", Tags: []string{"ToDo"}},
		{Title: "Not on the board", Tags: []string{"groceries"}},
		{Title: "Also on the board", Tags: []string{"urgent", "Done"}, Status: StatusCompleted},
	}
	result := FilterKanban(tasks)
	assert.Len(t, result, 2)
	assert.Equal(t, "On the board", result[0].Title)
	assert.Equal(t, "Also on the board", result[1].Title)
}

func TestSetStatusTag(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		absPath := filepath.Join(dir, "Work.md")
		assert.NoError(t, os.WriteFile(absPath, []byte(content), 0644))
		return absPath
	}

	t.Run("adds a status tag to a task with none", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk [due::2026-08-20]\n")
		assert.NoError(t, SetStatusTag(absPath, 3, "ToDo"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "#ToDo")
		assert.Contains(t, string(content), "[due::2026-08-20]")
	})

	t.Run("moves a task between columns without touching other tags", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #groceries #ToDo [due::2026-08-20]\n")
		assert.NoError(t, SetStatusTag(absPath, 3, "InProgress"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "#groceries")
		assert.Contains(t, text, "#InProgress")
		assert.NotContains(t, text, "#ToDo")
	})

	t.Run("removes the status tag entirely when status is empty", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #ToDo\n")
		assert.NoError(t, SetStatusTag(absPath, 3, ""))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.NotContains(t, string(content), "#ToDo")
	})

	t.Run("is case-insensitive when removing the existing tag", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #todo\n")
		assert.NoError(t, SetStatusTag(absPath, 3, "Done"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "#Done")
		assert.NotContains(t, text, "#todo")
	})

	t.Run("leaves the checkbox status untouched", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [x] Buy milk #Done\n")
		assert.NoError(t, SetStatusTag(absPath, 3, "InProgress"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- [x] Buy milk #InProgress")
	})
}

func TestKanbanStatusInAndFilterKanbanColumns(t *testing.T) {
	columns := []string{"Backlog", "Review", "Done"}

	assert.Equal(t, "Review", KanbanStatusIn(Task{Tags: []string{"urgent", "review"}}, columns))
	assert.Equal(t, "", KanbanStatusIn(Task{Tags: []string{"ToDo"}}, columns))

	tasks := []Task{
		{Title: "custom", Tags: []string{"Backlog"}},
		{Title: "default only", Tags: []string{"ToDo"}},
	}
	result := FilterKanbanColumns(tasks, columns)
	assert.Len(t, result, 1)
	assert.Equal(t, "custom", result[0].Title)
}

func TestValidColumnTag(t *testing.T) {
	assert.True(t, ValidColumnTag("Backlog"))
	assert.True(t, ValidColumnTag("code_review"))
	assert.False(t, ValidColumnTag(""))
	assert.False(t, ValidColumnTag("in review"))
	assert.False(t, ValidColumnTag("a,b"))
}

func TestSetStatusTagIn(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "Work.md")
	assert.NoError(t, os.WriteFile(absPath, []byte("# Work\n\n- [ ] Buy milk #Backlog #urgent\n"), 0644))

	assert.NoError(t, SetStatusTagIn(absPath, 3, "Review", []string{"Backlog", "Review", "Done"}))

	content, err := os.ReadFile(absPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "#urgent #Review")
	assert.NotContains(t, string(content), "#Backlog")
}

package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestEditTask(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		absPath := filepath.Join(dir, "Work.md")
		assert.NoError(t, os.WriteFile(absPath, []byte(content), 0644))
		return absPath
	}

	t.Run("sets fields that were nil before, leaves others untouched", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk [google_id::abc123]\n")
		err := EditTask(absPath, 3, TaskEdit{
			Due:      strPtr("2026-08-20"),
			Priority: strPtr("high"),
		})
		assert.NoError(t, err)

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "[due::2026-08-20]")
		assert.Contains(t, text, "[priority::high]")
		assert.Contains(t, text, "[google_id::abc123]")
	})

	t.Run("omitted fields are left completely unchanged", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk [due::2026-08-20] [priority::high]\n")
		assert.NoError(t, EditTask(absPath, 3, TaskEdit{Priority: strPtr("low")}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "[due::2026-08-20]")
		assert.Contains(t, text, "[priority::low]")
		assert.NotContains(t, text, "priority::high")
	})

	t.Run("empty string clears a field", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk [due::2026-08-20] [priority::high]\n")
		assert.NoError(t, EditTask(absPath, 3, TaskEdit{Due: strPtr("")}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.NotContains(t, text, "due::")
		assert.Contains(t, text, "[priority::high]")
	})

	t.Run("updates title while preserving fields and tags", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #groceries [due::2026-08-20]\n")
		assert.NoError(t, EditTask(absPath, 3, TaskEdit{Title: strPtr("Buy oat milk")}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "Buy oat milk")
		assert.Contains(t, text, "#groceries")
		assert.Contains(t, text, "[due::2026-08-20]")
		assert.NotContains(t, text, "Buy milk\n")
	})

	t.Run("replaces the entire tag set when Tags is provided", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #groceries #ToDo\n")
		newTags := []string{"urgent"}
		assert.NoError(t, EditTask(absPath, 3, TaskEdit{Tags: &newTags}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "#urgent")
		assert.NotContains(t, text, "#groceries")
		assert.NotContains(t, text, "#ToDo")
	})

	t.Run("nil Tags leaves existing tags untouched", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #groceries\n")
		assert.NoError(t, EditTask(absPath, 3, TaskEdit{Title: strPtr("Buy bread")}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "#groceries")
	})

	t.Run("leaves the checkbox status untouched", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [x] Buy milk\n")
		assert.NoError(t, EditTask(absPath, 3, TaskEdit{Priority: strPtr("high")}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- [x] Buy milk [priority::high]")
	})

	t.Run("editing all fields at once", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Old title #old [due::2026-01-01]\n")
		newTags := []string{"new"}
		err := EditTask(absPath, 3, TaskEdit{
			Title:     strPtr("New title"),
			Due:       strPtr("2026-08-20"),
			Scheduled: strPtr("2026-08-19"),
			Priority:  strPtr("high"),
			Repeat:    strPtr("every day"),
			Tags:      &newTags,
		})
		assert.NoError(t, err)

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "New title")
		assert.Contains(t, text, "[due::2026-08-20]")
		assert.Contains(t, text, "[scheduled::2026-08-19]")
		assert.Contains(t, text, "[priority::high]")
		assert.Contains(t, text, "[repeat::every day]")
		assert.Contains(t, text, "#new")
		assert.NotContains(t, text, "#old")
		assert.NotContains(t, text, "Old title")
	})
}

func TestAppendSubtask(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		absPath := filepath.Join(dir, "Work.md")
		assert.NoError(t, os.WriteFile(absPath, []byte(content), 0644))
		return absPath
	}

	t.Run("inserts a subtask indented directly under a parent with no children", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Parent task\n- [ ] Unrelated task\n")
		assert.NoError(t, AppendSubtask(absPath, 3, "Child task"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		lines := strings.Split(string(content), "\n")
		assert.Equal(t, "- [ ] Parent task", lines[2])
		assert.Equal(t, "    - [ ] Child task", lines[3])
		assert.Equal(t, "- [ ] Unrelated task", lines[4])
	})

	t.Run("inserts after existing children, not before them", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Parent task\n    - [ ] First child\n- [ ] Sibling\n")
		assert.NoError(t, AppendSubtask(absPath, 3, "Second child"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		lines := strings.Split(string(content), "\n")
		assert.Equal(t, "- [ ] Parent task", lines[2])
		assert.Equal(t, "    - [ ] First child", lines[3])
		assert.Equal(t, "    - [ ] Second child", lines[4])
		assert.Equal(t, "- [ ] Sibling", lines[5])
	})

	t.Run("inserts after a grandchild, one level under the direct parent", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Parent task\n    - [ ] Child\n        - [ ] Grandchild\n- [ ] Sibling\n")
		assert.NoError(t, AppendSubtask(absPath, 3, "Second child"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		lines := strings.Split(string(content), "\n")
		assert.Equal(t, "        - [ ] Grandchild", lines[4])
		assert.Equal(t, "    - [ ] Second child", lines[5])
		assert.Equal(t, "- [ ] Sibling", lines[6])
	})

	t.Run("is a no-op when parentLine is out of range", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Parent task\n")
		assert.NoError(t, AppendSubtask(absPath, 99, "Child task"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.NotContains(t, string(content), "Child task")
	})

	t.Run("is a no-op when parentLine isn't a task line", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Parent task\n")
		assert.NoError(t, AppendSubtask(absPath, 1, "Child task"))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.NotContains(t, string(content), "Child task")
	})
}

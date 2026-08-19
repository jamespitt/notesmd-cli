package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetTags(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		absPath := filepath.Join(dir, "Work.md")
		assert.NoError(t, os.WriteFile(absPath, []byte(content), 0644))
		return absPath
	}

	t.Run("adds tags to a task with none", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk [due::2026-08-20]\n")
		assert.NoError(t, SetTags(absPath, 3, []string{"groceries", "urgent"}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "#groceries")
		assert.Contains(t, text, "#urgent")
		assert.Contains(t, text, "[due::2026-08-20]")
	})

	t.Run("replaces the entire existing tag set", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #groceries #ToDo\n")
		assert.NoError(t, SetTags(absPath, 3, []string{"urgent"}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.Contains(t, text, "#urgent")
		assert.NotContains(t, text, "#groceries")
		assert.NotContains(t, text, "#ToDo")
	})

	t.Run("removes all tags when given an empty slice", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk #groceries #urgent\n")
		assert.NoError(t, SetTags(absPath, 3, []string{}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		lines := strings.Split(string(content), "\n")
		assert.Equal(t, "- [ ] Buy milk", lines[2])
	})

	t.Run("preserves tag order", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk\n")
		assert.NoError(t, SetTags(absPath, 3, []string{"b", "a", "c"}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		text := string(content)
		assert.True(t, strings.Index(text, "#b") < strings.Index(text, "#a"))
		assert.True(t, strings.Index(text, "#a") < strings.Index(text, "#c"))
	})

	t.Run("leaves the checkbox status and title untouched", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [x] Buy milk #ToDo\n")
		assert.NoError(t, SetTags(absPath, 3, []string{"Done"}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- [x] Buy milk #Done")
	})

	t.Run("ignores blank tags", func(t *testing.T) {
		absPath := writeFile(t, "# Work\n\n- [ ] Buy milk\n")
		assert.NoError(t, SetTags(absPath, 3, []string{"", "groceries", "  "}))

		content, err := os.ReadFile(absPath)
		assert.NoError(t, err)
		lines := strings.Split(string(content), "\n")
		assert.Equal(t, "- [ ] Buy milk #groceries", lines[2])
	})
}

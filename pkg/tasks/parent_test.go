package tasks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParentID(t *testing.T) {
	writeVault := func(t *testing.T, content string) string {
		t.Helper()
		vaultPath := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(vaultPath, "Work.md"), []byte(content), 0644))
		return vaultPath
	}

	byTitle := func(tasks []Task, title string) Task {
		for _, tk := range tasks {
			if tk.Title == title {
				return tk
			}
		}
		return Task{}
	}

	t.Run("top-level tasks have no parent", func(t *testing.T) {
		vaultPath := writeVault(t, "# Work\n\n- [ ] Parent task\n- [ ] Another parent\n")
		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)
		assert.Empty(t, byTitle(tasks, "Parent task").ParentID)
		assert.Empty(t, byTitle(tasks, "Another parent").ParentID)
	})

	t.Run("a child's ParentID matches the parent's file_path:line_num", func(t *testing.T) {
		vaultPath := writeVault(t, "# Work\n\n- [ ] Parent task\n    - [ ] Child task\n")
		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)

		parent := byTitle(tasks, "Parent task")
		child := byTitle(tasks, "Child task")
		assert.Equal(t, 3, parent.LineNum)
		assert.Equal(t, "Work.md:3", child.ParentID)
	})

	t.Run("a grandchild points at its direct parent, not the grandparent", func(t *testing.T) {
		vaultPath := writeVault(t, "# Work\n\n- [ ] Grandparent\n    - [ ] Parent\n        - [ ] Grandchild\n")
		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)

		grandparent := byTitle(tasks, "Grandparent")
		parent := byTitle(tasks, "Parent")
		grandchild := byTitle(tasks, "Grandchild")
		assert.Equal(t, parentID("Work.md", grandparent.LineNum), parent.ParentID)
		assert.Equal(t, parentID("Work.md", parent.LineNum), grandchild.ParentID)
	})

	t.Run("a sibling after a child does not inherit the child's parent", func(t *testing.T) {
		vaultPath := writeVault(t, "# Work\n\n- [ ] Parent task\n    - [ ] Child task\n- [ ] Sibling task\n")
		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)
		assert.Empty(t, byTitle(tasks, "Sibling task").ParentID)
	})

	t.Run("dedenting back to a shallower level closes deeper ancestors", func(t *testing.T) {
		vaultPath := writeVault(t, ""+
			"# Work\n\n"+
			"- [ ] A\n"+
			"    - [ ] B\n"+
			"        - [ ] C\n"+
			"    - [ ] D\n") // D is a sibling of B, not a child of C
		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)

		a := byTitle(tasks, "A")
		b := byTitle(tasks, "B")
		d := byTitle(tasks, "D")
		assert.Equal(t, parentID("Work.md", a.LineNum), b.ParentID)
		assert.Equal(t, parentID("Work.md", a.LineNum), d.ParentID)
	})

	t.Run("parent/child relationships don't cross files", func(t *testing.T) {
		vaultPath := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(vaultPath, "A.md"), []byte("# A\n\n- [ ] Trailing parent\n    - [ ] Child in A\n"), 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(vaultPath, "B.md"), []byte("# B\n\n    - [ ] Indented but parentless\n"), 0644))

		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)
		assert.Contains(t, byTitle(tasks, "Child in A").ParentID, "A.md")
		assert.Empty(t, byTitle(tasks, "Indented but parentless").ParentID)
	})
}

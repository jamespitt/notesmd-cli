package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoveTask(t *testing.T) {
	writeFiles := func(t *testing.T, src, dst string) (string, string) {
		t.Helper()
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "Src.md")
		dstPath := filepath.Join(dir, "Dst.md")
		assert.NoError(t, os.WriteFile(srcPath, []byte(src), 0644))
		assert.NoError(t, os.WriteFile(dstPath, []byte(dst), 0644))
		return srcPath, dstPath
	}

	t.Run("moves a task with no children", func(t *testing.T) {
		srcPath, dstPath := writeFiles(t,
			"# Src\n\n- [ ] Task A\n- [ ] Task B\n",
			"# Dst\n\n- [ ] Existing\n",
		)
		assert.NoError(t, MoveTask(srcPath, 3, dstPath))

		srcContent, _ := os.ReadFile(srcPath)
		dstContent, _ := os.ReadFile(dstPath)
		assert.NotContains(t, string(srcContent), "Task A")
		assert.Contains(t, string(srcContent), "Task B")
		assert.Contains(t, string(dstContent), "Existing")
		assert.Contains(t, string(dstContent), "Task A")
	})

	t.Run("moves a task together with its children", func(t *testing.T) {
		srcPath, dstPath := writeFiles(t,
			"# Src\n\n- [ ] Parent\n    - [ ] Child 1\n    - [ ] Child 2\n- [ ] Unrelated\n",
			"# Dst\n\n- [ ] Existing\n",
		)
		assert.NoError(t, MoveTask(srcPath, 3, dstPath))

		srcContent, _ := os.ReadFile(srcPath)
		dstContent, _ := os.ReadFile(dstPath)

		srcText := string(srcContent)
		assert.NotContains(t, srcText, "Parent")
		assert.NotContains(t, srcText, "Child 1")
		assert.NotContains(t, srcText, "Child 2")
		assert.Contains(t, srcText, "Unrelated")

		dstLines := strings.Split(string(dstContent), "\n")
		assert.Contains(t, dstLines, "- [ ] Parent")
		assert.Contains(t, dstLines, "    - [ ] Child 1")
		assert.Contains(t, dstLines, "    - [ ] Child 2")
	})

	t.Run("moved children keep their relative indentation and reparse correctly", func(t *testing.T) {
		srcPath, dstPath := writeFiles(t,
			"# Src\n\n- [ ] Parent\n    - [ ] Child\n        - [ ] Grandchild\n",
			"# Dst\n\n",
		)
		assert.NoError(t, MoveTask(srcPath, 3, dstPath))

		vaultPath := filepath.Dir(dstPath)
		tasks, err := ParseFolders(vaultPath, nil)
		assert.NoError(t, err)

		byTitle := func(title string) Task {
			for _, tk := range tasks {
				if tk.Title == title {
					return tk
				}
			}
			return Task{}
		}
		parent := byTitle("Parent")
		child := byTitle("Child")
		grandchild := byTitle("Grandchild")
		assert.Equal(t, "Dst.md", parent.FilePath)
		assert.Empty(t, parent.ParentID)
		assert.Equal(t, parentID("Dst.md", parent.LineNum), child.ParentID)
		assert.Equal(t, parentID("Dst.md", child.LineNum), grandchild.ParentID)
	})

	t.Run("does not sweep up a following sibling", func(t *testing.T) {
		srcPath, dstPath := writeFiles(t,
			"# Src\n\n- [ ] Parent\n    - [ ] Child\n- [ ] Sibling\n",
			"# Dst\n\n",
		)
		assert.NoError(t, MoveTask(srcPath, 3, dstPath))

		srcContent, _ := os.ReadFile(srcPath)
		dstContent, _ := os.ReadFile(dstPath)
		assert.Contains(t, string(srcContent), "Sibling")
		assert.NotContains(t, string(dstContent), "Sibling")
	})

	t.Run("moving a child on its own does not take its own children beyond it", func(t *testing.T) {
		srcPath, dstPath := writeFiles(t,
			"# Src\n\n- [ ] Parent\n    - [ ] Child\n        - [ ] Grandchild\n    - [ ] Other child\n",
			"# Dst\n\n",
		)
		// Move "Child" (line 4), which should bring "Grandchild" (its own
		// child) but leave "Other child" (Parent's other child) behind.
		assert.NoError(t, MoveTask(srcPath, 4, dstPath))

		srcContent, _ := os.ReadFile(srcPath)
		dstContent, _ := os.ReadFile(dstPath)
		srcText := string(srcContent)
		dstText := string(dstContent)

		assert.Contains(t, srcText, "Parent")
		assert.Contains(t, srcText, "Other child")
		assert.NotContains(t, srcText, "Grandchild")

		assert.Contains(t, dstText, "Child")
		assert.Contains(t, dstText, "Grandchild")
		assert.NotContains(t, dstText, "Other child")
	})

	t.Run("is a no-op when lineNum is out of range", func(t *testing.T) {
		srcPath, dstPath := writeFiles(t, "# Src\n\n- [ ] Task A\n", "# Dst\n\n")
		assert.NoError(t, MoveTask(srcPath, 99, dstPath))

		srcContent, _ := os.ReadFile(srcPath)
		dstContent, _ := os.ReadFile(dstPath)
		assert.Contains(t, string(srcContent), "Task A")
		assert.NotContains(t, string(dstContent), "Task A")
	})
}

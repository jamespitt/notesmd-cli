package projects

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestListPages(t *testing.T) {
	vault := t.TempDir()
	proj := filepath.Join(vault, "Projects", "Trip")
	writeFile(t, filepath.Join(proj, "Trip.md"), "---\ntags: Project\n---\n# Trip\n")
	writeFile(t, filepath.Join(proj, "Diary.md"), "# Trip Diary\n")
	writeFile(t, filepath.Join(proj, "Notes", "Budget.md"), "# Budget\n")
	writeFile(t, filepath.Join(proj, "image.png"), "not markdown")
	writeFile(t, filepath.Join(proj, ".obsidian", "hidden.md"), "# hidden\n")

	pages, err := ListPages(vault, "Projects", "Trip")
	assert.NoError(t, err)

	var rels []string
	for _, p := range pages {
		rels = append(rels, p.Rel)
	}
	assert.Equal(t, []string{"Diary.md", "Notes/Budget.md", "Trip.md"}, rels)

	assert.Equal(t, "Projects/Trip/Notes/Budget.md", pages[1].Path)
	assert.Equal(t, "Notes/Budget", pages[1].Name)
}

func TestListPagesMissingProject(t *testing.T) {
	vault := t.TempDir()
	_, err := ListPages(vault, "Projects", "Nope")
	assert.Error(t, err)
}

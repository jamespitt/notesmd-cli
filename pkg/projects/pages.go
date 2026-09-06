package projects

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Page is a single markdown file that belongs to a project directory.
type Page struct {
	Path string `json:"path"` // vault-relative, e.g. "Projects/Foo/Notes/Budget.md"
	Name string `json:"name"` // display name, e.g. "Notes/Budget"
	Rel  string `json:"rel"`  // path relative to the project directory, e.g. "Notes/Budget.md"
}

// ListPages returns every .md file within the named project's directory,
// recursing into subfolders. Dot-directories are skipped. Results are sorted
// by relative path.
func ListPages(vaultPath, projectsFolder, projectName string) ([]Page, error) {
	projectRel := filepath.Join(projectsFolder, projectName)
	projectAbs := filepath.Join(vaultPath, projectRel)

	if info, err := os.Stat(projectAbs); err != nil || !info.IsDir() {
		if err != nil {
			return nil, err
		}
		return []Page{}, nil
	}

	var pages []Page
	err := filepath.WalkDir(projectAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != projectAbs && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(projectAbs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		pages = append(pages, Page{
			Path: filepath.ToSlash(filepath.Join(projectRel, rel)),
			Name: strings.TrimSuffix(rel, ".md"),
			Rel:  rel,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(pages, func(i, j int) bool { return pages[i].Rel < pages[j].Rel })
	if pages == nil {
		pages = []Page{}
	}
	return pages, nil
}

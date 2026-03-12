// Package projects provides parsing and querying of Obsidian project notes.
package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/frontmatter"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
)

// Project represents a parsed Obsidian project directory.
type Project struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Status   string `json:"status,omitempty"`
	Deadline string `json:"deadline,omitempty"`
	Goal     string `json:"goal,omitempty"`
	DirPath  string `json:"dir_path"` // vault-relative path, e.g. "Projects/Center Parcs Trip"
}

// ParseProjects scans projectsFolder within vaultPath for project subdirectories.
// A valid project must have a main .md file with the same name as the directory
// and frontmatter containing `tags: Project`.
func ParseProjects(vaultPath, projectsFolder string) ([]Project, error) {
	rootPath := filepath.Join(vaultPath, projectsFolder)

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	var projects []Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		mainFile := filepath.Join(rootPath, dirName, dirName+".md")

		content, err := os.ReadFile(mainFile)
		if err != nil {
			continue // no canonical main file — skip
		}

		fm, _, _ := frontmatter.Parse(string(content))

		if !hasProjectTag(fm) {
			continue
		}

		proj := Project{
			Name:    dirName,
			DirPath: filepath.Join(projectsFolder, dirName),
			Title:   dirName, // fallback
		}

		if v, ok := fm["title"].(string); ok && v != "" {
			proj.Title = v
		}
		if v, ok := fm["status"].(string); ok {
			proj.Status = v
		}
		if v, ok := fm["deadline"].(string); ok {
			proj.Deadline = v
		}
		if v, ok := fm["goal"].(string); ok {
			proj.Goal = v
		}

		projects = append(projects, proj)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

// GetProjectTasks returns all tasks relevant to the named project:
//  1. All tasks from .md files within the project directory.
//  2. Tasks from the configured task folders whose title contains [[projectName]].
//
// Results are deduplicated by file path + line number.
func GetProjectTasks(vaultPath, projectsFolder, projectName string, taskFolders []string) ([]tasks.Task, error) {
	projectAbsDir := filepath.Join(vaultPath, projectsFolder, projectName)

	seen := make(map[string]struct{})
	var result []tasks.Task

	add := func(t tasks.Task) {
		key := fmt.Sprintf("%s:%d", t.FilePath, t.LineNum)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, t)
		}
	}

	// 1. Tasks from the project directory itself
	projectTasks, err := tasks.ParseDir(vaultPath, projectAbsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, t := range projectTasks {
		add(t)
	}

	// 2. Tasks from task folders that contain [[projectName]] in the title
	taskFolderTasks, err := tasks.ParseFolders(vaultPath, taskFolders)
	if err != nil {
		return nil, err
	}
	wikilinkRe := regexp.MustCompile(`(?i)\[\[` + regexp.QuoteMeta(projectName) + `\]\]`)
	for _, t := range taskFolderTasks {
		if wikilinkRe.MatchString(t.Title) {
			add(t)
		}
	}

	return result, nil
}

// hasProjectTag checks whether the frontmatter "tags" field contains "Project".
func hasProjectTag(fm map[string]interface{}) bool {
	raw, ok := fm["tags"]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "project")
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.EqualFold(strings.TrimSpace(s), "project") {
				return true
			}
		}
	}
	return false
}

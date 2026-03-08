package obsidian

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AddMdSuffix adds .md extension if not already present.
func AddMdSuffix(s string) string {
	if strings.HasSuffix(s, ".md") {
		return s
	}
	return s + ".md"
}

// RemoveMdSuffix removes .md extension if present.
func RemoveMdSuffix(s string) string {
	return strings.TrimSuffix(s, ".md")
}

// ShouldSkipDirectoryOrFile returns true for directories, hidden files, and non-.md files.
func ShouldSkipDirectoryOrFile(info os.FileInfo) bool {
	if info.IsDir() {
		return true
	}
	name := info.Name()
	if strings.HasPrefix(name, ".") {
		return true
	}
	return !strings.HasSuffix(name, ".md")
}

// GenerateNoteLinkTexts returns the three wikilink prefix patterns for a note name.
func GenerateNoteLinkTexts(noteName string) [3]string {
	base := RemoveMdSuffix(filepath.Base(noteName))
	return [3]string{
		"[[" + base + "]]",
		"[[" + base + "|",
		"[[" + base + "#",
	}
}

// GenerateLinkReplacements returns a map of old→new string replacements for updating links.
func GenerateLinkReplacements(oldName, newName string) map[string]string {
	oldBase := RemoveMdSuffix(filepath.Base(oldName))
	newBase := RemoveMdSuffix(filepath.Base(newName))
	oldPath := RemoveMdSuffix(oldName)
	newPath := RemoveMdSuffix(newName)

	r := map[string]string{
		// Basename wikilinks
		"[[" + oldBase + "]]": "[[" + newBase + "]]",
		"[[" + oldBase + "|":  "[[" + newBase + "|",
		"[[" + oldBase + "#":  "[[" + newBase + "#",
		// Basename markdown links
		"](" + oldBase + ".md)":   "](" + newBase + ".md)",
		"](" + oldBase + ")":      "](" + newBase + ")",
		"](./" + oldBase + ".md)": "](./" + newBase + ".md)",
		"](./" + oldBase + ")":    "](./" + newBase + ")",
	}

	// Path-based patterns (only when name includes a directory)
	if oldPath != oldBase {
		r["[["+oldPath+"]]"] = "[[" + newPath + "]]"
		r["[["+oldPath+"|"] = "[[" + newPath + "|"
		r["[["+oldPath+"#"] = "[[" + newPath + "#"
		r["]("+oldPath+".md)"] = "](" + newPath + ".md)"
		r["]("+oldPath+")"] = "](" + newPath + ")"
		r["](./" + oldPath + ".md)"] = "](./" + newPath + ".md)"
		r["](./" + oldPath + ")"] = "](./" + newPath + ")"
	}

	return r
}

// ReplaceContent applies old→new string replacements to content.
func ReplaceContent(content []byte, replacements map[string]string) []byte {
	result := content
	for old, new := range replacements {
		result = bytes.ReplaceAll(result, []byte(old), []byte(new))
	}
	return result
}

// GenerateBacklinkSearchPatterns returns patterns to find links to a note.
func GenerateBacklinkSearchPatterns(noteName string) []string {
	base := RemoveMdSuffix(filepath.Base(noteName))
	return []string{
		"[[" + base + "]]",
		"[[" + base + "|",
		"[[" + base + "#",
		"(" + base + ".md)",
	}
}

func normalizePathSeparators(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

var guiEditors = map[string]bool{
	"code": true,
	"subl": true,
	"atom": true,
	"mate": true,
}

// OpenInEditor opens the given file path in the user's configured editor.
func OpenInEditor(filePath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	parts := strings.Fields(editor)
	cmd := parts[0]
	args := append([]string{}, parts[1:]...)

	if guiEditors[filepath.Base(cmd)] {
		args = append(args, "--wait")
	}

	args = append(args, filePath)
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("editor command failed: %w", err)
	}
	return nil
}

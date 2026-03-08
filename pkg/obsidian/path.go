package obsidian

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// ValidatePath resolves notePath relative to vaultPath and verifies it stays
// within the vault. Returns the absolute path or an error.
func ValidatePath(vaultPath, notePath string) (string, error) {
	var fullPath string
	if filepath.IsAbs(notePath) {
		fullPath = notePath
	} else {
		fullPath = filepath.Join(vaultPath, notePath)
	}

	rel, err := filepath.Rel(vaultPath, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New(VaultAccessError)
	}

	return fullPath, nil
}

// ListEntries returns relative paths of all .md files under vaultPath/subPath.
// If subPath is empty, the entire vault is listed.
func ListEntries(vaultPath, subPath string) ([]string, error) {
	searchPath := vaultPath
	if subPath != "" {
		searchPath = filepath.Join(vaultPath, subPath)
	}

	var notes []string
	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			relPath, err := filepath.Rel(vaultPath, path)
			if err != nil {
				return err
			}
			notes = append(notes, relPath)
		}
		return nil
	})
	return notes, err
}

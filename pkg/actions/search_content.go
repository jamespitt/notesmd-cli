package actions

import (
	"fmt"
	"path/filepath"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

func SearchNotesContent(vault obsidian.VaultManager, note obsidian.NoteManager, uri obsidian.UriManager, fuzzyFinder obsidian.FuzzyFinderManager, query string, useEditor bool) error {
	vaultName, err := vault.DefaultName()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return err
	}

	matches, err := note.SearchNotesWithSnippets(vaultPath, query)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	// If single match and using editor, open directly
	if len(matches) == 1 && useEditor {
		filePath := filepath.Join(vaultPath, matches[0].FilePath)
		return obsidian.OpenInEditor(filePath)
	}

	displayItems := make([]string, len(matches))
	for i, m := range matches {
		displayItems[i] = fmt.Sprintf("%s:%d: %s", m.FilePath, m.LineNumber, m.MatchLine)
	}

	index, err := fuzzyFinder.Find(displayItems, func(i int) string {
		return displayItems[i]
	})
	if err != nil {
		return err
	}

	selected := matches[index]

	if useEditor {
		filePath := filepath.Join(vaultPath, selected.FilePath)
		return obsidian.OpenInEditor(filePath)
	}

	obsidianUri := uri.Construct(ObsOpenUrl, map[string]string{
		"file":  selected.FilePath,
		"vault": vaultName,
	})
	return uri.Execute(obsidianUri)
}

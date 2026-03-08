package actions

import (
	"fmt"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

type PrintParams struct {
	NoteName        string
	IncludeMentions bool
}

func PrintNote(vault obsidian.VaultManager, note obsidian.NoteManager, params PrintParams) (string, error) {
	_, err := vault.DefaultName()
	if err != nil {
		return "", err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return "", err
	}

	contents, err := note.GetContents(vaultPath, params.NoteName)
	if err != nil {
		return "", err
	}

	if params.IncludeMentions {
		backlinks, err := note.FindBacklinks(vaultPath, params.NoteName)
		if err != nil {
			return "", err
		}
		if len(backlinks) > 0 {
			contents += "\n\n## Linked Mentions\n"
			for _, bl := range backlinks {
				contents += fmt.Sprintf("- [[%s]] (line %d): %s\n", bl.FilePath, bl.LineNumber, bl.MatchLine)
			}
		}
	}

	return contents, nil
}

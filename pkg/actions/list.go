package actions

import (
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

func ListNotes(vault obsidian.VaultManager, note obsidian.NoteManager) ([]string, error) {
	_, err := vault.DefaultName()
	if err != nil {
		return nil, err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return nil, err
	}

	return note.GetNotesList(vaultPath)
}

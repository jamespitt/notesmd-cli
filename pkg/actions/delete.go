package actions

import (
	"path/filepath"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

type DeleteParams struct {
	NoteName string
}

func DeleteNote(vault obsidian.VaultManager, note obsidian.NoteManager, params DeleteParams) error {
	_, err := vault.DefaultName()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return err
	}

	fullPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(params.NoteName))
	return note.Delete(fullPath)
}

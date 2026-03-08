package actions

import (
	"path/filepath"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

type MoveParams struct {
	CurrentNoteName string
	NewNoteName     string
}

func MoveNote(vault obsidian.VaultManager, note obsidian.NoteManager, params MoveParams) error {
	_, err := vault.DefaultName()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return err
	}

	oldPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(params.CurrentNoteName))
	newPath := filepath.Join(vaultPath, obsidian.AddMdSuffix(params.NewNoteName))

	if err := note.Move(oldPath, newPath); err != nil {
		return err
	}

	return note.UpdateLinks(vaultPath, params.CurrentNoteName, params.NewNoteName)
}

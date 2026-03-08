package actions

import (
	"path/filepath"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

type OpenParams struct {
	NoteName  string
	Section   string
	UseEditor bool
}

func OpenNote(vault obsidian.VaultManager, uri obsidian.UriManager, params OpenParams) error {
	vaultName, err := vault.DefaultName()
	if err != nil {
		return err
	}

	if params.UseEditor {
		vaultPath, err := vault.Path()
		if err != nil {
			return err
		}
		noteName := obsidian.AddMdSuffix(params.NoteName)
		fullPath := filepath.Join(vaultPath, noteName)
		return obsidian.OpenInEditor(fullPath)
	}

	file := params.NoteName
	if params.Section != "" {
		file = params.NoteName + "#" + params.Section
	}

	obsidianUri := uri.Construct(ObsOpenUrl, map[string]string{
		"file":  file,
		"vault": vaultName,
	})
	return uri.Execute(obsidianUri)
}

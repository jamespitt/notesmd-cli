package actions

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

type DailyParams struct {
	UseEditor bool
}

func DailyNote(vault obsidian.VaultManager, uri obsidian.UriManager, params DailyParams) error {
	vaultName, err := vault.DefaultName()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return err
	}

	cfg := obsidian.ReadDailyNotesConfig(vaultPath)

	format := cfg.Format
	if format == "" {
		format = "YYYY-MM-DD"
	}
	goFormat := obsidian.MomentToGoFormat(format)
	noteName := time.Now().Format(goFormat)

	folder := cfg.Folder
	noteRelPath := noteName + ".md"
	if folder != "" {
		noteRelPath = filepath.Join(folder, noteRelPath)
	}

	fullPath := filepath.Join(vaultPath, noteRelPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	// Only create if it doesn't exist
	if !fileExists(fullPath) {
		content := ""
		if cfg.Template != "" {
			templatePath := filepath.Join(vaultPath, obsidian.AddMdSuffix(cfg.Template))
			if data, err := os.ReadFile(templatePath); err == nil {
				content = string(data)
			}
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	if params.UseEditor {
		return obsidian.OpenInEditor(fullPath)
	}

	obsidianUri := uri.Construct(ObsOpenUrl, map[string]string{
		"file":  noteName,
		"vault": vaultName,
	})
	return uri.Execute(obsidianUri)
}

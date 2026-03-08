package actions

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

type CreateParams struct {
	NoteName        string
	Content         string
	ShouldOverwrite bool
	ShouldAppend    bool
	ShouldOpen      bool
	UseEditor       bool
}

func CreateNote(vault obsidian.VaultManager, uri obsidian.UriManager, params CreateParams) error {
	vaultName, err := vault.DefaultName()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return err
	}

	noteName := obsidian.ApplyDefaultFolder(params.NoteName, vaultPath)
	noteName = obsidian.AddMdSuffix(noteName)
	fullPath := filepath.Join(vaultPath, noteName)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	content := NormalizeContent(params.Content)

	if params.ShouldAppend {
		f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(content)
		if err != nil {
			return err
		}
	} else if params.ShouldOverwrite || !fileExists(fullPath) {
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	if params.ShouldOpen {
		if params.UseEditor {
			return obsidian.OpenInEditor(fullPath)
		}
		obsidianUri := uri.Construct(ObsOpenUrl, map[string]string{
			"file":  noteName,
			"vault": vaultName,
		})
		return uri.Execute(obsidianUri)
	}

	return nil
}

// NormalizeContent converts escape sequences like \n, \t into actual characters.
func NormalizeContent(input string) string {
	replacer := strings.NewReplacer(
		`\n`, "\n",
		`\t`, "\t",
		`\r`, "\r",
		`\"`, "\"",
		`\'`, "'",
		`\\`, "\\",
	)
	return replacer.Replace(input)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

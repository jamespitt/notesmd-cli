package obsidian

import (
	"encoding/json"
	"os"
)

func (v *Vault) TaskFolders() ([]string, error) {
	_, cliConfigFile, err := CliConfigPath()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(cliConfigFile)
	if err != nil {
		// Config file doesn't exist yet — no default folders configured
		return []string{}, nil
	}

	cliConfig := CliConfig{}
	if err := json.Unmarshal(content, &cliConfig); err != nil {
		return []string{}, nil
	}

	if cliConfig.DefaultTaskFolders == nil {
		return []string{}, nil
	}

	return cliConfig.DefaultTaskFolders, nil
}

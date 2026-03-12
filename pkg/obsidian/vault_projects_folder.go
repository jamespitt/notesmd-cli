package obsidian

import (
	"encoding/json"
	"os"
)

const defaultProjectsFolder = "Projects"

// ProjectsFolder returns the configured projects folder, defaulting to "Projects".
func (v *Vault) ProjectsFolder() (string, error) {
	_, cliConfigFile, err := CliConfigPath()
	if err != nil {
		return defaultProjectsFolder, nil //nolint:nilerr
	}

	content, err := os.ReadFile(cliConfigFile)
	if err != nil {
		return defaultProjectsFolder, nil //nolint:nilerr
	}

	cliConfig := CliConfig{}
	if err := json.Unmarshal(content, &cliConfig); err != nil {
		return defaultProjectsFolder, nil //nolint:nilerr
	}

	if cliConfig.DefaultProjectsFolder != "" {
		return cliConfig.DefaultProjectsFolder, nil
	}

	return defaultProjectsFolder, nil
}

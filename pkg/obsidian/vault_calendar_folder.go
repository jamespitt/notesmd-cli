package obsidian

import (
	"encoding/json"
	"os"
)

const defaultCalendarFolder = "Journal/Calendar"

// CalendarFolder returns the configured calendar folder, defaulting to "Journal/Calendar".
func (v *Vault) CalendarFolder() (string, error) {
	_, cliConfigFile, err := CliConfigPath()
	if err != nil {
		return defaultCalendarFolder, nil //nolint:nilerr
	}

	content, err := os.ReadFile(cliConfigFile)
	if err != nil {
		return defaultCalendarFolder, nil //nolint:nilerr
	}

	cliConfig := CliConfig{}
	if err := json.Unmarshal(content, &cliConfig); err != nil {
		return defaultCalendarFolder, nil //nolint:nilerr
	}

	if cliConfig.DefaultCalendarFolder != "" {
		return cliConfig.DefaultCalendarFolder, nil
	}

	return defaultCalendarFolder, nil
}

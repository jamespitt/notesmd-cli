package mocks

import (
	"os"
	"path/filepath"
	"testing"
)

// CreateMockCliConfigDirectories creates a temp config directory and returns
// (configDir, configFilePath). The file is not created — tests can write to it.
func CreateMockCliConfigDirectories(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "notesmd-cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(configDir, "config.json")
	return configDir, configFile
}

// CreateMockObsidianConfigFile creates a temp directory with an obsidian.json
// placeholder and returns the file path. The file is not written — tests can write to it.
func CreateMockObsidianConfigFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "obsidian")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(configDir, "obsidian.json")
}

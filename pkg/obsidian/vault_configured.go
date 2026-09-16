package obsidian

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ReadCliConfig reads preferences.json. A missing file is not an error - it
// yields a zero CliConfig, same as the individual Default* getters do.
func ReadCliConfig() (CliConfig, error) {
	cliConfig := CliConfig{}

	_, cliConfigFile, err := CliConfigPath()
	if err != nil {
		return cliConfig, err
	}

	content, err := os.ReadFile(cliConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return cliConfig, nil
		}
		return cliConfig, errors.New(ObsidianCLIConfigReadError)
	}

	if err := json.Unmarshal(content, &cliConfig); err != nil {
		return cliConfig, errors.New(ObsidianCLIConfigParseError)
	}

	return cliConfig, nil
}

// ConfiguredVaults returns the vaults declared under "vaults" in
// preferences.json, in file order (the first is the default one). Entries
// missing an id or a path are skipped rather than failing the whole read, so
// one bad line can't take the server down.
func ConfiguredVaults() ([]VaultConfig, error) {
	cliConfig, err := ReadCliConfig()
	if err != nil {
		return nil, err
	}

	var out []VaultConfig
	for _, v := range cliConfig.Vaults {
		if v.ID == "" || v.Path == "" {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// FindConfiguredVault looks up a vault by id (case-sensitive) among the
// configured vaults.
func FindConfiguredVault(id string) (VaultConfig, bool) {
	vaults, err := ConfiguredVaults()
	if err != nil {
		return VaultConfig{}, false
	}
	for _, v := range vaults {
		if v.ID == id {
			return v, true
		}
	}
	return VaultConfig{}, false
}

// ConfiguredVault adapts one VaultConfig entry to VaultManager, so a server
// handler can work against any configured vault through the same interface it
// already uses for the single default one.
//
// Folder fields left empty in the entry fall back to the global Default*
// preferences, then to the built-in defaults - that's what lets the existing
// single-vault config keep its behaviour once its vault is also listed under
// "vaults".
type ConfiguredVault struct {
	Config VaultConfig
}

// NewConfiguredVault returns a VaultManager for the given vault entry.
func NewConfiguredVault(config VaultConfig) *ConfiguredVault {
	return &ConfiguredVault{Config: config}
}

// DefaultName returns a display name for the vault: the Obsidian vault name
// when the entry is configured by name, or the directory's base name when it's
// an absolute path. Only used for `obsidian://` URIs (never built by the
// server) and error messages.
func (v *ConfiguredVault) DefaultName() (string, error) {
	if v.Config.Path == "" {
		return "", fmt.Errorf("vault %q has no path configured", v.Config.ID)
	}
	if filepath.IsAbs(v.Config.Path) {
		return filepath.Base(filepath.Clean(v.Config.Path)), nil
	}
	return v.Config.Path, nil
}

// SetDefaultName is not supported on a configured vault - the default vault is
// the first entry under "vaults", changed by editing preferences.json.
func (v *ConfiguredVault) SetDefaultName(string) error {
	return errors.New("cannot set the default vault name on a configured vault")
}

// Path resolves the entry's path the same way the default vault does: an
// absolute path is used as-is, a bare name is looked up in Obsidian's config.
func (v *ConfiguredVault) Path() (string, error) {
	return (&Vault{Name: v.Config.Path}).Path()
}

func (v *ConfiguredVault) DefaultOpenType() (string, error) {
	return (&Vault{}).DefaultOpenType()
}

func (v *ConfiguredVault) TaskFolders() ([]string, error) {
	if v.Config.TaskFolders != nil {
		return v.Config.TaskFolders, nil
	}
	return (&Vault{}).TaskFolders()
}

func (v *ConfiguredVault) ProjectsFolder() (string, error) {
	if v.Config.ProjectsFolder != "" {
		return v.Config.ProjectsFolder, nil
	}
	return (&Vault{}).ProjectsFolder()
}

func (v *ConfiguredVault) CalendarFolder() (string, error) {
	if v.Config.CalendarFolder != "" {
		return v.Config.CalendarFolder, nil
	}
	return (&Vault{}).CalendarFolder()
}

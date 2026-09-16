package obsidian

// CliConfig is the on-disk shape of ~/.config/notesmd-cli/preferences.json.
//
// The Default* fields describe a single vault (the original single-vault
// setup). Vaults, when non-empty, adds named vaults that clients can switch
// between at request time - see VaultConfig and the server's /api/vaults.
// A vault entry that omits a folder field falls back to the matching
// Default* value, so an existing single-vault config keeps working when its
// vault is also listed in Vaults.
type CliConfig struct {
	DefaultVaultName      string        `json:"default_vault_name"`
	DefaultOpenType       string        `json:"default_open_type,omitempty"`
	DefaultTaskFolders    []string      `json:"default_task_folders,omitempty"`
	DefaultProjectsFolder string        `json:"default_projects_folder,omitempty"`
	DefaultCalendarFolder string        `json:"default_calendar_folder,omitempty"`
	Vaults                []VaultConfig `json:"vaults,omitempty"`
}

// VaultConfig is one switchable vault: an id clients pass as ?vault=<id>,
// a human label for pickers, the vault itself (an Obsidian vault name or an
// absolute path), and the per-vault folder layout. Work and personal vaults
// rarely agree on where tasks live, so each entry carries its own folders
// rather than sharing the global Default* ones.
type VaultConfig struct {
	ID             string   `json:"id"`
	Label          string   `json:"label,omitempty"`
	Path           string   `json:"path"`
	TaskFolders    []string `json:"task_folders,omitempty"`
	ProjectsFolder string   `json:"projects_folder,omitempty"`
	CalendarFolder string   `json:"calendar_folder,omitempty"`
}

// DisplayLabel is the label to show in a vault picker, falling back to the id.
func (c VaultConfig) DisplayLabel() string {
	if c.Label != "" {
		return c.Label
	}
	return c.ID
}

type ObsidianVaultConfig struct {
	Vaults map[string]struct {
		Path string `json:"path"`
	} `json:"vaults"`
}

type VaultManager interface {
	DefaultName() (string, error)
	SetDefaultName(name string) error
	Path() (string, error)
	DefaultOpenType() (string, error)
	TaskFolders() ([]string, error)
	ProjectsFolder() (string, error)
	CalendarFolder() (string, error)
}

type Vault struct {
	Name string
}

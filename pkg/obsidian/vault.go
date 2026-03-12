package obsidian

type CliConfig struct {
	DefaultVaultName      string   `json:"default_vault_name"`
	DefaultOpenType       string   `json:"default_open_type,omitempty"`
	DefaultTaskFolders    []string `json:"default_task_folders,omitempty"`
	DefaultProjectsFolder string   `json:"default_projects_folder,omitempty"`
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
}

type Vault struct {
	Name string
}

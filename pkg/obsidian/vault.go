package obsidian

const (
	NoteDoesNotExistError              = "note does not exist"
	VaultReadError                     = "error reading vault"
	VaultWriteError                    = "error writing to vault"
	VaultAccessError                   = "error accessing vault"
	ObsidianConfigReadError            = "error reading obsidian config"
	ObsidianConfigParseError           = "error parsing obsidian config"
	ObsidianConfigVaultNotFoundError   = "vault not found in obsidian config"
	ObsidianCLIConfigReadError         = "error reading cli config"
	ObsidianCLIConfigParseError        = "error parsing cli config"
	ObsidianCLIConfigGenerateJSONError = "error generating cli config json"
	ObsidianCLIConfigDirWriteEror      = "error creating cli config directory"
	ObsidianCLIConfigWriteError        = "error writing cli config"
)

// ObsidianVaultConfig represents the structure of obsidian.json.
type ObsidianVaultConfig struct {
	Vaults map[string]VaultEntry `json:"vaults"`
}

type VaultEntry struct {
	Path string `json:"path"`
}

// CliConfig is the structure of the notesmd-cli config file.
type CliConfig struct {
	DefaultVaultName string `json:"default_vault_name"`
	DefaultOpenType  string `json:"default_open_type,omitempty"`
}

// Vault represents an Obsidian vault.
type Vault struct {
	Name string
}

// VaultManager defines the operations available on a vault.
type VaultManager interface {
	DefaultName() (string, error)
	SetDefaultName(string) error
	Path() (string, error)
	DefaultOpenType() (string, error)
	SetDefaultOpenType(string) error
}

// UriManager defines how to construct and execute Obsidian URIs.
type UriManager interface {
	Construct(url string, params map[string]string) string
	Execute(uri string) error
}

// FuzzyFinderManager defines the fuzzy finder interface.
type FuzzyFinderManager interface {
	Find(items []string, itemFunc func(i int) string) (int, error)
}

package cmd

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/server"
	"github.com/spf13/cobra"
)

var (
	servePort   int
	serveVaults []string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an HTTP API server for the vault",
	Long: `Start an HTTP API server for one or more vaults.

With no --vault flag, the vaults listed under "vaults" in
~/.config/notesmd-cli/preferences.json are all served (the first one is the
default); if none are configured, the single default vault is served.

--vault may be repeated to serve a specific set instead. Each value is a
configured vault id, an Obsidian vault name, or an absolute path.

Clients pick a vault per request with ?vault=<id> (or an X-Vault header) and
can discover what's available from GET /api/vaults.`,
	Run: func(cmd *cobra.Command, args []string) {
		vaults, err := resolveServeVaults(serveVaults)
		if err != nil {
			log.Fatal(err)
		}

		srv := server.NewMulti(vaults, &obsidian.Note{})
		addr := fmt.Sprintf(":%d", servePort)

		for i, v := range vaults {
			path, err := v.Manager.Path()
			if err != nil {
				log.Fatalf("vault %q: %v", v.ID, err)
			}
			suffix := ""
			if i == 0 {
				suffix = " (default)"
			}
			log.Printf("serving vault %q [%s] -> %s%s", v.ID, v.Label, path, suffix)
		}

		srv.WarmCache()

		log.Printf("notesmd-cli server listening on %s", addr)
		if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
			log.Fatal(err)
		}
	},
}

// resolveServeVaults decides which vaults `serve` exposes: the ones named by
// --vault if any were given, otherwise everything under "vaults" in the CLI
// config, otherwise the single default vault (the original behaviour).
func resolveServeVaults(names []string) ([]server.Vault, error) {
	if len(names) > 0 {
		vaults := make([]server.Vault, 0, len(names))
		for _, name := range names {
			if config, ok := obsidian.FindConfiguredVault(name); ok {
				vaults = append(vaults, server.VaultsFromConfig([]obsidian.VaultConfig{config})...)
				continue
			}
			vaults = append(vaults, server.Vault{
				ID:      vaultIDFromName(name),
				Label:   vaultIDFromName(name),
				Manager: &obsidian.Vault{Name: name},
			})
		}
		return vaults, nil
	}

	configured, err := obsidian.ConfiguredVaults()
	if err != nil {
		return nil, err
	}
	if len(configured) > 0 {
		return server.VaultsFromConfig(configured), nil
	}

	return []server.Vault{{
		ID:      server.DefaultVaultID,
		Label:   "Default",
		Manager: &obsidian.Vault{},
	}}, nil
}

// vaultIDFromName turns a --vault value that isn't a configured id into one:
// the directory/vault name, lowercased with spaces hyphenated, so it stays
// usable as a ?vault= value.
func vaultIDFromName(name string) string {
	base := filepath.Base(filepath.Clean(name))
	return strings.ToLower(strings.ReplaceAll(base, " ", "-"))
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 7070, "port to listen on")
	serveCmd.Flags().StringArrayVarP(&serveVaults, "vault", "v", nil,
		"vault to serve: a configured vault id, vault name, or absolute path (repeatable; defaults to the configured vaults)")
	rootCmd.AddCommand(serveCmd)
}

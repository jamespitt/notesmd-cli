package cmd

import (
	"fmt"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/spf13/cobra"
)

var vaultsCmd = &cobra.Command{
	Use:   "vaults",
	Short: "List the vaults configured for switching",
	Long: `List the vaults declared under "vaults" in ~/.config/notesmd-cli/preferences.json.

These are the vaults ` + "`serve`" + ` exposes and that clients switch between with
?vault=<id>. The first one is the default.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configured, err := obsidian.ConfiguredVaults()
		if err != nil {
			return err
		}

		if len(configured) == 0 {
			vault := obsidian.Vault{}
			name, err := vault.DefaultName()
			if err != nil {
				return fmt.Errorf("no vaults configured and no default vault set: %w", err)
			}
			path, err := vault.Path()
			if err != nil {
				return err
			}
			fmt.Printf("No switchable vaults configured - serving the default vault only.\n\n")
			fmt.Printf("  %s -> %s\n", name, path)
			return nil
		}

		for i, v := range configured {
			marker := " "
			if i == 0 {
				marker = "*"
			}
			path, pathErr := obsidian.NewConfiguredVault(v).Path()
			if pathErr != nil {
				path = fmt.Sprintf("%s (unresolved: %v)", v.Path, pathErr)
			}
			fmt.Printf("%s %-10s %-12s %s\n", marker, v.ID, v.DisplayLabel(), path)

			if len(v.TaskFolders) > 0 {
				fmt.Printf("    tasks:    %s\n", strings.Join(v.TaskFolders, ", "))
			}
			if v.ProjectsFolder != "" {
				fmt.Printf("    projects: %s\n", v.ProjectsFolder)
			}
			if v.CalendarFolder != "" {
				fmt.Printf("    calendar: %s\n", v.CalendarFolder)
			}
		}
		fmt.Printf("\n* = default vault (used when a request sends no ?vault= parameter)\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vaultsCmd)
}

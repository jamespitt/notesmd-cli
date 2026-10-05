package cmd

import (
	"log"

	"github.com/Yakitrak/notesmd-cli/pkg/actions"
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks/client"

	"github.com/spf13/cobra"
)

var taskFolders []string
var taskTags []string
var taskDate string
var taskFrom string
var taskTo string
var taskToday bool

var (
	tasksLocal bool
	tasksJSON  bool
)

func initTaskClient() (client.TaskClient, error) {
	cfg, _ := obsidian.ReadCliConfig()

	serverURL := cfg.ServerURL
	serverUsername := cfg.ServerUsername
	serverPassword := cfg.ServerPassword
	targetVault := vaultName

	if targetVault != "" {
		if v, ok := obsidian.FindConfiguredVault(targetVault); ok {
			if v.ServerURL != "" {
				serverURL = v.ServerURL
				if v.ServerUsername != "" {
					serverUsername = v.ServerUsername
					serverPassword = v.ServerPassword
				}
			}
		}
	} else if len(cfg.Vaults) > 0 {
		v := cfg.Vaults[0]
		targetVault = v.ID
		if v.ServerURL != "" {
			serverURL = v.ServerURL
			if v.ServerUsername != "" {
				serverUsername = v.ServerUsername
				serverPassword = v.ServerPassword
			}
		}
	}

	return client.NewClient(client.Config{
		ForceLocal:     tasksLocal,
		ServerURL:      serverURL,
		ServerUsername: serverUsername,
		ServerPassword: serverPassword,
		VaultID:        targetVault,
	})
}

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Search and manage tasks in vault",
	Long:  "Search or manage tasks across notes in the vault. Run without subcommands to search, or use subcommands (list, move, edit, etc.).",
	Run: func(cmd *cobra.Command, args []string) {
		vault := obsidian.Vault{Name: vaultName}
		note := obsidian.Note{}

		err := actions.SearchTasks(&vault, &note, actions.TaskParams{
			Folders: taskFolders,
			Tags:    taskTags,
			Date:    taskDate,
			From:    taskFrom,
			To:      taskTo,
			Today:   taskToday,
		})
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	tasksCmd.PersistentFlags().StringVarP(&vaultName, "vault", "v", "", "vault name")
	tasksCmd.PersistentFlags().BoolVar(&tasksLocal, "local", false, "force direct vault operations instead of HTTP API")
	tasksCmd.PersistentFlags().BoolVar(&tasksJSON, "json", false, "output results in JSON format")

	tasksCmd.Flags().StringArrayVarP(&taskFolders, "folder", "f", []string{}, "folder to search (relative to vault root, repeatable; overrides config defaults)")
	tasksCmd.Flags().StringArrayVarP(&taskTags, "tag", "t", []string{}, "filter by tag (repeatable, OR logic)")
	tasksCmd.Flags().StringVarP(&taskDate, "date", "d", "", "filter by exact scheduled date (YYYY-MM-DD)")
	tasksCmd.Flags().StringVar(&taskFrom, "from", "", "filter by scheduled date range start (YYYY-MM-DD)")
	tasksCmd.Flags().StringVar(&taskTo, "to", "", "filter by scheduled date range end (YYYY-MM-DD)")
	tasksCmd.Flags().BoolVar(&taskToday, "today", false, "shorthand for --tag today --date <today>")
	rootCmd.AddCommand(tasksCmd)
}

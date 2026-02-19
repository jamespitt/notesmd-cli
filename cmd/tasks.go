package cmd

import (
	"log"

	"github.com/Yakitrak/notesmd-cli/pkg/actions"
	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"

	"github.com/spf13/cobra"
)

var taskFolders []string
var taskTags []string
var taskDate string
var taskFrom string
var taskTo string
var taskToday bool

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Search for tasks in vault",
	Long:  "Search for markdown checkbox tasks across notes in the vault, filtered by tag and/or date.",
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
	tasksCmd.Flags().StringVarP(&vaultName, "vault", "v", "", "vault name")
	tasksCmd.Flags().StringArrayVarP(&taskFolders, "folder", "f", []string{}, "folder to search (relative to vault root, repeatable; overrides config defaults)")
	tasksCmd.Flags().StringArrayVarP(&taskTags, "tag", "t", []string{}, "filter by tag (repeatable, OR logic)")
	tasksCmd.Flags().StringVarP(&taskDate, "date", "d", "", "filter by exact scheduled date (YYYY-MM-DD)")
	tasksCmd.Flags().StringVar(&taskFrom, "from", "", "filter by scheduled date range start (YYYY-MM-DD)")
	tasksCmd.Flags().StringVar(&taskTo, "to", "", "filter by scheduled date range end (YYYY-MM-DD)")
	tasksCmd.Flags().BoolVar(&taskToday, "today", false, "shorthand for --tag today --date <today>")
	rootCmd.AddCommand(tasksCmd)
}

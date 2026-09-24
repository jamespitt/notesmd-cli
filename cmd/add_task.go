package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks"

	"github.com/spf13/cobra"
)

var (
	addTaskList    string
	addTaskTag     string
	addTaskSource  string
	addTaskUsers   []string
	addTaskCreated string
	addTaskFolders []string
)

var addTaskCmd = &cobra.Command{
	Use:   "add-task <title>",
	Short: "Add a task to a task list file",
	Long: `Append a task to a list file (default: Obsidian) under the shared vault lock, so it
can't interleave with the Google/Todoist sync or the git auto-commit job. Adding a
title that is already in the list is a no-op. Use this instead of editing list files directly.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vault := obsidian.Vault{Name: vaultName}
		if _, err := vault.DefaultName(); err != nil {
			log.Fatal(err)
		}
		vaultPath, err := vault.Path()
		if err != nil {
			log.Fatal(err)
		}
		folders := addTaskFolders
		if len(folders) == 0 {
			if folders, err = vault.TaskFolders(); err != nil {
				log.Fatal(err)
			}
		}
		file, err := tasks.FindListFile(vaultPath, folders, addTaskList)
		if err != nil {
			log.Fatalf("list %q not found in task folders %v", addTaskList, folders)
		}
		created := addTaskCreated
		if created == "" {
			created = time.Now().Format("2006-01-02")
		}
		added, err := tasks.AddTask(file, tasks.NewTask{
			Title: args[0], Tag: addTaskTag, Created: created, Source: addTaskSource, Users: addTaskUsers,
		})
		if err != nil {
			log.Fatal(err)
		}
		if added {
			fmt.Printf("Added to %s\n", file)
		} else {
			fmt.Printf("Already present in %s, skipped\n", file)
		}
	},
}

func init() {
	addTaskCmd.Flags().StringVarP(&vaultName, "vault", "v", "", "vault name")
	addTaskCmd.Flags().StringArrayVarP(&addTaskFolders, "folder", "f", []string{}, "task folder (relative to vault root; overrides config defaults)")
	addTaskCmd.Flags().StringVarP(&addTaskList, "list", "l", "Obsidian", "list (file name without .md) to add to")
	addTaskCmd.Flags().StringVarP(&addTaskTag, "tag", "t", "", "tag to add, e.g. ToTriage")
	addTaskCmd.Flags().StringVar(&addTaskSource, "source", "", "note the task came from")
	addTaskCmd.Flags().StringArrayVar(&addTaskUsers, "user", []string{}, "person involved (repeatable)")
	addTaskCmd.Flags().StringVar(&addTaskCreated, "created", "", "created date YYYY-MM-DD (default today)")
	rootCmd.AddCommand(addTaskCmd)
}

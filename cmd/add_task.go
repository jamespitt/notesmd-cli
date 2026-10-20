package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
	"github.com/Yakitrak/notesmd-cli/pkg/tasks/client"

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
title that is already in the list is a no-op. Use this instead of editing list files directly.

When server_url is configured in preferences.json (or NOTESMD_SERVER_URL is set) the task
is added over the HTTP API, so the vault file is only written by the server. Pass --local
to write the vault file on this machine instead.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !tasksLocal {
			if cli, err := initTaskClient(); err == nil {
				if _, remote := cli.(*client.HTTPClient); remote {
					addTaskOverHTTP(cli, args[0])
					return
				}
			}
		}

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

// addTaskOverHTTP adds the task through the server's API. The API only takes a
// title, so the tag, source, users and created date are written into the title
// text (the server's own stamping fills in the id and updated fields, and adds
// #ToTriage when no tag is given). De-duplication is done here against the
// list's current tasks, matching the local path's "already present" behaviour.
func addTaskOverHTTP(cli client.TaskClient, title string) {
	ctx := context.Background()

	if len(addTaskFolders) > 0 {
		fmt.Fprintln(os.Stderr, "note: --folder is ignored over HTTP; the server uses its configured task folders (use --local to override)")
	}

	existing, err := cli.ListTasks(ctx, "all")
	if err != nil {
		log.Fatal(err)
	}
	want := strings.ToLower(strings.TrimSpace(title))
	for _, t := range existing {
		if t.ListName == addTaskList && strings.ToLower(strings.TrimSpace(t.Title)) == want {
			fmt.Printf("Already present in %s, skipped\n", addTaskList)
			return
		}
	}

	created := addTaskCreated
	if created == "" {
		created = time.Now().Format("2006-01-02")
	}
	parts := []string{title}
	if addTaskTag != "" {
		parts = append(parts, "#"+strings.TrimPrefix(addTaskTag, "#"))
	}
	parts = append(parts, "[created::"+created+"]")
	if addTaskSource != "" {
		parts = append(parts, "[source:: "+addTaskSource+"]")
	}
	if len(addTaskUsers) > 0 {
		parts = append(parts, "[user:: "+strings.Join(addTaskUsers, ", ")+"]")
	}

	if err := cli.AddTask(ctx, addTaskList, strings.Join(parts, " "), false); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Added to %s (via server)\n", addTaskList)
}

func init() {
	addTaskCmd.Flags().StringVarP(&vaultName, "vault", "v", "", "vault name")
	addTaskCmd.Flags().StringArrayVarP(&addTaskFolders, "folder", "f", []string{}, "task folder (relative to vault root; overrides config defaults; local only)")
	addTaskCmd.Flags().StringVarP(&addTaskList, "list", "l", "Obsidian", "list (file name without .md) to add to")
	addTaskCmd.Flags().StringVarP(&addTaskTag, "tag", "t", "", "tag to add, e.g. ToDo (default ToTriage)")
	addTaskCmd.Flags().StringVar(&addTaskSource, "source", "", "note the task came from")
	addTaskCmd.Flags().StringArrayVar(&addTaskUsers, "user", []string{}, "person involved (repeatable)")
	addTaskCmd.Flags().StringVar(&addTaskCreated, "created", "", "created date YYYY-MM-DD (default today)")
	addTaskCmd.Flags().BoolVar(&tasksLocal, "local", false, "write the vault file directly instead of using the HTTP API")
	rootCmd.AddCommand(addTaskCmd)
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var taskAddDone bool

var tasksAddCmd = &cobra.Command{
	Use:   "add <list> <title>",
	Short: "Add a task to a list",
	Long:  "Append a task to the specified list file (matching POST /api/tasks/list/{name}).",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		listName := args[0]
		title := args[1]

		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		if err := client.AddTask(ctx, listName, title, taskAddDone); err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status": "created",
				"list":   listName,
				"title":  title,
				"done":   taskAddDone,
			})
			return
		}

		statusStr := ""
		if taskAddDone {
			statusStr = " (completed)"
		}
		fmt.Printf("Added task %q to %s%s\n", title, listName, statusStr)
	},
}

func init() {
	tasksAddCmd.Flags().BoolVar(&taskAddDone, "done", false, "mark task as completed")
	tasksCmd.AddCommand(tasksAddCmd)
}

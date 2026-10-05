package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var tasksRenameCmd = &cobra.Command{
	Use:   "rename <id> <new-title>",
	Short: "Rename a task",
	Long:  "Rename the title of the task identified by Google ID or task ID.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		taskID := args[0]
		newTitle := args[1]

		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		task, err := client.FindTask(ctx, taskID)
		if err != nil {
			log.Fatal(err)
		}

		if err := client.RenameTask(ctx, task, newTitle); err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":    "ok",
				"id":        taskID,
				"title":     newTitle,
				"file_path": task.FilePath,
				"line_num":  task.LineNum,
			})
			return
		}

		fmt.Printf("Renamed task %s to %q\n", taskID, newTitle)
	},
}

func init() {
	tasksCmd.AddCommand(tasksRenameCmd)
}

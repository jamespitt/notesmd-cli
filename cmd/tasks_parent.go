package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
	"github.com/spf13/cobra"
)

var parentTop bool

var tasksSetParentCmd = &cobra.Command{
	Use:   "set-parent <id> [parent-id]",
	Short: "Nest a task under a parent or promote to top level",
	Long:  "Nest the task identified by <id> under <parent-id>, or promote it to top-level if --top is specified or parent-id is omitted.",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		taskID := args[0]

		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		task, err := client.FindTask(ctx, taskID)
		if err != nil {
			log.Fatal(err)
		}

		var parent *tasks.Task
		if !parentTop && len(args) == 2 && args[1] != "" {
			p, err := client.FindTask(ctx, args[1])
			if err != nil {
				log.Fatal(err)
			}
			parent = p
		}

		if err := client.SetParent(ctx, task, parent); err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			parentID := ""
			if parent != nil {
				if parent.GoogleID != "" {
					parentID = parent.GoogleID
				} else {
					parentID = parent.ID
				}
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":    "ok",
				"id":        taskID,
				"parent_id": parentID,
			})
			return
		}

		if parent == nil {
			fmt.Printf("Promoted task %s to top level\n", taskID)
		} else {
			fmt.Printf("Set task %s as subtask of %s\n", taskID, args[1])
		}
	},
}

var tasksAddSubtaskCmd = &cobra.Command{
	Use:   "add-subtask <parent-id> <title>",
	Short: "Add a subtask under a parent task",
	Long:  "Append a new indented subtask under the parent task identified by <parent-id>.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		parentID := args[0]
		subtaskTitle := args[1]

		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		parent, err := client.FindTask(ctx, parentID)
		if err != nil {
			log.Fatal(err)
		}

		if err := client.AddSubtask(ctx, parent, subtaskTitle); err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":    "ok",
				"parent_id": parentID,
				"title":     subtaskTitle,
			})
			return
		}

		fmt.Printf("Added subtask %q under %s\n", subtaskTitle, parentID)
	},
}

func init() {
	tasksSetParentCmd.Flags().BoolVar(&parentTop, "top", false, "promote task to top level")
	tasksCmd.AddCommand(tasksSetParentCmd)
	tasksCmd.AddCommand(tasksAddSubtaskCmd)
}

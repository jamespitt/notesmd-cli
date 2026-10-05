package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/tasks"
	"github.com/spf13/cobra"
)

var moveColumns string

var tasksMoveCmd = &cobra.Command{
	Use:   "move <id> <status>",
	Short: "Move task to a Kanban status",
	Long:  "Move the task identified by Google ID or task ID to a Kanban status tag (e.g. ToDo, InProgress, Done, Delete).",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		taskID := args[0]
		status := args[1]

		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		task, err := client.FindTask(ctx, taskID)
		if err != nil {
			log.Fatal(err)
		}

		var cols []string
		if moveColumns != "" {
			for _, c := range strings.Split(moveColumns, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					cols = append(cols, c)
				}
			}
		}
		if len(cols) == 0 {
			cols = tasks.KanbanTags
		}

		if err := client.SetStatusTag(ctx, task, status, cols); err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":        "ok",
				"id":            taskID,
				"kanban_status": status,
				"file_path":     task.FilePath,
				"line_num":      task.LineNum,
			})
			return
		}

		fmt.Printf("Moved task %q (%s) to %s\n", task.Title, taskID, status)
	},
}

func init() {
	tasksMoveCmd.Flags().StringVar(&moveColumns, "columns", "", "comma-separated column tags (default: ToDo,InProgress,Done)")
	tasksCmd.AddCommand(tasksMoveCmd)
}

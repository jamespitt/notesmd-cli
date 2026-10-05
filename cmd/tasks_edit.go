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

var (
	editTitle          string
	editDue            string
	editClearDue       bool
	editScheduled      string
	editClearScheduled bool
	editPriority       string
	editClearPriority  bool
	editRepeat         string
	editClearRepeat    bool
)

var tasksEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit task fields",
	Long:  "Update metadata fields (due, scheduled, priority, repeat, title) on a task identified by Google ID or task ID.",
	Args:  cobra.ExactArgs(1),
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

		var edit tasks.TaskEdit
		empty := ""

		if cmd.Flags().Changed("title") {
			edit.Title = &editTitle
		}
		if editClearDue {
			edit.Due = &empty
		} else if cmd.Flags().Changed("due") {
			edit.Due = &editDue
		}

		if editClearScheduled {
			edit.Scheduled = &empty
		} else if cmd.Flags().Changed("scheduled") {
			edit.Scheduled = &editScheduled
		}

		if editClearPriority {
			edit.Priority = &empty
		} else if cmd.Flags().Changed("priority") {
			edit.Priority = &editPriority
		}

		if editClearRepeat {
			edit.Repeat = &empty
		} else if cmd.Flags().Changed("repeat") {
			edit.Repeat = &editRepeat
		}

		if err := client.EditTask(ctx, task, edit); err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":    "ok",
				"id":        taskID,
				"file_path": task.FilePath,
				"line_num":  task.LineNum,
			})
			return
		}

		fmt.Printf("Updated task %s\n", taskID)
	},
}

func init() {
	tasksEditCmd.Flags().StringVar(&editTitle, "title", "", "update title")
	tasksEditCmd.Flags().StringVar(&editDue, "due", "", "set due date (YYYY-MM-DD or ISO 8601)")
	tasksEditCmd.Flags().BoolVar(&editClearDue, "clear-due", false, "clear due date")
	tasksEditCmd.Flags().StringVar(&editScheduled, "scheduled", "", "set scheduled date/time")
	tasksEditCmd.Flags().BoolVar(&editClearScheduled, "clear-scheduled", false, "clear scheduled date/time")
	tasksEditCmd.Flags().StringVar(&editPriority, "priority", "", "set priority (e.g. high, medium, low)")
	tasksEditCmd.Flags().BoolVar(&editClearPriority, "clear-priority", false, "clear priority")
	tasksEditCmd.Flags().StringVar(&editRepeat, "repeat", "", "set repeat rule")
	tasksEditCmd.Flags().BoolVar(&editClearRepeat, "clear-repeat", false, "clear repeat rule")
	tasksCmd.AddCommand(tasksEditCmd)
}

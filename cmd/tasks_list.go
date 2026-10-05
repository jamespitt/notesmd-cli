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

var (
	listFilter    string
	kanbanColumns string
)

func formatTaskIdentifier(t tasks.Task) string {
	if t.GoogleID != "" {
		return "[" + t.GoogleID + "]"
	}
	if t.ID != "" {
		return "[" + t.ID + "]"
	}
	return ""
}

func printTaskLine(t tasks.Task) {
	statusMark := " "
	if t.Status == tasks.StatusCompleted {
		statusMark = "x"
	}
	idStr := formatTaskIdentifier(t)
	if idStr != "" {
		idStr += " "
	}

	var meta []string
	if t.Due != "" {
		meta = append(meta, "due:"+t.Due)
	}
	if t.Scheduled != "" {
		meta = append(meta, "sched:"+t.Scheduled)
	}
	if t.Priority != "" {
		meta = append(meta, "prio:"+t.Priority)
	}
	if t.Repeat != "" {
		meta = append(meta, "repeat:"+t.Repeat)
	}
	for _, tag := range t.Tags {
		meta = append(meta, "#"+tag)
	}

	metaStr := ""
	if len(meta) > 0 {
		metaStr = " (" + strings.Join(meta, ", ") + ")"
	}

	loc := fmt.Sprintf("%s:%d", t.FilePath, t.LineNum)
	fmt.Printf("- [%s] %s%s%s  %s\n", statusMark, idStr, t.Title, metaStr, loc)
}

var tasksListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List tasks by filter",
	Long:    "List tasks from the vault or HTTP API. Filter can be today, tomorrow, overdue, kanban, or all.",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		result, err := client.ListTasks(ctx, listFilter)
		if err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				log.Fatal(err)
			}
			return
		}

		if len(result) == 0 {
			fmt.Printf("No tasks found (filter: %s)\n", listFilter)
			return
		}

		for _, t := range result {
			printTaskLine(t)
		}
	},
}

var tasksKanbanCmd = &cobra.Command{
	Use:   "kanban",
	Short: "List tasks in Kanban board layout",
	Long:  "List Kanban cards grouped by columns, including nested subtasks.",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := initTaskClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		var cols []string
		if kanbanColumns != "" {
			for _, c := range strings.Split(kanbanColumns, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					cols = append(cols, c)
				}
			}
		}
		if len(cols) == 0 {
			cols = tasks.KanbanTags
		}

		result, err := client.ListKanban(ctx, cols)
		if err != nil {
			log.Fatal(err)
		}

		if tasksJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				log.Fatal(err)
			}
			return
		}

		colMap := make(map[string][]tasks.Task)
		for _, t := range result {
			status := tasks.KanbanStatusIn(t, cols)
			if status != "" {
				colMap[status] = append(colMap[status], t)
			}
		}

		for _, col := range cols {
			cards := colMap[col]
			fmt.Printf("\n=== %s (%d) ===\n", col, len(cards))
			for _, t := range cards {
				printTaskLine(t)
				for _, sub := range t.Subtasks {
					subMark := " "
					if sub.Status == tasks.StatusCompleted {
						subMark = "x"
					}
					indent := strings.Repeat("  ", sub.Level)
					fmt.Printf("  %s- [%s] %s (line %d)\n", indent, subMark, sub.Title, sub.LineNum)
				}
			}
		}
	},
}

func init() {
	tasksListCmd.Flags().StringVar(&listFilter, "filter", "today", "filter tasks (today, tomorrow, overdue, kanban, all)")
	tasksCmd.AddCommand(tasksListCmd)

	tasksKanbanCmd.Flags().StringVar(&kanbanColumns, "columns", "", "comma-separated column tags (default: ToDo,InProgress,Done)")
	tasksCmd.AddCommand(tasksKanbanCmd)
}

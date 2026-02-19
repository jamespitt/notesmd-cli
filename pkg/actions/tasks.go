package actions

import (
	"fmt"
	"sort"
	"time"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

const dateLayout = "2006-01-02"

type TaskParams struct {
	Folders []string
	Tags    []string
	Date    string
	From    string
	To      string
	Today   bool
}

func SearchTasks(vault obsidian.VaultManager, note obsidian.NoteManager, params TaskParams) error {
	_, err := vault.DefaultName()
	if err != nil {
		return err
	}

	vaultPath, err := vault.Path()
	if err != nil {
		return err
	}

	// Resolve search folders: flag overrides config defaults
	folders := params.Folders
	if len(folders) == 0 {
		folders, err = vault.TaskFolders()
		if err != nil {
			return err
		}
	}

	// Build filters
	tags := params.Tags
	date := params.Date
	if params.Today {
		tags = append([]string{"today"}, tags...)
		if date == "" {
			date = time.Now().Format(dateLayout)
		}
	}

	filters := obsidian.TaskFilters{
		Tags: tags,
	}

	if date != "" {
		t, err := time.Parse(dateLayout, date)
		if err != nil {
			return fmt.Errorf("invalid --date value %q: expected YYYY-MM-DD", date)
		}
		filters.Date = &t
	}

	if params.From != "" {
		t, err := time.Parse(dateLayout, params.From)
		if err != nil {
			return fmt.Errorf("invalid --from value %q: expected YYYY-MM-DD", params.From)
		}
		filters.From = &t
	}

	if params.To != "" {
		t, err := time.Parse(dateLayout, params.To)
		if err != nil {
			return fmt.Errorf("invalid --to value %q: expected YYYY-MM-DD", params.To)
		}
		filters.To = &t
	}

	tasks, err := note.SearchTasks(vaultPath, folders, filters)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found matching the given filters")
		return nil
	}

	// Partition: unscheduled first, then scheduled sorted by time
	var unscheduled, scheduled []obsidian.Task
	for _, task := range tasks {
		if task.Scheduled == nil {
			unscheduled = append(unscheduled, task)
		} else {
			scheduled = append(scheduled, task)
		}
	}
	sort.Slice(scheduled, func(i, j int) bool {
		return scheduled[i].Scheduled.Before(*scheduled[j].Scheduled)
	})

	for _, task := range append(unscheduled, scheduled...) {
		fmt.Printf("%s:%d\t%s\n", task.FilePath, task.LineNumber, task.RawText)
	}

	return nil
}

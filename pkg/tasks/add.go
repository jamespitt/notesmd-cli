package tasks

import (
	"os"
	"regexp"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/vaultlock"
)

// NewTask describes a task to add to a list file.
type NewTask struct {
	Title   string
	Tag     string   // without the leading '#'; optional
	Created string   // YYYY-MM-DD; optional
	Source  string   // note the task came from; optional
	Users   []string // people involved; optional
}

var newlineRe = regexp.MustCompile(`[\r\n]+`)

// Line renders the task in the vault's checkbox convention. Tag defaults to
// "ToTriage" when the caller doesn't supply one (every created task carries
// at least one tag), and Created defaults to today - see stampCreated, which
// applies the same two defaults to the other creation paths (AppendTask*,
// AppendSubtask). The task gets its [id::], and [updated::] is always set, to
// the current time.
func (n NewTask) Line() string {
	var b strings.Builder
	b.WriteString("- [ ] ")
	b.WriteString(strings.TrimSpace(newlineRe.ReplaceAllString(n.Title, " ")))
	tag := strings.TrimPrefix(n.Tag, "#")
	if tag == "" {
		tag = "ToTriage"
	}
	b.WriteString(" #" + tag)
	created := n.Created
	if created == "" {
		created = today()
	}
	b.WriteString(" [created::" + created + "]")
	if n.Source != "" {
		b.WriteString(" [source:: " + n.Source + "]")
	}
	if len(n.Users) > 0 {
		b.WriteString(" [user:: " + strings.Join(n.Users, ", ") + "]")
	}
	b.WriteString(" [id::" + newTaskID() + "]")
	b.WriteString(" [updated::" + nowStamp() + "]")
	return b.String()
}

// AddTask appends the task to absPath under the vault lock. It is idempotent:
// if the file already has a task with the same title (case-insensitive,
// ignoring tags and fields) nothing is written and added is false. That makes
// a re-run of an ingest harmless.
func AddTask(absPath string, n NewTask) (added bool, err error) {
	release, err := vaultlock.Lock()
	if err != nil {
		return false, err
	}
	defer release()

	content, err := os.ReadFile(absPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	// parseLine strips "[duration: …]" from Title, so compare without it too.
	want := strings.ToLower(strings.TrimSpace(durationRe.ReplaceAllString(newlineRe.ReplaceAllString(n.Title, " "), "")))
	for i, line := range strings.Split(string(content), "\n") {
		if t := parseLine(line, absPath, i+1); t != nil && strings.ToLower(strings.TrimSpace(t.Title)) == want {
			return false, nil
		}
	}

	out := string(content)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += n.Line() + "\n"
	return true, writeFileAtomic(absPath, []byte(out), 0644)
}

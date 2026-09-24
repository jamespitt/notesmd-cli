package tasks

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Yakitrak/notesmd-cli/pkg/vaultlock"
)

// KanbanCardsIn returns the tasks that should be cards on a board with the
// given columns, with subtasks folded into their parent:
//
//   - A task with an ancestor that is itself on the board is not a card - it is
//     one of that ancestor's Subtasks (the topmost on-board ancestor owns it).
//   - A task whose ancestors are all off the board stays a card of its own, so
//     a tagged subtask under an untagged parent never disappears.
//   - Every descendant of a card is listed in Subtasks, whatever its own tags.
func KanbanCardsIn(all []Task, columns []string) []Task {
	index := make(map[string]int, len(all)) // "file:line" -> position in all
	for i, t := range all {
		index[parentID(t.FilePath, t.LineNum)] = i
	}
	onBoard := make([]bool, len(all))
	for i, t := range all {
		onBoard[i] = KanbanStatusIn(t, columns) != ""
	}

	// owner[i] = index of the topmost on-board ancestor of all[i], or -1.
	owner := make([]int, len(all))
	for i := range all {
		owner[i] = -1
		seen := map[int]bool{i: true}
		for p := all[i].ParentID; p != ""; {
			pi, ok := index[p]
			if !ok || seen[pi] {
				break
			}
			seen[pi] = true
			if onBoard[pi] {
				owner[i] = pi
			}
			p = all[pi].ParentID
		}
	}

	subtasks := make(map[int][]Subtask)
	for i, t := range all {
		if o := owner[i]; o >= 0 {
			subtasks[o] = append(subtasks[o], Subtask{
				LineNum: t.LineNum,
				Title:   t.Title,
				Status:  t.Status,
				Level:   t.Level - all[o].Level,
			})
		}
	}

	var cards []Task
	for i, t := range all {
		if !onBoard[i] || owner[i] >= 0 {
			continue
		}
		t.Subtasks = subtasks[i]
		cards = append(cards, t)
	}
	return cards
}

// ErrSetParent is returned for a request that can't be honoured (a task can't
// become a subtask of itself or of one of its own subtasks, a line isn't a
// task, ...).
var ErrSetParent = errors.New("cannot set parent")

func setParentErr(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrSetParent, fmt.Sprintf(format, a...))
}

// blockEnd returns the index one past the task at idx and all its descendants
// (the contiguous task lines indented deeper than it).
func blockEnd(lines []string, idx int, indent string) int {
	end := idx + 1
	for end < len(lines) {
		cm := taskLineRe.FindStringSubmatch(lines[end])
		if cm == nil || len(cm[1]) <= len(indent) {
			break
		}
		end++
	}
	return end
}

// reindent shifts a line's leading whitespace by delta columns (spaces).
func reindent(line string, delta int) string {
	if delta >= 0 {
		return strings.Repeat(" ", delta) + line
	}
	remove := -delta
	i := 0
	for i < len(line) && i < remove && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[i:]
}

// SetParent moves the task at srcPath:lineNum (with its own subtasks) so that
// it becomes the last subtask of the task at dstPath:parentLine, indented one
// level under it. dstPath may be a different file: a subtask has to live in
// its parent's list, so the task moves there.
//
// parentLine == 0 promotes the task to top level instead: it is placed right
// after the subtree of its top-level ancestor, in the same file.
//
// It returns the task's new 1-based line number (in dstPath, or srcPath when
// promoting). The whole change is applied under the vault lock; when the
// files differ the destination is written before the source is trimmed, so a
// failure can duplicate a task but never lose one.
func SetParent(srcPath string, lineNum int, dstPath string, parentLine int) (int, error) {
	release, err := vaultlock.Lock()
	if err != nil {
		return 0, err
	}
	defer release()

	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		return 0, err
	}
	src := strings.Split(string(srcBytes), "\n")
	idx := lineNum - 1
	if idx < 0 || idx >= len(src) {
		return 0, setParentErr("line %d is out of range", lineNum)
	}
	m := taskLineRe.FindStringSubmatch(src[idx])
	if m == nil {
		return 0, setParentErr("line %d is not a task", lineNum)
	}
	taskIndent := m[1]
	end := blockEnd(src, idx, taskIndent)
	block := append([]string(nil), src[idx:end]...)

	if parentLine == 0 {
		return promote(srcPath, src, idx, end, block, taskIndent)
	}

	same := dstPath == "" || dstPath == srcPath
	var dst []string
	if same {
		dstPath = srcPath
	} else {
		dstBytes, err := os.ReadFile(dstPath)
		if err != nil {
			return 0, err
		}
		dst = strings.Split(string(dstBytes), "\n")
	}

	pIdx := parentLine - 1
	target := src
	if !same {
		target = dst
	}
	if pIdx < 0 || pIdx >= len(target) || taskLineRe.FindStringSubmatch(target[pIdx]) == nil {
		return 0, setParentErr("parent line %d is not a task", parentLine)
	}
	if same {
		if pIdx >= idx && pIdx < end {
			return 0, setParentErr("a task can't become a subtask of itself or of one of its own subtasks")
		}
	}

	// Take the block out of the source.
	remaining := make([]string, 0, len(src)-(end-idx))
	remaining = append(remaining, src[:idx]...)
	remaining = append(remaining, src[end:]...)
	if same {
		if pIdx >= end {
			pIdx -= end - idx
		}
		dst = remaining
	}

	parentIndent := taskLineRe.FindStringSubmatch(dst[pIdx])[1]
	insertAt := blockEnd(dst, pIdx, parentIndent)

	delta := len(parentIndent) + 4 - len(taskIndent)
	moved := make([]string, len(block))
	for i, l := range block {
		moved[i] = reindent(l, delta)
	}

	out := make([]string, 0, len(dst)+len(moved))
	out = append(out, dst[:insertAt]...)
	out = append(out, moved...)
	out = append(out, dst[insertAt:]...)

	if same {
		return insertAt + 1, writeFileAtomic(srcPath, []byte(strings.Join(out, "\n")), 0644)
	}
	if err := writeFileAtomic(dstPath, []byte(strings.Join(out, "\n")), 0644); err != nil {
		return 0, err
	}
	if err := writeFileAtomic(srcPath, []byte(strings.Join(remaining, "\n")), 0644); err != nil {
		return 0, err
	}
	return insertAt + 1, nil
}

// promote makes the task top level: it goes after the subtree of its
// top-level ancestor. A task that is already top level is left alone.
func promote(path string, src []string, idx, end int, block []string, taskIndent string) (int, error) {
	if len(taskIndent) == 0 {
		return idx + 1, nil
	}

	// Walk up to the top-level ancestor: the nearest earlier task line with
	// less indentation, repeated until indentation is 0.
	root := idx
	rootIndent := len(taskIndent)
	for i := idx - 1; i >= 0 && rootIndent > 0; i-- {
		cm := taskLineRe.FindStringSubmatch(src[i])
		if cm == nil {
			break
		}
		if len(cm[1]) < rootIndent {
			root, rootIndent = i, len(cm[1])
		}
	}
	if rootIndent != 0 {
		return 0, setParentErr("couldn't find the top-level task above line %d", idx+1)
	}

	remaining := make([]string, 0, len(src)-(end-idx))
	remaining = append(remaining, src[:idx]...)
	remaining = append(remaining, src[end:]...)

	insertAt := blockEnd(remaining, root, "")
	moved := make([]string, len(block))
	for i, l := range block {
		moved[i] = reindent(l, -len(taskIndent))
	}

	out := make([]string, 0, len(remaining)+len(moved))
	out = append(out, remaining[:insertAt]...)
	out = append(out, moved...)
	out = append(out, remaining[insertAt:]...)
	return insertAt + 1, writeFileAtomic(path, []byte(strings.Join(out, "\n")), 0644)
}

package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTmp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readTmp(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const tree = `# L
- [ ] A #ToDo
    - [ ] A1
        - [ ] A1a
    - [ ] A2
- [ ] B #ToDo
- [ ] C
- [ ] D
    - [ ] D1
`

func setupTree(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))
	dir := t.TempDir()
	return dir, writeTmp(t, dir, "L.md", tree)
}

func TestSetParentSameFile(t *testing.T) {
	// C (line 7) under A (line 2): lands after A's last descendant, one level deeper.
	dir, p := setupTree(t)
	_ = dir
	line, err := SetParent(p, 7, p, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := `# L
- [ ] A #ToDo
    - [ ] A1
        - [ ] A1a
    - [ ] A2
    - [ ] C
- [ ] B #ToDo
- [ ] D
    - [ ] D1
`
	if got := readTmp(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if line != 6 {
		t.Fatalf("new line = %d, want 6", line)
	}
}

func TestSetParentDeeperAndBackwards(t *testing.T) {
	// B (line 6) under D1 (line 9): two levels deep, parent is after the source.
	_, p := setupTree(t)
	line, err := SetParent(p, 6, p, 9)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readTmp(t, p), "- [ ] D\n    - [ ] D1\n        - [ ] B #ToDo\n") {
		t.Fatalf("B not nested under D1:\n%s", readTmp(t, p))
	}
	// After removing B (one line) D1 is line 8, so B is inserted at index 8 => line 9.
	if line != 9 {
		t.Fatalf("new line = %d, want 9", line)
	}
}

func TestSetParentMovesTheWholeSubtree(t *testing.T) {
	// A with A1/A1a/A2 under D (line 8): every level shifts by the same amount.
	_, p := setupTree(t)
	if _, err := SetParent(p, 2, p, 8); err != nil {
		t.Fatal(err)
	}
	want := `# L
- [ ] B #ToDo
- [ ] C
- [ ] D
    - [ ] D1
    - [ ] A #ToDo
        - [ ] A1
            - [ ] A1a
        - [ ] A2
`
	if got := readTmp(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetParentRejectsCycles(t *testing.T) {
	_, p := setupTree(t)
	before := readTmp(t, p)
	for _, parent := range []int{2, 3, 4, 5} { // A itself, A1, A1a, A2 are inside A's block
		if _, err := SetParent(p, 2, p, parent); !errors.Is(err, ErrSetParent) {
			t.Fatalf("parent %d: want ErrSetParent, got %v", parent, err)
		}
	}
	if readTmp(t, p) != before {
		t.Fatal("file changed despite rejected request")
	}
	if _, err := SetParent(p, 1, p, 2); !errors.Is(err, ErrSetParent) { // line 1 is a heading
		t.Fatalf("non-task source: got %v", err)
	}
	if _, err := SetParent(p, 7, p, 1); !errors.Is(err, ErrSetParent) { // parent is a heading
		t.Fatalf("non-task parent: got %v", err)
	}
	if _, err := SetParent(p, 7, p, 99); !errors.Is(err, ErrSetParent) {
		t.Fatalf("parent out of range: got %v", err)
	}
}

func TestSetParentAcrossFiles(t *testing.T) {
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))
	dir := t.TempDir()
	src := writeTmp(t, dir, "L.md", "# L\n- [ ] X #ToDo\n    - [ ] X1\n- [ ] Y\n")
	dst := writeTmp(t, dir, "M.md", "# M\n- [ ] P\n- [ ] Q\n")
	line, err := SetParent(src, 2, dst, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, src); got != "# L\n- [ ] Y\n" {
		t.Fatalf("source not trimmed: %q", got)
	}
	if got := readTmp(t, dst); got != "# M\n- [ ] P\n    - [ ] X #ToDo\n        - [ ] X1\n- [ ] Q\n" {
		t.Fatalf("destination wrong: %q", got)
	}
	if line != 3 {
		t.Fatalf("new line = %d, want 3", line)
	}
}

func TestSetParentPromote(t *testing.T) {
	// A1 (with A1a) becomes top level, placed after A's whole subtree.
	_, p := setupTree(t)
	line, err := SetParent(p, 3, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := `# L
- [ ] A #ToDo
    - [ ] A2
- [ ] A1
    - [ ] A1a
- [ ] B #ToDo
- [ ] C
- [ ] D
    - [ ] D1
`
	if got := readTmp(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if line != 4 {
		t.Fatalf("new line = %d, want 4", line)
	}
	// Already top level: untouched.
	before := readTmp(t, p)
	if l, err := SetParent(p, 6, p, 0); err != nil || l != 6 || readTmp(t, p) != before {
		t.Fatalf("top-level promote should be a no-op: line=%d err=%v", l, err)
	}
}

func TestSetParentPreservesFileWithoutTrailingNewline(t *testing.T) {
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "lock"))
	p := writeTmp(t, t.TempDir(), "L.md", "# L\n- [ ] A\n- [ ] B")
	if _, err := SetParent(p, 3, p, 2); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, p); got != "# L\n- [ ] A\n    - [ ] B" {
		t.Fatalf("got %q", got)
	}
}

func parseTree(t *testing.T, content string) []Task {
	t.Helper()
	p := writeTmp(t, t.TempDir(), "L.md", content)
	ts, err := parseFile(p, "L.md")
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestKanbanCardsFoldSubtasksIntoTheirParent(t *testing.T) {
	ts := parseTree(t, tree)
	cards := KanbanCardsIn(ts, KanbanTags)
	if len(cards) != 2 || cards[0].Title != "A" || cards[1].Title != "B" {
		t.Fatalf("cards = %+v", cards)
	}
	subs := cards[0].Subtasks
	if len(subs) != 3 || subs[0].Title != "A1" || subs[0].Level != 1 || subs[1].Title != "A1a" || subs[1].Level != 2 || subs[2].Title != "A2" || subs[2].Level != 1 {
		t.Fatalf("A.Subtasks = %+v", subs)
	}
	if len(cards[1].Subtasks) != 0 {
		t.Fatalf("B should have no subtasks: %+v", cards[1].Subtasks)
	}
}

func TestKanbanCardsKeepsATaggedSubtaskWhoseParentIsOffTheBoard(t *testing.T) {
	ts := parseTree(t, "# L\n- [ ] Epic\n    - [ ] Child #ToDo\n        - [ ] Grandchild\n")
	cards := KanbanCardsIn(ts, KanbanTags)
	if len(cards) != 1 || cards[0].Title != "Child" {
		t.Fatalf("Child must stay a card when Epic is off the board: %+v", cards)
	}
	if len(cards[0].Subtasks) != 1 || cards[0].Subtasks[0].Title != "Grandchild" || cards[0].Subtasks[0].Level != 1 {
		t.Fatalf("Grandchild should be Child's subtask: %+v", cards[0].Subtasks)
	}
}

func TestKanbanCardsTopmostOnBoardAncestorOwnsDescendants(t *testing.T) {
	ts := parseTree(t, "# L\n- [ ] Top #ToDo\n    - [ ] Mid #InProgress\n        - [ ] Leaf\n")
	cards := KanbanCardsIn(ts, KanbanTags)
	if len(cards) != 1 || cards[0].Title != "Top" {
		t.Fatalf("only Top is a card: %+v", cards)
	}
	if len(cards[0].Subtasks) != 2 {
		t.Fatalf("Top should own Mid and Leaf: %+v", cards[0].Subtasks)
	}
}

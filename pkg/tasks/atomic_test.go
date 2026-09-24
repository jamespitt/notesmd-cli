package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWriteFileAtomicPreservesModeAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.md")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Fatalf("content = %q", got)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, want 0600 preserved", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestWriteFileAtomicWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.md")
	link := filepath.Join(dir, "link.md")
	os.WriteFile(target, []byte("old"), 0644)
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unsupported")
	}
	if err := writeFileAtomic(link, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced by a regular file")
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Fatalf("target = %q", got)
	}
}

// A mutator must wait for another process (sync.py / git.sh) holding the
// vault lock instead of racing it.
func TestMutatorWaitsForVaultLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "vault.lock")
	t.Setenv("TASK_VAULT_LOCK", lockPath)

	file := filepath.Join(t.TempDir(), "list.md")
	os.WriteFile(file, []byte("# L\n\n- [ ] a"), 0644)

	foreign, _ := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	defer foreign.Close()
	syscall.Flock(int(foreign.Fd()), syscall.LOCK_EX)

	done := make(chan error, 1)
	go func() { done <- AppendTask(file, "b") }()

	select {
	case err := <-done:
		t.Fatalf("AppendTask returned while lock held: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if got, _ := os.ReadFile(file); strings.Contains(string(got), "- [ ] b") {
		t.Fatal("write happened while lock was held")
	}

	syscall.Flock(int(foreign.Fd()), syscall.LOCK_UN)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); !strings.Contains(string(got), "- [ ] b") {
		t.Fatalf("task not appended: %q", got)
	}
}

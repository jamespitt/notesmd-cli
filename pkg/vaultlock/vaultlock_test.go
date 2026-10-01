package vaultlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockBlocksGoroutinesUntilReleased(t *testing.T) {
	t.Setenv("TASK_VAULT_LOCK", filepath.Join(t.TempDir(), "sub", "vault.lock"))

	release, err := Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockTimeout(150 * time.Millisecond); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("second Lock should time out while held, got %v", err)
	}
	release()
	r2, err := LockTimeout(time.Second)
	if err != nil {
		t.Fatalf("Lock should succeed after release: %v", err)
	}
	r2()
}

// A separate open file description holding the lock stands in for another
// process (the Python sync or git.sh).
func TestLockWaitsForForeignHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.lock")
	t.Setenv("TASK_VAULT_LOCK", path)

	foreign, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	if err := tryLock(foreign); err != nil {
		t.Fatal(err)
	}

	if _, err := LockTimeout(200 * time.Millisecond); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("expected busy error while foreign lock held, got %v", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		unlock(foreign)
	}()
	release, err := LockTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("should acquire once foreign holder releases: %v", err)
	}
	release()
	release() // idempotent
}

// Both hosts mount ~/src; their home directories are separate, so the
// default lock file has to be under ~/src.
func TestDefaultPathIsOnTheSharedTree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TASK_VAULT_LOCK", "")
	t.Setenv("HOME", home)
	if got, want := Path(), filepath.Join(home, "src", ".task_vault.lock"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

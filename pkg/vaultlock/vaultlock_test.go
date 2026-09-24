package vaultlock

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// A separate open file description holding flock stands in for another
// process (the Python sync or git.sh).
func TestLockWaitsForForeignHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.lock")
	t.Setenv("TASK_VAULT_LOCK", path)

	foreign, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	if err := syscall.Flock(int(foreign.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}

	if _, err := LockTimeout(200 * time.Millisecond); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("expected busy error while foreign lock held, got %v", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		syscall.Flock(int(foreign.Fd()), syscall.LOCK_UN)
	}()
	release, err := LockTimeout(2 * time.Second)
	if err != nil {
		t.Fatalf("should acquire once foreign holder releases: %v", err)
	}
	release()
	release() // idempotent
}

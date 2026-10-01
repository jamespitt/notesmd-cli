// Package vaultlock serialises everything that rewrites task files in the
// vault: the Python sync (tasks/src/vault_lock.py), the git.sh auto-commit
// job, and notesmd-cli's own task mutators.
//
// They all take an exclusive lock on one shared lock file, so a
// read-modify-write in one process can never interleave with another's.
//
// The writers run on two hosts that share ~/src over NFS, so the lock file
// lives under ~/src (home directories aren't shared) and, on Linux, the lock
// is an open-file-description fcntl lock rather than flock(2): a flock taken
// on the NFS server's local disk and one taken through an NFS mount don't
// see each other, fcntl locks do. The lock path and lock type must match
// tasks/src/vault_lock.py (which git.sh locks through).
package vaultlock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// DefaultTimeout bounds how long a caller waits for the lock. A full sync
// run can hold it for a couple of minutes.
const DefaultTimeout = 3 * time.Minute

// errHeld is what tryLock returns while another holder has the lock.
var errHeld = errors.New("vault lock held elsewhere")

// sem serialises goroutines within this process with a bounded wait.
var sem = make(chan struct{}, 1)

// Path returns the shared lock file path: $TASK_VAULT_LOCK, else
// ~/src/.task_vault.lock.
func Path() string {
	if p := os.Getenv("TASK_VAULT_LOCK"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, "src", ".task_vault.lock")
}

// TryLockFile takes the vault lock's kind of lock on an already-open file
// without waiting, and UnlockFile drops it. They exist so tests elsewhere can
// stand in for a foreign holder (the Python sync, git.sh); everything else
// should use Lock.
func TryLockFile(f *os.File) error { return tryLock(f) }

// UnlockFile releases a lock taken with TryLockFile.
func UnlockFile(f *os.File) error { return unlock(f) }

// Lock acquires the vault lock, waiting up to DefaultTimeout, and returns a
// release func. It is not reentrant: never call it while already holding it.
func Lock() (release func(), err error) {
	return LockTimeout(DefaultTimeout)
}

// LockTimeout is Lock with an explicit wait limit.
func LockTimeout(timeout time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)
	path := Path()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case sem <- struct{}{}:
	case <-timer.C:
		return nil, fmt.Errorf("vault busy: timed out after %s waiting for %s", timeout, path)
	}
	unlockSem := func() { <-sem }

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		unlockSem()
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		unlockSem()
		return nil, err
	}

	for {
		err = tryLock(f)
		if err == nil {
			break
		}
		if !errors.Is(err, errHeld) && !errors.Is(err, syscall.EINTR) {
			f.Close()
			unlockSem()
			return nil, err
		}
		if time.Now().After(deadline) {
			f.Close()
			unlockSem()
			return nil, fmt.Errorf("vault busy: timed out after %s waiting for %s", timeout, path)
		}
		time.Sleep(100 * time.Millisecond)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = unlock(f)
			f.Close()
			unlockSem()
		})
	}, nil
}

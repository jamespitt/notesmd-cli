//go:build !linux

package vaultlock

import (
	"errors"
	"os"
	"syscall"
)

// tryLock falls back to flock(2) where OFD locks don't exist. That only
// excludes other holders on the same machine's local filesystem.
func tryLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errHeld
	}
	return err
}

func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

//go:build linux

package vaultlock

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// whole is a lock over the whole file. Pid must stay 0 for OFD locks.
func whole(lockType int16) *unix.Flock_t {
	return &unix.Flock_t{Type: lockType, Whence: io.SeekStart}
}

// tryLock takes an exclusive open-file-description lock without blocking.
func tryLock(f *os.File) error {
	err := unix.FcntlFlock(f.Fd(), unix.F_OFD_SETLK, whole(unix.F_WRLCK))
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EACCES) {
		return errHeld
	}
	return err
}

func unlock(f *os.File) error {
	return unix.FcntlFlock(f.Fd(), unix.F_OFD_SETLK, whole(unix.F_UNLCK))
}

// Package filelock takes an advisory lock that the system releases when its
// process exits, so that copies of go-voice with different bundle
// identifiers (debug and release) can tell which one owns a resource.
package filelock

import (
	"errors"
	"os"
	"syscall"
)

// ErrBusy is a lock that another open file, of this or another process,
// holds.
var ErrBusy = errors.New("filelock: held elsewhere")

// Lock is a held lock; Release gives it back.
type Lock struct{ f *os.File }

// TryLock takes the lock on path without waiting, creating the file.
func TryLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return &Lock{f}, nil
}

// Release gives the lock back; a nil lock is released already.
func (l *Lock) Release() {
	if l != nil && l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
}

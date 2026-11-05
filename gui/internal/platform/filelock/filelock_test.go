package filelock

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestOnlyOneHolderUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hotkey.lock")
	first, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TryLock(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second holder: %v", err)
	}
	first.Release()
	first.Release()
	second, err := TryLock(path)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	second.Release()
}

//go:build darwin || linux

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// AcquireSnapshotLock serializes git operations for one ctrlz store. Snapshot
// writers use a shared lock; a restore uses an exclusive lock across both its
// planning and apply phases. The lock is advisory, so every ctrlz git writer
// must acquire one before touching the store or work tree.
func AcquireSnapshotLock(storePath string, exclusive bool) (func(), error) {
	path := filepath.Join(filepath.Dir(storePath), "operation.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening operation lock: %w", err)
	}
	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(f.Fd()), mode); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("locking project operation: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

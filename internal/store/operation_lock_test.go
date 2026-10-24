//go:build darwin || linux

package store

import (
	"testing"
	"time"
)

func TestExclusiveOperationLockWaitsForSnapshotLock(t *testing.T) {
	storePath, _ := newTestStore(t)
	releaseShared, err := AcquireSnapshotLock(storePath, false)
	if err != nil {
		t.Fatalf("AcquireSnapshotLock(shared): %v", err)
	}
	defer releaseShared()

	acquired := make(chan func(), 1)
	errs := make(chan error, 1)
	go func() {
		release, err := AcquireSnapshotLock(storePath, true)
		if err != nil {
			errs <- err
			return
		}
		acquired <- release
	}()

	select {
	case <-acquired:
		t.Fatal("exclusive lock acquired while shared lock was held")
	case err := <-errs:
		t.Fatalf("AcquireSnapshotLock(exclusive): %v", err)
	case <-time.After(80 * time.Millisecond):
	}
	releaseShared()
	select {
	case release := <-acquired:
		release()
	case err := <-errs:
		t.Fatalf("AcquireSnapshotLock(exclusive): %v", err)
	case <-time.After(time.Second):
		t.Fatal("exclusive lock did not acquire after shared lock released")
	}
}

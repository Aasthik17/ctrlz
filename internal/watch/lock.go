package watch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// lockInfo is the contents of a project's watch.lock file (SPEC.md
// section 5): the pid and start time of the watch process holding it.
type lockInfo struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// AcquireLock writes watch.lock at path, refusing if another live watch
// process already holds it (its recorded pid is still running). A lock left
// behind by a process that's no longer running is stale and reclaimed.
func AcquireLock(path string) (release func(), err error) {
	if existing, err := readLock(path); err == nil && processAlive(existing.PID) {
		return nil, fmt.Errorf("already watching this path (pid %d, started %s)", existing.PID, existing.StartedAt.Format(time.RFC3339))
	}

	info := lockInfo{PID: os.Getpid(), StartedAt: time.Now().UTC()}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding watch lock: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	return func() { os.Remove(path) }, nil
}

// Status reports whether a live watch process currently holds the lock at
// path, for `ctrlz status`. A missing lock file just means no watch is
// active, not an error.
func Status(path string) (active bool, pid int, err error) {
	info, err := readLock(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	return processAlive(info.PID), info.PID, nil
}

func readLock(path string) (*lockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info lockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// processAlive reports whether pid is a currently running process, treating
// any ambiguous result (e.g. permission denied to signal it) as alive so a
// stale lock is only ever reclaimed when we're sure it's actually stale.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrProcessDone) {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == syscall.ESRCH {
		return false
	}
	return true
}

package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// deadPID returns a pid guaranteed to no longer be running, by spawning and
// waiting on a trivial subprocess.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running helper process: %v", err)
	}
	return cmd.Process.Pid
}

func TestAcquireLockRefusesWhileHeld(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watch.lock")

	release, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = AcquireLock(lockPath)
	if err == nil {
		t.Fatal("expected a second AcquireLock on the same path to fail")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("pid %d", os.Getpid())) {
		t.Fatalf("expected error to name the holding pid, got: %v", err)
	}
}

func TestAcquireLockReclaimsStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watch.lock")

	stale := lockInfo{PID: deadPID(t), StartedAt: time.Now().Add(-time.Hour)}
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	release, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("expected a stale lock to be reclaimed, got: %v", err)
	}
	defer release()

	info, err := readLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("expected reclaimed lock to record our own pid %d, got %d", os.Getpid(), info.PID)
	}
}

func TestAcquireLockReleaseRemovesFile(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watch.lock")

	release, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	release()

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file removed after release, stat err=%v", err)
	}
}

func TestStatusNoLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watch.lock")

	active, pid, err := Status(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if active || pid != 0 {
		t.Fatalf("expected inactive status with no lock file, got active=%v pid=%d", active, pid)
	}
}

func TestStatusActiveLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watch.lock")

	release, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	active, pid, err := Status(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !active || pid != os.Getpid() {
		t.Fatalf("expected active status with our own pid, got active=%v pid=%d", active, pid)
	}
}

func TestStatusStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "watch.lock")

	stale := lockInfo{PID: deadPID(t), StartedAt: time.Now().Add(-time.Hour)}
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	active, _, err := Status(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("expected a stale lock's pid to report inactive")
	}
}

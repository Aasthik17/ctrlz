package watch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Aasthik17/ctrlz/internal/store"
)

// TestWatchCoreScenario mirrors Phase 4's "done when": run a long-running
// command under `watch`, edit files while it runs, and confirm the
// resulting snapshot history matches what actually happened, with no
// missed intervals and no duplicate empty snapshots.
func TestWatchCoreScenario(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workTree := t.TempDir()

	project, _, err := store.LookupOrCreate(workTree)
	if err != nil {
		t.Fatal(err)
	}
	storePath, err := store.ProjectStorePath(project.ID)
	if err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(workTree, "a.txt")
	if err := os.WriteFile(filePath, []byte("v0"), 0o644); err != nil {
		t.Fatal(err)
	}

	editsDone := make(chan struct{})
	go func() {
		defer close(editsDone)
		time.Sleep(150 * time.Millisecond)
		os.WriteFile(filePath, []byte("v1"), 0o644)
		time.Sleep(150 * time.Millisecond)
		os.WriteFile(filePath, []byte("v2"), 0o644)
	}()

	exitCode, err := Run(Options{
		StorePath: storePath,
		WorkTree:  workTree,
		Interval:  80 * time.Millisecond,
		Quiet:     true,
		Command:   []string{"sleep", "0.6"},
	})
	<-editsDone
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	snapshots, err := store.ListSnapshots(storePath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) < 2 {
		t.Fatalf("expected at least 2 snapshots (an in-flight edit plus the final one), got %d: %+v", len(snapshots), snapshots)
	}
	for _, s := range snapshots {
		if s.Reason != store.ReasonInterval {
			t.Errorf("expected every snapshot tagged %q, got %q", store.ReasonInterval, s.Reason)
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil || string(data) != "v2" {
		t.Fatalf("expected final file content v2, got %q err=%v", data, err)
	}

	out, err := exec.Command("git", "--git-dir="+storePath, "show", "HEAD:a.txt").Output()
	if err != nil {
		t.Fatalf("reading HEAD:a.txt: %v", err)
	}
	if string(out) != "v2" {
		t.Fatalf("expected the final snapshot to capture v2, HEAD:a.txt = %q", out)
	}
}

// TestWatchTakesBaselineSnapshot guards against a real gap found while
// building the launch demo: if an agent's very first action is destructive,
// there must already be a snapshot of the pre-existing state to undo to,
// even though fsnotify only ever reports changes from the moment watching
// starts.
func TestWatchTakesBaselineSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workTree := t.TempDir()

	project, _, err := store.LookupOrCreate(workTree)
	if err != nil {
		t.Fatal(err)
	}
	storePath, err := store.ProjectStorePath(project.ID)
	if err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(workTree, "important.txt")
	if err := os.WriteFile(filePath, []byte("pre-existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	exitCode, err := Run(Options{
		StorePath: storePath,
		WorkTree:  workTree,
		Interval:  50 * time.Millisecond,
		Quiet:     true,
		Command:   []string{"rm", "-f", "important.txt"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("expected important.txt deleted, stat err=%v", err)
	}

	plan, err := store.PrepareUndo(storePath, workTree, "")
	if err != nil {
		t.Fatalf("PrepareUndo: %v", err)
	}
	if plan.Summary.ToRestore != 1 {
		t.Fatalf("expected undo to restore the pre-existing file, ToRestore = %d", plan.Summary.ToRestore)
	}
	if err := store.ApplyUndo(storePath, workTree, plan.Target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil || string(data) != "pre-existing content" {
		t.Fatalf("expected important.txt restored, got %q err=%v", data, err)
	}
}

// TestWatchCommandRunsInWorkTree guards against a real bug found while
// building the launch demo: the wrapped command must run with its cwd set
// to WorkTree, not wherever the ctrlz process itself happens to be running
// from, or a relative-path command like `rm -f *` silently operates on the
// wrong directory.
func TestWatchCommandRunsInWorkTree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workTree := t.TempDir()

	project, _, err := store.LookupOrCreate(workTree)
	if err != nil {
		t.Fatal(err)
	}
	storePath, err := store.ProjectStorePath(project.ID)
	if err != nil {
		t.Fatal(err)
	}

	exitCode, err := Run(Options{
		StorePath: storePath,
		WorkTree:  workTree,
		Interval:  50 * time.Millisecond,
		Quiet:     true,
		Command:   []string{"sh", "-c", "pwd > pwd.txt"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	data, err := os.ReadFile(filepath.Join(workTree, "pwd.txt"))
	if err != nil {
		t.Fatalf("expected pwd.txt inside workTree (command ran there): %v", err)
	}

	resolvedWorkTree, err := filepath.EvalSymlinks(workTree)
	if err != nil {
		t.Fatal(err)
	}
	got := filepath.Clean(string(data[:len(data)-1])) // strip trailing newline from `pwd`
	resolvedGot, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedGot != resolvedWorkTree {
		t.Fatalf("command ran in %q, want %q", resolvedGot, resolvedWorkTree)
	}
}

func TestWatchPlainForegroundExitsZeroOnInterrupt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workTree := t.TempDir()

	project, _, err := store.LookupOrCreate(workTree)
	if err != nil {
		t.Fatal(err)
	}
	storePath, err := store.ProjectStorePath(project.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(workTree, "a.txt"), []byte("v0"), 0o644); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct {
		code int
		err  error
	})
	go func() {
		code, err := Run(Options{
			StorePath: storePath,
			WorkTree:  workTree,
			Interval:  50 * time.Millisecond,
			Quiet:     true,
		})
		done <- struct {
			code int
			err  error
		}{code, err}
	}()

	time.Sleep(100 * time.Millisecond)
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("Run: %v", result.err)
		}
		if result.code != 0 {
			t.Fatalf("exitCode = %d, want 0", result.code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after interrupt")
	}

	snapshots, err := store.ListSnapshots(storePath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected exactly 1 final snapshot, got %d", len(snapshots))
	}
}

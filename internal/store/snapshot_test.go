package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestStore creates a fresh bare store and an empty work tree, isolated
// under a temp HOME so DefaultIgnoreFile() doesn't touch the real ~/.ctrlz.
func newTestStore(t *testing.T) (storePath, workTree string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	storePath = filepath.Join(t.TempDir(), "store")
	if err := initBareStore(storePath); err != nil {
		t.Fatalf("initBareStore: %v", err)
	}
	workTree = t.TempDir()
	return storePath, workTree
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTakeSnapshotSkipsEmptyCommits(t *testing.T) {
	storePath, workTree := newTestStore(t)

	_, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil {
		t.Fatalf("TakeSnapshot on empty dir: %v", err)
	}
	if taken {
		t.Fatal("expected no commit for an empty work tree")
	}

	snapshots, err := ListSnapshots(storePath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots, got %d", len(snapshots))
	}
}

// TestTakeSnapshotSequence mirrors Phase 2's "done when": add a file, edit
// it, delete it, snapshotting after each step, and confirm a clean,
// correctly ordered history with zero empty/no-op commits in between.
func TestTakeSnapshotSequence(t *testing.T) {
	storePath, workTree := newTestStore(t)
	filePath := filepath.Join(workTree, "a.txt")

	writeFile(t, filePath, "hello")
	hash1, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil || !taken {
		t.Fatalf("snapshot 1: taken=%v err=%v", taken, err)
	}

	writeFile(t, filePath, "hello world")
	hash2, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil || !taken {
		t.Fatalf("snapshot 2: taken=%v err=%v", taken, err)
	}
	if hash2 == hash1 {
		t.Fatal("expected a new hash after editing the file")
	}

	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
	hash3, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil || !taken {
		t.Fatalf("snapshot 3: taken=%v err=%v", taken, err)
	}
	if hash3 == hash2 {
		t.Fatal("expected a new hash after deleting the file")
	}

	// A fourth call with nothing changed must not add a commit.
	if _, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil || taken {
		t.Fatalf("expected no-op snapshot to be skipped: taken=%v err=%v", taken, err)
	}

	snapshots, err := ListSnapshots(storePath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("expected exactly 3 snapshots (no empty commits), got %d", len(snapshots))
	}

	wantOrder := []string{hash3, hash2, hash1}
	for i, want := range wantOrder {
		if snapshots[i].Hash != want {
			t.Fatalf("snapshot[%d] = %s, want %s (newest first)", i, snapshots[i].Hash, want)
		}
		if snapshots[i].Reason != ReasonInterval {
			t.Fatalf("snapshot[%d].Reason = %q, want %q", i, snapshots[i].Reason, ReasonInterval)
		}
	}
}

func TestListSnapshotsLimit(t *testing.T) {
	storePath, workTree := newTestStore(t)
	filePath := filepath.Join(workTree, "a.txt")

	for i := 0; i < 5; i++ {
		writeFile(t, filePath, strings.Repeat("x", i+1))
		if _, _, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil {
			t.Fatal(err)
		}
	}

	snapshots, err := ListSnapshots(storePath, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected limit=2 to return 2 snapshots, got %d", len(snapshots))
	}
}

func TestTakeSnapshotHonorsDefaultIgnoreList(t *testing.T) {
	storePath, workTree := newTestStore(t)

	writeFile(t, filepath.Join(workTree, "node_modules", "pkg", "index.js"), "junk")
	writeFile(t, filepath.Join(workTree, "real.txt"), "keep me")

	hash, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil || !taken {
		t.Fatalf("taken=%v err=%v", taken, err)
	}

	out, err := runGitOutput(storePath, "", nil, "ls-tree", "-r", "--name-only", hash)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "node_modules") {
		t.Fatalf("expected node_modules/ to be excluded from snapshot, got tree:\n%s", out)
	}
	if !strings.Contains(out, "real.txt") {
		t.Fatalf("expected real.txt in snapshot, got tree:\n%s", out)
	}
}

func TestTakeSnapshotExcludesNestedGitDir(t *testing.T) {
	storePath, workTree := newTestStore(t)

	writeFile(t, filepath.Join(workTree, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(workTree, "real.txt"), "keep me")

	hash, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil || !taken {
		t.Fatalf("taken=%v err=%v", taken, err)
	}

	out, err := runGitOutput(storePath, "", nil, "ls-tree", "-r", "--name-only", hash)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ".git") {
		t.Fatalf("expected nested .git/ to be excluded from snapshot, got tree:\n%s", out)
	}
}

func TestDiff(t *testing.T) {
	storePath, workTree := newTestStore(t)
	filePath := filepath.Join(workTree, "file.txt")

	writeFile(t, filePath, "v1\n")
	hash1, _, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, filePath, "v2\n")
	hash2, _, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil {
		t.Fatal(err)
	}

	out, err := Diff(storePath, workTree, hash1, hash2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-v1") || !strings.Contains(out, "+v2") {
		t.Fatalf("expected diff between snapshots to show v1 -> v2, got:\n%s", out)
	}

	writeFile(t, filePath, "v3\n")
	out, err = Diff(storePath, workTree, hash2, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-v2") || !strings.Contains(out, "+v3") {
		t.Fatalf("expected diff against working tree to show v2 -> v3, got:\n%s", out)
	}
}

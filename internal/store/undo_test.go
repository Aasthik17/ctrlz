package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareUndoNoSnapshots(t *testing.T) {
	storePath, workTree := newTestStore(t)

	_, err := PrepareUndo(storePath, workTree, "")
	if err == nil {
		t.Fatal("expected an error when undoing a project with no snapshots")
	}
	if !strings.Contains(err.Error(), "no snapshots yet") {
		t.Fatalf("expected a clear 'no snapshots yet' message, got: %v", err)
	}
}

// TestUndoCoreScenario mirrors Phase 3's "done when": create files, snapshot,
// delete everything, undo, confirm everything is back, then undo again and
// confirm it reverts the undo itself (via the pre-undo snapshot) back to the
// deleted state.
func TestUndoCoreScenario(t *testing.T) {
	storePath, workTree := newTestStore(t)
	aPath := filepath.Join(workTree, "a.txt")
	bPath := filepath.Join(workTree, "b.txt")

	writeFile(t, aPath, "A")
	writeFile(t, bPath, "B")
	hashA, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil || !taken {
		t.Fatalf("initial snapshot: taken=%v err=%v", taken, err)
	}

	if err := os.Remove(aPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bPath); err != nil {
		t.Fatal(err)
	}

	plan, err := PrepareUndo(storePath, workTree, "")
	if err != nil {
		t.Fatalf("PrepareUndo: %v", err)
	}
	if !plan.PreUndoTaken {
		t.Fatal("expected a pre-undo safety snapshot of the deleted state")
	}
	if plan.Target != hashA {
		t.Fatalf("expected default target to be the pre-delete snapshot %s, got %s", hashA, plan.Target)
	}
	if plan.Summary.ToRestore != 2 {
		t.Fatalf("expected 2 files to be restored, got %d", plan.Summary.ToRestore)
	}

	if err := ApplyUndo(storePath, workTree, plan.Target); err != nil {
		t.Fatalf("ApplyUndo: %v", err)
	}

	data, err := os.ReadFile(aPath)
	if err != nil || string(data) != "A" {
		t.Fatalf("expected a.txt restored with content 'A', got %q err=%v", data, err)
	}
	data, err = os.ReadFile(bPath)
	if err != nil || string(data) != "B" {
		t.Fatalf("expected b.txt restored with content 'B', got %q err=%v", data, err)
	}

	// A second undo should revert the first undo itself, via the pre-undo
	// snapshot the first undo took of the deleted state.
	plan2, err := PrepareUndo(storePath, workTree, "")
	if err != nil {
		t.Fatalf("second PrepareUndo: %v", err)
	}
	if plan2.Target != plan.PreUndoHash {
		t.Fatalf("expected second undo's target to be the first undo's pre-undo snapshot %s, got %s", plan.PreUndoHash, plan2.Target)
	}

	if err := ApplyUndo(storePath, workTree, plan2.Target); err != nil {
		t.Fatalf("second ApplyUndo: %v", err)
	}

	if _, err := os.Stat(aPath); !os.IsNotExist(err) {
		t.Fatalf("expected a.txt to be deleted again after the second undo, stat err=%v", err)
	}
	if _, err := os.Stat(bPath); !os.IsNotExist(err) {
		t.Fatalf("expected b.txt to be deleted again after the second undo, stat err=%v", err)
	}
}

func TestUndoToSpecificSnapshot(t *testing.T) {
	storePath, workTree := newTestStore(t)
	filePath := filepath.Join(workTree, "file.txt")

	writeFile(t, filePath, "v1")
	hash1, _, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filePath, "v2")
	if _, _, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filePath, "v3")
	if _, _, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil {
		t.Fatal(err)
	}

	plan, err := PrepareUndo(storePath, workTree, hash1[:12])
	if err != nil {
		t.Fatalf("PrepareUndo --to %s: %v", hash1[:12], err)
	}
	if plan.Target != hash1 {
		t.Fatalf("expected --to short hash to resolve to %s, got %s", hash1, plan.Target)
	}

	if err := ApplyUndo(storePath, workTree, plan.Target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil || string(data) != "v1" {
		t.Fatalf("expected file.txt reverted to 'v1', got %q err=%v", data, err)
	}
}

func TestDiffSummaryCounts(t *testing.T) {
	storePath, workTree := newTestStore(t)
	aPath := filepath.Join(workTree, "a.txt")
	bPath := filepath.Join(workTree, "b.txt")
	dPath := filepath.Join(workTree, "d.txt")

	writeFile(t, aPath, "1")
	writeFile(t, bPath, "2")
	hash1, _, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, aPath, "1-modified")
	if err := os.Remove(bPath); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dPath, "4")
	hash2, _, err := TakeSnapshot(storePath, workTree, ReasonInterval)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := diffSummary(storePath, hash1, hash2)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ToModify != 1 {
		t.Errorf("ToModify = %d, want 1 (a.txt)", summary.ToModify)
	}
	if summary.ToRemove != 1 {
		t.Errorf("ToRemove = %d, want 1 (b.txt)", summary.ToRemove)
	}
	if summary.ToRestore != 1 {
		t.Errorf("ToRestore = %d, want 1 (d.txt)", summary.ToRestore)
	}
}

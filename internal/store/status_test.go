package store

import (
	"path/filepath"
	"testing"
)

func TestSnapshotCount(t *testing.T) {
	storePath, workTree := newTestStore(t)

	n, err := SnapshotCount(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 snapshots in a fresh store, got %d", n)
	}

	filePath := filepath.Join(workTree, "a.txt")
	for i, content := range []string{"1", "2", "3"} {
		writeFile(t, filePath, content)
		if _, _, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil {
			t.Fatal(err)
		}
		n, err := SnapshotCount(storePath)
		if err != nil {
			t.Fatal(err)
		}
		if n != i+1 {
			t.Fatalf("after %d snapshots, SnapshotCount = %d, want %d", i+1, n, i+1)
		}
	}

	// A no-op TakeSnapshot must not bump the count.
	if _, taken, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil || taken {
		t.Fatalf("expected no-op snapshot: taken=%v err=%v", taken, err)
	}
	n, err = SnapshotCount(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected count to stay at 3 after a no-op snapshot, got %d", n)
	}
}

func TestStoreSize(t *testing.T) {
	storePath, workTree := newTestStore(t)

	empty, err := StoreSize(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if empty <= 0 {
		t.Fatalf("expected a fresh bare store to already take up some disk space, got %d bytes", empty)
	}

	writeFile(t, filepath.Join(workTree, "a.txt"), "some real content to make the store grow noticeably in size")
	if _, _, err := TakeSnapshot(storePath, workTree, ReasonInterval); err != nil {
		t.Fatal(err)
	}

	after, err := StoreSize(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if after <= empty {
		t.Fatalf("expected store size to grow after a snapshot: before=%d after=%d", empty, after)
	}
}

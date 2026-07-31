package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupOrCreate(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	projectDir := t.TempDir()

	p1, created, err := LookupOrCreate(projectDir)
	if err != nil {
		t.Fatalf("LookupOrCreate: %v", err)
	}
	if !created {
		t.Fatal("expected first call to report created=true")
	}
	if p1.ID == "" {
		t.Fatal("expected non-empty project id")
	}
	if len(p1.ID) != 16 {
		t.Fatalf("expected 16-character hex id, got %q", p1.ID)
	}

	registryPath := filepath.Join(tmpHome, ".ctrlz", "registry.json")
	if _, err := os.Stat(registryPath); err != nil {
		t.Fatalf("expected registry.json to exist: %v", err)
	}

	storePath, err := ProjectStorePath(p1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storePath, "HEAD")); err != nil {
		t.Fatalf("expected bare git store at %s: %v", storePath, err)
	}

	p2, created2, err := LookupOrCreate(projectDir)
	if err != nil {
		t.Fatalf("LookupOrCreate second call: %v", err)
	}
	if created2 {
		t.Fatal("expected second call to report created=false")
	}
	if p2.ID != p1.ID {
		t.Fatalf("expected same project id, got %s and %s", p1.ID, p2.ID)
	}

	reg, err := loadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 1 {
		t.Fatalf("expected 1 project in registry after two calls, got %d", len(reg.Projects))
	}
}

func TestLookupOrCreateDistinctPaths(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dirA := t.TempDir()
	dirB := t.TempDir()

	pA, _, err := LookupOrCreate(dirA)
	if err != nil {
		t.Fatal(err)
	}
	pB, _, err := LookupOrCreate(dirB)
	if err != nil {
		t.Fatal(err)
	}
	if pA.ID == pB.ID {
		t.Fatalf("expected distinct project ids for distinct paths, got %s for both", pA.ID)
	}
}

func TestDefaultIgnoreFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	path, err := DefaultIgnoreFile()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range DefaultIgnorePatterns {
		if !strings.Contains(string(data), pattern) {
			t.Fatalf("expected default ignore file to contain %q, got: %s", pattern, data)
		}
	}
}

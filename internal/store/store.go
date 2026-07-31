// Package store manages ctrlz's on-disk state: the project registry and
// each project's bare git snapshot store. Nothing here ever writes inside
// a watched directory; everything lives under ~/.ctrlz.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	lockRetryInterval = 20 * time.Millisecond
	lockTimeout       = 5 * time.Second
)

// Project is a single watched directory tracked in the registry.
type Project struct {
	ID            string     `json:"-"`
	Path          string     `json:"path"`
	CreatedAt     time.Time  `json:"created_at"`
	LastWatchedAt *time.Time `json:"last_watched_at,omitempty"`
}

type registryFile struct {
	Projects map[string]*Project `json:"projects"`
}

// Dir returns ~/.ctrlz, creating it if it doesn't exist.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	dir := filepath.Join(home, ".ctrlz")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	return dir, nil
}

func registryPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "registry.json"), nil
}

func projectsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projects"), nil
}

// ProjectStorePath returns the path to a project's bare git store.
func ProjectStorePath(id string) (string, error) {
	pd, err := projectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(pd, id, "store"), nil
}

// ResolvePath resolves path to the absolute, symlink-free form used as the
// registry lookup key, so the same directory reached via different symlinks
// still matches the same project.
func ResolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	return resolved, nil
}

func loadRegistry() (*registryFile, error) {
	path, err := registryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &registryFile{Projects: map[string]*Project{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var reg registryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("%s is corrupted (invalid JSON), check it by hand: %w", path, err)
	}
	if reg.Projects == nil {
		reg.Projects = map[string]*Project{}
	}
	return &reg, nil
}

func saveRegistry(reg *registryFile) error {
	path, err := registryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding registry: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmp, path, err)
	}
	return nil
}

// acquireRegistryLock takes a simple exclusive-create file lock adequate for
// a single-user CLI: concurrent ctrlz invocations are rare, so this only
// needs to stop two of them from corrupting registry.json if they happen to
// race, not arbitrate long-held contention.
func acquireRegistryLock(dir string) (release func(), err error) {
	lockPath := filepath.Join(dir, "registry.lock")
	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquiring lock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for lock %s (another ctrlz process may be stuck)", lockPath)
		}
		time.Sleep(lockRetryInterval)
	}
}

// LookupOrCreate resolves path and returns its registered project, creating
// a new bare store and registry entry if one doesn't already exist. The
// second return value reports whether a new project was created.
func LookupOrCreate(path string) (*Project, bool, error) {
	resolved, err := ResolvePath(path)
	if err != nil {
		return nil, false, err
	}

	dir, err := Dir()
	if err != nil {
		return nil, false, err
	}
	release, err := acquireRegistryLock(dir)
	if err != nil {
		return nil, false, err
	}
	defer release()

	reg, err := loadRegistry()
	if err != nil {
		return nil, false, err
	}

	for id, p := range reg.Projects {
		if p.Path == resolved {
			p.ID = id
			return p, false, nil
		}
	}

	id, err := newProjectID()
	if err != nil {
		return nil, false, err
	}

	storePath, err := ProjectStorePath(id)
	if err != nil {
		return nil, false, err
	}
	if err := initBareStore(storePath); err != nil {
		return nil, false, err
	}

	project := &Project{
		ID:        id,
		Path:      resolved,
		CreatedAt: time.Now().UTC(),
	}
	reg.Projects[id] = project

	if err := saveRegistry(reg); err != nil {
		return nil, false, err
	}

	return project, true, nil
}

func newProjectID() (string, error) {
	b := make([]byte, 8) // 8 random bytes -> 16 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating project id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// initBareStore creates a bare git repository at storePath to use as a
// project's snapshot store, and sets its git identity so commits never
// depend on (or touch) the user's own global git config.
func initBareStore(storePath string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("ctrlz requires git on PATH, but it was not found: %w", err)
	}
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", storePath, err)
	}
	if err := runGit("", "init", "--bare", "--quiet", storePath); err != nil {
		return err
	}
	if err := runGit(storePath, "config", "user.email", "snapshots@ctrlz.local"); err != nil {
		return err
	}
	if err := runGit(storePath, "config", "user.name", "ctrlz"); err != nil {
		return err
	}
	return nil
}

func runGit(gitDir string, args ...string) error {
	var cmd *exec.Cmd
	if gitDir != "" {
		cmd = exec.Command("git", append([]string{"-C", gitDir}, args...)...)
	} else {
		cmd = exec.Command("git", args...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

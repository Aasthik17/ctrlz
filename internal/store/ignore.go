package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnorePatterns is baked into the binary per SPEC.md section 4.
// Applied via `-c core.excludesFile=<DefaultIgnoreFile>` on every
// `git add -A`, in addition to (not instead of) whatever .gitignore already
// exists in the watched directory.
var DefaultIgnorePatterns = []string{
	".git/",
	"node_modules/",
	".venv/",
	"venv/",
	"__pycache__/",
	".next/",
	".cache/",
	"target/",
	"dist/",
	"build/",
	"vendor/",
}

// DefaultIgnoreFile writes the ctrlz-managed excludesFile containing
// DefaultIgnorePatterns and returns its path. It rewrites the file on every
// call so it can't drift from the patterns baked into the running binary.
func DefaultIgnoreFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "ignore")
	content := strings.Join(DefaultIgnorePatterns, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

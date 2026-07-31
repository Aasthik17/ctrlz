package store

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Snapshot reasons, per SPEC.md section 2.
const (
	ReasonInterval = "interval"
	ReasonPreUndo  = "pre-undo"
)

// Snapshot is one commit in a project's store.
type Snapshot struct {
	Hash      string
	Timestamp time.Time
	Reason    string
	Stat      string // one-line file-change summary, e.g. "2 files changed, 10 insertions(+), 3 deletions(-)"
}

// gitError carries a command's stderr so callers can pattern-match on git's
// own wording (e.g. an empty-repo message) instead of just an exit code.
type gitError struct {
	args   []string
	stderr string
	err    error
}

func (e *gitError) Error() string {
	if e.stderr != "" {
		return fmt.Sprintf("git %s: %s", strings.Join(e.args, " "), e.stderr)
	}
	return fmt.Sprintf("git %s: %v", strings.Join(e.args, " "), e.err)
}

func (e *gitError) Unwrap() error { return e.err }

func gitArgs(storePath, workTree string, configs []string, args ...string) []string {
	full := make([]string, 0, len(args)+len(configs)*2+2)
	if storePath != "" {
		full = append(full, "--git-dir="+storePath)
	}
	if workTree != "" {
		full = append(full, "--work-tree="+workTree)
	}
	for _, c := range configs {
		full = append(full, "-c", c)
	}
	return append(full, args...)
}

// runGitOutput runs git and returns stdout, wrapping failures in a
// *gitError that preserves stderr for callers that need to inspect it.
func runGitOutput(storePath, workTree string, configs []string, args ...string) (string, error) {
	full := gitArgs(storePath, workTree, configs, args...)
	cmd := exec.Command("git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &gitError{args: args, stderr: strings.TrimSpace(stderr.String()), err: err}
	}
	return stdout.String(), nil
}

func isEmptyRepoError(err error) bool {
	var ge *gitError
	if !errors.As(err, &ge) {
		return false
	}
	// git phrases this differently depending on the command: `log` and
	// `diff --cached` say "does not have any commits yet"; `rev-parse HEAD`
	// says HEAD is an "unknown revision" instead.
	return strings.Contains(ge.stderr, "does not have any commits yet") ||
		strings.Contains(ge.stderr, "unknown revision or path not in the working tree")
}

// TakeSnapshot stages every change in workTree (honoring the watched
// directory's own .gitignore plus DefaultIgnorePatterns) and commits it to
// storePath tagged with reason. If nothing changed since the last snapshot,
// no commit is made and taken is false, so history never fills up with
// empty entries.
func TakeSnapshot(storePath, workTree, reason string) (hash string, taken bool, err error) {
	ignoreFile, err := DefaultIgnoreFile()
	if err != nil {
		return "", false, err
	}

	configs := []string{"core.excludesFile=" + ignoreFile}
	if _, err := runGitOutput(storePath, workTree, configs, "add", "-A"); err != nil {
		return "", false, fmt.Errorf("staging changes: %w", err)
	}

	full := gitArgs(storePath, workTree, nil, "diff", "--cached", "--quiet")
	if err := exec.Command("git", full...).Run(); err == nil {
		// Exit 0: nothing staged, nothing to commit.
		return "", false, nil
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return "", false, fmt.Errorf("checking staged changes: %w", err)
		}
		// Exit 1: staged changes present, fall through to commit.
	}

	if _, err := runGitOutput(storePath, workTree, nil, "commit", "-m", reason, "--quiet"); err != nil {
		return "", false, fmt.Errorf("committing snapshot: %w", err)
	}

	out, err := runGitOutput(storePath, "", nil, "rev-parse", "HEAD")
	if err != nil {
		return "", false, fmt.Errorf("resolving new snapshot hash: %w", err)
	}
	return strings.TrimSpace(out), true, nil
}

// ListSnapshots returns a project's snapshots newest-first. limit <= 0 means
// unlimited. A project with no snapshots yet returns an empty slice, not an
// error.
func ListSnapshots(storePath string, limit int) ([]Snapshot, error) {
	args := []string{"log", "--pretty=format:%H|%at|%s"}
	if limit > 0 {
		args = append(args, "-n", strconv.Itoa(limit))
	}
	out, err := runGitOutput(storePath, "", nil, args...)
	if err != nil {
		if isEmptyRepoError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}

	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil, nil
	}

	lines := strings.Split(out, "\n")
	snapshots := make([]Snapshot, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected git log output: %q", line)
		}
		epoch, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing snapshot timestamp: %w", err)
		}
		stat, err := snapshotStat(storePath, parts[0])
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, Snapshot{
			Hash:      parts[0],
			Timestamp: time.Unix(epoch, 0),
			Reason:    parts[2],
			Stat:      stat,
		})
	}
	return snapshots, nil
}

func snapshotStat(storePath, hash string) (string, error) {
	out, err := runGitOutput(storePath, "", nil, "show", "--stat", "--format=", hash)
	if err != nil {
		return "", fmt.Errorf("reading stat for %s: %w", hash, err)
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return "", nil
	}
	lines := strings.Split(out, "\n")
	return strings.TrimSpace(lines[len(lines)-1]), nil
}

// Diff returns a unified diff between snapshots a and b. If b is empty, it
// diffs a against the current state of workTree instead.
func Diff(storePath, workTree, a, b string) (string, error) {
	if b == "" {
		out, err := runGitOutput(storePath, workTree, nil, "diff", a)
		if err != nil {
			return "", fmt.Errorf("diffing %s against the working tree: %w", a, err)
		}
		return out, nil
	}
	out, err := runGitOutput(storePath, "", nil, "diff", a, b)
	if err != nil {
		return "", fmt.Errorf("diffing %s against %s: %w", a, b, err)
	}
	return out, nil
}

// ChangeSummary counts what an undo would do to workTree, relative to the
// state a snapshot was taken from.
type ChangeSummary struct {
	ToRestore int // present in the target, missing now: will be brought back
	ToRemove  int // present now, missing in the target: will be deleted
	ToModify  int // present in both, different content: will be overwritten
}

// UndoPlan is the result of preparing an undo: everything computed and
// committed up front, before anything destructive happens.
type UndoPlan struct {
	Original     string // HEAD before anything in this undo happened
	PreUndoHash  string // hash of the pre-undo safety snapshot, "" if none was needed
	PreUndoTaken bool
	Target       string // resolved commit that will be reverted to
	Summary      ChangeSummary
}

// PrepareUndo performs the non-destructive steps of an undo (SPEC.md
// section 3, steps 1-4): it captures the current HEAD, unconditionally
// takes a pre-undo safety snapshot of however messy the current state is,
// resolves the target ref (the most recent snapshot by default, or --to),
// and summarizes what applying the plan would change. It does not touch
// workTree; call ApplyUndo with the returned Target to actually revert.
func PrepareUndo(storePath, workTree, to string) (*UndoPlan, error) {
	head, err := runGitOutput(storePath, "", nil, "rev-parse", "HEAD")
	if err != nil {
		if isEmptyRepoError(err) {
			return nil, errors.New("no snapshots yet, nothing to undo")
		}
		return nil, fmt.Errorf("resolving current state: %w", err)
	}
	original := strings.TrimSpace(head)

	preUndoHash, preUndoTaken, err := TakeSnapshot(storePath, workTree, ReasonPreUndo)
	if err != nil {
		return nil, fmt.Errorf("taking pre-undo safety snapshot: %w", err)
	}

	target := original
	if to != "" {
		resolved, err := runGitOutput(storePath, "", nil, "rev-parse", to)
		if err != nil {
			return nil, fmt.Errorf("resolving --to %s: %w", to, err)
		}
		target = strings.TrimSpace(resolved)
	}

	currentHead := original
	if preUndoTaken {
		currentHead = preUndoHash
	}

	summary, err := diffSummary(storePath, currentHead, target)
	if err != nil {
		return nil, err
	}

	return &UndoPlan{
		Original:     original,
		PreUndoHash:  preUndoHash,
		PreUndoTaken: preUndoTaken,
		Target:       target,
		Summary:      summary,
	}, nil
}

// diffSummary counts, going from -> to, files that will be added back
// (present in to, missing in from), removed (present in from, missing in
// to), and modified (present in both with different content).
func diffSummary(storePath, from, to string) (ChangeSummary, error) {
	out, err := runGitOutput(storePath, "", nil, "diff", "--name-status", from, to)
	if err != nil {
		return ChangeSummary{}, fmt.Errorf("comparing current state to %s: %w", to, err)
	}
	var s ChangeSummary
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return s, nil
	}
	for _, line := range strings.Split(out, "\n") {
		status, _, ok := strings.Cut(line, "\t")
		if !ok || status == "" {
			continue
		}
		switch status[0] {
		case 'A':
			s.ToRestore++
		case 'D':
			s.ToRemove++
		default: // M, T, etc.
			s.ToModify++
		}
	}
	return s, nil
}

// ApplyUndo performs the destructive half of an undo (SPEC.md section 3,
// step 6): it resets workTree to exactly match target, restoring anything
// deleted or modified since, and removing anything added since. Both
// commands are required together; neither alone produces a full revert.
func ApplyUndo(storePath, workTree, target string) error {
	if _, err := runGitOutput(storePath, workTree, nil, "read-tree", "--reset", "-u", target); err != nil {
		return fmt.Errorf("restoring files from %s: %w", target, err)
	}
	if _, err := runGitOutput(storePath, workTree, nil, "clean", "-fd"); err != nil {
		return fmt.Errorf("cleaning up files added since %s: %w", target, err)
	}
	return nil
}

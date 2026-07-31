// Package watch runs ctrlz's ticker loop: it periodically snapshots a
// project's work tree while an agent (or a plain foreground session) runs,
// using fsnotify only to cheaply skip ticks where nothing changed.
package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Aasthik17/ctrlz/internal/store"
)

// Options configures a watch run.
type Options struct {
	StorePath string
	WorkTree  string
	Interval  time.Duration
	Quiet     bool
	Command   []string // empty means plain foreground mode
}

// Run watches WorkTree until either Command exits or (with no Command) the
// process is interrupted. It takes an unconditional baseline snapshot of
// whatever already exists before anything runs, an `interval` snapshot on
// every tick that fsnotify saw a change since the last one, and one more
// unconditional snapshot when it stops. It returns the exit code the ctrlz
// process itself should use.
func Run(opts Options) (exitCode int, err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return 1, fmt.Errorf("starting filesystem watcher: %w", err)
	}
	defer watcher.Close()

	if err := addRecursive(watcher, opts.WorkTree); err != nil {
		return 1, fmt.Errorf("watching %s: %w", opts.WorkTree, err)
	}

	// fsnotify only ever reports changes from here on, so whatever already
	// exists in WorkTree at startup would otherwise never become a
	// snapshot: nothing would mark a tick dirty for it. Without this
	// baseline, an agent's very first destructive action could become the
	// *first* snapshot ever taken, leaving nothing earlier to undo to.
	if _, _, err := store.TakeSnapshot(opts.StorePath, opts.WorkTree, store.ReasonInterval); err != nil {
		return 1, fmt.Errorf("taking baseline snapshot: %w", err)
	}

	var dirty atomic.Bool
	go watchEvents(watcher, &dirty)

	ctx, cancel := context.WithCancel(context.Background())
	tickerDone := make(chan struct{})
	go func() {
		tickerLoop(ctx, opts, &dirty)
		close(tickerDone)
	}()

	if !opts.Quiet {
		fmt.Printf("ctrlz watching %s (every %s)\n", opts.WorkTree, opts.Interval)
	}

	if len(opts.Command) > 0 {
		exitCode, err = runCommand(opts.Command, opts.WorkTree)
	} else {
		exitCode, err = waitForInterrupt()
	}

	cancel()
	<-tickerDone

	if _, _, snapErr := store.TakeSnapshot(opts.StorePath, opts.WorkTree, store.ReasonInterval); snapErr != nil && err == nil {
		err = fmt.Errorf("taking final snapshot: %w", snapErr)
	}

	return exitCode, err
}

// ignoredDirName reports whether a directory named name should be skipped
// when watching, matching store.DefaultIgnorePatterns so ctrlz doesn't
// waste fsnotify watches (or hit OS watch-count limits) on huge trees like
// node_modules that are never going to be snapshotted anyway.
func ignoredDirName(name string) bool {
	for _, p := range store.DefaultIgnorePatterns {
		if strings.TrimSuffix(p, "/") == name {
			return true
		}
	}
	return false
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// Removed after the walk reached it (e.g. the agent
				// deleted it); nothing left to watch here.
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && ignoredDirName(d.Name()) {
			return filepath.SkipDir
		}
		if err := w.Add(path); err != nil {
			return fmt.Errorf("watching %s: %w", path, err)
		}
		return nil
	})
}

// watchEvents marks dirty on any filesystem event and, best-effort, starts
// watching newly created directories so later changes inside them are seen
// too. fsnotify is only ever an optimization here (SPEC.md section 5): the
// ticker is the timing guarantee, and the unconditional final snapshot in
// Run covers anything this loop misses.
func watchEvents(w *fsnotify.Watcher, dirty *atomic.Bool) {
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			dirty.Store(true)
			if event.Has(fsnotify.Create) && !ignoredDirName(filepath.Base(event.Name)) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = addRecursive(w, event.Name)
				}
			}
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

func tickerLoop(ctx context.Context, opts Options, dirty *atomic.Bool) {
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !dirty.Swap(false) {
				continue
			}
			if _, _, err := store.TakeSnapshot(opts.StorePath, opts.WorkTree, store.ReasonInterval); err != nil && !opts.Quiet {
				fmt.Fprintf(os.Stderr, "ctrlz: snapshot failed: %v\n", err)
			}
		}
	}
}

func runCommand(command []string, dir string) (int, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, fmt.Errorf("running %s: %w", command[0], err)
}

func waitForInterrupt() (int, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	<-sigCh
	return 0, nil
}

# Using ctrlz

A practical, command-by-command guide. For the exact internal behavior (what git plumbing runs, how conflicts are resolved, storage layout), see [`SPEC.md`](SPEC.md) instead — this doc is the "what do I type and what happens" version.

## Quick start

```
cd your-project
ctrlz watch -- claude-code
```

That one line registers the directory, starts taking snapshots every 5 seconds, and runs `claude-code` in the foreground. Work normally. If the agent does something bad:

```
ctrlz undo
```

That's the whole workflow. Everything below is detail on top of it.

## Command reference

| Command | What it does |
|---|---|
| `ctrlz init [path]` | Starts tracking a directory, without watching it yet. |
| `ctrlz watch [path] [-- command...]` | Takes automatic snapshots on a timer, optionally while running a wrapped command. |
| `ctrlz undo [path]` | Reverts the directory back to a prior snapshot. |
| `ctrlz log [path]` | Lists past snapshots, newest first. |
| `ctrlz diff [path] <a> [b]` | Shows exactly what changed between two snapshots (or a snapshot and now). |
| `ctrlz status [path]` | Shows a quick health check: how many snapshots exist, how big the store is, whether watching is active right now. |

`path` defaults to the current directory everywhere, so in practice you'll mostly run these with no arguments from inside the project.

### `ctrlz init [path]`

Registers `path` with ctrlz and creates its snapshot store, without starting to watch anything yet.

```
ctrlz init
```

You don't normally need this on its own — `ctrlz watch` calls it automatically the first time it sees a new directory. It's here for cases where you want the project registered (so `ctrlz status` works, say) before you're ready to start watching. Safe to run more than once; it just tells you the project already exists.

### `ctrlz watch [path] [--interval 5s] [--quiet] [-- command...]`

Starts protecting a directory. This is the command you actually run day to day.

```
ctrlz watch -- claude-code          # wrap an agent — stops watching when it exits
ctrlz watch                         # watch with nothing wrapped — Ctrl+C to stop
ctrlz watch --interval 10s -- codex # snapshot every 10s instead of the default 5s
```

- Everything after `--` is run as the wrapped command. Its own terminal output shows through normally; ctrlz doesn't get in the way of it.
- Takes one snapshot immediately at startup (so there's always something to undo to, even before the agent does anything), then one more every interval a change is detected, then one final snapshot when the wrapped command exits.
- Only one `watch` can run per directory at a time — starting a second one just tells you the first is still running.
- `--quiet` silences even the one-line "now watching" message ctrlz normally prints at startup.

### `ctrlz undo [path] [--to <ref>] [--yes]`

Reverts the directory to a prior snapshot. Deleted files come back, files added since get removed, modified files are restored.

```
ctrlz undo                  # revert to the most recent snapshot
ctrlz undo --yes            # same, but skip the y/n confirmation
ctrlz undo --to a1b2c3d     # revert to a specific snapshot (hash from `ctrlz log`)
ctrlz undo --to HEAD~2      # revert to two snapshots back
```

Before changing anything, it always takes a safety snapshot of the current state first — so `ctrlz undo` immediately followed by a second `ctrlz undo` undoes the undo, back to exactly where you were. Without `--yes`, it prints a summary of what's about to change (files restored / removed / modified) and asks for confirmation before touching anything.

### `ctrlz log [path] [--limit 20]`

Shows the snapshot timeline, newest first: short hash, how long ago, why it was taken, and a one-line summary of what changed.

```
ctrlz log
ctrlz log --limit 50   # show more than the default 20
```

Use this to find the hash you want to pass to `ctrlz diff` or `ctrlz undo --to`.

### `ctrlz diff [path] <a> [b]`

Shows exactly what changed, as a normal unified diff.

```
ctrlz diff a1b2c3d              # that snapshot vs. the current working directory
ctrlz diff a1b2c3d e4f5a6b       # between two specific snapshots
```

Handy before running `undo`, to double check what a revert will actually touch, or just to understand what an agent did in a given stretch of time.

### `ctrlz status [path]`

A quick, no-argument health check.

```
ctrlz status
```

Prints the project id, total snapshot count, store size on disk, time of the last snapshot, and whether a `watch` is currently running for this directory. Good first command to run if something seems off, or to confirm ctrlz is actually protecting the directory before you trust it.

## Typical workflow

```
cd your-project
ctrlz watch -- claude-code   # start the agent under protection
# ... agent does its thing ...
# something looks wrong
ctrlz status                 # confirm snapshots were actually taken
ctrlz log                    # find the point to go back to
ctrlz diff <hash>             # sanity-check what would change
ctrlz undo --to <hash>       # revert to it
```

## Notes and gotchas

- ctrlz never needs the directory to be a git repo, and if it is one, ctrlz's own history lives entirely separately from it — undoing never touches your real git history.
- The snapshot store lives outside the watched directory (`~/.ctrlz/`), so it survives even a full `rm -rf` of the project itself. See "Storage layout" in [`SPEC.md`](SPEC.md#1-storage-layout) for exactly where.
- ctrlz respects the project's own `.gitignore` in addition to its own default ignore list (`node_modules/`, `.venv/`, `dist/`, etc. — full list in [`SPEC.md`](SPEC.md#4-default-ignore-list)). A `.gitignore` rule that excludes a file from your repo also excludes it from ctrlz's protection — see the caveat in [`SPEC.md`](SPEC.md#2-snapshot-mechanism-verified).
- Requires the `git` binary on `PATH`. ctrlz uses it purely as an internal storage engine — you don't run any git commands yourself.

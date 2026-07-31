# ctrlz — technical specification (v1)

This is the reference for exact behavior. `IMPLEMENTATION_PLAN.md` is the roadmap; this is what each piece actually does. Where this document gives an exact command, it has been run and checked against real git, not just reasoned about.

## 1. Storage layout

All ctrlz state lives under `~/.ctrlz/`. Nothing is ever written inside a watched directory.

```
~/.ctrlz/
  registry.json
  projects/
    <id>/
      store/            a bare git repository (git init --bare); used only as an object + commit store
      watch.lock         present only while a watch process is active for this project (see section 5)
```

### registry.json

```json
{
  "projects": {
    "3f9a1c2b8e7d4a10": {
      "path": "/Users/aasthik/code/some-project",
      "created_at": "2026-07-29T10:00:00Z",
      "last_watched_at": "2026-07-29T10:31:00Z"
    }
  }
}
```

- The key is a 16-character random hex id generated at creation, not derived from the path, so renaming or moving the watched directory doesn't orphan its history. Re-pointing an existing id at a new path (after a move) is a v1.1 concern; for v1, a moved directory is treated as a new, separate project.
- Lookup is by resolved absolute path. Every command resolves the given path (or cwd) with `filepath.Abs` + `filepath.EvalSymlinks` before comparing against the registry, so the same directory reached via a different symlink still matches the same entry.
- If a path isn't found in the registry, treat it as "not yet initialized" and auto-create an entry. This is what lets `ctrlz watch` work standalone without a required separate `init` step first.

## 2. Snapshot mechanism (verified)

Every snapshot operation runs against `~/.ctrlz/projects/<id>/store` as `--git-dir`, with the watched directory as `--work-tree`. The store's local git identity is set once at creation time:

```
git -C <store> config user.email "snapshots@ctrlz.local"
git -C <store> config user.name  "ctrlz"
```

This is set on the store itself, never the user's global git config, so ctrlz works even on a machine with no git identity configured, and never touches the user's own identity.

**Taking a snapshot:**

```
git --git-dir=<store> --work-tree=<dir> -c core.excludesFile=<ctrlz-default-ignores> add -A
git --git-dir=<store> --work-tree=<dir> diff --cached --quiet   # exit 0 = nothing staged, skip the commit
git --git-dir=<store> --work-tree=<dir> commit -m "<reason>" --quiet
```

Verified directly: a `.git` directory that happens to exist inside `<dir>` (the watched directory is itself a real git repo) is automatically excluded by `add -A` and never appears in the snapshot, with no special-casing needed. Verified directly: a `.gitignore` file inside `<dir>` is read and honored normally even though the git metadata lives elsewhere, so `node_modules/`-style excludes work with zero extra code, on top of the ctrlz default list below.

`<reason>` is one of: `interval` (a regular tick from `ctrlz watch`), `pre-undo` (the automatic safety snapshot taken immediately before an undo is applied).

**Listing snapshots:**

```
git --git-dir=<store> log --pretty=format:'%H|%at|%s' -n <limit>
git --git-dir=<store> show --stat --format= <hash>    # per-snapshot file change summary
```

**Diffing:**

```
git --git-dir=<store> diff <a> <b>                             # between two snapshots
git --git-dir=<store> --work-tree=<dir> diff <a>                # snapshot vs current working state
```

## 3. Undo mechanism (verified)

Undo is two git operations, run in this order, both required:

```
git --git-dir=<store> --work-tree=<dir> read-tree --reset -u <target>
git --git-dir=<store> --work-tree=<dir> clean -fd
```

Verified directly: `read-tree --reset -u` alone restores deleted files and overwrites modified ones to match `<target>`, but does **not** remove files created after `<target>` was taken (a file added since the snapshot was left in place in testing). `clean -fd` alone would remove untracked additions but restores nothing. Both together, in this order, produce an exact revert. Neither step should ever ship alone.

Before running these two commands, `ctrlz undo` must:

1. Capture `original := git rev-parse HEAD` (the store's current HEAD, before anything else happens).
2. Take the safety snapshot from section 2, tagged `pre-undo`, unconditionally, regardless of how messy the current state is.
3. Resolve `<target>`: if `--to <ref>` was given, resolve that ref against the store (anything `git rev-parse` accepts: a full or short hash, `HEAD~2`, etc.). Otherwise, use `original` captured in step 1. This ordering matters: `original` is captured before the `pre-undo` commit exists, so the safety commit is never mistaken for the default revert target.
4. Print a plain-language summary of what will change (counts of files to be restored, removed, modified) by diffing current state against `<target>`.
5. Unless `--yes` was passed, prompt for y/n confirmation.
6. Apply the two-step revert against `<target>`.

Because step 2 always runs, `ctrlz undo` followed immediately by a second `ctrlz undo` reverts the undo itself, back to whatever the messy state was.

## 4. Default ignore list

Applied via `-c core.excludesFile=<path to a ctrlz-managed file>` on every `add -A`, in addition to (not instead of) whatever `.gitignore` already exists in the watched directory:

```
.git/
node_modules/
.venv/
venv/
__pycache__/
.next/
.cache/
target/
dist/
build/
vendor/
```

This list is baked into the binary for v1. A `.ctrlzignore` override is reasonable for a later version; it is not required now.

## 5. CLI reference

### `ctrlz init [path]`

Registers `path` (default: cwd) in `registry.json` and creates its bare store if one doesn't exist. Idempotent: running it again on an already-registered path is a no-op that prints the existing project's id and current snapshot count.

### `ctrlz watch [path] [--interval 5s] [--quiet] [-- command...]`

- Resolves or creates the project for `path` (default: cwd), same as `init`.
- Writes `watch.lock` (pid + start time) for this project. If a live lock already exists (the recorded pid is still running), refuses to start and prints the pid already watching this path. If the lock's pid is no longer running, treat it as stale, remove it, and proceed.
- Starts a ticker at `--interval` (default `5s`). On each tick, uses fsnotify to cheaply check whether anything changed since the last snapshot, and only calls the snapshot mechanism (tagged `interval`) when something did. The ticker, not fsnotify, is the timing guarantee; fsnotify is purely an optimization to skip no-op ticks.
- If a command follows `--`, runs it as a foreground subprocess with stdio passed straight through, takes one final `interval` snapshot when the subprocess exits, removes the lock, and exits with the subprocess's own exit code.
- If no command is given, runs until interrupted (Ctrl+C), takes a final snapshot, removes the lock, exits 0.
- Default output is quiet: one line when watching starts, nothing per tick, so it doesn't clutter an agent's own terminal output. `--quiet` suppresses even that line.

### `ctrlz undo [path] [--to <ref>] [--yes]`

- Resolves the project for `path` (default: cwd). Errors clearly if the project has no snapshots yet.
- Behavior exactly as specified in section 3.

### `ctrlz log [path] [--limit 20]`

Prints snapshots newest-first: short hash, relative time ("2 minutes ago"), reason tag, and a one-line file-change stat. No pagination beyond `--limit` in v1.

### `ctrlz diff [path] <a> [b]`

If `b` is omitted, diffs `a` against the current working tree. Prints a standard unified diff to stdout, pipeable to `less`, `delta`, or whatever the user already has. ctrlz does not reimplement a diff viewer.

### `ctrlz status [path]`

Prints: project id, total snapshot count, store size on disk (human-readable, e.g. "store: 4.2 MB"), time of the most recent snapshot, and whether a watch is currently active for this path (based on `watch.lock` liveness).

## 6. Error handling

- No `git` binary on `PATH`: fail immediately on any command with a message naming git as a hard requirement. No pure-Go fallback in v1.
- `undo` / `log` / `diff` / `status` called on a path with no project registered: clear message suggesting `ctrlz init` or `ctrlz watch`. Don't silently create an empty project for these read-oriented commands.
- `undo` called with zero snapshots: clear message, exit non-zero, no crash.
- Two `ctrlz watch` calls on the same path: the second exits non-zero immediately, naming the pid already running, per section 5.
- Corrupted or hand-edited `registry.json`: fail loudly and name the file to check, rather than silently discarding project history.

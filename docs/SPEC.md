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
- `filepath.EvalSymlinks` requires the path to already exist. This matters for full-directory-deletion recovery: if the watched directory itself (not just its contents) was removed, every command — including `undo` — fails to resolve it until an empty directory is recreated at the same path (`mkdir`), which `read-tree --reset -u` then populates. Confirmed directly against a real project: recovery is complete once that's done, but it isn't automatic.

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

This cuts both ways. Confirmed against a real project during manual testing: a bare `logo.png` line in that project's own `.gitignore` (added just to keep one specific asset out of *their* git history) silently excluded every file named `logo.png` anywhere in the tree from every ctrlz snapshot too, with no warning at snapshot time or at `undo` time. ctrlz cannot tell "disposable build output" apart from "real content the user just didn't want in their own repo" — it defers entirely to the project's `.gitignore`. A nonzero `ctrlz status` snapshot count does not imply everything on disk is protected.

A related consequence, worth stating plainly since it's easy to assume otherwise: because the watched directory's own `.git` is never snapshotted, restoring from ctrlz never reconstructs it. After recovering from a full directory deletion, file content comes back exactly (see section 1 on recreating the directory first), but a directory that used to be its own git repository comes back as a plain directory: no branches, no commit history, no staging area, no reflog. Re-attaching a remote afterward (`git init`, `git remote add`, `git fetch`, `git checkout`) is on the user; ctrlz has no part in it and was never meant to.

`<reason>` is one of: `interval` (a regular tick from `ctrlz watch`, including its baseline and final snapshots — see section 5), `pre-undo` (the automatic safety snapshot taken immediately before an undo is applied).

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
3. Resolve `<target>`:
   - If `--to <ref>` was given, resolve that ref against the store (anything `git rev-parse` accepts: a full or short hash, `HEAD~2`, etc.).
   - Otherwise, if step 2 committed something new, use `original` captured in step 1. This ordering matters: `original` is captured before the `pre-undo` commit exists, so the safety commit is never mistaken for the default revert target.
   - Otherwise (step 2 was a no-op — nothing had changed since `original` was recorded), fall back to `original`'s parent instead, erroring clearly ("already at the oldest snapshot") if `original` has no parent.
4. Print a plain-language summary of what will change (counts of files to be restored, removed, modified) by diffing current state against `<target>`.
5. Unless `--yes` was passed, prompt for y/n confirmation.
6. Apply the two-step revert against `<target>`.

The fallback in step 3 is load-bearing, not an edge case, and was added after a real failure found while building the launch demo: `ctrlz watch` snapshots unconditionally, with no concept of "good" vs "bad" content, so its own final snapshot routinely captures whatever an agent just destroyed as the newest commit (e.g. an agent's `rm -rf` running right before its wrapped process exits). In that situation `original` *is* the destructive state, and reverting to it is a no-op — defeating the entire point of `undo`. Falling back one snapshot further whenever step 2 found nothing to commit fixes this without weakening the double-undo guarantee below, because that path always has something new to commit in step 2 (the just-restored content differs from the still-messy `HEAD`) and so never touches the fallback branch.

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
- Takes one unconditional baseline snapshot (tagged `interval`) of whatever already exists in `path`, before the ticker loop or the wrapped command starts. This is necessary, not optional: fsnotify only ever reports changes from the moment watching begins, so without a baseline, an agent's very first destructive action could become the *first* snapshot ever taken — leaving nothing earlier to undo to. Found as a real gap while building the launch demo.
- Starts a ticker at `--interval` (default `5s`). On each tick, uses fsnotify to cheaply check whether anything changed since the last snapshot, and only calls the snapshot mechanism (tagged `interval`) when something did. The ticker, not fsnotify, is the timing guarantee; fsnotify is purely an optimization to skip no-op ticks.
- If a command follows `--`, runs it as a foreground subprocess with its working directory set to `path` (not wherever `ctrlz` itself was invoked from) and stdio passed straight through, takes one final `interval` snapshot when the subprocess exits, removes the lock, and exits with the subprocess's own exit code.
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
- `undo` with no `--to`, where nothing has changed since the most recent snapshot and that snapshot has no parent to fall back to (section 3, step 3): clear message ("already at the oldest snapshot"), exit non-zero, no crash. Distinct from the zero-snapshots case above — this is one snapshot in, just with nowhere earlier left to go.
- Two `ctrlz watch` calls on the same path: the second exits non-zero immediately, naming the pid already running, per section 5.
- Corrupted or hand-edited `registry.json`: fail loudly and name the file to check, rather than silently discarding project history.

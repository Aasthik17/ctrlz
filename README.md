# ctrlz

**An undo button for AI coding agents.**

Claude Code, Cursor, Codex, Cline, Replit's agent, whatever you're running: ctrlz sits underneath it and takes automatic, zero-config snapshots of your working directory. If an agent deletes something, breaks something, or leaves a mess, you get it all back with one command. Not a sandbox. Not a policy engine. Just a guaranteed way back to before.

*(demo recording goes here before launch, this is the first thing anyone should see)*

## Why

Agents run with real filesystem access and real autonomy now, and the failures are already public: agents have deleted family photos, wiped production databases, and taken down entire machines, in ordinary sessions where nobody expected it. The usual fixes are enterprise sandboxes and policy engines that require you to predict danger in advance. ctrlz doesn't predict anything. It just makes sure nothing is ever truly gone.

## How it works

```
ctrlz watch -- claude-code
```

That's it. ctrlz snapshots your directory every few seconds while the agent runs, into a store that lives entirely outside the directory itself, so even a full `rm -rf` of the whole project can't touch your history.

If something goes wrong:

```
ctrlz undo
```

Everything reverts to the last snapshot. Files that were deleted come back. Files added since get cleaned up. Run `ctrlz undo` again and it undoes the undo, since the state right before you reverted gets snapshotted too.

```
ctrlz log     # see the timeline
ctrlz diff    # see exactly what changed
ctrlz status  # snapshot count, store size, whether watch is active
```

No config file. No policy to write. Doesn't need the directory to be a git repo. Works with any agent, because it never integrates with any agent, it just watches the filesystem underneath whatever's running.

## Install

```
go install github.com/YOUR_GITHUB_USERNAME/ctrlz/cmd/ctrlz@latest
```

(prebuilt binaries once this has real users)

## Status

This is v1: local filesystem protection on a single machine. It does not yet protect remote databases or API-driven destruction. That's a real, known gap and a planned next step, not a hidden one. See `SPEC.md` for exactly what v1 does and doesn't cover.

## How it's built

The full technical design, including the exact git plumbing ctrlz uses underneath, is in `SPEC.md`. The build roadmap is in `IMPLEMENTATION_PLAN.md`.

## Contributing

See `CONTRIBUTING.md`. Real failure reports from running ctrlz against an actual agent session are worth more right now than speculative features.

## License

MIT, see `LICENSE`.

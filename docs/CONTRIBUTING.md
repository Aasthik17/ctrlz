# Contributing to ctrlz

ctrlz is early. The most useful things right now, in order:

- **Try it against a real agent session** and open an issue for anything that doesn't recover cleanly. Real failure reports are worth more than speculative features at this stage.
- **Read `SPEC.md` before sending a PR that touches the snapshot or undo mechanism.** Both are precisely specified there for a reason. Changes to that path need to preserve the core guarantees: full recovery after total directory deletion, correct behavior when the watched directory already has its own real `.git`, and honoring whatever `.gitignore` already exists.
- **Keep v1 in scope.** See "explicit non-goals" in `IMPLEMENTATION_PLAN.md`. Ideas for later versions (remote/database protection, a TUI, shell hooks) are welcome as issues, not as unsolicited PRs against this milestone.

No CLA, no formal process yet. Open an issue or a PR.

# ctrlz launch demo — recording script

This is the shot list for the terminal recording that goes at the top of the
README, right after the tagline (see the launch checklist in
`IMPLEMENTATION_PLAN.md`). It drives `scripts/demo.sh`, which does the actual
work on screen — this document is what to say/show around it, not a
replacement for it.

**Target length: 30–40 seconds.** Long enough to land the "it's really
gone, then it's really back" beat; short enough that nobody scrubs past it.

## Before you hit record

- **Pick a clean path.** Run with `CTRLZ_DEMO_DIR=~/agent-project bash
  scripts/demo.sh` instead of the default — the default uses `mktemp`,
  which produces an ugly `/var/folders/.../ctrlz-demo.XXXX` path that's
  hard to read on screen and adds visual noise for no reason. Delete
  `~/agent-project` afterward, or reuse it (the script refuses to run
  against a non-empty directory, so clear it between takes).
- **Terminal appearance:** dark theme, high-contrast (the script already
  color-codes the `$ command` prompts, so avoid a theme that fights that),
  font size large enough to read at 1080p with room to spare — 18–20pt in a
  window around 90 columns wide reads comfortably. Avoid oversized "demo
  fonts" that force awkward line-wrapping on the `ls -la` output.
- **Hide anything you don't want visible.** `ls -la` shows the OS username
  in the owner column. If that bothers you, crop it in post or swap in a
  throwaway user account — not worth scripting around.
- **Recording tool:** [asciinema](https://asciinema.org/) gives the
  crispest result for a terminal-only demo and is trivial to convert to a
  GIF (`agg` or `asciicast2gif`) for the README. A plain screen recording
  (QuickTime, etc.) trimmed in any video editor works just as well if you'd
  rather have a real video file. Either way, run the script live rather
  than typing along manually — the pacing is already tuned via the `sleep`
  calls, and a live take avoids retyping mistakes on camera.
- **Do one dry run off-camera first.** Confirms `ctrlz` is on `PATH`
  (`go install ./cmd/ctrlz` or `make build`) and that the timing still
  feels right before you're actually recording.

## The shot list

Everything below is exactly what `scripts/demo.sh` prints, in order. Times
are approximate, measured from when the recording starts.

---

**[0:00 – 0:02] Empty terminal, prompt visible. Hold for a beat before
running anything** — gives the viewer a second to orient before the first
command appears.

```
$ CTRLZ_DEMO_DIR=~/agent-project bash scripts/demo.sh
```

*(Optional caption if you're adding text overlays in post: none needed yet
— let the terminal speak for itself.)*

---

**[0:02 – 0:03]**

```
$ cd agent-project
```

A normal-looking project directory. Nothing about it suggests anything
special is about to happen — that's the point.

---

**[0:03 – 0:04]**

```
$ ctrlz watch -- ./agent.sh
ctrlz watching /Users/you/agent-project (every 1s)
```

*(Optional caption: "ctrlz runs underneath your agent — no config, no
setup.")* This is the entire integration surface. One command, wrapping
whatever the agent's own launch command is.

---

**[0:04 – 0:06]**

```
writing project files...
```

The stand-in "agent" (`agent.sh`, written inline by the demo script) writes
`src/main.py` and `README.md` — ordinary, unremarkable agent output. This
is where a real recording could optionally cut to a code editor briefly
showing the files existing, then cut back — not required, but sells the
"there was real work here" beat if you have time to shoot it.

---

**[0:06 – 0:08] The turn.**

```
oops: about to run a destructive cleanup command...
```

*(Optional caption: "Then it makes a mistake.")* Let this line sit on
screen for the full second before the next line — it's the setup for the
payoff, don't rush past it.

---

**[0:08 – 0:09] The damage.**

```
...and it's gone.
```

*(Optional caption: "Every agent failure story starts here.")*

---

**[0:09 – 0:10]**

```
$ ls -la
total 0
drwxr-xr-x@  2 you  staff    64 ... .
drwxr-xr-x@  3 you  staff    96 ... ..
```

Hold on this for a full second. An empty directory, nothing hedging it —
this is the moment the demo has to sell. Don't cut away early.

---

**[0:10 – 0:11] The undo.**

```
$ ctrlz undo
Reverting to <hash> will restore 3 file(s), remove 0 file(s), and modify 0 file(s).
Reverted /Users/you/agent-project to <hash>.
```

*(Optional caption: "One command.")* This is the whole product pitch in
one line of terminal output — no flags needed for the common case (the
script passes `--yes` to skip the confirmation prompt for a clean take;
mention in narration, if you're doing voiceover, that a real run asks for
confirmation first).

---

**[0:11 – 0:13] The payoff.**

```
$ ls -la
total 16
-rw-r--r--@  1 you  staff   64 ... README.md
-rwxr-xr-x@  1 you  staff  386 ... agent.sh
drwxr-xr-x@  3 you  staff   96 ... src

def main():
    print("hello from the agent")

if __name__ == "__main__":
    main()
```

Hold this even longer than the empty-directory shot — it's the actual
resolution. Showing `main.py`'s real content (not just that the file
exists) is what proves this is a byte-for-byte recovery, not just an empty
placeholder file reappearing.

*(Optional closing caption: "ctrlz — the undo button for AI coding
agents." Same line as the README tagline, for consistency.)*

---

**[0:13] Cut.** Don't let the terminal sit idle after the payoff — end on
the restored content, not on a blinking cursor.

## What NOT to include

- Don't show the `(demo directory: ...)` line at the very end of the
  script's output — it's diagnostic, not part of the story. Cut before it
  or crop it out.
- Don't narrate over the "gone" / "back" beats if you're adding a
  voiceover — let the terminal output carry those two moments in silence,
  then narrate around them.
- Don't speed up the empty-directory or restored-content shots to save
  time. Those two seconds are the entire argument for the tool; everything
  else is context.

## After recording

Convert to GIF (if using asciinema) or trim to a clean MP4, then drop it
into the README where the current placeholder comment is:

```
*(demo recording goes here before launch, this is the first thing anyone should see)*
```

Replace that line with the embedded GIF/video and delete the comment.

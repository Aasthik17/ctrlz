#!/usr/bin/env bash
# Scripted walkthrough of the core ctrlz scenario, paced for recording with
# asciinema, ttygif, or any terminal screen recorder. Run it as-is for the
# launch demo: `ctrlz watch` wraps a stand-in "agent" that writes some files
# and then wipes the directory, and `ctrlz undo` brings it all back.
#
# Requires `ctrlz` on PATH (`go install ./cmd/ctrlz` or `make build`).
set -euo pipefail

if ! command -v ctrlz >/dev/null 2>&1; then
	echo "ctrlz not found on PATH. Build it first: go install ./cmd/ctrlz (or make build)." >&2
	exit 1
fi

# Override with CTRLZ_DEMO_DIR for a clean, readable path when recording
# (e.g. CTRLZ_DEMO_DIR=~/agent-project) — the default mktemp path is fine
# for a quick functional check but ugly on screen. Refuses to run against an
# existing non-empty directory rather than risk clobbering real content.
DEMO_DIR="${CTRLZ_DEMO_DIR:-$(mktemp -d -t ctrlz-demo)}"
if [ -e "$DEMO_DIR" ] && [ -n "$(ls -A "$DEMO_DIR" 2>/dev/null)" ]; then
	echo "CTRLZ_DEMO_DIR ($DEMO_DIR) already exists and isn't empty. Pick an empty or nonexistent path." >&2
	exit 1
fi
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

type_cmd() {
	printf '\n\033[1;36m$ %s\033[0m\n' "$*"
	sleep 1
}

type_cmd "cd $(basename "$DEMO_DIR")"

type_cmd "ctrlz watch -- ./agent.sh"
cat >agent.sh <<'EOF'
#!/usr/bin/env bash
set -e
echo "writing project files..."
mkdir -p src
cat > src/main.py <<'PY'
def main():
    print("hello from the agent")

if __name__ == "__main__":
    main()
PY
cat > README.md <<'MD'
# demo project
Written by an "agent" for the ctrlz launch demo.
MD
sleep 2
echo "oops: about to run a destructive cleanup command..."
sleep 1
rm -rf ./*
echo "...and it's gone."
EOF
chmod +x agent.sh
ctrlz watch --interval 1s -- ./agent.sh

type_cmd "ls -la"
ls -la

type_cmd "ctrlz undo"
ctrlz undo --yes

type_cmd "ls -la"
ls -la
echo
cat src/main.py

echo
echo "(demo directory: $DEMO_DIR)"

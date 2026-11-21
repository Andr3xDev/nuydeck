#!/usr/bin/env bash
# Pulls the latest nuydeck and rebuilds the TUI binary. Never uses sudo and never starts the TUI.
set -euo pipefail

ROOT="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"

for tool in git go; do
  command -v "$tool" >/dev/null || { echo "update-tui: '$tool' is required but not on PATH" >&2; exit 1; }
done

if [ -n "$(git -C "$ROOT" status --porcelain --untracked-files=no)" ]; then
  echo "update-tui: tracked files have local changes; commit or stash them first" >&2
  exit 1
fi

export GIT_TERMINAL_PROMPT=0
if [ -z "${GIT_SSH_COMMAND:-}" ] && [ -z "${GIT_SSH:-}" ] && [ -z "$(git -C "$ROOT" config core.sshCommand || true)" ]; then
  export GIT_SSH_COMMAND="ssh -o BatchMode=yes"
fi

git -C "$ROOT" pull --ff-only

mkdir -p "$ROOT/tui/bin"
trap 'rm -f "$ROOT/tui/bin/nuydeck.new"' EXIT
(cd "$ROOT/tui" && go build -o bin/nuydeck.new ./cmd/nuydeck)
mv -f "$ROOT/tui/bin/nuydeck.new" "$ROOT/tui/bin/nuydeck"

echo "nuydeck updated. Restart it to run the new version."

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

if branch="$(git -C "$ROOT" symbolic-ref --short -q HEAD)"; then
  if git -C "$ROOT" rev-parse --abbrev-ref --symbolic-full-name '@{u}' >/dev/null 2>&1; then
    git -C "$ROOT" pull --ff-only
  elif ! git -C "$ROOT" remote get-url origin >/dev/null 2>&1; then
    echo "update-tui: no origin remote, building local code" >&2
  else
    # ls-remote --exit-code: 2 means no such ref; any other failure (network, auth) must abort.
    remote_status=0
    git -C "$ROOT" ls-remote --exit-code --heads origin "$branch" >/dev/null || remote_status=$?
    case "$remote_status" in
      0) git -C "$ROOT" pull --ff-only origin "$branch" ;;
      2) echo "update-tui: no remote branch for $branch, building local code" >&2 ;;
      *) echo "update-tui: cannot reach origin to check branch $branch (git exit $remote_status)" >&2; exit 1 ;;
    esac
  fi
else
  echo "update-tui: detached HEAD at $(git -C "$ROOT" rev-parse --short HEAD), building local code" >&2
fi

mkdir -p "$ROOT/tui/bin"
trap 'rm -f "$ROOT/tui/bin/nuydeck.new"' EXIT
(cd "$ROOT/tui" && go build -o bin/nuydeck.new ./cmd/nuydeck)
mv -f "$ROOT/tui/bin/nuydeck.new" "$ROOT/tui/bin/nuydeck"

echo "nuydeck updated. Restart it to run the new version."

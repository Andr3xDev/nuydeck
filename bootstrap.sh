#!/bin/bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "$(readlink -f -- "${BASH_SOURCE[0]}")")" && pwd)"
# Override exists as a test hook for the Arch check.
OS_RELEASE="${OS_RELEASE:-/etc/os-release}"
LINK_DIR="$HOME/.local/bin"
LINK="$LINK_DIR/nuydeck"

is_arch() {
    [ -r "$OS_RELEASE" ] || return 1
    # Subshell keeps os-release variables out of this script.
    # shellcheck disable=SC1090
    case "$(. "$OS_RELEASE"; echo " ${ID:-} ${ID_LIKE:-} ")" in
        *" arch "*) return 0 ;;
        *) return 1 ;;
    esac
}

if ! is_arch; then
    echo "bootstrap.sh: this system is not Arch or an Arch derivative; nothing was installed." >&2
    exit 1
fi

SUDO=(sudo)
[ "$EUID" -eq 0 ] && SUDO=()
"${SUDO[@]}" pacman -S --needed go git rsync pciutils

if ! (cd "$ROOT/tui" && go build -o bin/nuydeck ./cmd/nuydeck); then
    echo "bootstrap.sh: build failed; not launching." >&2
    exit 1
fi

mkdir -p "$LINK_DIR"
if [ -e "$LINK" ] && [ ! -L "$LINK" ]; then
    echo "bootstrap.sh: warning: $LINK exists and is not a symlink; leaving it untouched." >&2
else
    link_existed=false
    [ -L "$LINK" ] && link_existed=true
    ln -sfnr "$ROOT/tui/bin/nuydeck" "$LINK"
    if ! $link_existed; then
        case ":$PATH:" in
            *":$LINK_DIR:"*) ;;
            *) echo "bootstrap.sh: warning: $LINK_DIR is not on PATH." >&2 ;;
        esac
    fi
fi

exec "$ROOT/tui/bin/nuydeck" --repo "$ROOT" "$@"

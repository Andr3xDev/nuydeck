#!/bin/bash

# ---------------------------------------------------------------------------
# Kotofetch Theme Switcher
# Applies the themed kotofetch config by swapping the active toml file
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in themes/ as <flavor>.toml
# ---------------------------------------------------------------------------

cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1

THEME_DIR="themes"
CONFIG_FILE="config.toml"
THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"
TARGET="$THEME_DIR/${FLAVOR}.toml"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

if [ ! -f "$TARGET" ]; then
    log ERROR "Theme not found: '${FLAVOR}'"
    exit 1
fi

ln -sfn "$TARGET" "$CONFIG_FILE"

log SUCCESS "Theme applied successfully: ${FLAVOR}"

#!/usr/bin/env bash

# ---------------------------------------------------------------------------
# Fastfetch Theme Switcher
# Applies the themed fastfetch config by swapping the active jsonc file
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in themes/ as <flavor>.jsonc
# ---------------------------------------------------------------------------

BASE_DIR="$HOME/.config/fastfetch"
THEMES_DIR="$BASE_DIR/themes"
TARGET="$BASE_DIR/config.jsonc"

THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

SRC="$THEMES_DIR/${FLAVOR}.jsonc"

if [[ ! -f "$SRC" ]]; then
    log ERROR "Theme file not found: ${SRC}"
    exit 1
fi

ln -sfn "themes/${FLAVOR}.jsonc" "$TARGET"

log SUCCESS "Theme applied successfully: ${FLAVOR}"

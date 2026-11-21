#!/bin/bash

# ---------------------------------------------------------------------------
# Starship Theme Switcher
# Applies the themed prompt config by symlinking the selected starship.toml
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist as <flavor>/starship.toml
# ---------------------------------------------------------------------------

CONFIG_DIR="$HOME/.config/starship"
THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"
TARGET="$CONFIG_DIR/${FLAVOR}/starship.toml"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

if [ ! -f "$TARGET" ]; then
    log ERROR "Theme not found: '${FLAVOR}'"
    exit 1
fi

ln -sfn "starship/${FLAVOR}/starship.toml" "$HOME/.config/starship.toml"

log SUCCESS "Theme applied successfully: ${FLAVOR}"

#!/bin/bash

# ---------------------------------------------------------------------------
# Ghostty Theme Switcher
# Applies the active theme by updating the theme setting in config
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in ~/.config/ghostty/themes
# ---------------------------------------------------------------------------

CONFIG_FILE="${HOME}/.config/ghostty/config"
THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

if [ ! -f "$HOME/.config/ghostty/themes/${FLAVOR}" ]; then
    log ERROR "Theme not found: '${FLAVOR}'"
    exit 1
fi

if grep -q "^theme =" "${CONFIG_FILE}"; then
    sed -i "s/^theme = .*/theme = ${FLAVOR}/" "${CONFIG_FILE}"
    pkill -SIGUSR2 ghostty
    log SUCCESS "Theme applied successfully: ${FLAVOR}"
else
    log ERROR "Theme setting not found in config file"
    exit 1
fi

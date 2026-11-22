#!/bin/bash

# ---------------------------------------------------------------------------
# Btop Theme Switcher
# Applies the themed btop color scheme by pointing btop.conf at the theme file
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in themes/ as <flavor>.theme
# ---------------------------------------------------------------------------

CONFIG_DIR="${HOME}/.config/btop"

THEME_NAME="$(cat "${HOME}/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "${HOME}/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

THEME_FILE="${CONFIG_DIR}/themes/${FLAVOR}.theme"

if [[ ! -f "${THEME_FILE}" ]]; then
    log ERROR "Theme file not found: ${THEME_FILE}"
    exit 1
fi

sed -i "s|^color_theme = .*|color_theme = \"${THEME_FILE}\"|" "${CONFIG_DIR}/btop.conf"
pkill -SIGUSR1 btop || pkill -USR1 btop

log SUCCESS "Theme applied successfully: ${FLAVOR}"

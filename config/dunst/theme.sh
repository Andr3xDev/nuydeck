#!/bin/bash

# ---------------------------------------------------------------------------
# Dunst Theme Switcher
# Applies the themed dunst notification style to the active dunstrc
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in themes/ as <flavor>.conf
# ---------------------------------------------------------------------------

THEME_NAME="$(cat "${HOME}/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "${HOME}/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

THEME_PATH="$HOME/.config/dunst/themes/${FLAVOR}.conf"
CONFIG_PATH="$HOME/.config/dunst/dunstrc"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

if [ ! -f "${THEME_PATH}" ]; then
    log ERROR "Theme file not found: ${THEME_PATH}"
    exit 1
fi

ln -sfn "themes/${FLAVOR}.conf" "${CONFIG_PATH}"

killall dunst

notify-send "Theme" "changed to ${FLAVOR}"

log SUCCESS "Theme applied successfully: ${FLAVOR}"

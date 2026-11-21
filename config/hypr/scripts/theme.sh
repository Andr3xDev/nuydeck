#!/bin/bash

# ---------------------------------------------------------------------------
# Hyprland Theme Switcher
# Applies the themed Hyprlock and Hyprland color configs via symlinks
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in ~/.config/hypr/theme/themes
# ---------------------------------------------------------------------------

HYPR_DIR="$HOME/.config/hypr/theme/"
THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

if [ ! -f "$HYPR_DIR/themes/${FLAVOR}/${FLAVOR}.lua" ] || [ ! -f "$HYPR_DIR/themes/${FLAVOR}/${FLAVOR}.conf" ]; then
    log ERROR "Theme not found: '${FLAVOR}'"
    exit 1
fi

# Hyprlock (conf)
ln -sf themes/${FLAVOR}/${FLAVOR}.conf "$HYPR_DIR/colors.conf"
# Hyprland (lua)
echo "return \"${FLAVOR}\"" > "$HYPR_DIR/state.lua"

hyprctl reload

log SUCCESS "Theme applied successfully: ${FLAVOR}"

#!/bin/bash

# ---------------------------------------------------------------------------
# Spotify Theme Switcher
# Applies the matching Spicetify color scheme for the active theme
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# ---------------------------------------------------------------------------

THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

case "${FLAVOR}" in
    "nuytek-nadir")
        COLOR_SCHEME="Gray-Dark3"
        THEME_SPOTIFY="Matte"
        ;;
        
    "nuytek-zenit")
        COLOR_SCHEME="Porcelain"
        THEME_SPOTIFY="Matte"
        ;;
    *)
        log ERROR "Invalid theme: ${FLAVOR}"
        exit 1
        ;;
esac

killall spotify 2>/dev/null

spicetify backup apply
spicetify config current_theme "${THEME_SPOTIFY}" color_scheme "${COLOR_SCHEME}"
spicetify config inject_css 1 replace_colors 1 overwrite_assets 1 inject_theme_js 1
spicetify config sidebar_config 0
spicetify apply

log SUCCESS "Theme applied successfully: ${FLAVOR}"

#!/bin/bash

# ---------------------------------------------------------------------------
# Wallpaper Theme Switcher
# Applies the themed wallpaper across all connected monitors via swww
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# ---------------------------------------------------------------------------

THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"
WALLPAPER_DIR="$HOME/.config/wallpapers"
MONITORS=$(hyprctl monitors | grep -oP 'Monitor \K[^\s]+')

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

case "${FLAVOR}" in
    "nuytek-nadir")
        for MONITOR in $MONITORS; do
            awww img "$WALLPAPER_DIR/wall-d.jpg" \
                --transition-type fade \
                --transition-fps 60 \
                --transition-duration 2 \
                --outputs "$MONITOR" &
        done
        wait
        ;;
        
    "nuytek-zenit")
        for MONITOR in $MONITORS; do
            awww img "$WALLPAPER_DIR/wall_CR.jpg" \
                --transition-type fade \
                --transition-fps 60 \
                --transition-duration 2 \
                --outputs "$MONITOR" &
        done
        wait
        ;;
        
    *)
        log ERROR "Invalid theme: ${FLAVOR}"
        exit 1
        ;;
esac

log SUCCESS "Theme applied successfully: ${FLAVOR}"

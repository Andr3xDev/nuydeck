#!/bin/bash

# Master script that applies a theme (nadir, zenit) across all applications

THEME="$1"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "---------------------------------------------------------------"
log INFO "Applying theme: ${THEME} for all applications"

if [ -z "${THEME}" ]; then
    log ERROR "No theme specified"
    exit 1
fi

# Validate the variant sent by Quickshell (nadir|zenit)
case "${THEME}" in
    "nadir")
        VARIANT="nadir"
        ;;
    "zenit")
        VARIANT="zenit"
        ;;
    *)
        log ERROR "Unknown theme: ${THEME}"
        exit 1
        ;;
esac

~/.config/ghostty/theme.sh "${VARIANT}"

~/.config/hypr/scripts/theme.sh "${VARIANT}"

~/.config/yazi/theme.sh "${VARIANT}"

~/.config/fastfetch/theme.sh "${VARIANT}"

~/.config/btop/theme.sh "${VARIANT}"

~/.config/starship/theme.sh "${VARIANT}"

~/.config/awww/theme.sh "${VARIANT}"

~/.config/kotofetch/theme.sh "${VARIANT}"

~/.config/nuydeck/scripts/spotify-theme.sh "${VARIANT}"

~/.config/nuydeck/scripts/gtk-theme.sh "${VARIANT}"

~/.config/dunst/theme.sh "${VARIANT}"

log SUCCESS "All themes applied successfully: ${THEME}"

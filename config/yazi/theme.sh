#!/bin/bash

# ---------------------------------------------------------------------------
# Yazi Theme Switcher
# Applies the active flavor by writing the theme.toml file
# Theme name comes from ~/.config/nuydeck/theme/name (default nuytek)
# Variant comes from $1 or ~/.config/nuydeck/theme/variant
# Flavor = <name>-<variant>, must exist in ~/.config/yazi/flavors
# ---------------------------------------------------------------------------

THEME_FILE="$HOME/.config/yazi/theme.toml"
THEME_NAME="$(cat "$HOME/.config/nuydeck/theme/name" 2>/dev/null)"
THEME_NAME="${THEME_NAME:-nuytek}"
VARIANT="${1:-$(cat "$HOME/.config/nuydeck/theme/variant" 2>/dev/null)}"
FLAVOR="${THEME_NAME}-${VARIANT}"

source "${HOME}/.config/nuydeck/lib/logger.sh"
log INFO "-------------------------------"
log INFO "Applying theme: ${FLAVOR}"

if [ ! -d "$HOME/.config/yazi/flavors/${FLAVOR}.yazi" ]; then
    log ERROR "Flavor not found: '${FLAVOR}'"
    exit 1
fi

cat > "${THEME_FILE}" << EOT
[flavor]
dark = "${FLAVOR}"
light = "${FLAVOR}"
EOT

log SUCCESS "Theme applied successfully: ${FLAVOR}"

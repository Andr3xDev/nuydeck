#!/bin/bash

# ---------------------------------------------------------------------------
# GTK Theme Switcher
# Applies the active GTK theme variant to the running session
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
        GTK_THEME="Abysal-Obsidian"
        ICON_THEME="Papirus-Dark"
        COLOR_SCHEME="prefer-dark"
        CURSOR="phinger-cursors-light"
        CURSOR_SIZE=25
        ;;
    "nuytek-zenit")
        GTK_THEME="Abysal-Marble"
        ICON_THEME="Papirus-Light"
        COLOR_SCHEME="prefer-light"
        CURSOR="phinger-cursors-light"
        CURSOR_SIZE=25
        ;;
    *)
        log ERROR "Invalid theme: ${FLAVOR}"
        exit 1
        ;;
esac

# apply theme
gsettings set org.gnome.desktop.interface gtk-theme "$GTK_THEME"
gsettings set org.gnome.desktop.interface icon-theme "$ICON_THEME"
gsettings set org.gnome.desktop.interface cursor-theme "$CURSOR"
gsettings set org.gnome.desktop.interface cursor-size "$CURSOR_SIZE"
gsettings set org.gnome.desktop.interface color-scheme "$COLOR_SCHEME"

# GTK 3 config
GTK3_CONF="$HOME/.config/gtk-3.0/settings.ini"
if [ -f "$GTK3_CONF" ]; then
    sed -i "s/^gtk-theme-name=.*/gtk-theme-name=$GTK_THEME/" "$GTK3_CONF"
    sed -i "s/^gtk-icon-theme-name=.*/gtk-icon-theme-name=$ICON_THEME/" "$GTK3_CONF"
    sed -i "s/^gtk-cursor-theme-name=.*/gtk-cursor-theme-name=$CURSOR/" "$GTK3_CONF"
    if [ "$COLOR_SCHEME" == "prefer-dark" ]; then
        sed -i "s/^gtk-application-prefer-dark-theme=.*/gtk-application-prefer-dark-theme=1/" "$GTK3_CONF"
    else
        sed -i "s/^gtk-application-prefer-dark-theme=.*/gtk-application-prefer-dark-theme=0/" "$GTK3_CONF"
    fi
fi

# GTK 2 config
GTK2_CONF="$HOME/.gtkrc-2.0"
if [ -f "$GTK2_CONF" ]; then
    sed -i "s/^gtk-theme-name=.*/gtk-theme-name=\"$GTK_THEME\"/" "$GTK2_CONF"
    sed -i "s/^gtk-icon-theme-name=.*/gtk-icon-theme-name=\"$ICON_THEME\"/" "$GTK2_CONF"
    sed -i "s/^gtk-cursor-theme-name=.*/gtk-cursor-theme-name=\"$CURSOR\"/" "$GTK2_CONF"
    sed -i "s/^gtk-cursor-theme-size=.*/gtk-cursor-theme-size=\"$CURSOR_SIZE\"/" "$GTK2_CONF"
fi

# Hot Reload to XWayland
XSETTINGS_CONF="$HOME/.config/xsettingsd/xsettingsd.conf"
mkdir -p "$(dirname "$XSETTINGS_CONF")"

echo "Net/ThemeName \"$GTK_THEME\"" > "$XSETTINGS_CONF"
echo "Net/IconThemeName \"$ICON_THEME\"" >> "$XSETTINGS_CONF"
echo "Gtk/CursorThemeName \"$CURSOR\"" >> "$XSETTINGS_CONF"
echo "Gtk/CursorThemeSize $CURSOR_SIZE" >> "$XSETTINGS_CONF"
if [ "$COLOR_SCHEME" == "prefer-dark" ]; then
    echo "Gtk/ApplicationPreferDarkTheme 1" >> "$XSETTINGS_CONF"
else
    echo "Gtk/ApplicationPreferDarkTheme 0" >> "$XSETTINGS_CONF"
fi

pkill -HUP xsettingsd || xsettingsd &

# GTK 4 config
rm -rf "$HOME/.config/gtk-4.0"
for THEMES_DIR in "$HOME/.themes" "/usr/share/themes"; do
    if [ -d "$THEMES_DIR/$GTK_THEME/gtk-4.0" ]; then
        ln -sfn "$(realpath --relative-to="$HOME/.config" "$THEMES_DIR/$GTK_THEME/gtk-4.0")" "$HOME/.config/gtk-4.0"
        break
    fi
done

# hyprland cursor
hyprctl setcursor "$CURSOR" "$CURSOR_SIZE"

log SUCCESS "Theme applied successfully: ${FLAVOR}"

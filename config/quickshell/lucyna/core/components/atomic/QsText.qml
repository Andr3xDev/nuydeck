import QtQuick
import "../../theme" as Theme

/*!
    QsText — primitive text component.

    Always reads typography and color from ThemeManager semantic tokens.
    No business logic, no service imports.
*/
Text {
    id: root

    // Visual role: "body" | "subtitle" | "title" | "caption" | "mono" | "icon"
    property string role: "body"

    // Only consulted when role === "icon": "sm" | "md" | "lg" | "xl" | "xxl"
    property string iconSize: "sm"

    property bool muted: false

    color: muted
        ? Theme.Tokens.color.textMuted
        : Theme.Tokens.color.textPrimary

    font.family: {
        switch (role) {
            case "mono": return Theme.Tokens.font.mono
            case "icon": return Theme.Tokens.font.icon
            default:     return Theme.Tokens.font.sans
        }
    }

    font.pixelSize: {
        if (role === "icon") {
            switch (iconSize) {
                case "md":  return Theme.Tokens.text.iconMd
                case "lg":  return Theme.Tokens.text.iconLg
                case "xl":  return Theme.Tokens.text.iconXl
                case "xxl": return Theme.Tokens.text.iconXxl
                default:    return Theme.Tokens.text.icon
            }
        }
        switch (role) {
            case "caption":  return Theme.Tokens.text.compactXs
            case "title":    return Theme.Tokens.text.compactLg
            case "subtitle": return Theme.Tokens.text.compactMd
            default:         return Theme.Tokens.text.compactSm  // body, mono
        }
    }
}

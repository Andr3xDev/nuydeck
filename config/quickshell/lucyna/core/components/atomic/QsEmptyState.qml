import QtQuick
import "../../theme" as Theme

/*!
    QsEmptyState — centered icon + text placeholder for lists with no rows.

    Presentation-only: the host decides when to show it and what it says.
    `icon` is a Nerd Font glyph; sizes follow QsText (`iconSize`, `textRole`).
*/
Column {
    id: root

    property string icon: ""
    property string text: ""

    // QsText icon size: "sm" | "md" | "lg" | "xl" | "xxl"
    property string iconSize: "xl"
    property string textRole: "body"
    property color iconColor: Theme.Tokens.color.textMuted
    property color textColor: Theme.Tokens.color.textMuted
    property real iconOpacity: 0.6

    spacing: Theme.Tokens.space.sm

    QsText {
        anchors.horizontalCenter: parent.horizontalCenter
        visible: root.icon !== ""
        role: "icon"
        iconSize: root.iconSize
        text: root.icon
        color: root.iconColor
        opacity: root.iconOpacity
    }

    QsText {
        anchors.horizontalCenter: parent.horizontalCenter
        visible: root.text !== ""
        role: root.textRole
        textFormat: Text.PlainText
        text: root.text
        color: root.textColor
    }
}

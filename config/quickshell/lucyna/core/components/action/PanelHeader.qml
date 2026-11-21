import QtQuick
import "../../theme" as Theme
import "../atomic"

// Title + separator header shared by action-style panels.
Column {
    property string title: ""
    property real separatorWidth: 0

    spacing: Theme.Tokens.space.md

    QsText {
        anchors.horizontalCenter: parent.horizontalCenter
        text: parent.title
        role: "subtitle"
        color: Theme.Tokens.color.textSecondary
    }

    Rectangle {
        anchors.horizontalCenter: parent.horizontalCenter
        width: parent.separatorWidth
        height: 1
        color: Theme.Tokens.color.border
    }
}

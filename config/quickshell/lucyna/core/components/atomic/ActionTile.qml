import QtQuick
import "../../theme" as Theme
import "../../animations" as Animations

// Presentation-only square tile for ActionTiles; all state lives in the host.
Rectangle {
    id: root

    property string glyph: ""
    property bool fired: false
    property bool selected: false

    signal clicked()

    readonly property int side: Theme.Tokens.comp.actionTile.side

    implicitWidth: side
    implicitHeight: side
    radius: Theme.Tokens.radius.lg
    color: fired ? Theme.Tokens.color.accentSurface : Theme.Tokens.color.surface
    border.width: (fired || selected) ? Theme.Tokens.border.md : Theme.Tokens.border.sm
    border.color: (fired || selected) ? Theme.Tokens.color.accent : Theme.Tokens.color.border

    Behavior on color {
        Animations.MotionColorAnimation { role: "stateFast" }
    }
    Behavior on border.color {
        Animations.MotionColorAnimation { role: "stateFast" }
    }

    QsText {
        anchors.centerIn: parent
        text: root.glyph
        role: "icon"
        iconSize: "xxl"
        color: root.fired ? Theme.Tokens.color.accent : Theme.Tokens.color.textPrimary
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}

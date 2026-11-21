import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

/*!
    Square icon button (optional label) that toggles `expanded`.
*/
Rectangle {
    id: toggleIndicator
    implicitWidth: label !== ""
        ? buttonRow.implicitWidth + (hPad * 2)
        : implicitHeight
    implicitHeight: 20
    readonly property real hPad: Theme.Tokens.space.xs

    color: expanded
        ? Theme.Tokens.color.accentSurface
        : Theme.Tokens.color.bg
    radius: Theme.Tokens.radius.sm
    border.color: Theme.Tokens.color.borderStrong
    border.width: 1

    property string icon: ""
    property string label: ""
    property bool expanded: false

    Behavior on color {
        Animations.MotionColorAnimation { role: "stateStandard" }
    }

    RowLayout {
        id: buttonRow
        anchors.centerIn: parent
        spacing: Theme.Tokens.space.xs

        QsText {
            text: icon
            role: "icon"
            color: expanded
                ? Theme.Tokens.color.accent
                : Theme.Tokens.color.textPrimary
            Layout.alignment: Qt.AlignCenter

            Behavior on color {
                Animations.MotionColorAnimation { role: "stateStandard" }
            }
        }

        QsText {
            visible: label !== ""
            text: label
            role: "body"
            color: expanded
                ? Theme.Tokens.color.accent
                : Theme.Tokens.color.textPrimary
            Layout.alignment: Qt.AlignCenter

            Behavior on color {
                Animations.MotionColorAnimation { role: "stateStandard" }
            }
        }
    }

    Animations.HoverScale {
        anchors.fill: parent
        target: toggleIndicator
        cursorShape: Qt.PointingHandCursor
        onClicked: {
            expanded = !expanded
        }
    }
}

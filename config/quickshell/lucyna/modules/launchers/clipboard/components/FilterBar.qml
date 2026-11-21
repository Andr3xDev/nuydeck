import QtQuick
import "../../../../core/components/atomic"
import "../../../../core/components/input"
import "../../../../core/animations" as Animations
import "../../../../core/theme" as Theme

/*!
    FilterBar — shared SearchField with a mode chip, the list position and a
    mode-dependent cursor; the panel decides focus.
*/
SearchField {
    id: root

    // core/components/input/ModalKeymap instance: its mode drives the visuals, its handleKey gets the keys.
    property Item keymap
    property string position: ""

    readonly property bool _insert: keymap ? keymap.mode === "insert" : false
    readonly property string _normalLabel: "NORMAL"
    readonly property string _insertLabel: "INSERT"
    readonly property real _blockCursorOpacity: 0.5
    property real _blink: 1

    borderColor: Theme.Tokens.color.border
    iconColor: Theme.Tokens.color.aquamarine

    onKeyPressed: event => root.keymap.handleKey(event)

    // Empty delegate hides Qt's own cursor (cursorVisible is reset on focus gain);
    // the Rectangle below draws the cursor in both modes, even without focus.
    input.cursorDelegate: Item {}
    input.selectionColor: Theme.Tokens.comp.clip.selection
    input.selectedTextColor: Theme.Tokens.color.onAccent

    Connections {
        target: root.input
        function onCursorPositionChanged() { root._restartBlink(); }
        function onTextChanged() { root._restartBlink(); }
    }

    function _restartBlink() {
        root._blink = 1;
        blinkAnimation.restart();
    }

    SequentialAnimation {
        id: blinkAnimation
        running: root.visible
        loops: Animation.Infinite
        onRunningChanged: if (!running) root._blink = 1

        PauseAnimation { duration: Theme.Tokens.comp.clip.cursorIdleDelay }
        Animations.MotionAnimation { target: root; property: "_blink"; to: 0; role: "cursorBlink" }
        Animations.MotionAnimation { target: root; property: "_blink"; to: 1; role: "cursorBlink" }
    }

    FontMetrics {
        id: metrics
        font: root.input.font
    }

    TextMetrics {
        id: widestLabel
        font: chipLabel.font
        text: root._normalLabel
    }

    Rectangle {
        parent: root.input
        x: root.input.cursorRectangle.x
        y: root.input.cursorRectangle.y
        height: root.input.cursorRectangle.height
        width: root._insert ? Theme.Tokens.border.md : metrics.averageCharacterWidth
        color: Theme.Tokens.color.aquamarine
        opacity: (root._insert ? 1 : root._blockCursorOpacity) * root._blink
    }

    leadingContent: Item {
        implicitWidth: chip.width + Theme.Tokens.space.sm
        implicitHeight: chip.height

        Rectangle {
            id: chip
            width: widestLabel.width + 2 * Theme.Tokens.space.sm
            height: chipLabel.implicitHeight + 2 * Theme.Tokens.space.xxs
            radius: Theme.Tokens.radius.sm
            color: Theme.Tokens.color.aquamarineSurface

            QsText {
                id: chipLabel
                anchors.centerIn: parent
                role: "caption"
                text: root._insert ? root._insertLabel : root._normalLabel
                color: Theme.Tokens.color.aquamarine
            }
        }
    }

    trailingContent: QsText {
        text: root.position
        color: Theme.Tokens.color.textMuted
    }
}

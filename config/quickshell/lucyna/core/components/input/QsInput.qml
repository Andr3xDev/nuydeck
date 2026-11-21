import QtQuick
import QtQuick.Controls
import "../atomic"
import "../../theme" as Theme
import "../../animations" as Animations

/*!
    QsInput — text input atom with two styles over one TextField.

    Styles
    ------
    "boxed"     surface box, neutral border, glyph and leading/trailing content (default)
    "underline" no box, a single bottom line; no glyph, no slots.
                Placeholder fades out on input; with a centered field the
                cursor sits at the end of the placeholder.

    Unknown style renders as boxed; unknown size renders as md. Chrome is
    neutral: focus is shown by the cursor, never by the border or line.
    Key handling stays with the consumer through `accepted` and `keyPressed`
    (set `event.accepted` to consume). The atom never grabs focus; hosts call
    `focusInput()`. `flashError()` plays a transient shake plus danger flash.

    Usage:
        QsInput {
            style: "underline"
            placeholder: "Name"
            onAccepted: submit(text)
        }
*/
Item {
    id: root

    property alias text: field.text
    property string placeholder: ""
    readonly property alias input: field
    property string style: "boxed"
    property string size: "md"
    property string glyph: ""
    property color iconColor: Theme.Tokens.color.accent
    property real letterSpacing: 0
    property Item leadingContent
    property Item trailingContent
    property bool secret: false
    property bool selectAllOnFocus: false
    property color borderColor: Theme.Tokens.color.border

    signal accepted
    signal keyPressed(var event)

    function clear() {
        field.clear();
    }

    function focusInput() {
        field.forceActiveFocus();
    }

    function flashError() {
        errorFeedback.restart();
    }

    readonly property var _sizeTable: ({
        sm: {
            height: Theme.Tokens.space.xxl,
            textSize: Theme.Tokens.text.compactMd,
            radius: Theme.Tokens.radius.md,
            padV: Theme.Tokens.space.xs,
            inset: Theme.Tokens.space.sm,
            iconInset: Theme.Tokens.space.sm,
            glyphInset: Theme.Tokens.space.xxl,
            iconSize: Theme.Tokens.text.icon
        },
        md: {
            height: Theme.Tokens.space.xxl + Theme.Tokens.space.xs,
            textSize: Theme.Tokens.text.compactLg,
            radius: Theme.Tokens.radius.lg,
            padV: Theme.Tokens.space.sm,
            inset: Theme.Tokens.space.md,
            iconInset: Theme.Tokens.comp.searchField.iconInset,
            glyphInset: Theme.Tokens.comp.searchField.textInset,
            iconSize: Theme.Tokens.text.compactXl
        },
        lg: {
            height: Theme.Tokens.space.xxl + Theme.Tokens.space.md,
            textSize: Theme.Tokens.text.compactLg,
            radius: Theme.Tokens.radius.lg,
            padV: Theme.Tokens.comp.searchField.verticalPadding,
            inset: Theme.Tokens.space.md,
            iconInset: Theme.Tokens.comp.searchField.iconInset,
            glyphInset: Theme.Tokens.comp.searchField.textInset,
            iconSize: Theme.Tokens.text.compactXl
        }
    })
    readonly property var _size: _sizeTable.hasOwnProperty(size) ? _sizeTable[size] : _sizeTable.md
    readonly property bool _boxed: style !== "underline"
    readonly property real _shakeAmplitude: Theme.Tokens.space.xs
    readonly property real _shakeRebound: Theme.Tokens.space.xxs
    readonly property real _baseInset: glyph !== "" ? _size.glyphInset : _size.inset
    readonly property bool _placeholderShown: !_boxed && placeholder !== "" && field.text === "" && field.preeditText === ""

    // Scalar-driven so no bound color is ever animated (an animation would detach its binding).
    property real _errorLevel: 0
    readonly property color _errorTint: Qt.rgba(Theme.Tokens.color.danger.r, Theme.Tokens.color.danger.g, Theme.Tokens.color.danger.b, _errorLevel)

    onLeadingContentChanged: if (root.leadingContent) root.leadingContent.parent = leadingContentHolder
    onTrailingContentChanged: if (root.trailingContent) root.trailingContent.parent = trailingContentHolder

    implicitHeight: _boxed ? _size.height : field.implicitHeight + Theme.Tokens.space.xs + Theme.Tokens.border.sm
    implicitWidth: _boxed ? Theme.Tokens.comp.searchField.width : field.implicitWidth

    transform: Translate { id: shake }

    TextField {
        id: field
        anchors { left: parent.left; right: parent.right; top: parent.top }
        height: root._boxed ? parent.height : implicitHeight
        clip: !root._boxed
        echoMode: root.secret ? TextInput.Password : TextInput.Normal
        placeholderText: root._boxed ? root.placeholder : ""
        placeholderTextColor: Theme.Tokens.color.textMuted
        color: Qt.tint(Theme.Tokens.color.textPrimary, root._errorTint)
        font.pixelSize: root._size.textSize
        font.letterSpacing: root.letterSpacing
        leftPadding: root._boxed ? root._baseInset + leadingContentHolder.width : (root._placeholderShown && field.horizontalAlignment === TextInput.AlignHCenter ? Math.min(underlinePlaceholder.contentWidth, field.width / 2) : 0)
        rightPadding: root._boxed ? root._size.inset + trailingContentHolder.width : 0
        topPadding: root._boxed ? root._size.padV : 0
        bottomPadding: root._boxed ? root._size.padV : 0
        onAccepted: root.accepted()
        onActiveFocusChanged: if (activeFocus && root.selectAllOnFocus) selectAll()
        Keys.onPressed: event => root.keyPressed(event)
        Behavior on leftPadding {
            enabled: !root._boxed
            Animations.MotionAnimation { role: "stateStandard" }
        }
        background: Rectangle {
            visible: root._boxed
            color: Theme.Tokens.color.surface
            radius: root._size.radius
            border.width: Theme.Tokens.border.sm
            border.color: Qt.tint(root.borderColor, root._errorTint)
            QsText {
                visible: root.glyph !== ""
                text: root.glyph
                role: "icon"
                font.pixelSize: root._size.iconSize
                color: root.iconColor
                anchors { verticalCenter: parent.verticalCenter; left: parent.left; leftMargin: root._size.iconInset }
            }
        }
    }

    QsText {
        id: underlinePlaceholder
        anchors { left: field.left; right: field.right; verticalCenter: field.verticalCenter }
        opacity: root._placeholderShown ? 1 : 0
        visible: opacity > 0
        Behavior on opacity {
            Animations.MotionAnimation { role: "stateStandard" }
        }
        elide: Text.ElideRight
        textFormat: Text.PlainText
        text: root.placeholder
        horizontalAlignment: field.horizontalAlignment
        color: Theme.Tokens.color.textMuted
        font.family: field.font.family
        font.pixelSize: field.font.pixelSize
        font.letterSpacing: field.font.letterSpacing
    }

    Rectangle {
        visible: !root._boxed
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: Theme.Tokens.border.sm
        color: Qt.tint(Theme.Tokens.color.border, root._errorTint)
    }

    Item {
        id: leadingContentHolder
        visible: root._boxed
        anchors { verticalCenter: parent.verticalCenter; left: parent.left; leftMargin: root._baseInset }
        width: root.leadingContent ? root.leadingContent.implicitWidth : 0
        height: root.leadingContent ? root.leadingContent.implicitHeight : 0
    }

    Item {
        id: trailingContentHolder
        visible: root._boxed
        anchors { verticalCenter: parent.verticalCenter; right: parent.right; rightMargin: root._size.inset }
        width: root.trailingContent ? root.trailingContent.implicitWidth : 0
        height: root.trailingContent ? root.trailingContent.implicitHeight : 0
    }

    ParallelAnimation {
        id: errorFeedback

        SequentialAnimation {
            Animations.MotionAnimation { target: shake; property: "x"; from: 0; to: -root._shakeAmplitude; role: "stateFast" }
            Animations.MotionAnimation { target: shake; property: "x"; to: root._shakeRebound; role: "stateFast" }
            Animations.MotionAnimation { target: shake; property: "x"; to: 0; role: "stateFast" }
        }
        Animations.MotionAnimation { target: root; property: "_errorLevel"; from: 1; to: 0; role: "stateStandard" }
    }
}

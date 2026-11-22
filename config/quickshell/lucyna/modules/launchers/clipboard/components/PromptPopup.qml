import QtQuick
import QtQuick.Layouts
import "../../../../core/components/atomic"
import "../../../../core/components/input"
import "../../../../core/theme" as Theme
import "../../../../core/animations" as Animations

/*!
    PromptPopup — centered modal prompt over a dimmed panel. Fills its parent, takes no part in layout, and fades in and out with `prompt`.

    prompt (null when none):
      { kind: "pick",    message, options: [string] }  chooser; keys come from the panel, highlight is `pickIndex`
      { kind: "text",    message, text, secret, placeholder }  QsInput underline (password-style when secret; placeholder optional); emits accepted(text) / cancelled()
      { kind: "confirm", message }                     Yes/No buttons, highlight is `pickIndex`; keys come from the panel; emits confirmed(bool)
      { kind: "notice",  message }                     passive text, dismissed by the panel
    Every kind also takes an optional `error` (validation reason), shown unelided beside the message.
*/
FocusScope {
    id: root

    property var prompt: null
    property int pickIndex: 0
    property real radius: 0
    property int confirmYesIndex: 0

    signal accepted(string text)
    signal cancelled()
    signal confirmed(bool accepted)

    // Last non-null prompt: keeps content rendered while the popup fades out.
    property var _shown: null
    readonly property string _kind: _shown ? _shown.kind : ""
    readonly property bool _confirm: _kind === "confirm"
    readonly property int _popupW: 360
    readonly property int _confirmMinW: 200
    readonly property int _padH: Theme.Tokens.space.md + Theme.Tokens.space.xs
    readonly property int _padV: Theme.Tokens.space.md + Theme.Tokens.space.xs
    readonly property int _pickMaxRows: 8
    readonly property int _pickRowH: Theme.Tokens.space.xxl + Theme.Tokens.space.xs
    readonly property int _pickW: _popupW - Theme.Tokens.space.xxl
    readonly property int _messageMaxLines: 3

    opacity: prompt ? 1 : 0
    enabled: prompt !== null
    visible: opacity > 0

    Behavior on opacity {
        Animations.MotionAnimation { role: "stateSlow" }
    }

    function focusInput() {
        promptInput.focusInput();
        promptInput.input.selectAll();
    }

    function typeText(typed) {
        promptInput.input.remove(promptInput.input.selectionStart, promptInput.input.selectionEnd);
        promptInput.input.insert(promptInput.input.cursorPosition, typed);
    }

    onPromptChanged: {
        if (!prompt)
            return;
        _shown = prompt;
        if (_kind !== "text") {
            promptInput.text = "";
            return;
        }
        promptInput.text = prompt.text;
        focusInput();
        if (prompt.error)
            promptInput.flashError();
    }

    onVisibleChanged: if (!visible) { _shown = null; promptInput.clear(); }

    Rectangle {
        anchors.fill: parent
        radius: root.radius
        color: Theme.Tokens.color.scrim
    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
    }

    Rectangle {
        anchors.centerIn: parent
        width: root._confirm
            ? Math.min(root._popupW, Math.max(root._confirmMinW, column.implicitWidth + 2 * root._padH))
            : root._kind === "pick" ? root._pickW : root._popupW
        height: column.implicitHeight + 2 * root._padV
        radius: Theme.Tokens.radius.md
        border.width: Theme.Tokens.border.sm
        border.color: Theme.Tokens.color.border
        color: Theme.Tokens.color.bg

        ColumnLayout {
            id: column
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: root._padH
            anchors.rightMargin: root._padH
            spacing: Theme.Tokens.space.sm

            ColumnLayout {
                Layout.fillWidth: true
                spacing: Theme.Tokens.space.xs

                QsText {
                    Layout.fillWidth: true
                    role: "title"
                    wrapMode: Text.WordWrap
                    horizontalAlignment: Text.AlignHCenter
                    maximumLineCount: root._messageMaxLines
                    elide: Text.ElideRight
                    textFormat: Text.PlainText
                    text: root._shown ? root._shown.message : ""
                    color: root._kind === "text" ? Theme.Tokens.color.turquoise : Theme.Tokens.color.textPrimary
                }

                QsText {
                    Layout.fillWidth: true
                    visible: text !== ""
                    role: "body"
                    horizontalAlignment: Text.AlignHCenter
                    textFormat: Text.PlainText
                    text: root._shown && root._shown.error ? root._shown.error : ""
                    color: Theme.Tokens.color.danger
                }
            }

            QsInput {
                id: promptInput
                Layout.fillWidth: true
                Layout.leftMargin: Theme.Tokens.space.xxl
                Layout.rightMargin: Theme.Tokens.space.xxl
                visible: root._kind === "text"
                style: "underline"
                size: "md"
                placeholder: root._kind === "text" && root._shown.placeholder ? root._shown.placeholder : ""
                secret: root._kind === "text" && root._shown.secret
                input.font.family: Theme.Tokens.font.sans
                input.horizontalAlignment: TextInput.AlignHCenter
                input.selectionColor: Theme.Tokens.comp.clip.selection
                input.selectedTextColor: Theme.Tokens.color.onAccent

                onVisibleChanged: if (visible) root.focusInput()
                // Consume Return/Enter here: TextInput leaves it unaccepted after `accepted`, so it would bubble to the panel and fire its <CR> binding once the prompt closes.
                onKeyPressed: event => {
                    if (event.key === Qt.Key_Escape) {
                        event.accepted = true;
                        root.cancelled();
                    } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                        event.accepted = true;
                        root.accepted(text);
                    }
                }
            }

            RowLayout {
                Layout.alignment: Qt.AlignHCenter
                visible: root._confirm
                spacing: Theme.Tokens.space.sm

                QsButton {
                    label: "No"
                    selected: root.pickIndex !== root.confirmYesIndex
                    onClicked: root.confirmed(false)
                }

                QsButton {
                    label: "Yes"
                    variant: "filled"
                    hue: "danger"
                    selected: root.pickIndex === root.confirmYesIndex
                    onClicked: root.confirmed(true)
                }
            }

            ListView {
                id: pickList
                Layout.fillWidth: true
                Layout.preferredHeight: Math.min(contentHeight, root._pickMaxRows * root._pickRowH)
                visible: root._kind === "pick"
                clip: true
                interactive: false
                model: root._kind === "pick" ? root._shown.options : []

                onCountChanged: positionViewAtIndex(root.pickIndex, ListView.Contain)

                Connections {
                    target: root
                    function onPickIndexChanged() {
                        pickList.positionViewAtIndex(root.pickIndex, ListView.Contain);
                    }
                }

                delegate: Rectangle {
                    id: option
                    required property string modelData
                    required property int index
                    readonly property bool current: index === root.pickIndex

                    width: ListView.view.width
                    height: root._pickRowH
                    radius: Theme.Tokens.radius.sm
                    color: current ? Theme.Tokens.color.surface : "transparent"

                    QsText {
                        anchors.fill: parent
                        role: "title"
                        anchors.leftMargin: Theme.Tokens.space.md
                        anchors.rightMargin: Theme.Tokens.space.sm
                        verticalAlignment: Text.AlignVCenter
                        elide: Text.ElideRight
                        textFormat: Text.PlainText
                        text: option.modelData
                        color: Theme.Tokens.color.textPrimary
                    }
                }
            }
        }
    }
}

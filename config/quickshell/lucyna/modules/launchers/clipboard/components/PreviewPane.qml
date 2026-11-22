import QtQuick
import "../../../../core/components/atomic"
import "../../../../core/animations" as Animations
import "../../../../core/theme" as Theme
import "../state"

/*!
    PreviewPane: text or decoded image of the selected entry. Secret favorites
    render ClipboardState.maskText; the raw value is never bound to a Text.
    Implicit height animates between 0 (collapsed or no source) and the
    content height, clamped to previewMinLines..previewMaxH.
*/
Item {
    id: root

    property var source: null
    property bool expanded: false

    readonly property int previewScrollDivisor: 4
    // Bounds the Text layout cost of huge clips; the full text stays in ClipboardHistory.previewText.
    readonly property int previewMaxChars: 20000
    readonly property real _naturalHeight: flick.contentHeight + 2 * Theme.Tokens.comp.clip.pad
    readonly property real _minHeight: Theme.Tokens.comp.clip.previewMinLines * lineProbe.height + 2 * Theme.Tokens.comp.clip.pad
    readonly property bool _overflowing: _naturalHeight > Theme.Tokens.comp.clip.previewMaxH
    readonly property string _imageUrl: source && source.isImage ? ClipboardHistory.decodedPath(source.id) : ""
    readonly property string _previewText: {
        if (!source) return "";
        if (source.key !== undefined) return source.secret ? ClipboardState.maskText : source.value;
        return source.isImage === false ? ClipboardHistory.previewText : "";
    }

    implicitHeight: expanded && source ? Math.min(Math.max(_naturalHeight, _minHeight), Theme.Tokens.comp.clip.previewMaxH) : 0
    clip: true

    Behavior on implicitHeight {
        Animations.MotionAnimation { role: "stateStandard" }
    }

    onSourceChanged: {
        flick.contentY = 0;
        fade.restart();
    }

    TextMetrics {
        id: lineProbe
        font: previewLabel.font
        text: "M"
    }

    Animations.MotionAnimation {
        id: fade
        role: "effectsEnter"
        target: flick
        property: "opacity"
        from: 0
        to: 1
    }

    function scrollDown() { _scrollBy(flick.height / previewScrollDivisor); }
    function scrollUp() { _scrollBy(-flick.height / previewScrollDivisor); }

    function _scrollBy(delta) {
        if (!expanded) return;
        const maxY = Math.max(0, flick.contentHeight - flick.height);
        flick.contentY = Math.max(0, Math.min(maxY, flick.contentY + delta));
    }

    Flickable {
        id: flick
        anchors.fill: parent
        anchors.margins: Theme.Tokens.comp.clip.pad
        clip: true
        contentWidth: width
        contentHeight: content.height
        interactive: false

        Column {
            id: content
            width: flick.width
            y: Math.max(0, (flick.height - height) / 2)

            Image {
                visible: root._imageUrl !== ""
                source: root._imageUrl
                asynchronous: true
                fillMode: Image.PreserveAspectFit
                width: Math.min(implicitWidth, content.width)
                height: implicitWidth > 0 ? width * implicitHeight / implicitWidth : 0
            }

            QsText {
                id: previewLabel
                visible: root._imageUrl === ""
                width: content.width
                role: "mono"
                font.pixelSize: Theme.Tokens.text.compactLg
                wrapMode: Text.WrapAnywhere
                textFormat: Text.PlainText
                text: root._previewText.slice(0, root.previewMaxChars)
            }
        }
    }

    Rectangle {
        visible: root._overflowing
        anchors.right: parent.right
        width: Theme.Tokens.border.md
        height: Math.max(Theme.Tokens.space.sm, flick.height * flick.height / Math.max(1, flick.contentHeight))
        y: flick.y + (flick.height - height) * flick.contentY / Math.max(1, flick.contentHeight - flick.height)
        color: Theme.Tokens.color.textMuted
    }
}

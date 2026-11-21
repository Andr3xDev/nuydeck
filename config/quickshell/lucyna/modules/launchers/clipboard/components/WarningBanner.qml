import QtQuick
import "../../../../core/components/atomic"
import "../../../../core/theme" as Theme

/*!
    WarningBanner — favorites warning strip; collapses to zero height when `warning` is "".
*/
Rectangle {
    id: root

    property string warning: ""

    readonly property int _maxLines: 2

    visible: warning !== ""
    implicitHeight: visible ? warningText.implicitHeight + 2 * Theme.Tokens.space.xs : 0
    color: Theme.Tokens.color.warningSurface

    QsText {
        id: warningText
        anchors.fill: parent
        anchors.leftMargin: Theme.Tokens.comp.clip.pad
        anchors.rightMargin: Theme.Tokens.comp.clip.pad
        verticalAlignment: Text.AlignVCenter
        role: "body"
        wrapMode: Text.WordWrap
        maximumLineCount: root._maxLines
        elide: Text.ElideRight
        textFormat: Text.PlainText
        text: root.warning
        color: Theme.Tokens.color.warning
    }
}

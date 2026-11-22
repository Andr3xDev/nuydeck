import QtQuick
import "../../../core/theme" as Theme
import "../../../core/components/atomic"

Item {
    id: root
    implicitWidth: logoText.implicitWidth
    implicitHeight: parent.height

    QsText {
        id: logoText
        anchors.centerIn: parent
        text: "󰣇"
        role: "icon"
        color: Theme.Tokens.color.accent
    }
}

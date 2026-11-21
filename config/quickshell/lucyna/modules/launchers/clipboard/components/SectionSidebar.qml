import QtQuick
import QtQuick.Layouts
import "../../../../core/components/atomic"
import "../../../../core/theme" as Theme

/*!
    SectionSidebar — passive icon column marking the active section. Props only.
*/
Item {
    id: root

    // [{ icon: <glyph string> }]
    property var items: []
    property int currentIndex: 0

    ColumnLayout {
        anchors.fill: parent
        anchors.topMargin: Theme.Tokens.comp.clip.pad
        anchors.bottomMargin: Theme.Tokens.comp.clip.pad
        spacing: Theme.Tokens.space.xs

        Item { Layout.fillHeight: true }

        Repeater {
            model: root.items

            Item {
                id: entry
                required property var modelData
                required property int index
                readonly property bool active: index === root.currentIndex

                Layout.fillWidth: true
                implicitHeight: Theme.Tokens.comp.clip.rowH

                Rectangle {
                    visible: entry.active
                    width: Theme.Tokens.border.md
                    height: parent.height
                    color: Theme.Tokens.color.aquamarine
                }

                QsText {
                    id: icon
                    anchors.centerIn: parent
                    role: "icon"
                    iconSize: "md"
                    text: entry.modelData.icon
                    color: entry.active ? Theme.Tokens.color.aquamarine : Theme.Tokens.color.textMuted
                }
            }
        }

        Item { Layout.fillHeight: true }
    }
}

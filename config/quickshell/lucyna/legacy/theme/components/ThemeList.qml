import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

/*!
    List of available themes with preview color dots.
*/
Item {
    id: root
    implicitHeight: themeColumn.implicitHeight

    signal variantSelected(string variantId)

    ColumnLayout {
        id: themeColumn
        anchors.fill: parent
        spacing: 6

        Repeater {
            model: Theme.ThemeManager.availableVariants

            delegate: Rectangle {
                id: themeItem
                Layout.fillWidth: true
                Layout.preferredHeight: 44

                property bool isActive:  modelData === Theme.ThemeManager.currentVariant
                property bool isHovered: itemMouseArea.containsMouse

                color: isActive
                    ? Theme.Tokens.color.accent
                    : (isHovered ? Theme.Tokens.color.surface : Theme.Tokens.color.bg)
                radius: Theme.Tokens.radius.sm
                border.color: Theme.Tokens.color.borderStrong
                border.width: isActive ? 1 : 0

                Behavior on color {
                    Animations.MotionColorAnimation { role: "stateFast" }
                }

                RowLayout {
                    anchors {
                        fill: parent
                        leftMargin: 12
                        rightMargin: 12
                    }
                    spacing: 10

                    Rectangle {
                        Layout.preferredWidth: 3
                        Layout.preferredHeight: 18
                        radius: 1
                        color: Theme.Tokens.color.accent
                        opacity: themeItem.isActive ? 1 : 0

                        Behavior on opacity {
                            Animations.MotionAnimation { role: "stateFast" }
                        }
                    }

                    QsText {
                        Layout.fillWidth: true
                        text: Theme.ThemeManager.getVariantDisplayName(modelData)
                        role: "body"
                        color: themeItem.isActive
                            ? Theme.Tokens.color.onAccent
                            : Theme.Tokens.color.textMuted
                        font.bold: themeItem.isActive
                        elide: Text.ElideRight

                        Behavior on color {
                            Animations.MotionColorAnimation { role: "stateFast" }
                        }
                    }

                    Row {
                        spacing: 3

                        Repeater {
                            model: Theme.ThemeManager.previewColors(modelData)

                            delegate: Rectangle {
                                width: 12
                                height: 12
                                radius: 2
                                color: modelData
                                border.color: Theme.Tokens.color.border
                                border.width: 1
                                scale: themeItem.isHovered ? 1.1 : 1.0

                                Behavior on scale {
                                    Animations.MotionAnimation { role: "stateFast" }
                                }
                            }
                        }
                    }

                    QsText {
                        text: "󰄬"
                        role: "icon"
                        color: Theme.Tokens.color.textMuted
                        opacity: themeItem.isActive ? 1 : 0

                        Behavior on opacity {
                            Animations.MotionAnimation { role: "stateFast" }
                        }
                    }
                }

                MouseArea {
                    id: itemMouseArea
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (!themeItem.isActive)
                            root.variantSelected(modelData)
                    }
                }
            }
        }
    }
}

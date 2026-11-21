import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/services/system" as Services
import "../layout"
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

/*!
    Power profile selector: one icon per profile, the active one underlined in
    its profile color.
*/
ExpandableRow {
    id: root

    readonly property var profileColors: ({
        "power-saver": Theme.Tokens.color.info,
        "balanced": Theme.Tokens.color.warning,
        "performance": Theme.Tokens.color.danger
    })
    
    RowLayout {
        id: profileRow
        anchors.centerIn: parent
        spacing: 15  // raw literal: a spacing token here caused a layout shift
        opacity: root.expanded ? 1 : 0

        Behavior on opacity {
            Animations.MotionAnimation { role: "stateStandard" }
        }

        Repeater {
            model: Services.PowerService.profiles
            delegate: Item {
                id: profileButton
                Layout.preferredWidth: buttonContent.implicitWidth
                Layout.preferredHeight: buttonContent.implicitHeight
                
                property bool isActive: modelData.id === Services.PowerService.currentProfile
                property color profileColor: root.profileColors[modelData.id] || Theme.Tokens.color.textPrimary
                
                ColumnLayout {
                    id: buttonContent
                    spacing: 1
                    
                    QsText {
                        id: iconText
                        text: modelData.icon
                        role: "icon"
                        color: profileButton.isActive
                            ? profileButton.profileColor
                            : Theme.Tokens.color.textPrimary
                        Layout.alignment: Qt.AlignHCenter
                        
                        Behavior on color {
                            Animations.MotionColorAnimation { role: "stateStandard" }
                        }
                    }

                    Rectangle {
                        Layout.preferredWidth: iconText.implicitWidth
                        Layout.preferredHeight: 2
                        Layout.alignment: Qt.AlignHCenter
                        color: profileButton.profileColor
                        radius: 1
                        opacity: profileButton.isActive ? 1 : 0

                        Behavior on opacity {
                            Animations.MotionAnimation { role: "stateStandard" }
                        }
                    }
                }
                
                Animations.HoverScale {
                    anchors.fill: parent
                    target: iconText
                    cursorShape: Qt.PointingHandCursor
                    onClicked: Services.PowerService.setProfile(modelData.id)
                }
            }
        }
    }
}
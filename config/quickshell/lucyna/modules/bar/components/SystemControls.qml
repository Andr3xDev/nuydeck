import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/services/system" as Services
import "../layout"
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

/*!
    Network, Bluetooth and audio controls. Network and Bluetooth clicks open
    their managers; on audio the wheel adjusts volume and click toggles mute.
*/
ExpandableRow {
    id: root

    readonly property real iconSize: 24

    RowLayout {
        id: controlsRow
        anchors.centerIn: parent
        spacing: Theme.Tokens.space.xs
        opacity: root.expanded ? 1 : 0
        
        Behavior on opacity {
            Animations.MotionAnimation { role: "stateStandard" }
        }
        
        Item {
            Layout.preferredWidth: root.iconSize
            Layout.preferredHeight: root.iconSize

            QsText {
                id: networkIconText
                anchors.centerIn: parent
                text: Services.NetworkService.ethernetEnabled ? "󰈀" :
                      (Services.NetworkService.wifiEnabled ? "󰖩" : "󰖪")
                role: "icon"
                color: (Services.NetworkService.ethernetEnabled ||
                        Services.NetworkService.wifiEnabled)
                    ? Theme.Tokens.color.aquamarine
                    : Theme.Tokens.color.danger
            }

            Animations.HoverScale {
                anchors.fill: parent
                target: networkIconText
                cursorShape: Qt.PointingHandCursor
                onClicked: Services.NetworkService.openNetworkManager()
            }
        }
        
        Item {
            Layout.preferredWidth: root.iconSize
            Layout.preferredHeight: root.iconSize

            QsText {
                id: bluetoothIconText
                anchors.centerIn: parent
                text: Services.BluetoothService.enabled ? "󰂯" : "󰂲"
                role: "icon"
                color: Services.BluetoothService.enabled
                    ? Theme.Tokens.color.aquamarine
                    : Theme.Tokens.color.danger
            }

            Animations.HoverScale {
                anchors.fill: parent
                target: bluetoothIconText
                cursorShape: Qt.PointingHandCursor
                onClicked: Services.BluetoothService.openBluetoothManager()
            }
        }
        
        Item {
            Layout.preferredWidth: audioRow.implicitWidth
            Layout.preferredHeight: root.iconSize

            RowLayout {
                id: audioRow
                anchors.centerIn: parent
                spacing: Theme.Tokens.space.xs

                QsText {
                    id: aIconText
                    text: Services.AudioService.muted ? " 󰖁" :
                          Services.AudioService.volume > 50 ? " 󰕾" : " 󰖀"
                    role: "icon"
                    color: Services.AudioService.muted
                        ? Theme.Tokens.color.danger
                        : Theme.Tokens.color.aquamarine
                }

                QsText {
                    text: `${Services.AudioService.volume}%`
                    role: "mono"
                    color: Theme.Tokens.color.textPrimary
                }
            }

            Animations.HoverScale {
                anchors.fill: parent
                target: aIconText
                cursorShape: Qt.PointingHandCursor

                property int wheelAccumulator: 0
                onClicked: Services.AudioService.toggleMute()
                onWheel: wheel => {
                    wheelAccumulator += wheel.angleDelta.y
                    const stepThreshold = 120

                    while (wheelAccumulator >= stepThreshold) {
                        Services.AudioService.changeVolume("1%+")
                        wheelAccumulator -= stepThreshold
                    }
                    while (wheelAccumulator <= -stepThreshold) {
                        Services.AudioService.changeVolume("1%-")
                        wheelAccumulator += stepThreshold
                    }
                }
            }
        }
    }
}
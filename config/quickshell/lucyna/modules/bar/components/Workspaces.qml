import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland
import "../../../core/theme" as Theme
import "../../../core/animations" as Animations

/*!
    Hyprland workspace indicators for this screen's monitor; click activates.
*/
Item {
    id: root
    implicitWidth: workspaceRow.implicitWidth
    width: implicitWidth
    Layout.fillHeight: true

    property var screen: null
    property string monitorName: screen ? screen.name : ""
    property int numWorkspaces: 10

    property int lastFocusedId: 0
    Component.onCompleted: if (Hyprland.focusedWorkspace) lastFocusedId = Hyprland.focusedWorkspace.id

    readonly property int cellHeight: 25
    readonly property int indicatorSize: 8
    readonly property int indicatorActiveWidth: 20

    RowLayout {
        id: workspaceRow
        anchors.centerIn: parent
        spacing: Theme.Tokens.space.md

        Repeater {
            model: Hyprland.workspaces
            delegate: Item {
                visible: modelData.id > 0 && modelData.monitor !== null && modelData.monitor.name === monitorName

                // Cell follows the animated indicator so the visible gap between
                // indicators stays constant (= RowLayout spacing) in every state.
                implicitWidth: visible ? workspaceDot.width : 0
                implicitHeight: visible ? cellHeight : 0

                Layout.alignment: Qt.AlignVCenter

                readonly property bool isActive: modelData.focused || modelData.active

                Rectangle {
                    id: workspaceDot
                    anchors.centerIn: parent
                    width: isActive ? indicatorActiveWidth : indicatorSize
                    height: indicatorSize
                    radius: height / 2
                    // Sequence drives a separate property so the color binding is never overwritten.
                    property color focusSequenceColor: Theme.Tokens.color.accent
                    // Single flag gates both the color source and the Behavior, so their
                    // evaluation order cannot cause a stray fade when the sequence starts.
                    property bool sequencing: false
                    property color sweepColor: Theme.Tokens.color.accent
                    property real sweepProgress: 0
                    property bool sweepFromRight: false
                    property bool sweepReversed: false
                    color: sequencing
                        ? focusSequenceColor
                        : (isActive ? Theme.Tokens.color.accent : Theme.Tokens.color.textMuted)

                    Behavior on width { Animations.MotionAnimation { role: "stateFast" } }
                    Behavior on color {
                        enabled: !workspaceDot.sequencing
                        Animations.MotionColorAnimation { role: "stateFast" }
                    }

                    Connections {
                        target: modelData
                        function onFocusedChanged() {
                            if (modelData.focused) {
                                // Start from the displayed color so an already-accent pill does not dip to gray.
                                workspaceDot.focusSequenceColor = workspaceDot.color;
                                workspaceDot.sweepReversed = modelData.id < root.lastFocusedId;
                                root.lastFocusedId = modelData.id;
                                workspaceDot.sequencing = true;
                                focusSequence.start();
                            } else {
                                focusSequence.stop();
                            }
                        }
                    }

                    Rectangle {
                        x: workspaceDot.sweepFromRight ? workspaceDot.width - width : 0
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        width: workspaceDot.sweepProgress * workspaceDot.width
                        radius: workspaceDot.radius
                        color: workspaceDot.sweepColor
                    }

                    SequentialAnimation {
                        id: focusSequence
                        onRunningChanged: if (!running) {
                            workspaceDot.sweepProgress = 0;
                            workspaceDot.sequencing = false;
                        }

                        PropertyAction { target: workspaceDot; property: "sweepFromRight"; value: workspaceDot.sweepReversed }
                        PropertyAction { target: workspaceDot; property: "sweepColor"; value: Theme.Tokens.color.aquamarine }
                        Animations.MotionAnimation { target: workspaceDot; property: "sweepProgress"; from: 0; to: 1; role: "stateMedium" }
                        ScriptAction { script: { workspaceDot.focusSequenceColor = workspaceDot.sweepColor; workspaceDot.sweepProgress = 0; } }

                        PropertyAction { target: workspaceDot; property: "sweepFromRight"; value: !workspaceDot.sweepReversed }
                        PropertyAction { target: workspaceDot; property: "sweepColor"; value: Theme.Tokens.color.textMuted }
                        Animations.MotionAnimation { target: workspaceDot; property: "sweepProgress"; from: 0; to: 1; role: "stateMedium" }
                        ScriptAction { script: { workspaceDot.focusSequenceColor = workspaceDot.sweepColor; workspaceDot.sweepProgress = 0; } }

                        PropertyAction { target: workspaceDot; property: "sweepFromRight"; value: workspaceDot.sweepReversed }
                        PropertyAction { target: workspaceDot; property: "sweepColor"; value: Theme.Tokens.color.accent }
                        Animations.MotionAnimation { target: workspaceDot; property: "sweepProgress"; from: 0; to: 1; role: "stateSettle" }
                        ScriptAction { script: { workspaceDot.focusSequenceColor = workspaceDot.sweepColor; workspaceDot.sweepProgress = 0; } }
                    }
                }

                Animations.HoverScale {
                    anchors.fill: parent
                    // Extend the hit area across the inter-indicator gap
                    anchors.leftMargin: -workspaceRow.spacing / 2
                    anchors.rightMargin: -workspaceRow.spacing / 2
                    target: workspaceDot
                    cursorShape: Qt.PointingHandCursor
                    onClicked: modelData.activate()
                }
            }
        }
    }
}

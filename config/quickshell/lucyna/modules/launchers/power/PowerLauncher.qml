import "../../../core/services/system" as Services
import "../../../core/services/panel" as PanelServices
import "../../../core/theme" as Theme
import "../../../core/components/atomic"
import "../../../core/components/panel"
import "../../../core/components/input"
import "../../../core/animations" as Animations
import QtQuick
import Quickshell.Io

/*!
    Power overlay: lock, suspend, reboot, shutdown. Destructive actions
    require a second press to confirm. Floating PanelBase panel: screen-centered
    on a PanelCard, rising from below the bottom screen edge.

    Toggle: qs -c lucyna ipc call togglePower handle
*/
PanelBase {
    id: root

    name: "powerLauncher"
    placement: PanelPlacement {
        type: PanelServices.PanelRules.Type.Floating
        size: Qt.size(root.cardW, root.cardH)
    }
    focusTarget: root.contentItem
    onShown: {
        root.confirmIndex = -1;
        root.selectedIndex = 0;
        uptimeProcess.running = true;
    }
    onHiding: root.confirmIndex = -1

    ModalKeymap {
        id: keymap
        enabled: root.open
        useBase: true
        axis: "horizontal"
        wrap: true
        modifierPolicy: "exactArrows"
        readonly property var handlers: ({
            moveLeft: () => root.selectedIndex = keymap.step(root.selectedIndex, -1, root.actions.length),
            moveRight: () => root.selectedIndex = keymap.step(root.selectedIndex, 1, root.actions.length),
            activate: () => root.trigger(root.selectedIndex),
            cancel: () => {
                if (root.confirmIndex !== -1)
                    root.confirmIndex = -1;
                else
                    root.hide();
            }
        })
        onActionTriggered: action => keymap.handlerFor(handlers, action)?.()
    }

    property int confirmIndex: -1   // -1 = none pending
    property int selectedIndex: 0
    onSelectedIndexChanged: confirmIndex = -1
    property string uptimeText: ""

    readonly property int cardW: 320
    readonly property int cardH: 124
    readonly property int actionSize: 60
    readonly property real statusLetterSpacing: 0.4
    readonly property int secondsPerMinute: 60
    readonly property int secondsPerHour: 3600

    readonly property var actions: [
        {
            "label": "Lock",
            "icon": "󰌾",
            "destructive": false,
            "action": () => {
                Services.PowerOptionService.lock();
            }
        },
        {
            "label": "Suspend",
            "icon": "󰒲",
            "destructive": false,
            "action": () => {
                Services.PowerOptionService.suspend();
            }
        },
        {
            "label": "Reboot",
            "icon": "󰜉",
            "destructive": true,
            "action": () => {
                Services.PowerOptionService.reboot();
            }
        },
        {
            "label": "Shutdown",
            "icon": "󰐥",
            "destructive": true,
            "action": () => {
                Services.PowerOptionService.shutdown();
            }
        }
    ]

    function trigger(index) {
        if (!root.open)
            return;
        const a = actions[index];
        if (a.destructive && confirmIndex !== index) {
            confirmIndex = index;
            return;
        }
        confirmIndex = -1;
        root.closeThenRun(a.action);
    }

    Process {
        id: uptimeProcess
        command: ["cat", "/proc/uptime"]
        stdout: SplitParser {
            onRead: data => {
                const secs = Math.floor(parseFloat(data.trim().split(" ")[0]));
                const h = Math.floor(secs / root.secondsPerHour);
                const m = Math.floor((secs % root.secondsPerHour) / root.secondsPerMinute);
                root.uptimeText = h > 0 ? h + "h " + m + "m" : m + "m";
            }
        }
    }

    IpcHandler {
        function handle() {
            root.toggle();
        }
        target: "togglePower"
    }

    contentComponent: Component {
        Item {
            implicitWidth: root.cardW
            implicitHeight: root.cardH
            width: implicitWidth
            height: implicitHeight
            focusPolicy: Qt.StrongFocus
            Keys.onPressed: event => keymap.handleKey(event)

            Column {
                anchors.centerIn: parent
                spacing: Theme.Tokens.space.md

                Row {
                    anchors.horizontalCenter: parent.horizontalCenter
                    spacing: Theme.Tokens.space.md

                    Repeater {
                        model: root.actions

                        delegate: Rectangle {
                            readonly property bool isConfirming: root.confirmIndex === index
                            readonly property bool isSelected: root.selectedIndex === index
                            readonly property bool isHighlighted: isConfirming || isSelected || hoverArea.containsMouse

                            width: root.actionSize
                            height: root.actionSize
                            radius: Theme.Tokens.radius.lg
                            color: isConfirming ? Theme.Tokens.color.accentSurface : Theme.Tokens.color.surface
                            border.width: isSelected ? Theme.Tokens.border.md : Theme.Tokens.border.sm
                            border.color: isHighlighted ? Theme.Tokens.color.accent : Theme.Tokens.color.border

                            QsText {
                                anchors.centerIn: parent
                                text: modelData.icon
                                role: "icon"
                                iconSize: "xl"
                                color: Theme.Tokens.color.textPrimary
                            }

                            MouseArea {
                                id: hoverArea
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onEntered: root.selectedIndex = index
                                onClicked: root.trigger(index)
                            }

                            Behavior on color {
                                Animations.MotionColorAnimation { role: "stateFast" }
                            }
                            Behavior on border.color {
                                Animations.MotionColorAnimation { role: "stateFast" }
                            }
                        }
                    }
                }

                QsText {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: root.confirmIndex !== -1 ? "Press again to " + root.actions[root.confirmIndex].label.toLowerCase() : (root.uptimeText ? "󱑎  Uptime: " + root.uptimeText : " ")
                    role: "body"
                    color: root.confirmIndex !== -1 ? Theme.Tokens.color.accent : Theme.Tokens.color.textPrimary
                    font.letterSpacing: root.statusLetterSpacing

                    Behavior on color {
                        Animations.MotionColorAnimation { role: "stateFast" }
                    }
                }
            }
        }
    }
}

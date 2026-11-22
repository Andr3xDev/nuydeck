import "../../../core/components/panel"
import "../../../core/components/atomic"
import "../../../core/components/input"
import "../../../core/animations" as Animations
import "../../../core/services/panel" as Services
import "../../../core/theme" as Theme
import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Widgets
import "state"

/*!
    AppLauncher — search-and-launch panel hanging from the bar.
    Toggle: qs -c lucyna ipc call panel toggle launcher
*/
PanelBase {
    id: root
    readonly property int itemCount: 3
    readonly property int itemHeight: Theme.Tokens.comp.clip.rowH
    readonly property int searchH: Theme.Tokens.comp.searchField.height
    readonly property int pad: Theme.Tokens.space.md
    readonly property int iconSize: 23
    readonly property int emptyStateSpacing: 10
    readonly property int cardW: 380
    readonly property int cardH: searchH + (itemHeight * itemCount) + (pad * 3) + Theme.Tokens.space.sm

    name: "launcher"
    placement: PanelPlacement {
        type: Services.PanelRules.Type.BarAttached
        shape: Services.PanelRules.Shape.Wide
        size: Qt.size(root.cardW, root.cardH)
    }
    focusTarget: root.contentItem ? root.contentItem.searchField : null
    onShown: {
        AppLauncherState.query = "";
        if (root.contentItem)
            root.contentItem.searchField.clear();
    }
    // Drop text focus before the keyboard mode drops, or Qt warns on text-input disable.
    onHiding: {
        if (root.contentItem)
            root.contentItem.searchField.focus = false;
    }

    contentComponent: Component {
        Item {
            id: card
            property alias searchField: searchInput.input
            implicitWidth: root.cardW
            implicitHeight: root.cardH
            width: root.cardW
            height: root.cardH

            ModalKeymap {
                id: keymap
                enabled: root.open
                useBase: true
                axis: "vertical"
                shiftOnly: true
                modifierPolicy: "fallback"
                passThroughUnmapped: true
                readonly property var handlers: ({
                    moveDown: () => appList.incrementCurrentIndex(),
                    moveUp: () => appList.decrementCurrentIndex(),
                    activate: () => appList.launchCurrent(),
                    cancel: () => root.hide()
                })
                onActionTriggered: action => keymap.handlerFor(handlers, action)?.()
            }

            Column {
                spacing: Theme.Tokens.space.sm
                anchors { fill: parent; margins: root.pad }

                SearchField {
                    id: searchInput
                    width: parent.width
                    placeholder: "Only Binary..."
                    onTextChanged: { AppLauncherState.query = text; appList.currentIndex = 0; }
                    onKeyPressed: event => keymap.handleKey(event)
                }

                ListView {
                    id: appList
                    property point lastCursor: Qt.point(-1, -1)
                    function launchCurrent() { if (currentItem) { const entry = currentItem.entry; root.hideIfAutoCloses(); AppLauncherState.launch(entry); } }
                    width: parent.width
                    height: root.itemHeight * root.itemCount
                    clip: true
                    model: AppLauncherState.filteredApps
                    currentIndex: 0
                    keyNavigationWraps: true
                    highlightRangeMode: ListView.ApplyRange
                    preferredHighlightBegin: 0
                    preferredHighlightEnd: height
                    highlightMoveDuration: Theme.Tokens.motion.role.stateFast.duration
                    // Rows can arrive after creation (DesktopEntries fills late) and would shift currentIndex off the top.
                    onCountChanged: { currentIndex = 0; positionViewAtBeginning(); }
                    highlight: Rectangle {
                        radius: Theme.Tokens.radius.lg
                        color: Theme.Tokens.color.surface
                    }
                    delegate: Item {
                        id: delegateRoot
                        readonly property bool isCurrent: appList.currentIndex === index
                        property var entry: modelData
                        width: appList.width
                        height: root.itemHeight
                        Row {
                            spacing: Theme.Tokens.space.md
                            scale: delegateRoot.isCurrent ? Theme.Tokens.comp.clip.selectedScale : 1
                            transformOrigin: Item.Left
                            anchors { verticalCenter: parent.verticalCenter; left: parent.left; leftMargin: root.pad }
                            IconImage { width: root.iconSize; height: root.iconSize; anchors.verticalCenter: parent.verticalCenter; source: Quickshell.iconPath(modelData.icon) }
                            QsText {
                                text: modelData.name
                                role: "title"
                                color: delegateRoot.isCurrent ? Theme.Tokens.color.textPrimary : Theme.Tokens.color.textMuted
                                font.letterSpacing: 0.2
                                height: root.itemHeight
                                verticalAlignment: Text.AlignVCenter
                                Behavior on color { Animations.MotionColorAnimation { role: "effectsEnter" } }
                            }
                            Behavior on scale { Animations.MotionAnimation { role: "stateFast" } }
                        }
                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            onEntered: appList.lastCursor = mapToItem(null, mouseX, mouseY)
                            onPositionChanged: selectOnCursorMove()
                            function selectOnCursorMove() {
                                const p = mapToItem(null, mouseX, mouseY);
                                if (p.x !== appList.lastCursor.x || p.y !== appList.lastCursor.y) {
                                    appList.lastCursor = p;
                                    appList.currentIndex = index;
                                }
                            }
                            onClicked: { const app = modelData; root.hideIfAutoCloses(); AppLauncherState.launch(app); }
                        }
                    }
                }
            }

            Column {
                anchors.centerIn: parent
                anchors.verticalCenterOffset: root.emptyStateSpacing
                width: parent.width - (root.pad * 4)
                spacing: root.emptyStateSpacing
                visible: appList.count === 0
                QsText { text: "\"We all make choices, but in the end... our choices make us\""; role: "subtitle"; muted: true; font.italic: true; horizontalAlignment: Text.AlignHCenter; width: parent.width; wrapMode: Text.WordWrap }
                QsText { text: "Andrew Ryan"; role: "body"; muted: true; horizontalAlignment: Text.AlignHCenter; width: parent.width }
                QsText { text: "BioShock"; role: "body"; muted: true; font.italic: true; horizontalAlignment: Text.AlignHCenter; width: parent.width }
            }
        }
    }
}

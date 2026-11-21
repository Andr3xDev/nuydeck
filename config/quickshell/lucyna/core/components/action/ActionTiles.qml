import QtQuick
import "../../theme" as Theme
import "../atomic"
import "../input"

// Fire-and-close tile groups: owns selection, the flash and key dispatch
// (ModalKeymap first, then number keys). Number keys 1-9 select tiles across
// all groups; tiles past 9 only by arrows or click. The host closes and runs
// the action on fired(action) (see RunAfterClose) and calls reset() on shown.
Row {
    id: root

    // Array<{ title, tiles: Array<{ id, glyph, action }> }>
    // Tile ids must be unique across all groups: fire() and the fired flash key on id.
    required property var groups

    // The host binds this to its open state; a closed panel handles no keys.
    property bool keysEnabled: true
    property int selectedIndex: -1

    signal fired(string action)
    signal cancelled()

    // Reset via reset() on the host's next open, not on close, so the
    // flash persists through the closing fade.
    property string firedTileId: ""

    readonly property var tiles: root.groups.reduce((all, group) => all.concat(group.tiles), [])

    spacing: Theme.Tokens.comp.actionTile.groupGap
    focus: true

    function reset() {
        root.firedTileId = "";
        root.selectedIndex = -1;
    }

    function firstIndexOf(groupIndex) {
        let count = 0;
        for (let i = 0; i < groupIndex; i++)
            count += root.groups[i].tiles.length;
        return count;
    }

    // Flash synchronously, then defer emitting `fired` with Qt.callLater so
    // the flashed state gets its own render pass before the host starts
    // closing.
    function fire(tile) {
        // Re-entrancy guard: key and mouse handlers stay live until the deferred emit runs.
        if (root.firedTileId !== "")
            return;
        root.firedTileId = tile.id;
        Qt.callLater(() => root.fired(tile.action));
    }

    readonly property var _handlers: ({
        moveLeft: () => root.selectedIndex = keymap.step(root.selectedIndex, -1, root.tiles.length),
        moveRight: () => root.selectedIndex = keymap.step(root.selectedIndex, 1, root.tiles.length),
        activate: () => {
            if (root.selectedIndex >= 0)
                root.fire(root.tiles[root.selectedIndex]);
        },
        cancel: () => root.cancelled()
    })

    // Not a visible child, so the Row gives it no spacing slot.
    ModalKeymap {
        id: keymap
        visible: false
        enabled: root.keysEnabled
        useBase: true
        axis: "horizontal"
        modifierPolicy: "fallback"
        passThroughUnmapped: true
        onActionTriggered: action => {
            const handler = keymap.handlerFor(root._handlers, action);
            if (handler)
                handler();
        }
    }

    Keys.onPressed: event => {
        if (!root.keysEnabled)
            return;
        keymap.handleKey(event);
        if (event.accepted)
            return;
        const isTileKey = event.key >= Qt.Key_1 && event.key <= Qt.Key_9;
        const tile = isTileKey ? root.tiles[event.key - Qt.Key_1] : undefined;
        if (tile) {
            root.fire(tile);
            event.accepted = true;
        }
    }

    Repeater {
        model: root.groups

        delegate: Row {
            id: group

            readonly property var groupData: modelData
            readonly property int groupIndex: index

            spacing: Theme.Tokens.comp.actionTile.groupGap

            Rectangle {
                visible: group.groupIndex > 0
                width: Theme.Tokens.border.sm
                height: groupColumn.height
                color: Theme.Tokens.color.border
            }

            Column {
                id: groupColumn
                spacing: Theme.Tokens.space.lg

                PanelHeader {
                    anchors.horizontalCenter: parent.horizontalCenter
                    title: group.groupData.title
                    separatorWidth: tileRow.implicitWidth
                }

                Row {
                    id: tileRow
                    spacing: Theme.Tokens.comp.actionTile.gap

                    Repeater {
                        model: group.groupData.tiles

                        delegate: ActionTile {
                            readonly property int flatIndex: root.firstIndexOf(group.groupIndex) + index

                            glyph: modelData.glyph
                            fired: root.firedTileId === modelData.id
                            selected: root.selectedIndex === flatIndex
                            onClicked: root.fire(modelData)
                        }
                    }
                }
            }
        }
    }
}

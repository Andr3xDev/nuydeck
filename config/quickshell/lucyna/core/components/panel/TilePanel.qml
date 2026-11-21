import QtQuick
import "../action"
import "../../services/panel" as Services
import "../../theme" as Theme

/*!
    TilePanel — titled groups of fire-and-close action tiles on the screen-edge
    kind. A panel is a data declaration: its required `name`, `groups`
    (see ActionTiles) and `dispatch`, a function called with the resolved
    action name once the panel has closed. Tiles are chosen by number keys 1-9
    across all groups; tiles past 9 only by arrows or click. `exclusionGroup`
    (optional) joins a mutual-exclusion group.

    Usage:
        TilePanel {
            name: "cursor-tools"
            groups: [{ title: "Cursor tools", tiles: [{ id: "zoom", glyph: "󰍉", action: "zoomCursor" }] }]
            dispatch: action => CursorToolsService[action]()
        }
*/
PanelBase {
    id: root

    required property var groups
    required property var dispatch
    property var exclusionGroup: Services.PanelRules.Group.None

    readonly property real _paddingX: Theme.Tokens.comp.tilePanel.paddingX
    readonly property real _paddingY: Theme.Tokens.comp.tilePanel.paddingY
    // Counts tiles only: each group title must stay no wider than its tile row, since the PanelHeader separator follows the row.
    readonly property real _tilesWidth: root.groups.reduce((sum, group) => {
        const count = group.tiles.length;
        return sum + count * Theme.Tokens.comp.actionTile.side + (count - 1) * Theme.Tokens.comp.actionTile.gap;
    }, 0) + (root.groups.length - 1) * (Theme.Tokens.border.sm + 2 * Theme.Tokens.comp.actionTile.groupGap)

    placement: PanelPlacement {
        type: Services.PanelRules.Type.Edge
        shape: Services.PanelRules.Shape.Wide
        size: Qt.size(root._tilesWidth + 2 * root._paddingX,
            Theme.Tokens.comp.tilePanel.contentHeight + 2 * root._paddingY)
        exclusionGroup: root.exclusionGroup
    }
    focusTarget: root.contentItem ? root.contentItem.actionTiles : null
    onShown: {
        if (root.contentItem)
            root.contentItem.actionTiles.reset();
    }

    // Built here, not in contentComponent: the content unloads as the panel closes, before the scheduled action runs.
    function _fire(action) {
        root.closeThenRun(() => root.dispatch(action));
    }

    contentComponent: Component {
        Item {
            property alias actionTiles: actionTiles
            implicitWidth: actionTiles.implicitWidth + 2 * root._paddingX
            implicitHeight: actionTiles.implicitHeight + 2 * root._paddingY

            ActionTiles {
                id: actionTiles
                x: root._paddingX
                y: root._paddingY
                groups: root.groups
                keysEnabled: root.open
                onCancelled: root.hide()
                onFired: action => {
                    if (!root.open)
                        return;
                    root._fire(action);
                }
            }
        }
    }
}

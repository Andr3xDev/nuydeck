import QtQuick
import "../../core/components/panel"
import "../../core/services/panel" as Services

/*!
    CursorToolsPanel — pick-color / zoom tiles hosted on quickshell-top.
    Toggle: qs -c lucyna ipc call panel toggle cursor-tools
*/
TilePanel {
    name: "cursor-tools"
    exclusionGroup: Services.PanelRules.Group.Capture
    dispatch: action => CursorToolsService[action]()
    groups: [
        {
            title: "Cursor tools",
            tiles: [
                { id: "picker", glyph: "󰈊", action: "pickColor" },
                { id: "zoom", glyph: "󰍉", action: "zoomCursor" }
            ]
        }
    ]
}

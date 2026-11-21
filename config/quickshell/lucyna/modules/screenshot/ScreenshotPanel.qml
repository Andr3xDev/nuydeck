import QtQuick
import "../../core/components/panel"
import "../../core/services/panel" as Services

/*!
    ScreenshotPanel — region and focused-monitor capture tiles.
    Toggle: qs -c lucyna ipc call panel toggle screenshot
    Keys 1-3 capture a selected region, 4-6 the focused monitor.
*/
TilePanel {
    name: "screenshot"
    exclusionGroup: Services.PanelRules.Group.Capture
    dispatch: action => ScreenshotService[action]()
    groups: [
        {
            title: "Region",
            tiles: [
                { id: "copyRegion", glyph: "", action: "copyRegion" },
                { id: "editRegion", glyph: "", action: "editRegion" },
                { id: "saveRegion", glyph: "󰆓", action: "saveRegion" }
            ]
        },
        {
            title: "Screen",
            tiles: [
                { id: "copyScreen", glyph: "", action: "copyFocusedMonitor" },
                { id: "editScreen", glyph: "󱋆", action: "editFocusedMonitor" },
                { id: "saveScreen", glyph: "󰆔", action: "saveFocusedMonitor" }
            ]
        }
    ]
}

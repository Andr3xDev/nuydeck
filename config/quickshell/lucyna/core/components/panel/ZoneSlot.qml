import QtQuick
import "../../animations" as Animations
import "../../services/panel" as Services

/*!
    ZoneSlot — per-panel glue to the zone layout. Follows PanelService.zoneOffset
    with the panelEnter glide and reports `settled` once the panel is neither
    gliding, entering nor exiting. Bind `offset` to the panel's centre-offset
    anchor and expose `settled` as the panel's `zoneSettled`.
*/
QtObject {
    id: root

    required property var panel

    readonly property real target: Services.PanelService.zoneOffset(root.panel)
    property real offset: root.target

    // Disabled while hidden so a newcomer appears directly in its slot; a visible partner glides.
    Behavior on offset {
        enabled: root.panel.progress > 0
        Animations.MotionAnimation {
            id: glide
            role: "panelEnter"
        }
    }

    // Pixels the live size may differ from the declared one (rounding) before the drift warning.
    readonly property real sizeDriftTolerance: 1

    readonly property bool settled: !glide.running && root.panel.progress === (root.panel.open ? 1 : 0)

    onSettledChanged: {
        if (!root.settled || !root.panel.open)
            return
        const declared = Services.PanelRules.surfaceSizeOf(root.panel.placement)
        if (Math.abs(root.panel.width - declared.width) > root.sizeDriftTolerance || Math.abs(root.panel.height - declared.height) > root.sizeDriftTolerance)
            console.warn("ZoneSlot: panel '" + root.panel.name + "' declared " + declared.width + "x" + declared.height
                + " but is " + root.panel.width + "x" + root.panel.height)
    }
}

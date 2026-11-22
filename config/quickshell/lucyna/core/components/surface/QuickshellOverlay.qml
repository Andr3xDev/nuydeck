import Quickshell
import Quickshell.Wayland
import QtQuick

/*!
    QuickshellOverlay — per-screen Overlay-layer surface, mapped only while a
    consumer's own content is active (the OSD is the first).

    Hosting API — deliberately not `QuickshellTop`'s literal-children `data`
    alias: a literal child would be destroyed on every unmap, since the
    `LazyLoader` below tears the whole `PanelWindow` down rather than merely
    hiding it (idle must produce zero Wayland surfaces/frames). Instead a
    consumer supplies its visual as a `Component` (`content`) — instantiated
    fresh on every map, destroyed on every unmap — while the consumer's own
    persistent state (e.g. OsdWindow's show/hide state machine) lives outside
    this surface entirely and only toggles `hasActiveContent`.

    Passive: empty input mask, no keyboard focus, no exclusive zone.
*/
Item {
    id: root

    property var screen: undefined
    property bool hasActiveContent: false
    property Component content: null

    LazyLoader {
        active: root.hasActiveContent
        component: panelWindowComponent
    }

    Component {
        id: panelWindowComponent

        PanelWindow {
            screen: root.screen
            color: "transparent"

            anchors {
                top: true
                bottom: true
                left: true
                right: true
            }

            exclusiveZone: 0
            mask: Region {}

            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.namespace: "quickshell-overlay"

            Loader {
                anchors.fill: parent
                sourceComponent: root.content
            }
        }
    }
}

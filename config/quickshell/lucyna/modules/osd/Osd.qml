import QtQuick
import Quickshell
import "items/volume"

Scope {
    id: root

    /*!
        Osd — module entry point.

        One OsdWindow per screen (Variants, same pattern as Bar); only the
        instance on the keyboard-focused screen displays. VolumeTrigger
        stays global: exactly one, it fires the
        OsdService requests that the per-screen windows react to.

        `hosts` is shell.qml's per-screen QuickshellOverlay Variants
        instances (mirrors ScreenshotPanel/CursorToolsPanel/PowerLauncher/
        CalendarPanel's own `hosts` wiring) — each OsdWindow finds the
        overlay surface matching its own screen.
    */
    required property var hosts

    VolumeTrigger {}
    Variants {
        // Restricted to screens that already have a matching overlay host
        // (mirrors shell.qml's own `!isExcluded(...)` filter on
        // `quickshellOverlayHosts` and stays correct if exclusions are ever
        // added there) — also empty until `hosts` populates, so no
        // OsdWindow is created before its `overlay` can resolve.
        model: Quickshell.screens.filter(scr => root.hosts.some(h => h.screen === scr))
        Scope {
            id: instance
            property var modelData
            OsdWindow {
                screen: instance.modelData
                overlay: root.hosts.find(h => h.screen === instance.modelData)
            }
        }
    }
}

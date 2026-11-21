import QtQuick
import "../../core/theme" as Theme
import "../../core/animations" as Animations
import "../../core/services/panel" as Services
import "../../core/bridge" as Ipc

/*!
    Top bar, one per screen: a thin registration wrapper hosted inside
    quickshell-top (core/components/surface/QuickshellTop.qml), not its own
    PanelWindow. quickshell-exclusion owns the reserved screen space.
    Registered in PanelService as PanelService.barNameFor(screen) so
    bar-attached panels can read its geometry (PanelService.barOffset).
    Content is in BarContent.qml.

    `open` drives PanelService/exclusion immediately; `visible` stays true
    while progress > 0 so the exit slide is seen.
*/
Item {
    id: bar

    required property QtObject screen

    anchors {
        left: parent.left
        top: parent.top
        right: parent.right
    }
    implicitHeight: Theme.Tokens.comp.bar.height
    height: implicitHeight

    readonly property string name: Services.PanelService.barNameFor(bar.screen)
    readonly property real visualExtent: bar.implicitHeight - Theme.Tokens.comp.panelSurface.depthInset

    property string parentName: ""
    property bool open: true
    readonly property bool active: bar.open

    // The bar hides while its own screen is fullscreen, without touching `open`/exclusion, so a
    // panel opened over fullscreen (QuickshellTop remaps above the fullscreen client) shows alone.
    readonly property bool _fullscreenOnThisScreen: Ipc.HyprlandIpc.isFullscreen(bar.screen)

    // Initialized from `open` directly (not via onOpenChanged) so no Behavior fires on shell start.
    property real progress: bar.open ? 1 : 0
    // Fullscreen hides and reappears instantly: `progress` is untouched, so the user's own slide still animates.
    visible: (bar.open || bar.progress > 0) && !bar._fullscreenOnThisScreen

    Behavior on progress {
        Animations.MotionAnimation {
            role: bar.open ? "panelEnter" : "panelExit"
        }
    }

    onOpenChanged: bar.progress = bar.open ? 1 : 0

    transform: Translate {
        y: -(1 - bar.progress) * bar.height
    }

    function show() {
        bar.open = true;
    }

    function hide() {
        bar.open = false;
    }

    function toggle() {
        if (bar.open)
            bar.hide();
        else
            bar.show();
    }

    Component.onCompleted: Services.PanelService.register(bar.name, bar)
    Component.onDestruction: Services.PanelService.unregister(bar.name, bar)

    BarContent {
        anchors.fill: parent
        screen: bar.screen
    }
}

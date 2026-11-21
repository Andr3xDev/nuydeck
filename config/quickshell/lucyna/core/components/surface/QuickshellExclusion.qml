import Quickshell
import Quickshell.Wayland
import QtQuick
import "../../theme" as Theme
import "../../services/panel" as Services

/*!
    QuickshellExclusion — reserves the bar's exclusive zone independent of
    quickshell-top's fullscreen-hide state, one instance per screen.

    Draws nothing (transparent, empty mask) and is always visible/mapped —
    its only job is holding screen space so the desktop/other windows never
    tile into the bar's strip, but only while the bar itself is open:
    hiding the bar (PanelService.toggle(PanelService.barNameFor(screen)), e.g.
    the toggle_bar keybind) frees the strip too.

    exclusiveZone height matches Bar.qml's own visible height exactly
    (Theme.Tokens.comp.bar.height, no added margin): any extra would leave a
    transparent strip between the bar and the first tiled window, visible as
    a thin light line across the screen.

    _barInstance is read through PanelService.getInstance() inside a binding,
    so it re-evaluates when the bar registers; no instantiation order is
    needed. _barOpen defaults to true until the bar registers, so the strip is
    reserved from first paint.
*/
PanelWindow {
    id: root

    readonly property var _barInstance: Services.PanelService.getInstance(Services.PanelService.barNameFor(root.screen))
    readonly property bool _barOpen: root._barInstance ? root._barInstance.open : true

    anchors {
        top: true
        left: true
        right: true
    }

    implicitHeight: Theme.Tokens.comp.bar.height
    exclusiveZone: root._barOpen ? Theme.Tokens.comp.bar.height : 0
    color: "transparent"
    visible: true

    WlrLayershell.layer: WlrLayer.Top
    WlrLayershell.namespace: "quickshell-exclusion"

    mask: Region {}
}

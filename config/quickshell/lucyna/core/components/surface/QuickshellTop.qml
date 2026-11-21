import Quickshell
import Quickshell.Wayland
import QtQuick
import "../../theme" as Theme
import "../../services/panel" as Services
import "../../animations" as Animations
import "../../bridge" as Ipc

/*!
    QuickshellTop — per-screen Top-layer surface hosting the bar and every
    panel (PanelBase contract: progress, scrimEnabled, needsKeyboard,
    inputRect, open). One instance per screen, namespace "quickshell-top"
    (no per-screen suffix; Variants supplies the per-screen split).

    Hosting API:
    - `data` (default property): panel/bar content Items live here as
      children, positioned by their own anchors within this window.
    - `panels`: the hosted panels this surface aggregates mask and keyboard
      focus from, written only by attachPanel()/detachPanel(). Each entry
      needs { progress, open, needsKeyboard, scrimEnabled, inputRect, host }.
    - `barItem`: the bar content's own Item, contributing its region to the
      mask while the bar is open and its screen is not fullscreen. Gated on
      `open` rather than `visible`: `visible` stays true through the exit
      slide, but a bar mid-slide-out must not accept clicks through its mask.

    Mask (union): each hosted panel's inputRect while progress > 0; the bar
    region while barItem is open and the screen is not fullscreen; the
    full-window region while the scrim is visible.

    Keyboard focus is Exclusive only on the focused monitor while a hosted
    panel is open and needsKeyboard, or a queued panel holds the keyboard.
    refocusKeyboard() drops to None and rebinds to force Hyprland to re-grant
    focus when handing it between open panels; panels pass their content Item
    for forceActiveFocus().

    No exclusiveZone here — quickshell-exclusion (QuickshellExclusion.qml)
    owns reserving the bar's screen space, independent of this surface's
    fullscreen-hide state: it is a separate PanelWindow with its own
    always-`visible: true`.

    Shared scrim: visible while any hosted panel has scrimEnabled and
    progress > 0, or PanelService.scrimHeld; opacity is the max progress (1
    when held), no Behavior. Painted below panels. Click calls
    OverlayService.closeTopOn().
*/
PanelWindow {
    id: root

    property var panels: []
    property Item barItem: null
    default property alias data: contentItem.data
    // Exposes the content Item directly so a single logical panel can
    // dynamically reparent itself onto whichever screen's QuickshellTop is
    // the current show() target, instead of being declared as this surface's own child.
    readonly property alias contentHost: contentItem

    // A queued replacement or join keeps Exclusive through the wait, so keys never reach the window underneath.
    readonly property bool _needsKeyboard: root.panels.some(p => p.open && p.needsKeyboard) || Services.PanelService.keyboardHeld(root.screen)

    // Hidden only while this screen is fullscreen AND no hosted panel has
    // progress > 0 — a panel mid-open/close keeps the surface visible so its
    // own animation isn't cut off. A fullscreen window renders above the Top
    // layer on this repo's Hyprland config.
    readonly property bool _fullscreenOnThisScreen: Ipc.HyprlandIpc.isFullscreen(root.screen)
    readonly property bool _anyPanelProgressAboveZero: root.panels.some(p => p.progress > 0)

    // progress > 0 (not open) so the scrim fades out with the panel's own
    // exit animation instead of vanishing the instant `open` flips false.
    readonly property var _scrimPanels: root.panels.filter(p => p.scrimEnabled && p.progress > 0)
    // Pending-hold keeps the scrim full across the gap of a replacement or join, so it never dips (no scrim Behavior).
    readonly property bool _scrimHeld: Services.PanelService.scrimHeld(root.screen)
    readonly property bool _scrimVisible: root._scrimPanels.length > 0 || root._scrimHeld
    readonly property real _scrimOpacity: Math.max(root._scrimPanels.length > 0 ? Math.max(...root._scrimPanels.map(p => p.progress)) : 0, root._scrimHeld ? 1 : 0)

    anchors {
        top: true
        bottom: true
        left: true
        right: true
    }

    color: "transparent"

    visible: !(root._fullscreenOnThisScreen && !root._anyPanelProgressAboveZero)

    WlrLayershell.layer: WlrLayer.Top
    WlrLayershell.namespace: "quickshell-top"
    WlrLayershell.keyboardFocus: root._keyboardMode
    WlrLayershell.exclusionMode: ExclusionMode.Ignore

    // Only the focused monitor's surface holds the keyboard; an open panel on another monitor must not steal it.
    readonly property bool _monitorFocused: Ipc.HyprlandIpc.isFocused(root.screen)

    // Hyprland snaps focus back to the old monitor 4-9 ms after the user leaves it while an Exclusive layer holds keys;
    // that one transient is answered by re-dispatching the move, and ownership stays off until the window expires.
    readonly property int monitorSnapbackWindowMs: 50
    property string _reassertTarget: ""
    Timer {
        id: _reassertWindow
        interval: root.monitorSnapbackWindowMs
        onTriggered: root._reassertTarget = ""
    }

    readonly property bool _keyboardOwner: root._monitorFocused && !_reassertWindow.running
    readonly property var _keyboardMode: root._needsKeyboard && root._keyboardOwner ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    on_MonitorFocusedChanged: {
        if (!root._monitorFocused) {
            const focusedName = Ipc.HyprlandIpc.focusedMonitorName();
            if (root._needsKeyboard && !_reassertWindow.running && focusedName !== "") {
                root._reassertTarget = focusedName;
                _reassertWindow.restart();
            }
        } else if (root._reassertTarget !== "") {
            Ipc.HyprlandIpc.focusMonitor(root._reassertTarget);
            root._reassertTarget = "";
        }
    }

    on_KeyboardOwnerChanged: {
        if (!root._keyboardOwner || !root._needsKeyboard)
            return;
        const top = Services.OverlayService.topOn(root.screen);
        if (top && top.focus)
            top.focus();
    }

    function refocusKeyboard(focusTarget) {
        root.WlrLayershell.keyboardFocus = WlrKeyboardFocus.None;
        root.WlrLayershell.keyboardFocus = Qt.binding(function () {
            return root._keyboardMode;
        });
        if (focusTarget)
            focusTarget.forceActiveFocus();
    }

    // The only writers of `panels`; attaching moves a panel off its previous host.
    function attachPanel(panel) {
        if (panel.host && panel.host !== root)
            panel.host.detachPanel(panel);
        root.panels = root.panels.concat([panel]);
    }

    function detachPanel(panel) {
        root.panels = root.panels.filter(p => p !== panel);
    }

    // Shortcuts live in an Item: PanelWindow is a proxy object, not a window, so a non-Item parent may not resolve.
    // Always consumed while a panel holds the keyboard, even with no target; inert otherwise.
    Item {
        Shortcut {
            sequence: "Ctrl+H"
            autoRepeat: false
            onActivated: Services.PanelService.focusDirection(root.screen, "left")
        }
        Shortcut {
            sequence: "Ctrl+J"
            autoRepeat: false
            onActivated: Services.PanelService.focusDirection(root.screen, "down")
        }
        Shortcut {
            sequence: "Ctrl+K"
            autoRepeat: false
            onActivated: Services.PanelService.focusDirection(root.screen, "up")
        }
        Shortcut {
            sequence: "Ctrl+L"
            autoRepeat: false
            onActivated: Services.PanelService.focusDirection(root.screen, "right")
        }
    }

    Rectangle {
        id: scrim
        anchors.fill: parent
        color: Theme.Tokens.color.scrim
        visible: root._scrimVisible
        opacity: root._scrimOpacity

        MouseArea {
            anchors.fill: parent
            onClicked: Services.OverlayService.closeTopOn(root.screen)
        }
    }

    // Initial appearance: the content fades in once at shell start, using
    // Tokens.motion.role.panelEnter; BarContent plays no entry animation of its own.
    Item {
        id: contentItem
        anchors.fill: parent
        opacity: 0

        Behavior on opacity {
            Animations.MotionAnimation {
                role: "panelEnter"
            }
        }

        Component.onCompleted: contentItem.opacity = 1
    }

    // Bound to barItem's untransformed x/y/width/height, not `item:`: item mode maps through the slide
    // Translate but only recomputes on geometry signals, so on show() it would commit a mid-slide,
    // off-screen rect and never refresh it.
    Region {
        id: barRegion
        x: root.barItem ? root.barItem.x : 0
        y: root.barItem ? root.barItem.y : 0
        width: root.barItem ? root.barItem.width : 0
        height: root.barItem ? root.barItem.height : 0
    }

    Region {
        id: scrimRegion
        width: root._scrimVisible ? root.width : 0
        height: root._scrimVisible ? root.height : 0
    }

    Variants {
        id: panelRegions
        model: root.panels
        delegate: Region {
            required property var modelData

            x: modelData.inputRect.x
            y: modelData.inputRect.y
            width: modelData.progress > 0 ? modelData.inputRect.width : 0
            height: modelData.progress > 0 ? modelData.inputRect.height : 0
        }
    }

    mask: Region {
        regions: (root.barItem && root.barItem.open && !root._fullscreenOnThisScreen ? [barRegion] : []).concat(root._scrimVisible ? [scrimRegion] : []).concat(panelRegions.instances)
    }
}

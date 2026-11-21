import QtQuick
import "../../animations" as Animations
import "../../services/panel" as Services
import "../../services/system" as SystemServices
import "../../theme" as Theme

/*!
    PanelBase — the lifecycle every hosted panel shares: show/hide/toggle,
    progress motion, host attach, OverlayService claim, PanelService
    registration, zone slot and the chrome. A panel declares its name,
    placement and contentComponent; this is the public contract that
    PanelService, QuickshellTop and OverlayService read.

    The panel is declared once under ShellRoot and reparented onto the
    QuickshellTop of the screen `targetScreen()` names when shown. `hosts` is
    the per-screen QuickshellTop instances (shell.qml's Variants.instances).
    Panel-specific reset runs in `onShown`; `onHiding` runs just before the
    request to close, `onKeyboardFocused` after the keyboard focus is re-armed.

    The kind comes from `placement.type` and selects a row of
    PanelRules.typeSpecs: where the panel attaches (bar with
    `barOffset`/`joinBand`, screen edge, or centered), its motion (staged
    base-first inside an EdgeClip, or a whole-panel rise) and its chrome
    (PanelShell or PanelCard).
*/
Item {
    id: root

    required property var hosts
    required property string name
    required property PanelPlacement placement
    required property Component contentComponent
    property string parentName: ""
    property Item focusTarget: null
    property var targetScreen: () => SystemServices.ScreenService.focusedScreen()
    property bool scrimEnabled: true
    property bool needsKeyboard: root.open

    property bool _open: false
    readonly property bool open: root._open
    readonly property bool active: root._open
    readonly property bool present: root.open || root.progress > 0
    property real progress: 0

    property var _host: null
    readonly property var host: root._host
    readonly property string attachEdge: Services.PanelRules.zoneOf(root.placement)
    readonly property bool zoneSettled: zoneSlot.settled
    // Resting rect, not the slide-translated one: transform moves paint only, not x/y.
    // Mid-slide the mask briefly covers the landing spot; harmless.
    readonly property rect inputRect: Qt.rect(root.x, root.y, root.width, root.height)
    readonly property Item contentItem: root.chrome ? root.chrome.contentItem : null

    readonly property var _spec: Services.PanelRules.specOf(root.placement.type)
    readonly property bool _centered: root._spec.centered
    readonly property bool _staged: root._spec.staged
    readonly property bool _verticalEdge: Services.PanelRules.isVerticalZone(root.attachEdge)

    // Bar-attached only on the top edge: a BarAttached panel on a side zone gets no bar offset, join band or stroke tip at the bar line.
    readonly property bool _barAttached: root._spec.barAttached && root.attachEdge === "top"

    // The bar's slide-following offset on the host screen; 0 when the panel is not bar-attached or the bar
    // is missing or hidden, and the join band with it.
    readonly property real barOffset: root._barAttached && root.host && root.host.screen ? Services.PanelService.barOffset(root.host.screen) : 0
    readonly property real joinBand: root.barOffset > 0 ? Theme.Tokens.comp.panelSurface.joinOverlap : 0

    // Outside the shell content: a scheduled action must outlive the close unload. The
    // settle margin keeps a capture from grabbing the panel's last frame.
    readonly property alias closeRunner: closeRunner
    RunAfterClose {
        id: closeRunner
        progress: root.progress
        settleDelay: Theme.Tokens.motion.role.stateFast.duration
    }

    signal shown
    signal hiding
    signal keyboardFocused

    Behavior on progress {
        Animations.MotionAnimation {
            role: root.open ? "panelEnter" : "panelExit"
        }
    }
    onOpenChanged: root.progress = root.open ? 1 : 0

    // Anchored to the host's content Item directly, not the literal QML `parent`: until the
    // first show() reparents it, this Item sits under ShellRoot, not under any QuickshellTop.
    anchors.top: root.attachEdge === "top" && root.host ? root.host.contentHost.top : undefined
    anchors.topMargin: root.barOffset
    anchors.bottom: root.attachEdge === "bottom" && root.host ? root.host.contentHost.bottom : undefined
    anchors.left: root.attachEdge === "left" && root.host ? root.host.contentHost.left : undefined
    anchors.right: root.attachEdge === "right" && root.host ? root.host.contentHost.right : undefined
    anchors.centerIn: root._centered && root.host ? root.host.contentHost : undefined
    anchors.horizontalCenter: !root._centered && !root._verticalEdge && root.host ? root.host.contentHost.horizontalCenter : undefined
    anchors.horizontalCenterOffset: root._verticalEdge ? 0 : zoneSlot.offset
    anchors.verticalCenter: !root._centered && root._verticalEdge && root.host ? root.host.contentHost.verticalCenter : undefined
    anchors.verticalCenterOffset: root._verticalEdge ? zoneSlot.offset : 0

    ZoneSlot {
        id: zoneSlot
        panel: root
    }

    readonly property Item chrome: chromeLoader.item

    implicitWidth: root.chrome ? root.chrome.panelWidth : 0
    implicitHeight: root.chrome ? root.chrome.panelHeight : 0
    width: implicitWidth
    height: implicitHeight
    visible: root.present

    // Whole-panel rise for non-staged panels: the centered card travels down to the screen bottom.
    // Staged panels move inside their EdgeClip. No opacity fade: quickshell-top's window bounds clip it.
    readonly property real _slideDistance: {
        if (root._staged || !root._centered)
            return 0;
        return root.host && root.host.screen ? (root.host.screen.height - root.y) + root.height : 0;
    }
    transform: Translate {
        y: (1 - root.progress) * root._slideDistance
    }

    function show() {
        if (root.open) {
            root.focusKeyboard();
            return;
        }
        const target = root.targetScreen();
        const host = root.hosts.find(h => h.screen === target);
        if (!host) {
            console.warn("PanelBase: no quickshell-top host for screen '" + (target ? target.name : "null") + "' (" + root.name + ")");
            Services.PanelService.cancelPending(root.name);
            return;
        }
        if (!Services.PanelService.makeRoom(root, host))
            return;
        if (root.host !== host) {
            host.attachPanel(root);
            root.parent = host.contentHost;
            root._host = host;
        }
        Services.OverlayService.claim({
            name: root.name,
            screen: target,
            close: () => root.hide(),
            focus: () => root.focusKeyboard()
        });
        closeRunner.cancel();
        root._open = true;
        root.shown();
        root.focusKeyboard();
    }

    function hide() {
        Services.PanelService.cancelPending(root.name);
        Services.OverlayService.release(root.name);
        if (!root.open)
            return;
        root.hiding();
        root._open = false;
    }

    function hideIfAutoCloses() {
        if (Services.PanelRules.autoCloses(root.placement))
            root.hide();
    }

    // Runs `action` once the panel has finished closing; it waits for a manual close when the panel does not auto-close.
    function closeThenRun(action) {
        root.hideIfAutoCloses();
        closeRunner.schedule(action);
    }

    function toggle() {
        if (root.open || Services.PanelService.isPending(root.name))
            root.hide();
        else
            root.show();
    }

    // Focus hook for OverlayService: re-arms keyboard focus when the consumer above this one closes.
    function focusKeyboard() {
        if (!root.open || !root.host)
            return;
        root.host.refocusKeyboard(root.focusTarget);
        root.keyboardFocused();
    }

    Component.onCompleted: Services.PanelService.register(root.name, root)
    Component.onDestruction: {
        if (root.host)
            root.host.detachPanel(root);
        Services.OverlayService.release(root.name);
        Services.PanelService.unregister(root.name, root);
    }

    EdgeClip {
        width: root.width
        height: root.height
        attachEdge: root._staged ? root.attachEdge : ""
        joinOutset: root._staged && root.chrome ? root.joinBand * root.chrome.baseReveal : 0

        Loader {
            id: chromeLoader
            sourceComponent: root._spec.card ? cardChrome : shellChrome
        }
    }

    Component {
        id: shellChrome
        PanelShell {
            attachEdge: root.attachEdge
            rayTone: Services.PanelService.focusRayTone(root.name)
            progress: root._staged ? root.progress : 1
            strokeTipOutset: root._barAttached ? 0 : Theme.Tokens.comp.panelSurface.depthInset
            availableWidth: root.host && root.host.screen ? root.host.screen.width : 0
            availableHeight: root.host && root.host.screen ? root.host.screen.height : 0
            contentActive: root.present
            contentComponent: root.contentComponent
        }
    }

    Component {
        id: cardChrome
        PanelCard {
            rayTone: Services.PanelService.focusRayTone(root.name)
            contentActive: root.present
            contentComponent: root.contentComponent
        }
    }
}

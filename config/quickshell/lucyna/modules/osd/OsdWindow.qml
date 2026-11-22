import QtQuick
import "../../core/services/system" as Services
import "../../core/animations" as Animations
import "../../core/theme" as Theme

/*!
    OsdWindow — one of the module's per-screen instances (mounted by
    Osd.qml's Variants; the delegate assigns `screen` and `overlay`, the
    matching screen's QuickshellOverlay instance from shell.qml's
    `quickshellOverlayHosts`).

    No longer owns a `PanelWindow`/namespace of its own (fixes the doubled
    "osd" namespace bug, `quickshell-osd-osd-<screen>`, pre-migration
    `OsdWindow.qml:60,62`) — this `Item` is permanent (never destroyed,
    listens to `OsdService` regardless of visibility) and only supplies
    `overlay.content`, a `Component` closing over this file's own `root.*`
    state, so `QuickshellOverlay`'s `LazyLoader` can create/destroy the
    actual pill visual while this state machine survives every map/unmap
    cycle untouched.

    Single-progress-property pattern (panel-motion-pattern spec): `progress`
    is the only animated property, driven by exactly one `Behavior`, and
    `overlay.hasActiveContent` mirrors `progress > 0` — the overlay surface
    maps only while the pill is showing/visible/hiding, and unmaps the
    instant the exit animation settles at 0 (idle produces no frames).

    Display state machine (idle / showing / visible / hiding), all
    transitions driven by OsdService signals:
      - activated(idle)        → mount view, slide in, arm timer
      - activated(visible)     → swap view, arm timer
      - activated(hiding)      → CANCEL hide first, then swap/keep view,
                                 slide back in, arm timer
      - refreshed(idle, focused) → displays like an activation;
        refreshed(visible)       → restart timer only, view untouched;
        during hiding this also cancels the hide
      - slide-out stopped while STILL hiding → OsdService.hidden(),
                                  state idle

    The `stopped` guard is what makes cancellation safe: a canceled hide is
    never still in `hiding`, so it can never emit a spurious hidden() that
    clears a freshly activated request. The instance on the focused screen
    wins the display; all instances share the OsdService singleton, so
    arbitration stays global.
*/
Item {
    id: root

    required property var screen
    required property var overlay

    property string state: "idle"
    property var currentRequest: null
    property real progress: 0

    // ~10% of screen height — settled design decision (design D-e)
    readonly property int restMargin: root.screen ? Math.round(root.screen.height / 10) : 0

    Behavior on progress {
        Animations.MotionAnimation {
            role: root.state === "hiding" ? "panelExit" : "panelEnter"

            onStopped: {
                // Complete ONLY on natural exit end: state hiding AND
                // progress truly 0. An interrupted exit (retarget) also
                // fires stopped, but with a mid-way progress — the extra
                // check prevents a spurious hidden() that would clear a
                // freshly activated request.
                if (root.state === "hiding" && root.progress === 0) {
                    OsdService.hidden()
                    root.state = "idle"
                } else if (root.state === "showing") {
                    root.state = "visible"
                }
            }
        }
    }

    // Maps/unmaps QuickshellOverlay exactly while this OSD is
    // showing/visible/hiding — idle (progress 0) unmaps it entirely.
    //
    // Both bindings below are guarded by `when` rather than assigned
    // imperatively in Component.onCompleted: `overlay` is a property
    // binding (`root.hosts.find(...)` in Osd.qml) that can still be
    // undefined the instant this delegate completes — Osd.qml's model
    // now only admits screens with a matching host, but a declarative
    // Binding is the defensive, self-healing form (re-evaluates once
    // `overlay` resolves) instead of a one-shot dereference that would
    // throw if it ever ran too early again.
    Binding {
        target: root.overlay
        property: "hasActiveContent"
        value: root.progress > 0
        when: root.overlay !== undefined && root.overlay !== null
    }

    Binding {
        target: root.overlay
        property: "content"
        value: root._pillComponent
        when: root.overlay !== undefined && root.overlay !== null
    }

    Connections {
        target: OsdService
        function onActivated(request) { root._activate(request) }
        function onRefreshed(request) { root._refresh(request) }
    }

    /*!
        Activated: new or replaced request — mount/swap the view, show it,
        (re)arm the auto-hide timer.
    */
    function _activate(request) {
        // Only the instance on the keyboard-focused screen displays.
        const focused = Services.ScreenService.focusedScreen()
        if (!focused || focused.name !== root.screen.name) return
        root.currentRequest = request
        if (root.state === "hiding") {
            root.state = "visible"   // cancel hide BEFORE retargeting progress
        } else if (root.state === "idle") {
            root.state = "showing"
        }
        root.progress = 1
        hideTimer.restart()
    }

    /*!
        Refreshed: same id re-request — record update (new timeout honored),
        timer restart, view instance untouched.
    */
    function _refresh(request) {
        // Only the focused-screen instance handles it; the visible instance already owns the display
        const focused = Services.ScreenService.focusedScreen()
        if (!focused || focused.name !== root.screen.name) return
        // Cross-screen same-id: this idle instance owns the focused screen,
        // so it must display the request, not just restart a timer.
        if (root.state === "idle") {
            root._activate(request)
            return
        }
        root.currentRequest = request
        if (root.state === "hiding") {
            root.state = "visible"   // cancel hide, view untouched
            root.progress = 1
        }
        hideTimer.restart()
    }

    Timer {
        id: hideTimer
        interval: (root.currentRequest && root.currentRequest.timeout) > 0
            ? root.currentRequest.timeout
            // default auto-hide, user-approved (design D-e)
            : 1500
        onTriggered: {
            root.state = "hiding"
            root.progress = 0
        }
    }

    // The pill's own visual — a Component so QuickshellOverlay's LazyLoader
    // can create/destroy it on every map/unmap while still reading this
    // file's `root.*` state (Components close over their declaring file's
    // ids, regardless of where they end up instantiated).
    property Component _pillComponent: Component {
        Item {
            // Transparent full-window wrapper — QuickshellOverlay's own
            // Loader anchors.fill's this component's root to the whole
            // screen (its fixed contract). The *visible* pill below is a
            // normal child positioned within this invisible area and is never
            // itself anchors.fill'd, so it sizes from its own content instead
            // of inheriting the window's size.
            anchors.fill: parent

            Rectangle {
                id: pill
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom
                anchors.bottomMargin: root.restMargin

                // Reactive sizing: follows the loaded view's own implicit
                // size (Loader mirrors its loaded item's implicitWidth/
                // implicitHeight) plus padding — same padding-token pattern
                // already used by QsButton.qml:70-71. VolumeView's track
                // animates its own implicitWidth, so the pill resizes with
                // it automatically.
                implicitWidth: viewLoader.implicitWidth + Theme.Tokens.space.md * 2
                implicitHeight: viewLoader.implicitHeight + Theme.Tokens.space.sm * 2

                radius: Theme.Tokens.radius.lg
                color: Theme.Tokens.color.surface

                // Slide from below the screen edge (full extent: the pill's
                // own height plus its resting margin) up to its resting
                // spot — no fade, single progress-driven transform
                // (panel-motion-pattern).
                transform: Translate {
                    y: (1 - root.progress) * (pill.height + root.restMargin)
                }

                Loader {
                    id: viewLoader
                    anchors.centerIn: parent
                    sourceComponent: root.currentRequest ? root.currentRequest.view : null
                }
            }
        }
    }
}

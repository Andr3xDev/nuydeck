import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import "services"
import "components"
import "../../core/bridge" as Ipc
import "../../core/theme" as Theme
import "../../core/animations" as Animations

/*!
    MonitorManager — Mediator + PanelWindow.

    Single instantiation point for the monitor module. Wires services
    to child components; contains no layout and no business logic.

    Stays on its own standalone PanelWindow (decision 2026-09-29, option 1 —
    see runtime-checks.md's 9.3 entry): this is the one component whose own
    action (disabling the monitor it is currently displayed on, live via
    `hyprctl keyword monitor`) can destroy a per-screen QuickshellTop host,
    which would take a hosted panel down with it. It still adopts the
    single-progress-property motion pattern internally (`_isOpen`/`progress`,
    one Behavior, slide-only transform, scrim tracking `progress`) so its
    motion is consistent with every quickshell-top-hosted panel even though
    its hosting model is not.

    Toggle via: qs ipc call monitorManager.handle
*/
PanelWindow {
    id: root

    // ── Layout constants ─────────────────────────────────
    readonly property int _panelW: 900
    readonly property int _panelH: 580

    // ── Internals ─────────────────────────────────────────
    property string _pendingProfileId: ""
    property bool   _startupDone:      false
    property string _selectedName:     ""
    property string _activeProfileId:  ""   // persists across open/close

    // ── Motion state ──────────────────────────────────────
    // Logical open/closed state, distinct from the window's own `visible`
    // (which must stay true through the exit animation — see below). Named
    // `_isOpen` rather than `open` since `open()` is this component's public
    // toggle function (IpcHandler/Escape callers), and QML disallows a
    // property and a method sharing one name on the same object.
    property bool _isOpen: false
    property real progress: 0

    // Single animation owner for this panel's whole open/close motion —
    // same shape PowerLauncher.qml uses.
    Behavior on progress {
        Animations.MotionAnimation {
            role: root._isOpen ? "panelEnter" : "panelExit"
        }
    }

    readonly property bool _shouldBeActive: root._isOpen || root.progress > 0

    // ── Window setup ─────────────────────────────────────
    visible:                     root._shouldBeActive
    color:                       "transparent" // layer-shell root — must not paint, real panel bg is ManagerPanel
    WlrLayershell.layer:         WlrLayer.Overlay
    WlrLayershell.namespace:     "quickshell-monitor-manager"
    WlrLayershell.keyboardFocus: root._isOpen
                                     ? WlrKeyboardFocus.Exclusive
                                     : WlrKeyboardFocus.None
    exclusionMode:               ExclusionMode.Ignore

    anchors { top: true; bottom: true; left: true; right: true }

    // ── IPC entry (keybind / external) ───────────────────
    IpcHandler {
        target: "monitorManager"
        function handle() { root.toggle() }
    }

    // ── Service listeners ─────────────────────────────────
    Connections {
        target: MonitorService

        function onCommitted(ok, errorMessage) {
            if (ok) {
                root._pendingProfileId = ""
            } else {
                console.warn("MonitorManager: commit failed —", errorMessage)
            }
        }

        function onMonitorsChanged() { root._tryStartupApply() }
    }

    // Startup restore: fires when ProfileService finishes loading from disk.
    property Connections _profileConnections: Connections {
        target: ProfileService
        function onLoaded() { root._tryStartupApply() }
    }

    // ── Public API ────────────────────────────────────────
    function toggle() { root._isOpen ? close() : open() }

    function open() {
        root._isOpen  = true
        root.progress = 1
        MonitorService.refresh()
    }

    function close() {
        root._isOpen            = false
        root.progress           = 0
        root._pendingProfileId  = ""
        root._selectedName      = ""
        panel.reset()
    }

    // ── Escape to close ───────────────────────────────────
    Shortcut {
        sequence:    "Escape"
        enabled:     root._isOpen
        onActivated: root.close()
    }

    // ── Dark overlay (click-outside closes) ──────────────
    // Opacity tracks `progress` directly — no independent clock/Behavior,
    // same "scrim follows the single progress property" shape used across
    // the panel-motion-pattern migration.
    Rectangle {
        id: overlay
        anchors.fill: parent
        color: Theme.Tokens.color.scrim
        opacity: root.progress

        MouseArea { anchors.fill: parent; onClicked: root.close() }
    }

    // Distance from the panel's centered resting position down past the
    // bottom edge of the (screen-filling) window, plus its own height, so
    // progress:0 sits fully below the visible area — same "slide over full
    // extent, no fade" shape PowerLauncher.qml uses for its own centered
    // panel.
    readonly property real _slideDistance: (root.height - panel.y) + panel.height

    // ── Panel ─────────────────────────────────────────────
    ManagerPanel {
        id: panel

        anchors.centerIn: parent
        width:  root._panelW
        height: root._panelH

        transform: Translate {
            y: (1 - root.progress) * root._slideDistance
        }

        selectedName:    root._selectedName
        activeProfileId: root._activeProfileId

        onCloseRequested:      root.close()
        onApplyRequested:       MonitorService.commit()
        onSaveLayoutRequested:  ProfileService.updateFromCurrentState(root._activeProfileId)
        onMonitorSelected:     function(name)     { root._selectedName = name }
        onMonitorMoved:        function(name, pos) { MonitorService.applyEphemeral(name, { position: pos }) }
        onAliasChanged:        function(s, a)      { IdentityService.update(s, { alias: a }); MonitorService.refresh() }
        onModeChanged:         function(name, m)   { MonitorService.applyEphemeral(name, { mode: m }) }
        onMonitorScaleChanged: function(name, s)   { MonitorService.applyEphemeral(name, { scale: s }) }
        onTransformChanged:    function(name, t)   { MonitorService.applyEphemeral(name, { transform: t }) }
        onEnabledToggled:      function(name, e)   { MonitorService.applyEphemeral(name, { enabled: e }) }
        onPositionChanged:     function(name, pos) { MonitorService.applyEphemeral(name, { position: pos }) }
        onCmChanged:           function(name, cm)  { MonitorService.applyEphemeral(name, { cm: cm }) }

        onProfileSelected: function(id) {
            const profile = ProfileService.profiles.find(p => p.id === id)
            if (!profile) return
            root._applyProfileToMonitors(profile, MonitorService.monitors.filter(m => m.connected))
            ProfileService.setLastApplied(id)
            MonitorService.commit()
            root._pendingProfileId = id
            root._activeProfileId  = id
        }

        onSaveRequested:   function(name) { ProfileService.createFromCurrentState(name) }
        onDeleteRequested: function(id)   { ProfileService.remove(id) }
    }

    // ── Startup: restore last applied profile ─────────────
    function _tryStartupApply() {
        if (_startupDone) return
        if (MonitorService.monitors.length === 0) return      // monitors not ready yet

        const id = ProfileService.lastAppliedId
        if (!id) { _startupDone = true; return }              // nothing saved → done

        const profile = ProfileService.profiles.find(function(p) { return p.id === id })
        if (!profile) return                                   // profiles not loaded yet → wait

        _startupDone     = true
        _activeProfileId = id
        _pendingProfileId = id

        const connected = MonitorService.monitors.filter(m => m.connected)
        _applyProfileToMonitors(profile, connected)
        MonitorService.commit()
    }

    // ── Shared: map profile entries → applyEphemeral + alias update ─
    function _applyProfileToMonitors(profile, connected) {
        for (var i = 0; i < connected.length; i++) {
            const mon   = connected[i]
            const entry = profile.monitors.find(function(pm) { return pm.identitySerial === mon.serial })
            if (!entry) continue
            MonitorService.applyProfileEntry(mon.name, entry)
            if (entry.alias) IdentityService.update(mon.serial, { alias: entry.alias })
        }
    }

}

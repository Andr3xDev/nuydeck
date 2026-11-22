pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io

/*!
    HyprlandIpc — Facade for all Hyprland compositor communication.

    Single point of contact with hyprctl. If Hyprland changes its API,
    only this file changes. Exposes domain-semantic methods, never raw
    dispatcher strings.
*/
Singleton {
    id: root

    // Fire-and-forget dispatcher — for commands that produce no output.
    property Component _dispatch: Component {
        Process { onExited: destroy() }
    }

    property Component _query: Component {
        Process {
            id: proc
            property var _callback: null
            property string _buffer: ""

            stdout: SplitParser {
                // Use id reference — parent is not defined for property-assigned objects.
                onRead: function(line) { proc._buffer += line + "\n" }
            }

            onExited: {
                let result = []
                try {
                    result = JSON.parse(proc._buffer)
                } catch (e) {
                    console.warn("HyprlandIpc: JSON parse failed:", e)
                }
                if (proc._callback) proc._callback(result)
                proc.destroy()
            }
        }
    }

    function reload() {
        _dispatch.createObject(root, {
            command: ["hyprctl", "reload"],
            running: true
        })
    }

    function setKeyword(key, value) {
        _dispatch.createObject(root, {
            command: ["hyprctl", "keyword", key, value],
            running: true
        })
    }

    // Hyprland socket2 event name constants — owned here, used only here.
    readonly property var _monitorEvents: Object.freeze({
        added:   "monitoradded",
        removed: "monitorremoved"
    })

    /*!
        Queries all monitors (including disconnected) and delivers the raw
        JSON array to callback. Transformation to MonitorState happens in
        MonitorAdapter, not here.
    */
    function getMonitors(callback) {
        _query.createObject(root, {
            command: ["hyprctl", "-j", "monitors", "all"],
            _callback: callback,
            running: true
        })
    }

    /*!
        Queries all input devices and delivers the raw JSON object to
        callback. Transformation (capsLock OR-derivation) happens in the
        OSD trigger, not here.
    */
    function getDevices(callback) {
        _query.createObject(root, {
            command: ["hyprctl", "-j", "devices"],
            _callback: callback,
            running: true
        })
    }

    /*!
        Reactive per-screen fullscreen state. HyprlandMonitor has no `fullscreen`
        property; activeWorkspace.hasFullscreen is the NOTIFY-backed path. QML
        tracks reads through this synchronous call, so bindings re-evaluate; no
        polling.
    */
    function isFullscreen(screen) {
        const monitor = Hyprland.monitorFor(screen)
        return !!(monitor && monitor.activeWorkspace && monitor.activeWorkspace.hasFullscreen)
    }

    // Fails open on null so a missing monitor never strips the keyboard.
    function isFocused(screen) {
        const focused = Hyprland.focusedMonitor
        const monitor = Hyprland.monitorFor(screen)
        return !focused || !monitor || focused === monitor
    }

    function focusMonitor(name) {
        Hyprland.dispatch('hl.dsp.focus({monitor="' + name + '"})')
    }

    function focusedMonitorName() {
        return Hyprland.focusedMonitor ? Hyprland.focusedMonitor.name : ""
    }

    // Socket2 listener: monitor plug/unplug events → EventBus signals.
    // Quickshell 0.3.0 API: Hyprland.rawEvent(HyprlandEvent event)
    Connections {
        target: Hyprland
        function onRawEvent(event) {
            if (event.name === root._monitorEvents.added)   EventBus.monitorConnected(event.data)
            if (event.name === root._monitorEvents.removed) EventBus.monitorDisconnected(event.data)
        }
    }
}

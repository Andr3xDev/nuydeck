import QtQuick
import Quickshell
import "../../"
import "../../../../core/bridge" as Ipc

/*!
    CapsLockTrigger — caps lock OSD pair's trigger half.

    Polls hyprctl devices via the HyprlandIpc.getDevices facade every 250 ms
    and derives the state as OR across all keyboards (any keyboard with
    capsLock === true means locked). The first computation after start is a
    silent baseline: _primed is raised after the first write to the proxied
    `locked` property, so a stale caps-on state at shell start never fires an
    OSD. Every subsequent state change — both directions — updates
    CapsLockState and fires an arbiter request. The view stays a Component
    reference; the window instantiates it in its own Loader.
*/
Scope {
    id: root

    // two levels up here: module root, where OsdService lives
    property Component view: Component { CapsLockView {} }

    // Proxied state — mirrors VolumeTrigger; assignment order below (locked
    // before _primed) is load-bearing.
    property bool locked: false
    property bool _primed: false
    onLockedChanged: if (root._primed) root._request()

    Timer {
        interval: 250
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: Ipc.HyprlandIpc.getDevices(root._onDevices)
    }

    function _onDevices(result) {
        const keyboards = result ? result.keyboards : undefined
        // Malformed/failed query: no array → keep previous state. A failed
        // poll must not be read as "unlocked" — that would fire a spurious
        // OSD on recovery. (=== true on capsLock is strict, so a missing or
        // renamed field can never fake "locked".)
        if (!Array.isArray(keyboards)) return
        root.locked = keyboards.some(kb => kb && kb.capsLock === true)
        root._primed = true
    }

    function _request() {
        CapsLockState.locked = root.locked
        OsdService.request("capslock", 0, 0, root.view)
    }
}

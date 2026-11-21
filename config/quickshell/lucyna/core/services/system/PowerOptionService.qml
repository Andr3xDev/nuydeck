pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    function suspend()  { _run(["systemctl", "suspend"]) }
    function shutdown() { _run(["systemctl", "poweroff"]) }
    function reboot()   { _run(["systemctl", "reboot"]) }
    function lock()     { Quickshell.execDetached(["sh", "-c", "pidof hyprlock || hyprlock"]) }

    // Tracked Process, killed on Quickshell exit/reload. lock() uses
    // execDetached so hyprlock survives a reload (see ScreenshotService.qml).
    function _run(cmd) {
        const proc = _process.createObject(root, { command: cmd })
        proc.running = true
    }

    property Component _process: Component {
        Process { onExited: destroy() }
    }
}

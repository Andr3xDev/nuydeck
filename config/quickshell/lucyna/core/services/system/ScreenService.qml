pragma Singleton

import QtQuick
import Quickshell
import "../../bridge" as Ipc

Singleton {

    /*!
        Get the screen matching the focused Hyprland monitor by name, the first screen when none
        matches, null when there are no screens
    */
    function focusedScreen() {
        const name = Ipc.HyprlandIpc.focusedMonitorName()
        return Quickshell.screens.find(s => s.name === name) ?? Quickshell.screens[0] ?? null
    }
}

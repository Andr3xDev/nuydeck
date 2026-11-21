pragma Singleton

import QtQuick
import Quickshell

Singleton {
    id: root

    function pickColor() { _run(["hyprpicker"]) }
    function zoomCursor() { _run(["hyprctl", "eval", "hl.config({ cursor = { zoom_factor = 2 } }); hl.dispatch(hl.dsp.submap('zoom'))"]) }

    function _run(cmd) {
        Quickshell.execDetached(cmd);
    }
}

pragma Singleton

import QtQuick
import Quickshell
import "../../core/services/system" as Services

Singleton {
    id: root

    function copyRegion()         { _capture(false, "- | wl-copy && notify-send 'Screenshot' 'Copied to clipboard'") }
    function copyFocusedMonitor() { _capture(true,  "- | wl-copy && notify-send 'Screenshot' 'Copied to clipboard'") }
    function saveRegion()         { _capture(false, "~/media/screenshots/$(date '+%m%d-%Hh-%Mm-%Ss').png && notify-send 'Screenshot' 'Saved in media directory'") }
    function saveFocusedMonitor() { _capture(true,  "~/media/screenshots/$(date '+%m%d-%Hh-%Mm-%Ss').png && notify-send 'Screenshot' 'Saved in media directory'") }
    function editRegion()         { _capture(false, "-t ppm - | satty --filename -") }
    function editFocusedMonitor() { _capture(true,  "-t ppm - | satty --filename -") }

    // The monitor name comes from the compositor, not user input, so it is interpolated into the shell command as-is.
    function _capture(useFocusedMonitor, sink) {
        let source = "-g \"$(slurp -b 00000050)\"";
        if (useFocusedMonitor) {
            const screen = Services.ScreenService.focusedScreen();
            if (!screen) {
                console.warn("ScreenshotService: no focused screen, aborting capture");
                return;
            }
            source = "-o \"" + screen.name + "\"";
        }
        _run(["sh", "-c", "grim " + source + " " + sink]);
    }

    function _run(cmd) {
        Quickshell.execDetached(cmd);
    }
}

//@ pragma ShellId lucyna
//--------------------------------------------------------------------------------
//                                                                  
//                                                   
//                        ,--.                                           
//                        |  |   ,--.,--. ,---.,--. ,--.,--,--,  ,--,--. 
//                        |  |   |  ||  || .--' \  '  / |      \' ,-.  | 
//                        |  '--.'  ''  '\ `--.  \   '  |  ||  |\ '-'  | 
//                        `-----' `----'  `---'.-'  /   `--''--' `--`--' 
//                                             `---'                     
//              _______
//　　　　　  /  ＞　　フ     
//　　　　　 | 　_　 _ l      SUMMARY: QuickShell system config, includes bars,
//　 　　　 ／` ミ＿xノ               launchers, widgets & more.
//　　 　  /　　　 　 |
//　　　  /　 ヽ　　 ﾉ
//　  　 │　　|　|　|         AUTHOR:  Andr3xDev
//　 ／￣|　　 |　|　|        URL: https://github.com/Andr3xDev/lucyna
//　| (￣ヽ＿_ヽ_)__)
//　＼二つ
//--------------------------------------------------------------------------------


import Quickshell
import "core/components/surface"
import "modules/bar"
import "modules/launchers/apps"
import "modules/launchers/power"
import "modules/screenshot"
import "modules/cursortools"
import "modules/osd"
import "legacy/theme"
import "modules/launchers/clipboard"
import "modules/calendar"
import "legacy/monitors"
import QtQuick

/*!
    Composition root: instantiates the per-screen surfaces and every module.
    The only logic is the screen filter isExcluded().
*/
ShellRoot {

    // Top-layer surface per screen hosting the bar and every panel. Hyprland's
    // no_anim layer_rule applies (windows_rules.lua, "quickshell elements").
    // barItem puts the bar's region in the input mask.
    Variants {
        id: quickshellTopHosts
        model: Quickshell.screens.filter(scr => !isExcluded(scr.name))

        QuickshellTop {
            property var modelData
            screen: modelData
            barItem: barInstance

            Bar {
                id: barInstance
                screen: modelData
            }
        }
    }

    // Reserves the bar's exclusive zone independent of quickshell-top's
    // visibility (fullscreen-hide).
    Variants {
        model: Quickshell.screens.filter(scr => !isExcluded(scr.name))

        QuickshellExclusion {
            property var modelData
            screen: modelData
        }
    }

    // Overlay-layer surface per screen, mapped only while its hosted content
    // (the OSD) is active.
    Variants {
        id: quickshellOverlayHosts
        model: Quickshell.screens.filter(scr => !isExcluded(scr.name))

        QuickshellOverlay {
            property var modelData
            screen: modelData
        }
    }

    // Panels below open on whichever screen's quickshell-top is focused at
    // open() time. hosts is the QuickshellTop Variants' own instances.

    // Toggle: qs -c lucyna ipc call themeLauncher toggle
    ThemeLauncher {
        id: themeLauncher
        hosts: quickshellTopHosts.instances
        externalScriptPath: Quickshell.shellDir + "/scripts/apply-theme.sh"
    }

    AppLauncher {
        id: appLauncher
        hosts: quickshellTopHosts.instances
    }

    PowerLauncher {
        id: powerLauncher
        hosts: quickshellTopHosts.instances
    }

    // Toggle: qs -c lucyna ipc call toggleClip handle
    ClipboardPanel {
        id: clipLauncher
        hosts: quickshellTopHosts.instances
    }

    ScreenshotPanel {
        id: screenshotPanel
        hosts: quickshellTopHosts.instances
    }

    CursorToolsPanel {
        id: cursorToolsPanel
        hosts: quickshellTopHosts.instances
    }

    CalendarPanel {
        id: calendarPanel
        hosts: quickshellTopHosts.instances
    }

    // OSD pill shown on the focused monitor. hosts are the QuickshellOverlay
    // Variants' own instances.
    Osd {
        id: osd
        hosts: quickshellOverlayHosts.instances
    }

    // Toggle: qs -c lucyna ipc call monitorManager handle
    MonitorManager {
        id: monitorManager
    }

    // Screens that get no shell surfaces (top, exclusion, overlay).
    function isExcluded(screenName) {
        const excluded = [];
        return excluded.includes(screenName);
    }

}

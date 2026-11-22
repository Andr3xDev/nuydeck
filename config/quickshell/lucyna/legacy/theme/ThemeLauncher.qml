import "../../core/services/system" as SystemServices
import "../../core/services/panel" as Services
import "../../core/theme" as Theme
import "../../core/components/atomic"
import "../../core/animations" as Animations
import "components"
import QtQuick
import QtQuick.Layouts
import Quickshell.Io

/*!
    ThemeLauncher — standalone panel-contract Item hosted on quickshell-top
    (design.md Decision 2), mirrors CursorToolsPanel.qml/ScreenshotPanel.qml's
    pattern (task 9.1). No own PanelWindow/namespace — this also fixes the
    unregistered "quickshell-theme-launcher" namespace (explore.md §3 #5),
    since Hyprland now only ever sees quickshell-top's own no_anim map/unmap.
    Toggle via: quickshell ipc --config lucyna call themeLauncher toggle
    (unchanged IPC target/function shape, keybind SUPER + Y,
    configs/keyblinds.lua:105 — that bind is Lua-commented "-- DEPRECATED";
    see runtime-checks.md for the finding, not changed here, out of scope).

    Theme selector popup — toggle between available color themes. Bottom
    anchored, not full-screen (explore.md §9.7), closest in shape to the
    edge-anchored panel family — slides up from below over its own full
    extent plus resting margin, same "slide over full extent, no fade" shape
    as CursorToolsPanel/ScreenshotPanel.

    `hosts` (the per-screen QuickshellTop instances, shell.qml) lets this
    single logical instance find and reparent onto the focused screen's
    surface dynamically, same dance CursorToolsPanel.qml/PowerLauncher.qml use.
*/
Item {
    id: root

    // -- PanelService registration contract (name/parentName/open/active/closed/show/hide/toggle) --
    readonly property string name: "themeLauncher"
    readonly property string parentName: ""
    property bool _open: false
    readonly property bool open: root._open
    readonly property bool active: root._open
    signal closed

    // -- quickshell-top panel contract (design.md Decision 2) --
    property real progress: 0
    readonly property string anchorEdge: "bottom"
    // false: this panel had no scrim/dim behind it in the old WlrLayer.Overlay
    // window (explore.md §13 #1 — ThemeLauncher is one of only two profiled
    // panels with no scrim contract today) — preserved as-is.
    readonly property bool scrimEnabled: false
    readonly property bool needsKeyboard: root.open
    // Resting rect, not the slide-translated one — see ScreenshotPanel.qml's
    // own inputRect comment for the full rationale (unchanged here).
    readonly property rect inputRect: Qt.rect(root.x, root.y, root.width, root.height)

    // Single animation owner for this panel's whole open/close motion.
    Behavior on progress {
        Animations.MotionAnimation {
            role: root.open ? "panelEnter" : "panelExit"
        }
    }
    onOpenChanged: root.progress = root.open ? 1 : 0

    readonly property bool _shouldBeActive: root.open || root.progress > 0

    // Per-screen QuickshellTop instances (shell.qml's Variants.instances).
    required property var hosts
    property var _currentHost: null

    property string externalScriptPath: ""

    // Matches the old window's anchors.bottom + background.anchors.bottomMargin
    // (ThemeLauncher.qml:28,45 before this migration).
    readonly property int bottomMargin: 50

    // Anchored to the current host's content Item directly (not the literal
    // QML `parent`) — this Item is declared once under ShellRoot in
    // shell.qml, not under any QuickshellTop, until the first show() call
    // reparents it (see show() below).
    anchors.bottom: root._currentHost ? root._currentHost.contentHost.bottom : undefined
    anchors.bottomMargin: root.bottomMargin
    anchors.horizontalCenter: root._currentHost ? root._currentHost.contentHost.horizontalCenter : undefined

    width: 280
    height: contentLoader.item ? contentLoader.item.implicitHeight : 0
    visible: root._shouldBeActive

    // Slide up from below, over the panel's own full height plus its resting
    // bottom margin, so progress:0 sits fully below the visible area — same
    // "slide over full extent, no fade" shape CursorToolsPanel.qml uses from
    // its own anchored edge.
    transform: Translate {
        y: (1 - root.progress) * (root.height + root.bottomMargin)
    }

    function show() {
        if (!root.open) {
            const target = SystemServices.ScreenService.focusedScreen();
            const host = root.hosts.find(h => h.screen === target);
            if (!host) {
                console.warn("ThemeLauncher: no quickshell-top host for screen '" + (target ? target.name : "null") + "'");
                return;
            }
            if (root._currentHost !== host) {
                if (root._currentHost)
                    root._currentHost.panels = root._currentHost.panels.filter(p => p !== root);
                root.parent = host.contentHost;
                host.panels = host.panels.concat([root]);
                root._currentHost = host;
            }
            Services.OverlayService.claim({
                name: root.name,
                screen: target,
                close: () => root.hide(),
                focus: () => root.focusKeyboard()
            });
        }
        root._open = true;
        root.focusKeyboard();
    }

    function hide() {
        Services.OverlayService.release(root.name);
        if (!root.open)
            return;
        root._open = false;
        root.closed();
    }

    function toggle() {
        if (root.open)
            root.hide();
        else
            root.show();
    }

    // Focus hook for OverlayService: re-arms keyboard focus when the
    // consumer above this one closes (keyboard-first, FILO), same as
    // CursorToolsPanel.qml/PowerLauncher.qml. Replaces the old profile's
    // unverified QML-only `background.focus: true` (explore.md §9.7) —
    // quickshell-top's own refocusKeyboard() now owns the layer-shell grant.
    function focusKeyboard() {
        if (!root.open || !root._currentHost)
            return;
        root._currentHost.refocusKeyboard(contentLoader.item);
    }

    // ActionTiles/ThemeList has no Keys handler of its own, so Escape
    // bubbles up unaccepted — catch it here, mirroring CursorToolsPanel.qml's
    // root-level Keys.onEscapePressed.
    Keys.onEscapePressed: root.hide()

    Component.onCompleted: Services.PanelService.register(root.name, root)
    Component.onDestruction: {
        Services.OverlayService.release(root.name);
        Services.PanelService.unregister(root.name);
    }

    // Same IPC target/method shape as before migration (explore.md §9.7 —
    // toggle() as the handler function itself, not handle()/toggle(name)) so
    // the existing keybind (configs/keyblinds.lua:105) keeps working unchanged.
    IpcHandler {
        target: "themeLauncher"
        function toggle() {
            root.toggle();
        }
    }

    Process {
        id: scriptProcess
        running: false
    }

    // Content Loader destroyed only after exit completes (panel-motion-
    // pattern spec) — stays active through the close animation via
    // _shouldBeActive, same as CursorToolsPanel.qml/PowerLauncher.qml's
    // contentLoader.
    Loader {
        id: contentLoader
        width: root.width
        active: root._shouldBeActive

        sourceComponent: Component {
            Rectangle {
                id: background
                width: 280
                implicitHeight: contentColumn.implicitHeight + 32
                height: implicitHeight
                color: Theme.Tokens.color.surface
                radius: Theme.Tokens.radius.none
                border.color: Theme.Tokens.color.borderStrong
                border.width: 1
                focusPolicy: Qt.StrongFocus

                ColumnLayout {
                    id: contentColumn
                    anchors {
                        fill: parent
                        margins: Theme.Tokens.space.lg
                    }
                    spacing: Theme.Tokens.space.md

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: Theme.Tokens.space.sm

                        QsText {
                            text: "󰔎"
                            role: "icon"
                            iconSize: "lg"
                            color: Theme.Tokens.color.textMuted
                        }

                        QsText {
                            text: "Themes"
                            role: "title"
                            color: Theme.Tokens.color.textPrimary
                            font.bold: true
                        }

                        Item { Layout.fillWidth: true }

                        Rectangle {
                            width: 20
                            height: 20
                            radius: 2
                            color: closeMouseArea.containsMouse
                                ? Theme.Tokens.color.accentSurface
                                : Theme.Tokens.color.bg

                            QsText {
                                anchors.centerIn: parent
                                text: "󰅖"
                                role: "icon"
                                color: Theme.Tokens.color.accent
                            }

                            MouseArea {
                                id: closeMouseArea
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: root.hide()
                            }
                        }
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.preferredHeight: 1
                        color: Theme.Tokens.color.border
                    }

                    ThemeList {
                        id: themeList
                        Layout.fillWidth: true

                        onVariantSelected: function(variantId) {
                            if (Theme.ThemeManager.setVariant(variantId))
                                root._awaitingPersist = true

                            root.hide()
                        }
                    }
                }
            }
        }
    }

    // The hub must be persisted before the script runs: standalone sibling scripts read it.
    property bool _awaitingPersist: false

    Connections {
        target: Theme.ThemeManager

        function onPersisted() {
            if (!root._awaitingPersist) return
            root._awaitingPersist = false
            if (root.externalScriptPath !== "") {
                scriptProcess.command = ["bash", root.externalScriptPath, Theme.ThemeManager.currentVariant]
                scriptProcess.running = true
            }
        }
    }
}

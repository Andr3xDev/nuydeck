import QtQuick
import QtQuick.Layouts
import "../../../../core/services/system" as Services
import "../../../../core/theme" as Theme
import "../../../../core/components/atomic"
import "../../../../core/animations" as Animations

/*!
    VolumeView — volume OSD pair's view half.

    Content-only: fills the OsdWindow pill's content area and binds directly
    to AudioService — it never requests OSDs, it only renders state. Icon
    grammar and accent progress fill lifted from the legacy Volume pill.

    Track spans 0→100: the accent fill paints below, and the accentSecondary
    layer paints above it from the left base, so the tail only emerges beyond 100%.
*/
RowLayout {
    // No anchors.fill: the pill now derives its own size from this view's
    // implicitWidth/implicitHeight (OsdWindow.qml's `_pillComponent`) — the
    // pill's own padding (Theme.Tokens.space.md) replaces the margins this
    // view used to apply to itself.
    spacing: Theme.Tokens.space.lg

    QsText {
        Layout.preferredWidth: Theme.Tokens.space.xxl
        horizontalAlignment: Text.AlignHCenter
        text: Services.AudioService.muted ? "󰖁"
            : (Services.AudioService.volume <= 10) ? "󰕿"
            : (Services.AudioService.volume <= 65) ? "󰖀"
            : "󰕾"
        role: "icon"
        iconSize: "xl"
        color: Theme.Tokens.color.textPrimary
    }

    Rectangle {
        // A fill-style progress track has no natural implicit width, so it
        // needs an explicit content-layer size now that the pill no longer
        // imposes one from outside — this is the pre-reactive-sizing pill's
        // own footprint (OsdWindow.qml used to hardcode 400x50) minus the
        // pill's padding (space.md*2), the icon (space.xxl), and this row's
        // spacing (space.lg), i.e. 400 - 24 - 32 - 16 = 328, rounded to a
        // plain constant since none of those tokens carries this magnitude.
        Layout.preferredWidth: 328
        implicitHeight: 8
        radius: Theme.Tokens.radius.sm
        color: Theme.Tokens.color.accentSurface

        // Main fill: 0→100, accent (red when muted), painted BELOW.
        Rectangle {
            anchors {
                left: parent.left
                top: parent.top
                bottom: parent.bottom
            }
            implicitWidth: parent.width * Math.min(Services.AudioService.volume, 100) / 100
            radius: parent.radius
            color: Services.AudioService.muted
                ? Theme.Tokens.color.danger
                : Theme.Tokens.color.accent

            Behavior on implicitWidth {
                Animations.MotionAnimation { role: "stateMedium" }
            }
            Behavior on color {
                Animations.MotionColorAnimation { role: "stateMedium" }
            }
        }

        // Accent-secondary: grows from the left base OVER the accent fill.
        // 0→50 of the track mapped to volume 100→150 (double rate);
        // pinned at 150 — Pipewire allows a bit more, the bar does not.
        // Width 0 at ≤100% => invisible.
        Rectangle {
            anchors {
                left: parent.left
                top: parent.top
                bottom: parent.bottom
            }
            implicitWidth: parent.width * Math.min(Math.max(0, Services.AudioService.volume - 100), 50) / 50
            radius: parent.radius
            color: Services.AudioService.muted
                ? Theme.Tokens.color.dangerSurface
                : Theme.Tokens.color.accentSecondary

            Behavior on implicitWidth {
                Animations.MotionAnimation { role: "stateMedium" }
            }
            Behavior on color {
                Animations.MotionColorAnimation { role: "stateMedium" }
            }
        }
    }
}

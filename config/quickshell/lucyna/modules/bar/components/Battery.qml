import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/services/system" as Services
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

RowLayout {
    id: root
    spacing: Theme.Tokens.space.xs

    readonly property color _statusColor: Services.BatteryService.isCritical()
        ? Theme.Tokens.color.danger
        : Theme.Tokens.color.textPrimary

    QsText {
        Layout.alignment: Qt.AlignVCenter
        visible: Services.BatteryService.battery !== null
        text: Services.BatteryService.getBatteryIcon()
        role: "icon"
        color: root._statusColor

        SequentialAnimation on color {
            running: Services.BatteryService.isCritical()
            loops: Animation.Infinite
            Animations.MotionColorAnimation { role: "alertPulse"; from: Theme.Tokens.color.danger; to: Theme.Tokens.color.textMuted }
            Animations.MotionColorAnimation { role: "alertPulse"; from: Theme.Tokens.color.textMuted; to: Theme.Tokens.color.danger }
        }
    }

    QsText {
        visible: !Services.BatteryService.shouldHideLevel()
        text: Services.BatteryService.battery
            ? Math.round(Services.BatteryService.batteryLevel) + "%"
            : "N/A"
        role: "body"
        color: root._statusColor
        font.bold: Services.BatteryService.isCritical()
    }
}

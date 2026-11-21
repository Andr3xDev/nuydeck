import Quickshell
import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/bridge" as Ipc
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

/*!
    Clock text to show current time & day of the week.
    Clicking it toggles the floating calendar panel via EventBus
*/
Item {
    id: clock
    implicitWidth: clockRow.implicitWidth
    Layout.fillHeight: true

    // Driven through EventBus so the bar does not depend on the calendar module.
    property bool _calendarOpen: false

    Connections {
        target: Ipc.EventBus
        function onCalendarVisibilityChanged(visible) {
            clock._calendarOpen = visible
        }
    }

    Timer {
        interval: 60000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: clock.updateTime()
    }

    function updateTime() {
        var now = new Date()
        var timeStr = Qt.formatDateTime(now, "hh:mm ap")
        var dateStr = Qt.formatDateTime(now, "dd - ddd")
        timeText.text = timeStr
        dateText.text = dateStr
    }

    component ClockText: QsText {
        role:      "mono"
        font.bold: true
        color: clock._calendarOpen
            ? Theme.Tokens.color.accent
            : Theme.Tokens.color.textPrimary

        Behavior on color { Animations.MotionColorAnimation { role: "stateFast" } }
    }

    // Divider gap: three mono-space characters wide, so it scales with the clock font.
    TextMetrics {
        id: dividerGap
        font: dividerText.font
        text: "   "
    }

    RowLayout {
        id: clockRow
        anchors.centerIn: parent
        spacing: dividerGap.advanceWidth

        ClockText { id: timeText }
        ClockText { id: dividerText; text: "|" }
        ClockText { id: dateText }
    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        cursorShape:  Qt.PointingHandCursor
        onClicked:    Ipc.EventBus.calendarToggleRequested()
    }
}

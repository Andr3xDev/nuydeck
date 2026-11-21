import "." as Local
import "../../../core/animations" as Animations
import "../../../core/theme" as Theme
import "../../../core/components/atomic"
import QtQuick
import QtQuick.Layouts

/*!
    Monthly calendar grid with header navigation, weekday labels, and a stable
    6 × 7 day grid. Today and holidays are colour-coded.
*/
Item {
    id: root

    readonly property int _cellSize: 34
    readonly property int _cellGap: Theme.Tokens.space.xs
    readonly property int _pad: Theme.Tokens.space.md
    readonly property int _headerH: 36
    readonly property int _weekdayH: 20
    readonly property int _headerSpacing: 6
    readonly property int _headerBottomMargin: 3
    readonly property int _panelWidth: 7 * _cellSize + 6 * _cellGap + 2 * _pad
    // Equal left/right slots keep the title centered.
    readonly property int _headerSlotW: Math.max(prevButton.implicitWidth, nextButton.implicitWidth)
    property int displayMonth: _todayMonth
    property int displayYear: _todayYear
    readonly property bool _isCurrentMonth: displayMonth === _todayMonth && displayYear === _todayYear

    implicitWidth: _panelWidth
    implicitHeight: content.implicitHeight + 2 * _pad
    property var _today: new Date()
    readonly property int _todayDay:   _today.getDate()
    readonly property int _todayMonth: _today.getMonth() + 1
    readonly property int _todayYear:  _today.getFullYear()
    readonly property color _todayColor: Theme.Tokens.color.accentSurface
    readonly property var _monthNames: ["January","February","March","April","May","June","July","August","September","October","November","December"]
    readonly property var _weekLabels: ["Su","Mo","Tu","We","Th","Fr","Sa"]

    readonly property var cells: {
        const y = displayYear, m = displayMonth;
        const firstDow   = new Date(y, m - 1, 1).getDay();
        const daysInMonth = new Date(y, m, 0).getDate();
        const prevDays   = new Date(y, m - 1, 0).getDate();
        const prevM = m === 1 ? 12 : m - 1;
        const prevY = m === 1 ? y - 1 : y;
        const nextM = m === 12 ? 1 : m + 1;
        const nextY = m === 12 ? y + 1 : y;
        const out = [];
        for (let i = 0; i < 42; i++) {
            const off = i - firstDow;
            let day, cm, cy;
            if (off < 0) {
                day = prevDays + off + 1; cm = prevM; cy = prevY;
            } else if (off < daysInMonth) {
                day = off + 1; cm = m; cy = y;
            } else {
                day = off - daysInMonth + 1; cm = nextM; cy = nextY;
            }
            const cur = off >= 0 && off < daysInMonth;
            out.push({
                "day": day, "month": cm, "year": cy,
                "isCurrentMonth": cur,
                "isToday": cur && day === _todayDay && cm === _todayMonth && cy === _todayYear,
                "isSunday": (i % 7) === 0,
                "isHoliday": cur && Local.HolidayProvider.isHoliday(cy, cm, day)
            });
        }
        return out;
    }

    function prevMonth() {
        if (displayMonth === 1) { displayYear--;  displayMonth = 12; }
        else                    { displayMonth--; }
    }

    function nextMonth() {
        if (displayMonth === 12) { displayYear++; displayMonth = 1; }
        else                     { displayMonth++; }
    }

    function goToday() {
        displayMonth = _todayMonth;
        displayYear  = _todayYear;
    }

    Timer {
        interval: 60000
        running: true
        repeat: true
        onTriggered: root._today = new Date()
    }

    ColumnLayout {
        id: content
        anchors.fill: parent
        anchors.margins: root._pad
        spacing: root._headerSpacing

        RowLayout {
            Layout.fillWidth: true
            Layout.bottomMargin: root._headerBottomMargin
            Layout.preferredHeight: root._headerH
            spacing: Theme.Tokens.space.xs

            Item {
                Layout.preferredWidth: root._headerSlotW
                Layout.fillHeight: true

                QsButton {
                    id: prevButton
                    anchors.verticalCenter: parent.verticalCenter
                    label: "‹"
                    onClicked: root.prevMonth()
                }
            }

            Item {
                id: title
                readonly property bool _showToday: !root._isCurrentMonth && titleHover.containsMouse

                Layout.fillWidth: true
                Layout.alignment: Qt.AlignVCenter
                implicitHeight: monthLabel.implicitHeight
                clip: true

                component TitleLabel: QsText {
                    property bool shown
                    property int hiddenY

                    width: parent.width
                    y: shown ? 0 : hiddenY
                    opacity: shown ? 1 : 0
                    horizontalAlignment: Text.AlignHCenter
                    role: "subtitle"
                    font.bold: true

                    Behavior on y { Animations.MotionAnimation { role: "stateFast" } }
                    Behavior on opacity { Animations.MotionAnimation { role: "stateFast" } }
                }

                TitleLabel {
                    id: monthLabel
                    shown: !title._showToday
                    hiddenY: -title.height
                    text: root._monthNames[root.displayMonth - 1] + " " + root.displayYear
                }

                TitleLabel {
                    shown: title._showToday
                    hiddenY: title.height
                    text: "Today"
                    color: Theme.Tokens.color.aquamarine
                }

                MouseArea {
                    id: titleHover
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: root._isCurrentMonth ? Qt.ArrowCursor : Qt.PointingHandCursor
                    onClicked: if (!root._isCurrentMonth) root.goToday()
                }
            }

            Item {
                Layout.preferredWidth: root._headerSlotW
                Layout.fillHeight: true

                QsButton {
                    id: nextButton
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    label: "›"
                    onClicked: root.nextMonth()
                }
            }
        }

        Row {
            Layout.alignment: Qt.AlignHCenter
            spacing: root._cellGap

            Repeater {
                model: root._weekLabels

                QsText {
                    width: root._cellSize
                    height: root._weekdayH
                    text: modelData
                    role: "caption"
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                    color: Theme.Tokens.color.textMuted
                    font.bold: true
                }
            }
        }

        Grid {
            Layout.alignment: Qt.AlignHCenter
            columns: 7
            rowSpacing: root._cellGap
            columnSpacing: root._cellGap

            Repeater {
                model: root.cells

                Item {
                    width: root._cellSize
                    height: root._cellSize

                    Rectangle {
                        anchors.centerIn: parent
                        width: root._cellSize - root._cellGap
                        height: width
                        radius: width / 2
                        color: modelData.isToday ? root._todayColor : Theme.Tokens.color.bg
                    }

                    QsText {
                        anchors.centerIn: parent
                        text: modelData.day
                        role: "body"
                        font.bold: modelData.isToday
                        color: !modelData.isCurrentMonth
                            ? Theme.Tokens.color.textMuted
                            : (modelData.isToday || modelData.isSunday || modelData.isHoliday)
                                ? Theme.Tokens.color.accent
                                : Theme.Tokens.color.textPrimary
                    }
                }
            }
        }
    }
}

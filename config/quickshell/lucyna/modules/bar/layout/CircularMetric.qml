import QtQuick
import "../../../core/theme" as Theme
import "../../../core/components/atomic"

/*!
    Circular gauge: an arc of value (0-100) around a centered icon; critical
    color from 75.
*/
Item {
    id: circularMetric
    width: size
    height: size

    property real value: 0
    property string icon: ""
    property real iconSize: Theme.Tokens.text.compactXs
    property real size: 24
    property real lineWidth: 2
    readonly property color effectiveColor: value >= 75 ? Theme.Tokens.severity("critical") : Theme.Tokens.color.aquamarine

    Canvas {
        id: canvas
        anchors.fill: parent
        z: 0
        onPaint: {
            var ctx = getContext("2d")
            ctx.reset()

            var centerX = width / 2
            var centerY = height / 2
            var radius = width / 2 - circularMetric.lineWidth

            ctx.beginPath()
            ctx.arc(centerX, centerY, radius, 0, 2 * Math.PI)
            ctx.lineWidth = circularMetric.lineWidth
            ctx.strokeStyle = circularMetric.effectiveColor
            ctx.globalAlpha = 0.2
            ctx.stroke()

            if (circularMetric.value > 0) {
                ctx.beginPath()
                var startAngle = -Math.PI / 2
                var endAngle = startAngle + (circularMetric.value / 100) * 2 * Math.PI
                ctx.arc(centerX, centerY, radius, startAngle, endAngle)
                ctx.lineWidth = circularMetric.lineWidth
                ctx.strokeStyle = circularMetric.effectiveColor
                ctx.globalAlpha = 1
                ctx.stroke()
            }
        }

        Connections {
            target: circularMetric
            function onValueChanged() { canvas.requestPaint() }
            function onEffectiveColorChanged() { canvas.requestPaint() }
        }
    }

    QsText {
        anchors.centerIn: parent
        text: circularMetric.icon
        role: "icon"
        font.pixelSize: circularMetric.iconSize
        color: circularMetric.effectiveColor
        z: 1
    }
}

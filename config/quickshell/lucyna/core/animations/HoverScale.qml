import QtQuick
import "../theme" as Theme

/*!
    HoverScale — reusable hover-to-scale interaction.

    A MouseArea that animates `target.scale` between 1.0 and `scaleAmount`
    on hover enter/exit, using the shared fast-duration/standard-easing
    motion tokens. Defaults `target` to its parent, matching the common
    "scale the icon I'm layered over" usage.
*/
MouseArea {
    id: root

    property Item target: parent

    // Scale applied while hovered. Normalized to 1.1 across all call sites.
    property real scaleAmount: 1.1

    hoverEnabled: true

    onEntered: {
        _scaleAnimation.to = root.scaleAmount
        _scaleAnimation.start()
    }
    onExited: {
        _scaleAnimation.to = 1.0
        _scaleAnimation.start()
    }

    MotionAnimation {
        id: _scaleAnimation
        role: "stateFast"
        target: root.target
        property: "scale"
    }
}

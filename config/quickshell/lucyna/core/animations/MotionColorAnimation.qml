import QtQuick
import "../theme" as Theme

/*!
    MotionColorAnimation — ColorAnimation equivalent of MotionAnimation.qml,
    same `role`-selects-a-role contract. Use inside a `Behavior on color`.
*/
ColorAnimation {
    id: root

    property string role: "effectsEnter"

    readonly property QtObject _role: Theme.Tokens.motion.role[root.role]

    duration: root._role.duration
    easing.type: Easing.BezierSpline
    easing.bezierCurve: root._role.curve
}

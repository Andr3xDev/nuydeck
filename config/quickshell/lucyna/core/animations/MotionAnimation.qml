import QtQuick
import "../theme" as Theme

/*!
    MotionAnimation — NumberAnimation wrapper. `role` selects one
    `Theme.Tokens.motion.role.*` entry; duration and
    easing curve resolve from that role, never a literal value here. Use
    inside a `Behavior on <property>`.
*/
NumberAnimation {
    id: root

    property string role: "panelEnter"

    readonly property QtObject _role: Theme.Tokens.motion.role[root.role]

    duration: root._role.duration
    easing.type: Easing.BezierSpline
    easing.bezierCurve: root._role.curve
}

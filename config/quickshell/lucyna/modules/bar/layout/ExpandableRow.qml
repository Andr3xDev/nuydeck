import QtQuick
import "../../../core/theme" as Theme
import "../../../core/animations" as Animations

/*!
    Collapsible bar-widget shell. Width animates between 0 and the first
    child's implicitWidth. Neighbors in the bar's RowLayout shift to make
    room; the right group is right-anchored, so expanding only shifts left
    neighbors. Row content is a plain child (children[0]).

    Usage:
        ExpandableRow {
            expanded: someCondition

            RowLayout {
                anchors.centerIn: parent
                opacity: expanded ? 1 : 0
            }
        }
*/
Item {
    id: root

    property bool expanded: false

    implicitWidth: expanded && children.length > 0 ? children[0].implicitWidth : 0
    implicitHeight: parent.height
    clip: true

    Behavior on implicitWidth {
        Animations.MotionAnimation { role: "stateStandard" }
    }
}

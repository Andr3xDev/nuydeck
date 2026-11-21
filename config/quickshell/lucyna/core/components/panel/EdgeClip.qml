import QtQuick
import "../../services/panel" as Services

/*!
    EdgeClip — clips its children to the panel rect so a panel sliding in
    from `attachEdge` is hidden beyond that edge. Children keep panel coordinates
    (0,0 is the panel's top-left). `joinOutset` opens the window past the
    edge, so the panel's join band over the bar border stays visible at rest.

    Size it to the panel; it reads no parent geometry. `attachEdge` "" disables
    the clip.
*/
Item {
    id: root

    property string attachEdge: ""
    property real joinOutset: 0

    default property alias content: body.data

    readonly property string _axis: Services.PanelRules.slideAxisOf(root.attachEdge)
    readonly property real _sign: Services.PanelRules.slideSignOf(root.attachEdge)
    readonly property real _outsetLeft: root._axis === "x" && root._sign < 0 ? root.joinOutset : 0
    readonly property real _outsetTop: root._axis === "y" && root._sign < 0 ? root.joinOutset : 0
    readonly property real _outsetRight: root._axis === "x" && root._sign > 0 ? root.joinOutset : 0
    readonly property real _outsetBottom: root._axis === "y" && root._sign > 0 ? root.joinOutset : 0

    Item {
        x: -root._outsetLeft
        y: -root._outsetTop
        width: root.width + root._outsetLeft + root._outsetRight
        height: root.height + root._outsetTop + root._outsetBottom
        clip: root.attachEdge !== ""

        Item {
            id: body
            x: root._outsetLeft
            y: root._outsetTop
            width: root.width
            height: root.height
        }
    }
}

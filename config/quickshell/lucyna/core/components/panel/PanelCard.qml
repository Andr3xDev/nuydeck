import QtQuick
import "../../theme" as Theme

/*!
    PanelCard — rounded card chrome for floating panels: background, border and
    a FocusRay on the border centerline. Sizes itself to the content's implicit
    size and hosts `contentComponent` in a Loader, like PanelShell, so PanelBase
    reads the same `panelWidth`/`panelHeight`/`contentItem` from either chrome.
*/
Rectangle {
    id: root

    property Component contentComponent
    property bool contentActive: true
    property string rayTone: "none"

    readonly property Item contentItem: contentLoader.item
    readonly property real panelWidth: root.contentItem ? root.contentItem.implicitWidth : 0
    readonly property real panelHeight: root.contentItem ? root.contentItem.implicitHeight : 0

    width: root.panelWidth
    height: root.panelHeight
    color: Theme.Tokens.color.bg
    radius: Theme.Tokens.radius.panel
    border.width: Theme.Tokens.comp.panelSurface.strokeWidth
    border.color: Theme.Tokens.color.border

    // Traces the border's centerline: rect inset by half the stroke, corner radius reduced by the same.
    FocusRay {
        id: ray
        readonly property real inset: strokeWidth / 2
        readonly property real maxX: width - inset
        readonly property real maxY: height - inset
        readonly property real corner: root.radius - inset

        anchors.fill: parent
        tone: root.rayTone
        strokeWidth: root.border.width
        outlineLength: 2 * (width + height - 4 * inset) - 8 * corner + 2 * Math.PI * corner
        startX: inset + corner
        startY: inset

        PathLine { x: ray.maxX - ray.corner; y: ray.inset }
        PathArc { x: ray.maxX; y: ray.inset + ray.corner; radiusX: ray.corner; radiusY: ray.corner }
        PathLine { x: ray.maxX; y: ray.maxY - ray.corner }
        PathArc { x: ray.maxX - ray.corner; y: ray.maxY; radiusX: ray.corner; radiusY: ray.corner }
        PathLine { x: ray.inset + ray.corner; y: ray.maxY }
        PathArc { x: ray.inset; y: ray.maxY - ray.corner; radiusX: ray.corner; radiusY: ray.corner }
        PathLine { x: ray.inset; y: ray.inset + ray.corner }
        PathArc { x: ray.inset + ray.corner; y: ray.inset; radiusX: ray.corner; radiusY: ray.corner }
    }

    // Under the content: absorbs clicks on the whole card so they never reach the scrim behind it.
    MouseArea {
        anchors.fill: parent
    }

    Loader {
        id: contentLoader
        active: root.contentActive
        sourceComponent: root.contentComponent
    }
}

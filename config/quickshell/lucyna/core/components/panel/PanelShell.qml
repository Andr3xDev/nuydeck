import QtQuick
import "../../services/panel" as Services
import "../../theme" as Theme

/*!
    PanelShell — PanelSurfaceShape plus the curve-safe content gutter, shared
    by every panel. Hosts `contentComponent` in a Loader.

    Content is inset by PanelRules.shellPad: the plain gutter, plus bodyInset
    on the axis along the attached edge, because the tapered fillet strip eats
    bodyInset off each end of that axis. Size is content implicit size plus
    that pad, clamped to availableWidth/availableHeight.
*/
PanelSurfaceShape {
    id: root

    property Component contentComponent
    property bool contentActive: true
    property real availableWidth: 0
    property real availableHeight: 0
    // Staged motion input: 1 is settled, no staging. The base strip slides out from the attached edge
    // first, then the body unfolds (reversed as progress falls).
    property real progress: 1

    readonly property real baseReveal: root._smoothstep(root.progress / Theme.Tokens.comp.panelSurface.stageBaseShare)
    readonly property real _bodyStart: Theme.Tokens.comp.panelSurface.stageBodyStart
    readonly property real _contentSlide: root._slideSign * (1 - root.bodyReveal) * root._vFull

    function _smoothstep(t) {
        const c = Math.min(1, Math.max(0, t));
        return c * c * (3 - 2 * c);
    }

    readonly property Item contentItem: contentLoader.item
    readonly property size _pad: Services.PanelRules.shellPad(root._verticalEdge)
    readonly property real padWidth: root._pad.width
    readonly property real padHeight: root._pad.height

    fillColor: Theme.Tokens.color.bg
    strokeColor: Theme.Tokens.color.border
    strokeWidth: Theme.Tokens.comp.panelSurface.strokeWidth
    touchesEdge: true
    bodyInset: Theme.Tokens.comp.panelSurface.bodyInset
    filletRadius: Theme.Tokens.space.md
    curveDepth: Theme.Tokens.radius.panel
    depthInset: Theme.Tokens.comp.panelSurface.depthInset
    bodyReveal: root._smoothstep((root.progress - root._bodyStart) / (1 - root._bodyStart))

    transform: Translate {
        x: root._slideAxis === "x" ? root._slideSign * (1 - root.baseReveal) * root.baseExtent : 0
        y: root._slideAxis === "y" ? root._slideSign * (1 - root.baseReveal) * root.baseExtent : 0
    }

    function _surfaceExtent(childImplicit, available, pad) {
        return Math.min(childImplicit + 2 * pad, Math.max(0, available));
    }

    width: root.contentItem ? root._surfaceExtent(root.contentItem.implicitWidth, root.availableWidth, root.padWidth) : 0
    height: root.contentItem ? root._surfaceExtent(root.contentItem.implicitHeight, root.availableHeight, root.padHeight) : 0

    // Under the content: absorbs clicks on the whole surface so they never reach the scrim behind it.
    MouseArea {
        anchors.fill: parent
    }

    Loader {
        id: contentLoader
        x: root.padWidth + (root._slideAxis === "x" ? root._contentSlide : 0)
        y: root.padHeight + (root._slideAxis === "y" ? root._contentSlide : 0)
        active: root.contentActive
        sourceComponent: root.contentComponent
    }
}

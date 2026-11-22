import QtQuick
import QtQuick.Shapes
import "../../services/panel" as Services
import "../../theme" as Theme

/*!
    PanelSurfaceShape — shared hanging-silhouette primitive for panel and bar
    backgrounds. An Item wrapper around a private `Shape` (PathSvg): it draws
    the panel silhouette, the join band and the focus ray: body with concave
    fillets along the tapered edge, convex corners of radius `curveDepth` at
    the far end, and the anchored edge flat to the screen.

    Critical:
    - `panelWidth`/`panelHeight` are one-way derived from width/height. Never
      bind them back (e.g. no `implicitWidth: panelWidth` on a consumer
      container) — that reintroduces the binding loop this type exists to
      remove. Consumers that size from this shape must read `panelWidth`,
      never its own `width`.
    - Which panel edge the silhouette hangs from is decided by `attachEdge`;
      all path coordinates go through the u/v mapping driven by that edge.

    Note: the stroke path is assembled as an SVG string because QML cannot
    conditionally omit a path element at runtime. The stroke is open along
    the anchored edge and closed otherwise (`_strokePath`).
*/
Item {
    id: root

    property string attachEdge: "top"
    property bool touchesEdge: true
    // Layout: curveDepth = far convex corners; bodyInset/filletRadius = where/radius the tapered edge meets the body.
    property real curveDepth: Theme.Tokens.radius.panel
    property color fillColor: "transparent"
    property color strokeColor: "transparent"
    property real strokeWidth: Theme.Tokens.border.md

    property real bodyInset: 0
    property real filletRadius: Theme.Tokens.radius.panel

    // depthInset pushes the far bound inward so the centered stroke clips fully off-window.
    property real depthInset: Theme.Tokens.radius.none

    // How far outside the anchored edge the open stroke's flares become tangent. Half a pen width
    // puts the whole pen off-screen at the tips (screen edge); panels hung from a bar set 0 so the
    // tip lands on the bar's border centerline and merges into its stroke.
    property real strokeTipOutset: strokeWidth / 2

    // Focus ray tone ("none" | "dim" | "bright"): a dash sweeps along the stroke path unless "none" (see FocusRay).
    property string rayTone: "none"

    readonly property real panelWidth: width
    readonly property real panelHeight: height

    readonly property real _kappa: 0.5523 // standard quarter-circle cubic-bezier approximation

    readonly property string _slideAxis: Services.PanelRules.slideAxisOf(attachEdge)
    readonly property real _slideSign: Services.PanelRules.slideSignOf(attachEdge)
    readonly property bool _verticalEdge: Services.PanelRules.isVerticalZone(attachEdge)
    readonly property real _uMax: _verticalEdge ? panelHeight : panelWidth
    // 0..1 reveal of the body below the flares; the path shrinks toward flare height as it falls to 0.
    property real bodyReveal: 1
    readonly property real _vFull: _verticalEdge ? panelWidth : panelHeight
    // Depth of the flares alone: the body-less base a staged reveal starts from.
    readonly property real baseExtent: filletRadius + depthInset + curveDepth
    readonly property real _vMax: _vFull - (1 - Math.min(1, Math.max(0, bodyReveal))) * Math.max(0, _vFull - baseExtent)

    readonly property real _cornerNear: touchesEdge ? 0 : curveDepth
    readonly property real _cornerFar: curveDepth

    // v coordinate of the attached edge.
    readonly property real _v0: 0
    readonly property real _farV: _vMax - depthInset

    readonly property real _filletR: (touchesEdge && bodyInset > 0 && filletRadius > 0) ? Math.max(0, Math.min(filletRadius, (_vMax - depthInset) - curveDepth)) : 0
    readonly property bool _stripActive: _filletR > 0
    readonly property real _bodyStartU: _stripActive ? bodyInset : 0
    readonly property real _bodyEndU: _stripActive ? _uMax - bodyInset : _uMax
    readonly property real _endFilletStartU: _bodyEndU + _filletR
    readonly property real _bottomLineTargetU: _bodyStartU + _cornerFar

    function _fmtNum(value) {
        return value.toFixed(3);
    }

    function _mapX(u, v) {
        if (!root._verticalEdge)
            return u;
        return root._slideSign > 0 ? root.panelWidth - v : v;
    }

    function _mapY(u, v) {
        if (root._verticalEdge)
            return u;
        return root._slideSign > 0 ? root.panelHeight - v : v;
    }

    function _pt(u, v) {
        return root._fmtNum(root._mapX(u, v)) + " " + root._fmtNum(root._mapY(u, v));
    }

    function _lineTo(u, v) {
        return " L " + root._pt(u, v);
    }

    function _cubicTo(c1u, c1v, c2u, c2v, u, v) {
        return " C " + root._pt(c1u, c1v) + " " + root._pt(c2u, c2v) + " " + root._pt(u, v);
    }

    function _nearEndCorner() {
        return root._cubicTo(root._uMax - root._cornerNear * (1 - root._kappa), root._v0, root._uMax, root._cornerNear * (1 - root._kappa), root._uMax, root._cornerNear);
    }

    function _nearStartCorner() {
        return root._cubicTo(0, root._cornerNear * (1 - root._kappa), root._cornerNear * (1 - root._kappa), root._v0, root._cornerNear, root._v0);
    }

    function _farEndCorner(edgeU) {
        return root._cubicTo(edgeU, root._vMax - root._cornerFar * (1 - root._kappa), edgeU - root._cornerFar * (1 - root._kappa), root._farV, edgeU - root._cornerFar, root._farV);
    }

    function _farStartCorner(edgeU) {
        return root._cubicTo(edgeU + root._cornerFar * (1 - root._kappa), root._farV, edgeU, root._vMax - root._cornerFar * (1 - root._kappa), edgeU, root._vMax - root._cornerFar);
    }

    function _endFilletCubic(tipV) {
        return root._cubicTo(root._bodyEndU + root._filletR * (1 - root._kappa), tipV, root._bodyEndU, root._filletR * (1 - root._kappa), root._bodyEndU, root._filletR);
    }

    function _startFilletCubic(tipV) {
        return root._cubicTo(root._bodyStartU, root._filletR * (1 - root._kappa), root._bodyStartU - root._filletR * (1 - root._kappa), tipV, root._bodyStartU - root._filletR, tipV);
    }

    // tipV: v where the flares become tangent to the anchored edge.
    function _bodyFragment(tipV) {
        let d = root._endFilletCubic(tipV);
        d += root._lineTo(root._bodyEndU, root._vMax - root._cornerFar);
        d += root._farEndCorner(root._bodyEndU);
        d += root._lineTo(root._bottomLineTargetU, root._farV);
        d += root._farStartCorner(root._bodyStartU);
        d += root._lineTo(root._bodyStartU, root._filletR);
        d += root._startFilletCubic(tipV);
        return d;
    }

    function _closedOutline() {
        let d = "M " + root._pt(root._cornerNear, root._v0);
        d += root._lineTo(root._uMax - root._cornerNear, root._v0);
        d += root._nearEndCorner();
        d += root._lineTo(root._uMax, root._v0);
        d += root._lineTo(root._endFilletStartU, root._v0);
        d += root._fillBodyPath;
        d += root._lineTo(0, root._v0);
        d += root._lineTo(0, root.touchesEdge ? root._v0 : root._cornerNear);
        d += root._nearStartCorner();
        return d;
    }

    function _openOutline() {
        return "M " + root._pt(root._endFilletStartU, root._strokeTipV) + root._strokeBodyPath;
    }

    // Path strings are readonly properties so the shared `_bodyFragment` is evaluated once per geometry change.
    readonly property real _strokeTipV: -strokeTipOutset
    readonly property string _fillBodyPath: root._bodyFragment(root._v0)
    readonly property string _strokeBodyPath: root._bodyFragment(root._strokeTipV)
    readonly property string _closedPath: root._closedOutline()
    readonly property string _strokePath: root.touchesEdge ? root._openOutline() : root._closedPath

    // Fill-only strip just outside the anchored edge, across the body and both flares: a panel hung from a
    // bar covers the bar's inner stroke half so the two read as one surface. Clipped away when no bar is there.
    readonly property string _joinBandPath: {
        if (!_stripActive)
            return "";
        const bandStartU = _bodyStartU - _filletR;
        const bandHeight = Theme.Tokens.comp.panelSurface.joinOverlap;
        return "M " + _pt(bandStartU, -bandHeight) + _lineTo(_endFilletStartU, -bandHeight) + _lineTo(_endFilletStartU, _v0) + _lineTo(bandStartU, _v0) + " Z";
    }

    // Arc length of `_strokePath`, corners taken as true quarter circles. The
    // closed path retraces `curveDepth` at each near corner (-4, not -8, per corner pair).
    readonly property real _strokeLength: touchesEdge
        ? Math.PI * (_filletR + curveDepth) + 2 * (_vMax - _filletR - curveDepth) + (_bodyEndU - _bodyStartU - 2 * curveDepth)
        : 2 * (_uMax + _vMax) - 4 * curveDepth + 2 * Math.PI * curveDepth

    Shape {
        id: _surface
        // Paint viewport uses panelWidth/panelHeight — the exact size the path math (_uMax/_vMax, _mapX/_mapY) is drawn against; anchors.fill: parent would track raw width/height instead.
        x: 0
        y: 0
        width: root.panelWidth
        height: root.panelHeight

        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: root.fillColor
            strokeColor: "transparent"
            strokeWidth: 0

            PathSvg {
                path: root._closedPath
            }
        }

        ShapePath {
            fillColor: root.fillColor
            strokeColor: "transparent"
            strokeWidth: 0

            PathSvg {
                path: root._joinBandPath
            }
        }
    }

    Shape {
        width: root.panelWidth
        height: root.panelHeight
        preferredRendererType: Shape.CurveRenderer

        ShapePath {
            fillColor: "transparent"
            strokeColor: root.strokeColor
            strokeWidth: root.strokeWidth
            // A round cap would stick half the pen width past the anchored edge at the open path ends.
            capStyle: root.touchesEdge ? ShapePath.FlatCap : ShapePath.RoundCap
            joinStyle: ShapePath.RoundJoin

            PathSvg {
                path: root._strokePath
            }
        }
    }

    // Held off while the body unfolds: a sweep started on the short outline would be spent before the panel is full size.
    readonly property string _effectiveRayTone: root.bodyReveal < 1 ? "none" : root.rayTone

    FocusRay {
        width: root.panelWidth
        height: root.panelHeight
        tone: root._effectiveRayTone
        strokeWidth: root.strokeWidth
        outlineLength: root._strokeLength

        PathSvg {
            path: root._effectiveRayTone !== "none" ? root._strokePath : ""
        }
    }
}

import QtQuick
import QtQuick.Shapes
import "../../theme" as Theme

/*!
    FocusRay — one dash that sweeps along a caller-supplied outline about
    every `focusSweepInterval` while `tone` is not "none", moving at constant
    speed from the path start to its end, where it disappears until the next
    pass. `tone` "dim" is the lone-panel color, "bright" the focused one. The
    outline is declared as children (path elements, e.g. PathSvg or
    PathLine/PathArc), with `startX`/`startY` for the first point. Size this
    Shape to the coordinate space of that path. `outlineLength` (length of the
    outline path) sets the travel distance.

    Usage:
        FocusRay {
            anchors.fill: parent
            tone: "dim"
            strokeWidth: Theme.Tokens.border.sm
            outlineLength: ...
            PathSvg { path: "..." }
        }
*/
Shape {
    id: root

    property string tone: "none" // "none" | "dim" | "bright"
    property real strokeWidth: Theme.Tokens.border.md
    property real outlineLength: 0
    property alias startX: rayPath.startX
    property alias startY: rayPath.startY
    default property alias outline: rayPath.pathElements

    readonly property int focusSweepInterval: 7000
    readonly property real focusSweepSpeedPxPerMs: 0.7
    readonly property real focusSweepSegmentPx: 48
    readonly property int focusSweepMinPauseMs: 1000
    // The dash enters and fully leaves the outline, so it travels segment + outline length.
    readonly property real passDuration: (focusSweepSegmentPx + Math.max(0, outlineLength)) / focusSweepSpeedPxPerMs

    visible: root.tone !== "none"
    // Default GeometryRenderer on purpose: CurveRenderer's DashLine support is undocumented.

    ShapePath {
        id: rayPath
        // The gap exceeds the outline length, so exactly one dash exists on the path.
        readonly property real gap: root.outlineLength + root.strokeWidth

        fillColor: "transparent"
        strokeColor: root.tone === "bright" ? Theme.Tokens.color.accent : Theme.Tokens.color.accentSecondarySurface
        strokeWidth: root.strokeWidth
        strokeStyle: ShapePath.DashLine
        dashPattern: [root.focusSweepSegmentPx / root.strokeWidth, rayPath.gap / root.strokeWidth]
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin
    }

    onToneChanged: {
        if (root.tone === "none")
            sweep.stop()
        else if (!sweep.running)
            sweep.start()
    }
    Component.onCompleted: if (root.tone !== "none") sweep.start()

    // Restarted by hand after each pause: a running animation keeps the duration and
    // `to` it started with, so the panel's final size would be ignored by a plain loop.
    SequentialAnimation {
        id: sweep
        onStopped: if (root.tone !== "none") sweep.start()

        NumberAnimation {
            target: rayPath
            property: "dashOffset"
            from: root.focusSweepSegmentPx / root.strokeWidth
            to: -root.outlineLength / root.strokeWidth
            duration: root.passDuration
            easing.type: Easing.Linear
        }
        PauseAnimation {
            duration: Math.max(root.focusSweepMinPauseMs, root.focusSweepInterval - root.passDuration)
        }
    }
}

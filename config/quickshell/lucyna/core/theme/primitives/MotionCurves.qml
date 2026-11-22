pragma Singleton

import QtQuick

/*!
    MotionCurves — layer 1 non-color primitive: bezier control-point
    arrays (Qt Easing.BezierSpline format, flat [x1,y1, x2,y2, ..., 1,1])
    and their durations.
*/
QtObject {
    readonly property QtObject expressiveFastSpatial: QtObject {
        readonly property var points: [0.42, 1.67, 0.21, 0.90, 1, 1]
        readonly property int duration: 350
    }
    readonly property QtObject expressiveDefaultSpatial: QtObject {
        readonly property var points: [0.38, 1.21, 0.22, 1.00, 1, 1]
        readonly property int duration: 500
    }
    readonly property QtObject expressiveSlowSpatial: QtObject {
        readonly property var points: [0.39, 1.29, 0.35, 0.98, 1, 1]
        readonly property int duration: 650
    }

    // Same control points across fast/default/slow (brief.md §5) — only
    // duration differs between the three effects variants.
    readonly property QtObject expressiveEffects: QtObject {
        readonly property QtObject fast: QtObject {
            readonly property var points: [0.34, 0.80, 0.34, 1.00, 1, 1]
            readonly property int duration: 150
        }
        readonly property QtObject default_: QtObject {
            readonly property var points: [0.34, 0.80, 0.34, 1.00, 1, 1]
            readonly property int duration: 200
        }
        readonly property QtObject slow: QtObject {
            readonly property var points: [0.34, 0.80, 0.34, 1.00, 1, 1]
            readonly property int duration: 300
        }
    }

    // Multi-segment curve: duration is tabulated as a 400-500 range, not a
    // single value. Assumes 400 (the range's lower bound); no role references this curve directly.
    readonly property QtObject emphasized: QtObject {
        readonly property var points: [0.05, 0, 2 / 15, 0.06, 1 / 6, 0.4, 5 / 24, 0.82, 0.25, 1, 1, 1]
        readonly property int duration: 400
    }

    readonly property QtObject standard: QtObject {
        readonly property var points: [0.2, 0, 0, 1, 1, 1]
        readonly property int duration: 300
    }
    readonly property QtObject standardDecel: QtObject {
        readonly property var points: [0, 0, 0, 1, 1, 1]
        readonly property int duration: 250
    }
    readonly property QtObject standardAccel: QtObject {
        readonly property var points: [0.3, 0, 1, 1, 1, 1]
        readonly property int duration: 200
    }
    readonly property QtObject fastOutSlowIn: QtObject {
        readonly property var points: [0.4, 0, 0.2, 1, 1, 1]
        readonly property int duration: 300
    }
}

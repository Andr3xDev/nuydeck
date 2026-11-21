pragma Singleton

import QtQuick

/*!
    Nadir — nuytek design system
*/
QtObject {
    id: palette

    readonly property string id:   "nadir"
    readonly property string name: "Nuytek Nadir"

    readonly property QtObject neutral: QtObject {
        readonly property color bg:             "#151515"
        readonly property color surface:        "#232323"
        readonly property color border:         "#323232"
        readonly property color borderMuted:    "#2A2A2A"
        readonly property color borderStrong:   "#494949"
        readonly property color textMuted:      "#72837D"
        readonly property color textSecondary:  "#A7B4B0"
        readonly property color textPrimary:    "#E9EDEB"
        readonly property color onAccent:       "#151515"
    }

    readonly property QtObject hue: QtObject {
        readonly property QtObject turquoise:  QtObject { readonly property color fill: "#5AE2DD"; readonly property color surface: "#143937" }
        readonly property QtObject aquamarine: QtObject { readonly property color fill: "#67E4B2"; readonly property color surface: "#14392A" }
        readonly property QtObject blue:       QtObject { readonly property color fill: "#A6D5F2"; readonly property color surface: "#162A36" }
        readonly property QtObject red:        QtObject { readonly property color fill: "#EF616D"; readonly property color surface: "#391417" }
        readonly property QtObject green:      QtObject { readonly property color fill: "#84EB8D"; readonly property color surface: "#143917" }
        readonly property QtObject orange:     QtObject { readonly property color fill: "#F39049"; readonly property color surface: "#392314" }
        readonly property QtObject yellow:     QtObject { readonly property color fill: "#ECEC8D"; readonly property color surface: "#393914" }
        readonly property QtObject purple:     QtObject { readonly property color fill: "#B590EA"; readonly property color surface: "#231439" }
        readonly property QtObject pink:       QtObject { readonly property color fill: "#EA71AD"; readonly property color surface: "#391426" }
    }
}

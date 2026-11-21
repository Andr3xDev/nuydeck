pragma Singleton

import QtQuick

/*!
    Zenit — nuytek design system
*/
QtObject {
    id: palette

    readonly property string id:   "zenit"
    readonly property string name: "Nuytek Zenit"

    readonly property QtObject neutral: QtObject {
        readonly property color bg:             "#F3F4F4"
        readonly property color surface:        "#FAFAFA"
        readonly property color border:         "#DADCDC"
        readonly property color borderMuted:    "#E6E7E7"
        readonly property color borderStrong:   "#B9BCBC"
        readonly property color textMuted:      "#5E6D67"
        readonly property color textSecondary:  "#4B5A54"
        readonly property color textPrimary:    "#16211E"
        readonly property color onAccent:       "#F7F8F8"
    }

    readonly property QtObject hue: QtObject {
        readonly property QtObject turquoise:  QtObject { readonly property color fill: "#0B726D"; readonly property color surface: "#DFF3F1" }
        readonly property QtObject aquamarine: QtObject { readonly property color fill: "#0E7A4C"; readonly property color surface: "#DFF3E7" }
        readonly property QtObject blue:       QtObject { readonly property color fill: "#146496"; readonly property color surface: "#DFEBF7" }
        readonly property QtObject red:        QtObject { readonly property color fill: "#C23B46"; readonly property color surface: "#FBE4E5" }
        readonly property QtObject green:      QtObject { readonly property color fill: "#1E8A3B"; readonly property color surface: "#E1F5E5" }
        readonly property QtObject orange:     QtObject { readonly property color fill: "#B85F1D"; readonly property color surface: "#FBEADC" }
        readonly property QtObject yellow:     QtObject { readonly property color fill: "#8A7C12"; readonly property color surface: "#F5F0D6" }
        readonly property QtObject purple:     QtObject { readonly property color fill: "#6E4FA0"; readonly property color surface: "#EDE5F7" }
        readonly property QtObject pink:       QtObject { readonly property color fill: "#B23E76"; readonly property color surface: "#F7E3ED" }
    }
}

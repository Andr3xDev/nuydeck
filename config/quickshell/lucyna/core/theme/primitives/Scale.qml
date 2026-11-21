pragma Singleton

import QtQuick

/*!
    Scale — layer 1 non-color primitives: spacing, radius, text, font, border.
*/
QtObject {
    readonly property QtObject space: QtObject {
        readonly property int xxs: 2
        readonly property int xs: 4
        readonly property int sm: 8
        readonly property int md: 12
        readonly property int lg: 16
        readonly property int xxl: 32
    }

    readonly property QtObject radius: QtObject {
        readonly property int none: 0
        readonly property int sm: 5
        readonly property int md: 9
        readonly property int lg: 12
        readonly property int panel: 10
        readonly property int full: 9999
    }

    readonly property QtObject text: QtObject {
        readonly property int compactXs: 8
        readonly property int compactSm: 10
        readonly property int compactMd: 11
        readonly property int compactLg: 13
        readonly property int compactXl: 16
        readonly property int icon: 12
        readonly property int iconMd: 16
        readonly property int iconLg: 20
        readonly property int iconXl: 24
        readonly property int iconXxl: 32
    }

    readonly property QtObject font: QtObject {
        readonly property string sans: "Geist"
        readonly property string mono: "JetBrainsMono Nerd Font Mono"
        readonly property string icon: "Symbols Nerd Font"
    }

    readonly property QtObject border: QtObject {
        readonly property int sm: 1
        readonly property int md: 2
        readonly property int lg: 4
    }
}

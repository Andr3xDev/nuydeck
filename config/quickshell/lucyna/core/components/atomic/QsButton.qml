import QtQuick
import "../../theme" as Theme
import "../../animations" as Animations

/*!
    QsButton — polivalent button for module UIs.

    Not for bar widgets (those have active/toggle state and their own components).

    Variants
    --------
    "ghost"   neutral bg, lighter on hover, textPrimary label  (default)
    "outline" ghost + neutral border
    "filled"  hue surface bg, hue fill label

    `hue` names the color pair of "filled" (accent, accentSecondary, info,
    success, warning, danger). Unknown variant renders as ghost; unknown hue as
    accent; unknown size as md. An explicit color prop replaces the variant
    color for that part only.

    Focus is host-driven: the button never grabs focus. `selected` (set by the
    host) or its own activeFocus draws the focus border; Enter/Return/Space
    activate only while it has activeFocus.
*/
Rectangle {
    id: root

    property string label: ""
    property string glyph: ""
    property string trailingGlyph: ""

    property Item leadingContent: null
    property Item trailingContent: null

    property string variant: "ghost"
    property string hue: "accent"
    property string size: "md"

    property bool selected: false
    property color focusColor: _colored ? _hueFill : _neutralFocusColor

    property color bgColor:      _variantBg
    property color hoverColor:   _variantHover
    property color pressedColor: _variantPressed
    property color labelColor:   _variantLabel
    property color hoverLabelColor: labelColor

    property int   borderWidth: _variant === "outline" ? Theme.Tokens.border.sm : 0
    property color borderColor: Theme.Tokens.color.border

    signal clicked()

    function activate() {
        if (enabled)
            clicked()
    }

    readonly property var _sizeTable: ({
        sm: { labelRole: "body",  iconSize: "sm", padH: Theme.Tokens.space.sm, padV: Theme.Tokens.space.xs, radius: Theme.Tokens.radius.sm },
        md: { labelRole: "body",  iconSize: "sm", padH: Theme.Tokens.space.md, padV: Theme.Tokens.space.sm, radius: Theme.Tokens.radius.sm },
        lg: { labelRole: "title", iconSize: "md", padH: Theme.Tokens.space.lg, padV: Theme.Tokens.space.sm, radius: Theme.Tokens.radius.md }
    })
    readonly property var _size: _sizeTable.hasOwnProperty(size) ? _sizeTable[size] : _sizeTable.md

    readonly property string _variant: {
        switch (variant) {
            case "filled":
            case "outline": return variant
            default:        return "ghost"
        }
    }
    readonly property bool  _colored:     _variant === "filled"
    readonly property var _hueTable: ({
        accent:          { fill: Theme.Tokens.color.accent,          surface: Theme.Tokens.color.accentSurface },
        accentSecondary: { fill: Theme.Tokens.color.accentSecondary, surface: Theme.Tokens.color.accentSecondarySurface },
        info:            { fill: Theme.Tokens.color.info,            surface: Theme.Tokens.color.infoSurface },
        success:         { fill: Theme.Tokens.color.success,         surface: Theme.Tokens.color.successSurface },
        warning:         { fill: Theme.Tokens.color.warning,         surface: Theme.Tokens.color.warningSurface },
        danger:          { fill: Theme.Tokens.color.danger,          surface: Theme.Tokens.color.dangerSurface }
    })
    readonly property var   _hue:         _hueTable.hasOwnProperty(hue) ? _hueTable[hue] : _hueTable.accent
    readonly property color _hueFill:     _hue.fill
    readonly property color _hueSurface:  _hue.surface
    readonly property color _neutralFocusColor: Theme.Tokens.color.accentSurface

    readonly property color _variantBg:      _colored ? _hueSurface : Theme.Tokens.color.surface
    readonly property color _variantHover:   _colored ? Qt.lighter(_hueSurface) : Theme.Tokens.color.borderMuted
    readonly property color _variantPressed: _colored ? Qt.darker(_hueSurface)  : Theme.Tokens.color.bg
    readonly property color _variantLabel:   _colored ? _hueFill : Theme.Tokens.color.textPrimary

    readonly property bool  _focused:    selected || activeFocus
    readonly property color _labelShown: !enabled ? Theme.Tokens.color.textMuted
                                       : _area.containsMouse ? hoverLabelColor : labelColor

    implicitWidth:  _row.implicitWidth  + _size.padH * 2
    implicitHeight: _row.implicitHeight + _size.padV * 2

    color: !enabled ? bgColor
         : _area.pressed ? pressedColor
         : _area.containsMouse ? hoverColor : bgColor
    radius:       _size.radius
    border.width: _focused ? Theme.Tokens.border.sm : borderWidth
    border.color: _focused ? focusColor : borderColor

    Behavior on color { Animations.MotionColorAnimation { role: "stateFast" } }

    onLeadingContentChanged:  if (leadingContent)  leadingContent.parent  = _leadingHolder
    onTrailingContentChanged: if (trailingContent) trailingContent.parent = _trailingHolder

    Keys.onPressed: event => {
        if (!enabled || event.isAutoRepeat)
            return
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            event.accepted = true
            activate()
        }
    }

    Row {
        id: _row
        anchors.centerIn: parent
        spacing: Theme.Tokens.space.xs

        Item {
            id: _leadingHolder
            visible: root.leadingContent !== null
            implicitWidth:  childrenRect.width
            implicitHeight: childrenRect.height
            anchors.verticalCenter: parent.verticalCenter
        }

        QsText {
            visible:  root.glyph !== ""
            text:     root.glyph
            role:     "icon"
            iconSize: root._size.iconSize
            color:    root._labelShown
            anchors.verticalCenter: parent.verticalCenter
        }

        QsText {
            visible: root.label !== ""
            text:    root.label
            role:    root._size.labelRole
            color:   root._labelShown
            anchors.verticalCenter: parent.verticalCenter
        }

        QsText {
            visible:  root.trailingGlyph !== ""
            text:     root.trailingGlyph
            role:     "icon"
            iconSize: root._size.iconSize
            color:    root._labelShown
            anchors.verticalCenter: parent.verticalCenter
        }

        Item {
            id: _trailingHolder
            visible: root.trailingContent !== null
            implicitWidth:  childrenRect.width
            implicitHeight: childrenRect.height
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    MouseArea {
        id: _area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape:  root.enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
        onClicked:    root.activate()
    }
}

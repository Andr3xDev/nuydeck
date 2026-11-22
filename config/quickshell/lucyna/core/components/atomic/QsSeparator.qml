import QtQuick
import "../../theme" as Theme

/*!
    QsSeparator — thin divider for Row/ColumnLayout children.

    A 1px Rectangle sized to a ratio of its parent's height (vertical, the
    default) or width (horizontal). Layout attached properties (alignment,
    fill, ...) are set by the consuming layout, so this works as a drop-in
    child with no extra wiring.

    Usage:
        RowLayout {
            Label {}
            QsSeparator {}
            Label {}
        }
*/
Rectangle {
    id: root

    property int orientation: Qt.Vertical
    // Fraction of the parent's length along the separator's axis.
    property real lengthRatio: 0.7

    // Trims a whole-pixel margin from each end instead of rounding the length,
    // so a centered separator keeps a whole-pixel offset (no blurry edges).
    function snappedLength(parentLength) {
        return parentLength - 2 * Math.round(parentLength * (1 - root.lengthRatio) / 2);
    }

    implicitWidth: root.orientation === Qt.Horizontal ? snappedLength(parent.width) : 1
    implicitHeight: root.orientation === Qt.Horizontal ? 1 : snappedLength(parent.height)
    color: Theme.Tokens.color.borderMuted
    radius: Theme.Tokens.radius.full
}

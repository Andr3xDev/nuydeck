import QtQuick
import "../../../../core/theme" as Theme
import "../../../../core/components/atomic"

/*!
    CapsLockView — caps lock OSD pair's view half.

    Content-only: fills the OsdWindow pill's content area and binds directly
    to CapsLockState — it never requests OSDs, it only renders state. Warning
    color when locked, plain text color when unlocked.
*/
QsText {
    // No anchors.fill: the pill derives its own size from this view's
    // implicit (text-metric) size (OsdWindow.qml's `_pillComponent`) —
    // filling an undefined-size parent would be circular.
    text: CapsLockState.locked ? "󰌌" : "󰌎"
    role: "icon"
    iconSize: "lg"
    color: CapsLockState.locked
        ? Theme.Tokens.color.warning
        : Theme.Tokens.color.textPrimary
}

import QtQuick
import "../../theme" as Theme

/*!
    SearchField — QsInput preset (boxed, lg) with a leading search icon and
    optional `leadingContent`/`trailingContent` items. Generic: key handling stays with the
    consumer through `accepted` and `keyPressed` (set `event.accepted` to
    consume). Unhandled keys bubble to ancestors.

    Usage:
        SearchField {
            placeholder: "Search..."
            onAccepted: launch()
            trailingContent: QsText { text: "esc" }
        }
*/
QsInput {
    size: "lg"
    style: "boxed"
    glyph: " 󰍉 "
    letterSpacing: Theme.Tokens.comp.searchField.letterSpacing
}

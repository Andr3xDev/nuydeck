import QtQuick
import QtQuick.Layouts
import "../../../../core/theme" as Theme
import "../state"

/*!
    SearchPane — filter field plus the result list, with one even margin on every side.
    The filter text mirrors ClipboardState.filter; typing writes back only on real change.
    `keymap` is forwarded to FilterBar.
*/
ColumnLayout {
    id: root

    property Item keymap

    function focusFilter() {
        filterBar.focusInput();
    }

    spacing: Theme.Tokens.space.sm

    FilterBar {
        id: filterBar
        Layout.fillWidth: true
        Layout.topMargin: Theme.Tokens.comp.clip.pad
        Layout.leftMargin: Theme.Tokens.comp.clip.pad
        Layout.rightMargin: Theme.Tokens.comp.clip.pad
        keymap: root.keymap
        position: ClipboardState.positionText

        Binding {
            target: filterBar
            property: "text"
            value: ClipboardState.filter
        }
        onTextChanged: {
            if (text !== ClipboardState.filter)
                ClipboardState.setFilter(text);
        }
    }

    ClipList {
        Layout.fillWidth: true
        Layout.preferredHeight: Theme.Tokens.comp.clip.listRows * Theme.Tokens.comp.clip.rowH
        Layout.leftMargin: Theme.Tokens.comp.clip.pad
        Layout.rightMargin: Theme.Tokens.comp.clip.pad
        Layout.bottomMargin: Theme.Tokens.comp.clip.pad
        model: ClipboardState.rows
        currentIndex: ClipboardState.selectedIndex
    }
}

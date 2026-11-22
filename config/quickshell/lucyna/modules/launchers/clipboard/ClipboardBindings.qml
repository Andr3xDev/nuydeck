import QtQuick

/*!
    ClipboardBindings — key tables of the clipboard panel (data only): mode -> { token or sequence -> action name }, consumed by core/components/input/ModalKeymap.
*/
QtObject {
    readonly property var normalHistory: ({
        "j": "moveDown", "k": "moveUp",
        "gg": "jumpFirst", "G": "jumpLast",
        "<C-d>": "halfPageDown", "<C-u>": "halfPageUp",
        "y": "copy", "<CR>": "copy",
        "1": "copyIndex", "2": "copyIndex", "3": "copyIndex",
        "4": "copyIndex", "5": "copyIndex", "6": "copyIndex",
        "7": "copyIndex", "8": "copyIndex", "9": "copyIndex",
        "d": "deleteEntry",
        "m": "addToFavorites",
        "D": "wipeHistory",
        "/": "enterInsert", "i": "enterInsert",
        "q": "close", "<Esc>": "escape",
        "<Tab>": "nextSection", "<S-Tab>": "prevSection",
        "gt": "nextSection", "gT": "prevSection",
        "p": "togglePreview",
        "J": "previewScrollDown", "K": "previewScrollUp"
    })

    readonly property var normalFavorites: ({
        "j": "moveDown", "k": "moveUp",
        "gg": "jumpFirst", "G": "jumpLast",
        "<C-d>": "halfPageDown", "<C-u>": "halfPageUp",
        "l": "activate", "<CR>": "activate",
        "h": "leaveCollection",
        "y": "copy",
        "1": "copyIndex", "2": "copyIndex", "3": "copyIndex",
        "4": "copyIndex", "5": "copyIndex", "6": "copyIndex",
        "7": "copyIndex", "8": "copyIndex", "9": "copyIndex",
        "a": "add", "r": "rename", "c": "changeValue", "s": "markSecret",
        "d": "deleteEntry",
        "/": "enterInsert", "i": "enterInsert",
        "q": "close", "<Esc>": "escape",
        "<Tab>": "nextSection", "<S-Tab>": "prevSection",
        "gt": "nextSection", "gT": "prevSection",
        "p": "togglePreview",
        "J": "previewScrollDown", "K": "previewScrollUp"
    })

    readonly property var insert: ({
        "<Esc>": "leaveInsert", "<CR>": "activate",
        "<Tab>": "nextSection", "<S-Tab>": "prevSection",
        "<C-p>": "togglePreview"
    })

    readonly property var pick: ({
        "j": "pickNext", "k": "pickPrev",
        "<CR>": "pickAccept", "<Esc>": "pickCancel"
    })

    readonly property var confirm: ({
        "y": "confirmAccept", "<CR>": "confirmActivate",
        "n": "confirmDecline", "<Esc>": "confirmDecline",
        "h": "confirmToggle", "l": "confirmToggle",
        "<Left>": "confirmToggle", "<Right>": "confirmToggle",
        "<Tab>": "confirmToggle", "<S-Tab>": "confirmToggle"
    })
}

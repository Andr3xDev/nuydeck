pragma Singleton

import QtQuick

/*!
    CapsLockState — the caps lock pair's state source.

    Caps lock has no persistent service like AudioService, so the pair gets
    this local singleton: the trigger writes the derived locked state, the
    view binds it directly.
*/
QtObject {
    property bool locked: false
}

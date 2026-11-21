import QtQuick
import QtQuick.Layouts
import "../../../../core/components/atomic"
import "../../../../core/animations" as Animations
import "../state"

/*!
    ContentPane — toggleable preview box above the search box (ClipboardState.previewShown),
    divided by a separator that fades out with the preview. `keymap` is forwarded to SearchPane/FilterBar.
*/
ColumnLayout {
    id: root

    property Item keymap

    function focusFilter() { searchPane.focusFilter(); }
    function scrollPreviewDown() { previewPane.scrollDown(); }
    function scrollPreviewUp() { previewPane.scrollUp(); }

    spacing: 0

    PreviewPane {
        id: previewPane
        Layout.fillWidth: true
        source: ClipboardState.selectedSource
        expanded: ClipboardState.previewShown
    }

    QsSeparator {
        Layout.fillWidth: true
        Layout.preferredWidth: 0
        orientation: Qt.Horizontal
        lengthRatio: 1
        opacity: previewPane.implicitHeight > 0 ? 1 : 0

        Behavior on opacity {
            Animations.MotionAnimation { role: "stateStandard" }
        }
    }

    SearchPane {
        id: searchPane
        Layout.fillWidth: true
        keymap: root.keymap
    }
}

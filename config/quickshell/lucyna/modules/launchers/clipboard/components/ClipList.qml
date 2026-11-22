import QtQuick
import QtQuick.Layouts
import "../../../../core/components/atomic"
import "../../../../core/animations" as Animations
import "../../../../core/theme" as Theme
import "../state"

/*!
    ClipList — one list for History, collection and entry rows.

    `model` is ClipboardState.rows ({ kind, title, subtitle, masked, imageId });
    `currentIndex` is ClipboardState.selectedIndex. Selection is never owned by the
    ListView: its own currentIndex drifts when rows arrive after creation, so it is
    re-synced from `currentIndex` (which also slides the highlight) and the view is scrolled explicitly.
    Thumbnails come straight from ClipboardHistory (ClipboardState only decodes the selected row).
*/
Item {
    id: root

    property var model
    property int currentIndex: 0

    readonly property real _subtitleShare: 0.4
    readonly property string iconSearch:     "\uf002"
    readonly property string iconCollection: "\uf07c"
    readonly property string textNoResults:    "No results"
    readonly property string textNoEntries:    "Empty collection"
    readonly property string textNoCollections: "No collections"
    readonly property string textNoHistory:    "No history"
    readonly property bool _favorites: ClipboardState.section === ClipboardState.sectionFavorites
    readonly property var _empty: {
        if (ClipboardState.filter !== "")
            return { icon: iconSearch, text: textNoResults };
        if (!_favorites)
            return { icon: ClipboardState.iconHistory, text: textNoHistory };
        if (ClipboardState.collection !== "")
            return { icon: iconCollection, text: textNoEntries };
        return { icon: ClipboardState.iconFavorites, text: textNoCollections };
    }

    onCurrentIndexChanged: view._syncCurrent()

    ListView {
        id: view
        anchors.fill: parent
        clip: true
        model: root.model
        interactive: false
        highlightMoveDuration: Theme.Tokens.motion.role.stateFast.duration
        highlightResizeDuration: 0

        function _syncCurrent() {
            currentIndex = root.currentIndex;
            positionViewAtIndex(root.currentIndex, ListView.Contain);
        }

        onCountChanged: _syncCurrent()
        onHeightChanged: positionViewAtIndex(root.currentIndex, ListView.Contain)

        highlight: Rectangle {
            radius: Theme.Tokens.radius.lg
            color: Theme.Tokens.color.surface
        }

        delegate: Item {
            id: rowItem
            required property var modelData
            required property int index
            readonly property bool selected: index === root.currentIndex
            readonly property string imageUrl: modelData.imageId !== ""
                ? ClipboardHistory.decodedPath(modelData.imageId) : ""

            readonly property bool isEntry: modelData.kind === ClipboardState.kindEntry

            width: view.width
            height: isEntry ? Theme.Tokens.comp.clip.rowTallH : Theme.Tokens.comp.clip.rowH

            Component.onCompleted: {
                if (modelData.imageId !== "")
                    ClipboardHistory.requestDecode(modelData.imageId);
            }

            RowLayout {
                id: row
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.leftMargin: Theme.Tokens.comp.clip.pad
                anchors.rightMargin: Theme.Tokens.comp.clip.pad
                anchors.verticalCenter: parent.verticalCenter
                spacing: Theme.Tokens.space.sm

                // Only this leading content scales; the trailing subtitle keeps its position.
                Item {
                    id: lead
                    Layout.fillWidth: true
                    Layout.preferredHeight: leadContent.implicitHeight

                    ColumnLayout {
                        id: leadContent
                        // Pre-shrunk so the scaled content still ends at the lead's right edge.
                        width: lead.width / Theme.Tokens.comp.clip.selectedScale
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 0
                        scale: rowItem.selected ? Theme.Tokens.comp.clip.selectedScale : 1
                        transformOrigin: Item.Left

                        Behavior on scale {
                            Animations.MotionAnimation { role: "stateFast" }
                        }

                        Image {
                            visible: rowItem.imageUrl !== ""
                            Layout.preferredWidth: Theme.Tokens.comp.clip.rowH - 2 * Theme.Tokens.space.sm
                            Layout.preferredHeight: Layout.preferredWidth
                            source: rowItem.imageUrl
                            sourceSize.width: Layout.preferredWidth
                            sourceSize.height: Layout.preferredHeight
                            fillMode: Image.PreserveAspectFit
                            asynchronous: true
                        }

                        QsText {
                            Layout.fillWidth: true
                            visible: rowItem.imageUrl === ""
                            role: "title"
                            elide: Text.ElideRight
                            maximumLineCount: 1
                            textFormat: Text.PlainText
                            text: rowItem.modelData.title
                            color: rowItem.selected ? Theme.Tokens.color.textPrimary : Theme.Tokens.color.textSecondary

                            Behavior on color {
                                Animations.MotionColorAnimation { role: "stateFast" }
                            }
                        }

                        QsText {
                            Layout.fillWidth: true
                            visible: rowItem.isEntry
                            role: "subtitle"
                            elide: Text.ElideRight
                            maximumLineCount: 1
                            textFormat: Text.PlainText
                            text: rowItem.modelData.subtitle
                            color: Theme.Tokens.color.textMuted
                        }
                    }
                }

                QsText {
                    Layout.maximumWidth: row.width * root._subtitleShare
                    visible: !rowItem.isEntry && text !== ""
                    elide: Text.ElideRight
                    maximumLineCount: 1
                    textFormat: Text.PlainText
                    text: rowItem.modelData.subtitle
                    color: Theme.Tokens.color.textMuted
                }
            }
        }
    }

    QsEmptyState {
        anchors.centerIn: parent
        visible: ClipboardState.rowCount === 0
        icon: root._empty.icon
        text: root._empty.text
    }
}

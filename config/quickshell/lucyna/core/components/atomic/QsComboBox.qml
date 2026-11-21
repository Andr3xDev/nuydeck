import QtQuick
import QtQuick.Controls
import "../../theme" as Theme

/*!
    QsComboBox — drop-in replacement for ComboBox with shell visual style.
*/
ComboBox {
    id: root

    property int maxPopupHeight: 200

    implicitHeight: 32

    background: Rectangle {
        color:        Theme.Tokens.color.surface
        radius:       Theme.Tokens.radius.lg
        border.width: 1
        border.color: root.activeFocus
                          ? Theme.Tokens.color.accent
                          : Theme.Tokens.color.border
    }

    contentItem: QsText {
        leftPadding:       8
        text:              root.displayText
        role:              "subtitle"
        verticalAlignment: Text.AlignVCenter
        elide:             Text.ElideRight
    }

    delegate: ItemDelegate {
        width:       root.width
        highlighted: root.highlightedIndex === index

        contentItem: QsText {
            leftPadding:       8
            text:              modelData
            role:              "subtitle"
            color:             highlighted
                                   ? Theme.Tokens.color.accent
                                   : Theme.Tokens.color.textPrimary
            verticalAlignment: Text.AlignVCenter
        }
        background: Rectangle {
            color:  highlighted
                        ? Theme.Tokens.color.accentSurface
                        : Theme.Tokens.color.surface
            radius: Theme.Tokens.radius.md
        }
    }

    popup: Popup {
        y:       root.height + 2
        width:   root.width
        padding: 0

        background: Rectangle {
            color:        Theme.Tokens.color.surface
            radius:       Theme.Tokens.radius.lg
            border.width: 1
            border.color: Theme.Tokens.color.border
        }

        contentItem: Rectangle {
            color:  "transparent"
            radius: Theme.Tokens.radius.lg
            clip:   true

            implicitHeight: Math.min(_list.contentHeight + Theme.Tokens.space.sm * 2,
                                     root.maxPopupHeight)

            ListView {
                id: _list
                anchors {
                    fill:         parent
                    topMargin:    Theme.Tokens.space.xs
                    bottomMargin: Theme.Tokens.space.xs
                    leftMargin:   Theme.Tokens.space.xs
                    rightMargin:  Theme.Tokens.space.xs
                }
                clip:         true
                model:        root.delegateModel
                currentIndex: root.highlightedIndex
                ScrollIndicator.vertical: ScrollIndicator { }
            }
        }
    }
}

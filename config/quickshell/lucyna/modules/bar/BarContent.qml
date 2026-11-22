import QtQuick
import QtQuick.Layouts
import "../../core/theme" as Theme
import "../../core/components/panel" as Core
import "../../core/components/atomic"
import "components"
import "layout"

/*!
    The bar's content tree: surface shape plus left and right sections. Hosted
    by Bar.qml, which owns the PanelService registration.
*/
Item {
    id: root

    property QtObject screen: null
    readonly property real lateralVisualMargin: Theme.Tokens.comp.bar.bodyInset
    readonly property real separatorLengthRatio: 0.75

    Core.PanelSurfaceShape {
        anchors.fill: parent
        attachEdge: "top"
        touchesEdge: true
        bodyInset: root.lateralVisualMargin
        filletRadius: Theme.Tokens.space.md
        strokeWidth: Theme.Tokens.comp.panelSurface.strokeWidth
        depthInset: Theme.Tokens.comp.panelSurface.depthInset
        fillColor: Theme.Tokens.color.bg
        strokeColor: Theme.Tokens.color.border
    }

    // Mirrors the surface's bodyInset via the same lateralVisualMargin.
    Item {
        anchors {
            fill: parent
            leftMargin: root.lateralVisualMargin
            rightMargin: root.lateralVisualMargin
            // Whole pixels keep centered separators crisp on 1x screens.
            bottomMargin: Math.round(Theme.Tokens.comp.panelSurface.strokeWidth)
        }

        Item {
            anchors {
                left: parent.left
                top: parent.top
                bottom: parent.bottom
            }
            width: leftContent.implicitWidth

            RowLayout {
                id: leftContent
                anchors.fill: parent
                spacing: Theme.Tokens.space.sm

                // sm spacing + md spacer mirrors the right section's edge inset
                Item {
                    Layout.preferredWidth: Theme.Tokens.space.md
                }

                ArchLogo {}

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                QsSeparator {
                    lengthRatio: root.separatorLengthRatio
                    Layout.alignment: Qt.AlignVCenter
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                Clock {}

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                QsSeparator {
                    lengthRatio: root.separatorLengthRatio
                    Layout.alignment: Qt.AlignVCenter
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.sm
                }

                Workspaces {
                    screen: root.screen
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.sm
                }

                QsSeparator {
                    lengthRatio: root.separatorLengthRatio
                    Layout.alignment: Qt.AlignVCenter
                }
            }
        }

        Item {
            anchors {
                right: parent.right
                top: parent.top
                bottom: parent.bottom
            }
            width: rightContent.implicitWidth

            RowLayout {
                id: rightContent
                anchors.fill: parent
                spacing: Theme.Tokens.space.sm

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                PowerProfile {
                    expanded: powerToggle.expanded
                }

                ToggleIndicator {
                    id: powerToggle
                    icon: "󱐋"
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                SystemControls {
                    expanded: controlsToggle.expanded
                }

                ToggleIndicator {
                    id: controlsToggle
                    icon: "󰒓"
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                SystemTemperatures {
                    expanded: tempToggle.expanded
                }

                ToggleIndicator {
                    id: tempToggle
                    icon: "󰔏"
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                SystemMetrics {
                    expanded: metricsToggle.expanded
                }

                ToggleIndicator {
                    id: metricsToggle
                    icon: "󰕮"
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                QsSeparator {
                    lengthRatio: root.separatorLengthRatio
                    Layout.alignment: Qt.AlignVCenter
                }

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.xs
                }

                Battery {}

                Item {
                    Layout.preferredWidth: Theme.Tokens.space.md
                }
            }
        }
    }
}

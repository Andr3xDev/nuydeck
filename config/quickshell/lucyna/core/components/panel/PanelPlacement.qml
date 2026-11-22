import QtQuick
import "../../services/panel" as Services

/*!
    PanelPlacement — a panel's declared placement facts (see PanelRules for the
    vocabulary and the derivation). Enum fields are `var` on purpose: a
    misspelled enum name stores `undefined`, which PanelRules.validate rejects,
    where an `int` property could turn it into a silent 0.
*/
QtObject {
    property var type: Services.PanelRules.unset
    property size size: Qt.size(0, 0)
    property var shape: Services.PanelRules.Shape.Auto
    property var side: Services.PanelRules.Side.None
    property var exclusionGroup: Services.PanelRules.Group.None
    property var autoClose: Services.PanelRules.AutoClose.On
}

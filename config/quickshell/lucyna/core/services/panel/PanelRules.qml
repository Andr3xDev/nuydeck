pragma Singleton

import QtQuick
import Quickshell
import "../../theme" as Theme

/*!
    PanelRules — vocabulary and pure derivation for panel placement.
    No state and no registry: PanelService owns the state and calls in here.

    A panel declares a PanelPlacement (type, content size, shape, side, exclusionGroup,
    autoClose) using the enums below; zone, shape, surface size, extent and auto-close are
    derived here, never written by the panel. The declared size is the CONTENT size: the
    chrome pad is added by surfaceSizeOf(), and every zone calculation uses the surface.
    Adding a type is one enum line and one row in typeSpecs (validate() checks a type
    against typeSpecs, not a range); adding a shape, side, group or auto-close mode is one
    enum line and, when it becomes the last member, the upper bound of its entry in the
    enumFields list of validate().

    A placement that fails validate() is invalid: the panel is registered but
    never placed or opened. There is no floating fallback; zoneOf() returns ""
    for a valid Floating placement (and for an invalid one, which is never placed).
    An object with no placement (bar, legacy panels) is unplaced and silent.
    The OSD is treated as floating and is not registered.
*/
Singleton {
    id: root

    // Each enum starts at its own base so a value of another enum (Side.Left for a
    // type) falls outside the range check instead of passing as a plain int.
    enum Type { BarAttached = 100, Edge, Floating }
    enum Shape { Auto = 200, Wide, Tall }
    enum Side { None = 300, Left, Right }
    enum Group { None = 400, Capture }
    enum AutoClose { On = 500, Off }

    readonly property int zoneCap: 2
    readonly property real zoneGap: Theme.Tokens.space.md / 2
    readonly property real wideAspectMin: 1.3
    readonly property real tallAspectMax: 0.77
    readonly property int unset: -1

    // Zone ids stay internal strings; panels never write them.
    // Kind table, one row per Type, every row with the same columns. zone: the default zone ("" for none).
    // barAttached: offset below the bar and join its border, while attached to the top edge.
    // centered: placed mid-screen, no zone. staged: base first, then body (else the whole panel rises).
    // card: card chrome (else the shell). exclusive: closes every other placed panel on its screen, and is closed by them.
    readonly property var typeSpecs: ({
        [PanelRules.Type.BarAttached]: { zone: "top", barAttached: true, centered: false, staged: true, card: false, exclusive: false },
        [PanelRules.Type.Edge]: { zone: "bottom", barAttached: false, centered: false, staged: true, card: false, exclusive: false },
        [PanelRules.Type.Floating]: { zone: "", barAttached: false, centered: true, staged: false, card: true, exclusive: true }
    })

    // Row for an unknown type, so a panel that fails validate() can still evaluate its bindings.
    readonly property var inertSpec: ({ zone: "", barAttached: false, centered: false, staged: false, card: false, exclusive: false })

    function specOf(type) {
        return root.typeSpecs[type] ?? root.inertSpec
    }

    function isExclusive(placement) {
        return root.specOf(placement.type).exclusive
    }

    // Per attached edge: the axis the panel slides along and the sign of the direction toward the edge.
    readonly property var edgeSpecs: ({
        top: { axis: "y", sign: -1 },
        bottom: { axis: "y", sign: 1 },
        left: { axis: "x", sign: -1 },
        right: { axis: "x", sign: 1 }
    })

    // "" and 0 for a non-edge, so callers need no guard.
    function slideAxisOf(edge) {
        return root.edgeSpecs[edge] ? root.edgeSpecs[edge].axis : ""
    }

    function slideSignOf(edge) {
        return root.edgeSpecs[edge] ? root.edgeSpecs[edge].sign : 0
    }

    // A screen side (left or right): the panel slides along x.
    function isVerticalZone(zone) {
        return root.slideAxisOf(zone) === "x"
    }

    // QML enum objects are not enumerable, so the range check takes the first and last member.
    function _isMember(firstMember, lastMember, value) {
        return Number.isInteger(value) && value >= firstMember && value <= lastMember
    }

    function _isMeasurable(placement) {
        return placement.size.width > 0 && placement.size.height > 0
    }

    // Wide or Tall when the ratio is clear, Auto inside the ambiguous band.
    function _clearShapeOf(placement) {
        if (!root._isMeasurable(placement))
            return PanelRules.Shape.Auto
        const ratio = placement.size.width / placement.size.height
        if (ratio > root.wideAspectMin)
            return PanelRules.Shape.Wide
        if (ratio < root.tallAspectMax)
            return PanelRules.Shape.Tall
        return PanelRules.Shape.Auto
    }

    function shapeOf(placement) {
        const clear = root._clearShapeOf(placement)
        if (clear !== PanelRules.Shape.Auto)
            return clear
        return placement.shape === PanelRules.Shape.Tall ? PanelRules.Shape.Tall : PanelRules.Shape.Wide
    }

    function zoneOf(placement) {
        const spec = root.specOf(placement.type)
        if (spec.zone === "")
            return ""
        if (root.shapeOf(placement) === PanelRules.Shape.Tall) {
            if (placement.side === PanelRules.Side.Left)
                return "left"
            if (placement.side === PanelRules.Side.Right)
                return "right"
            return ""
        }
        return spec.zone
    }

    // Per-side pad PanelShell leaves around its content: the gutter, plus bodyInset on the axis along the edge.
    function shellPad(verticalEdge) {
        const gutter = Theme.Tokens.comp.panelSurface.contentGutter
        const alongEdge = Theme.Tokens.comp.panelSurface.alongEdgePad
        return verticalEdge ? Qt.size(gutter, alongEdge) : Qt.size(alongEdge, gutter)
    }

    // The panel's full size: the declared content size plus the shell pad of its zone. A card adds none.
    function surfaceSizeOf(placement) {
        if (root.specOf(placement.type).card)
            return placement.size
        const pad = root.shellPad(root.isVerticalZone(root.zoneOf(placement)))
        return Qt.size(placement.size.width + 2 * pad.width, placement.size.height + 2 * pad.height)
    }

    function extentOf(placement, zone) {
        const surface = root.surfaceSizeOf(placement)
        return root.isVerticalZone(zone) ? surface.height : surface.width
    }

    function autoCloses(placement) {
        return placement.autoClose !== PanelRules.AutoClose.Off
    }

    function validate(name, placement) {
        const errors = []
        const warnings = []
        const fail = (field, reason) => errors.push("PanelService: " + name + ".placement." + field + ": " + reason)

        const typeValid = Number.isInteger(placement.type) && root.typeSpecs[placement.type] !== undefined
        if (!typeValid)
            fail("type", placement.type === root.unset ? "required but not set" : "not a PanelRules.Type value")

        const enumFields = [
            ["shape", PanelRules.Shape.Auto, PanelRules.Shape.Tall],
            ["side", PanelRules.Side.None, PanelRules.Side.Right],
            ["exclusionGroup", PanelRules.Group.None, PanelRules.Group.Capture],
            ["autoClose", PanelRules.AutoClose.On, PanelRules.AutoClose.Off]
        ]
        let enumsValid = typeValid
        enumFields.forEach(function (entry) {
            if (!root._isMember(entry[1], entry[2], placement[entry[0]])) {
                fail(entry[0], "not a value of its PanelRules enum")
                enumsValid = false
            }
        })
        if (!enumsValid)
            return { errors: errors, warnings: warnings }

        if (root.specOf(placement.type).zone === "") {
            if (placement.exclusionGroup !== PanelRules.Group.None)
                fail("exclusionGroup", "a floating panel takes no part in exclusion")
            if (placement.side !== PanelRules.Side.None)
                fail("side", "a floating panel has no zone")
            if (placement.shape !== PanelRules.Shape.Auto)
                fail("shape", "a floating panel has no shape")
            return { errors: errors, warnings: warnings }
        }

        if (!root._isMeasurable(placement))
            fail("size", "must be positive on both axes")

        const computed = root.shapeOf(placement)
        if (computed === PanelRules.Shape.Tall && placement.side === PanelRules.Side.None)
            fail("side", "a tall panel needs a side (Left or Right)")
        if (computed !== PanelRules.Shape.Tall && placement.side !== PanelRules.Side.None)
            fail("side", "a side needs a tall shape, this panel resolves wide")

        const clear = root._clearShapeOf(placement)
        if (placement.shape !== PanelRules.Shape.Auto && clear !== PanelRules.Shape.Auto && placement.shape !== clear) {
            const ratio = (placement.size.width / placement.size.height).toFixed(2)
            warnings.push("PanelService: " + name + ".placement.shape: declared "
                + (placement.shape === PanelRules.Shape.Wide ? "Wide" : "Tall") + " contradicts the computed shape "
                + (clear === PanelRules.Shape.Wide ? "Wide" : "Tall") + " (ratio " + ratio + "), computed wins")
        }

        return { errors: errors, warnings: warnings }
    }
}

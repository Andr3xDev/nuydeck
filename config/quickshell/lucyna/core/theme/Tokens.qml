pragma Singleton

import QtQuick
import "./primitives" as Primitives

/*!
    Tokens — Public design-token API (layers 2 + 3).

    Reads the active palette's primitives via ThemeManager.p (layer 1,
    reactive on currentTheme) and the static Scale singleton (layer 1,
    non-color), and exposes the semantic roles consumers should use.

    ThemeManager itself lives in this same directory, so it is visible
    here with no import statement (implicit directory import). Scale
    lives under ./primitives and needs the explicit import above.
*/
QtObject {
    readonly property QtObject color: QtObject {
        readonly property color bg:             ThemeManager.p.neutral.bg
        readonly property color surface:        ThemeManager.p.neutral.surface
        readonly property color border:         ThemeManager.p.neutral.border
        readonly property color borderMuted:    ThemeManager.p.neutral.borderMuted
        readonly property color borderStrong:   ThemeManager.p.neutral.borderStrong
        readonly property color textPrimary:    ThemeManager.p.neutral.textPrimary
        readonly property color textSecondary:  ThemeManager.p.neutral.textSecondary
        readonly property color textMuted:      ThemeManager.p.neutral.textMuted

        readonly property color accent:                ThemeManager.p.hue.turquoise.fill
        readonly property color accentSurface:          ThemeManager.p.hue.turquoise.surface
        readonly property color accentSecondary:        ThemeManager.p.hue.aquamarine.fill
        readonly property color accentSecondarySurface: ThemeManager.p.hue.aquamarine.surface
        readonly property color info:                   ThemeManager.p.hue.blue.fill
        readonly property color infoSurface:            ThemeManager.p.hue.blue.surface
        readonly property color onAccent:               ThemeManager.p.neutral.onAccent

        readonly property color danger:         ThemeManager.p.hue.red.fill
        readonly property color dangerSurface:  ThemeManager.p.hue.red.surface
        readonly property color success:        ThemeManager.p.hue.green.fill
        readonly property color successSurface: ThemeManager.p.hue.green.surface
        readonly property color warning:        ThemeManager.p.hue.orange.fill
        readonly property color warningSurface: ThemeManager.p.hue.orange.surface

        readonly property color yellow:        ThemeManager.p.hue.yellow.fill
        readonly property color yellowSurface: ThemeManager.p.hue.yellow.surface
        readonly property color purple:        ThemeManager.p.hue.purple.fill
        readonly property color purpleSurface: ThemeManager.p.hue.purple.surface
        readonly property color pink:          ThemeManager.p.hue.pink.fill
        readonly property color pinkSurface:   ThemeManager.p.hue.pink.surface

        // Reference-only aliases (doc: "hue names are aliases now"). Same
        // values as their role counterpart, kept for non-role-specific
        // consumers (per-metric icon color coding, palette/wallpaper talk).
        readonly property color turquoise:         accent
        readonly property color turquoiseSurface:  accentSurface
        readonly property color aquamarine:        accentSecondary
        readonly property color aquamarineSurface: accentSecondarySurface
        readonly property color blue:        info
        readonly property color blueSurface: infoSurface

        // Transparency exception: the scrim dims content that isn't ours
        // (wallpaper/windows behind launchers/panels), which no opaque
        // palette color can achieve. Never permitted elsewhere.
        // QML parses hex as #AARRGGBB, not CSS's #RRGGBBAA — this is
        // #151515 at 40% alpha (the design doc writes it #15151566 in CSS order).
        readonly property color scrim: "#77151515"
    }

    // Direct passthrough to Scale — these scale names are already role names,
    // so a 1:1 semantic alias would be pure indirection. A semantic alias gets
    // added only when a role diverges from the scale.
    readonly property QtObject space:  Primitives.Scale.space
    readonly property QtObject radius: Primitives.Scale.radius
    readonly property QtObject text:   Primitives.Scale.text
    readonly property QtObject font:   Primitives.Scale.font
    readonly property QtObject border: Primitives.Scale.border

    readonly property QtObject motion: QtObject {
        // Bezier curve primitives — passthrough of MotionCurves.qml,
        // never read directly by a consumer (see role below).
        readonly property QtObject curve: QtObject {
            readonly property QtObject expressiveFastSpatial: Primitives.MotionCurves.expressiveFastSpatial
            readonly property QtObject expressiveDefaultSpatial: Primitives.MotionCurves.expressiveDefaultSpatial
            readonly property QtObject expressiveSlowSpatial: Primitives.MotionCurves.expressiveSlowSpatial
            readonly property QtObject expressiveEffects: Primitives.MotionCurves.expressiveEffects
            readonly property QtObject emphasized: Primitives.MotionCurves.emphasized
            readonly property QtObject standard: Primitives.MotionCurves.standard
            readonly property QtObject standardDecel: Primitives.MotionCurves.standardDecel
            readonly property QtObject standardAccel: Primitives.MotionCurves.standardAccel
            readonly property QtObject fastOutSlowIn: Primitives.MotionCurves.fastOutSlowIn
        }

        // Semantic roles: every Behavior reads exactly one role entry, never curve.* directly.
        readonly property QtObject role: QtObject {
            readonly property QtObject panelEnter: QtObject {
                readonly property var curve: Primitives.MotionCurves.fastOutSlowIn.points
                readonly property int duration: Primitives.MotionCurves.fastOutSlowIn.duration
            }
            readonly property QtObject panelExit: QtObject {
                readonly property var curve: Primitives.MotionCurves.fastOutSlowIn.points
                readonly property int duration: Primitives.MotionCurves.fastOutSlowIn.duration
            }
            readonly property QtObject effectsEnter: QtObject {
                readonly property var curve: Primitives.MotionCurves.expressiveEffects.default_.points
                readonly property int duration: 200
            }
            readonly property QtObject effectsExit: QtObject {
                readonly property var curve: Primitives.MotionCurves.expressiveEffects.fast.points
                readonly property int duration: 150
            }
            readonly property QtObject toggleSpatial: QtObject {
                readonly property var curve: Primitives.MotionCurves.expressiveFastSpatial.points
                readonly property int duration: 350
            }

            // Micro-interaction roles for hover, toggle and state-color changes; they share
            // the `standard` curve. stateSettle is the slow settle for transient accents
            // (e.g. workspace focus).
            readonly property QtObject stateFast: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 100
            }
            readonly property QtObject stateMedium: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 150
            }
            readonly property QtObject stateStandard: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 200
            }
            readonly property QtObject stateSlow: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 300
            }
            readonly property QtObject stateSettle: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 350
            }

            // Critical-battery blink: infinite alternating color pulse, one leg per duration.
            readonly property QtObject alertPulse: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 800
            }

            // Text-cursor blink — one fade leg (out, then in).
            readonly property QtObject cursorBlink: QtObject {
                readonly property var curve: Primitives.MotionCurves.standard.points
                readonly property int duration: 350
            }
        }
    }

    // Layer 3 — component-local constants promoted here only once a SECOND
    // file reads the same value; otherwise it stays local to its component.
    // This is the documented exception, never the default.
    readonly property QtObject comp: QtObject {
        // Read by both BarContent.qml (bar wrapper's own height) and
        // QuickshellExclusion.qml (its mirrored exclusiveZone) — promoted
        // here per this file's own "second file reads it" rule.
        readonly property QtObject bar: QtObject {
            readonly property real height: 30
            // Where the bar's tapered edges meet its body, and the content margin that mirrors it.
            readonly property real bodyInset: 160
        }
        readonly property QtObject clip: QtObject {
            readonly property int rowH:    48
            readonly property int rowTallH: 64
            readonly property int pad:     12
            readonly property int contentW:     624
            readonly property int sidebarW:     56
            readonly property int previewMaxH:  150
            readonly property int previewMinLines: 2
            readonly property int listRows:     5
            readonly property real selectedScale: 1.1
            readonly property color selection:       color.accent
            // Cursor stays solid this long after typing or moving, before blinking resumes.
            readonly property int cursorIdleDelay: 600
        }
        // Read by ActionTile and ActionTiles; TilePanel derives its content width from them.
        readonly property QtObject actionTile: QtObject {
            readonly property int side: space.md * 2 + text.iconXl + space.sm + text.compactSm
            readonly property int gap: space.lg + border.sm
            // Each side of the divider between tile groups.
            readonly property int groupGap: space.xxl
        }
        // Read by TilePanel (header plus one row of action tiles).
        readonly property QtObject tilePanel: QtObject {
            // Extra content pad on each side, on top of the shell gutter.
            readonly property real paddingX: space.lg
            readonly property real paddingY: space.sm
            // Measured from the header's text-dependent layout; ZoneSlot warns when it drifts.
            readonly property int contentHeight: 110
        }
        readonly property QtObject searchField: QtObject {
            readonly property int height: 44
            readonly property int width: 200
            // Icon's left margin; the text/slots start after it at textInset.
            readonly property int iconInset: 10
            readonly property int textInset: 34
            readonly property int verticalPadding: 10
            readonly property real letterSpacing: 0.3
        }
        readonly property QtObject panelSurface: QtObject {
            readonly property real strokeWidth: 2
            // Half of strokeWidth: the far-edge stroke is centered on its path, so half falls outside
            // the content bounds unless the path is pulled inward by this amount.
            readonly property real depthInset: strokeWidth / 2

            // Plain content gutter on every side; PanelRules.shellPad adds bodyInset on the axis along the attached edge.
            readonly property real contentGutter: space.lg
            readonly property real bodyInset: space.xxl
            // Per-side content pad on the axis along the attached edge.
            readonly property real alongEdgePad: bodyInset + contentGutter

            // Staged motion over one eased `progress`: the base strip takes the first stageBaseShare of it;
            // the body starts earlier (stageBodyStart) so the two stages overlap.
            readonly property real stageBaseShare: 0.4
            readonly property real stageBodyStart: 0.25

            // The bar and its panels share one layer-shell window, but the
            // centered stroke leaves its outer half outside the surface bounds
            // and fractional scaling can round the two edges differently. The
            // panel must cover the bar's full stroke plus slack against that.
            // Invisible: both surfaces paint color.bg. Raising joinRoundingSlack
            // is the only knob if a hairline reappears.
            readonly property real joinRoundingSlack: 2
            readonly property real joinOverlap: strokeWidth + joinRoundingSlack
        }
    }

    // Hover rule: use color.surface for navigation/list-row hover states;
    // use color.accentSurface for accent-owned affordances (delete, confirm,
    // select) where the hover should read as "this action matters".

    // Shared severity->color decision. Callers keep their own thresholds;
    // only the color mapping is centralized here.
    function severity(level) {
        if (level === "critical") return color.danger
        if (level === "warn")     return color.warning
        return color.accent
    }
}

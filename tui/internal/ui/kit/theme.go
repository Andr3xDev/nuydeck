// Package kit is the UI framework shared by the shell and the screens: theme, key bindings, boxes, lists, tables, popups and scrolling.
// It knows nothing about the shell or the domain packages.
package kit

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

const variantZenit = "zenit"

// palette is the subset of Nuytek tokens the TUI uses.
type palette struct {
	dark          bool
	border        color.Color
	textMuted     color.Color
	textSecondary color.Color
	textPrimary   color.Color
	onAccent      color.Color
	accent        color.Color
	warning       color.Color
	failure       color.Color
	success       color.Color
}

// Tokens copied from ~/.config/quickshell/lucyna/core/theme/primitives/nuytek/Nadir.qml:15-35.
var nadir = palette{
	dark:          true,
	border:        lipgloss.Color("#323232"),
	textMuted:     lipgloss.Color("#72837D"),
	textSecondary: lipgloss.Color("#A7B4B0"),
	textPrimary:   lipgloss.Color("#E9EDEB"),
	onAccent:      lipgloss.Color("#151515"),
	accent:        lipgloss.Color("#5AE2DD"),
	warning:       lipgloss.Color("#F39049"),
	failure:       lipgloss.Color("#EF616D"),
	success:       lipgloss.Color("#84EB8D"),
}

// Tokens copied from ~/.config/quickshell/lucyna/core/theme/primitives/nuytek/Zenit.qml:15-35.
var zenit = palette{
	dark:          false,
	border:        lipgloss.Color("#DADCDC"),
	textMuted:     lipgloss.Color("#5E6D67"),
	textSecondary: lipgloss.Color("#4B5A54"),
	textPrimary:   lipgloss.Color("#16211E"),
	onAccent:      lipgloss.Color("#F7F8F8"),
	accent:        lipgloss.Color("#0B726D"),
	warning:       lipgloss.Color("#B85F1D"),
	failure:       lipgloss.Color("#C23B46"),
	success:       lipgloss.Color("#1E8A3B"),
}

// Theme is the styles every screen renders with; its zero value renders unstyled.
type Theme struct {
	Dark bool

	Plain, Secondary, Muted, Title lipgloss.Style
	Accent, OnAccent, Selected     lipgloss.Style
	Section                        lipgloss.Style
	Border, BorderFocus            lipgloss.Style
	OK, Warning, Err, Badge        lipgloss.Style

	// PanelHasFocus reports whether the content panel holds focus; nil means always, as for popups and a zero theme.
	// Theme values must not be compared with ==: a func field makes that a compile error.
	PanelHasFocus func() bool
}

// PanelFocused reports whether the panel has focus; a theme without PanelHasFocus is always focused.
func (t Theme) PanelFocused() bool { return t.PanelHasFocus == nil || t.PanelHasFocus() }

func newTheme(p palette) Theme {
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return Theme{
		Dark:        p.dark,
		Plain:       fg(p.textPrimary),
		Secondary:   fg(p.textSecondary),
		Muted:       fg(p.textMuted),
		Title:       lipgloss.NewStyle().Bold(true),
		Accent:      fg(p.accent),
		OnAccent:    fg(p.onAccent).Background(p.accent),
		Selected:    fg(p.onAccent).Background(p.accent).Bold(true),
		Section:     fg(p.accent).Bold(true),
		Border:      fg(p.border),
		BorderFocus: fg(p.accent),
		OK:          fg(p.success),
		Warning:     fg(p.warning),
		Err:         fg(p.failure),
		Badge:       fg(p.onAccent).Background(p.warning).Bold(true).Padding(0, 1),
	}
}

// BorderFor is the border style for a focused or unfocused box.
func (t Theme) BorderFor(focused bool) lipgloss.Style {
	if focused {
		return t.BorderFocus
	}
	return t.Border
}

// ForVariant returns Zenit for "zenit" and Nadir for anything else, so a
// missing or invalid variant never surfaces as an error.
func ForVariant(variant string) Theme {
	if variant == variantZenit {
		return newTheme(zenit)
	}
	return newTheme(nadir)
}

package ui

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

// theme is the styles every screen renders with; its zero value renders unstyled.
type theme struct {
	Dark bool

	plain, secondary, muted, title lipgloss.Style
	accent, onAccent, selected     lipgloss.Style
	section                        lipgloss.Style
	border, borderFocus            lipgloss.Style
	ok, warning, err, badge        lipgloss.Style

	// panelHasFocus reports whether the content panel holds focus; nil means always, as for popups and a zero theme.
	// theme values must not be compared with ==: a func field makes that a compile error.
	panelHasFocus func() bool
}

func (t theme) panelFocused() bool { return t.panelHasFocus == nil || t.panelHasFocus() }

func newTheme(p palette) theme {
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return theme{
		Dark:        p.dark,
		plain:       fg(p.textPrimary),
		secondary:   fg(p.textSecondary),
		muted:       fg(p.textMuted),
		title:       lipgloss.NewStyle().Bold(true),
		accent:      fg(p.accent),
		onAccent:    fg(p.onAccent).Background(p.accent),
		selected:    fg(p.onAccent).Background(p.accent).Bold(true),
		section:     fg(p.accent).Bold(true),
		border:      fg(p.border),
		borderFocus: fg(p.accent),
		ok:          fg(p.success),
		warning:     fg(p.warning),
		err:         fg(p.failure),
		badge:       fg(p.onAccent).Background(p.warning).Bold(true).Padding(0, 1),
	}
}

func (t theme) borderFor(focused bool) lipgloss.Style {
	if focused {
		return t.borderFocus
	}
	return t.border
}

// ForVariant returns Zenit for "zenit" and Nadir for anything else, so a
// missing or invalid variant never surfaces as an error.
func ForVariant(variant string) theme {
	if variant == variantZenit {
		return newTheme(zenit)
	}
	return newTheme(nadir)
}

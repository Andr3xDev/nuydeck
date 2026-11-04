// Package theme holds the Nuytek terminal palettes.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

const variantZenit = "zenit"

// Palette is the subset of Nuytek tokens the TUI uses.
type Palette struct {
	Dark          bool
	Bg            color.Color
	Surface       color.Color
	Border        color.Color
	TextMuted     color.Color
	TextSecondary color.Color
	TextPrimary   color.Color
	OnAccent      color.Color
	Accent        color.Color
	Warning       color.Color
	Error         color.Color
	Success       color.Color
}

// Tokens copied from ~/.config/quickshell/lucyna/core/theme/primitives/nuytek/Nadir.qml:15-35.
var nadir = Palette{
	Dark:          true,
	Bg:            lipgloss.Color("#151515"),
	Surface:       lipgloss.Color("#232323"),
	Border:        lipgloss.Color("#323232"),
	TextMuted:     lipgloss.Color("#72837D"),
	TextSecondary: lipgloss.Color("#A7B4B0"),
	TextPrimary:   lipgloss.Color("#E9EDEB"),
	OnAccent:      lipgloss.Color("#151515"),
	Accent:        lipgloss.Color("#5AE2DD"),
	Warning:       lipgloss.Color("#F39049"),
	Error:         lipgloss.Color("#EF616D"),
	Success:       lipgloss.Color("#84EB8D"),
}

// Tokens copied from ~/.config/quickshell/lucyna/core/theme/primitives/nuytek/Zenit.qml:15-35.
var zenit = Palette{
	Dark:          false,
	Bg:            lipgloss.Color("#F3F4F4"),
	Surface:       lipgloss.Color("#FAFAFA"),
	Border:        lipgloss.Color("#DADCDC"),
	TextMuted:     lipgloss.Color("#5E6D67"),
	TextSecondary: lipgloss.Color("#4B5A54"),
	TextPrimary:   lipgloss.Color("#16211E"),
	OnAccent:      lipgloss.Color("#F7F8F8"),
	Accent:        lipgloss.Color("#0B726D"),
	Warning:       lipgloss.Color("#B85F1D"),
	Error:         lipgloss.Color("#C23B46"),
	Success:       lipgloss.Color("#1E8A3B"),
}

// ForVariant returns Zenit for "zenit" and Nadir for anything else, so a
// missing or invalid variant never surfaces as an error.
func ForVariant(variant string) Palette {
	if variant == variantZenit {
		return zenit
	}
	return nadir
}

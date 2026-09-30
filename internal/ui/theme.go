package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette, shared with assets/logo.svg.
var (
	colorNight   = color.NRGBA{R: 0x14, G: 0x1A, B: 0x2E, A: 0xFF}
	colorSurface = color.NRGBA{R: 0x1E, G: 0x26, B: 0x40, A: 0xFF}
	colorRaised  = color.NRGBA{R: 0x2A, G: 0x33, B: 0x52, A: 0xFF}
	colorAmber   = color.NRGBA{R: 0xE8, G: 0xA2, B: 0x3E, A: 0xFF}
	colorMuted   = color.NRGBA{R: 0x8C, G: 0x96, B: 0xB4, A: 0xFF}
)

// pangolinTheme is the default Fyne theme, always dark, with the logo's colours.
type pangolinTheme struct {
	fyne.Theme
}

func newTheme() fyne.Theme {
	return pangolinTheme{Theme: theme.DefaultTheme()}
}

func (t pangolinTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorNight
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return colorSurface
	case theme.ColorNameButton:
		return colorRaised
	case theme.ColorNamePrimary:
		return colorAmber
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		// Also the colour of secondary text, so it has to stay readable.
		return colorMuted
	case theme.ColorNameForegroundOnPrimary:
		return colorNight
	case theme.ColorNameFocus, theme.ColorNameSelection:
		return color.NRGBA{R: colorAmber.R, G: colorAmber.G, B: colorAmber.B, A: 0x55}
	}
	return t.Theme.Color(name, theme.VariantDark)
}

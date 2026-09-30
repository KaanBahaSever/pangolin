package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Theme modes, as stored in the preferences.
const (
	themeSystem = "system"
	themeLight  = "light"
	themeDark   = "dark"
)

// palette holds the colours that differ from the default Fyne theme. Both
// palettes are built around the amber and night blue of assets/logo.svg.
type palette struct {
	background color.NRGBA
	surface    color.NRGBA // inputs, dialogs, menus
	raised     color.NRGBA // buttons
	muted      color.NRGBA // secondary text, placeholders, disabled
}

var (
	colorNight = color.NRGBA{R: 0x14, G: 0x1A, B: 0x2E, A: 0xFF}
	colorAmber = color.NRGBA{R: 0xE8, G: 0xA2, B: 0x3E, A: 0xFF}

	darkPalette = palette{
		background: colorNight,
		surface:    color.NRGBA{R: 0x1E, G: 0x26, B: 0x40, A: 0xFF},
		raised:     color.NRGBA{R: 0x2A, G: 0x33, B: 0x52, A: 0xFF},
		muted:      color.NRGBA{R: 0x8C, G: 0x96, B: 0xB4, A: 0xFF},
	}
	lightPalette = palette{
		background: color.NRGBA{R: 0xFB, G: 0xF6, B: 0xEC, A: 0xFF},
		surface:    color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		raised:     color.NRGBA{R: 0xEE, G: 0xE4, B: 0xD0, A: 0xFF},
		muted:      color.NRGBA{R: 0x5F, G: 0x68, B: 0x84, A: 0xFF},
	}
)

// pangolinTheme is the default Fyne theme in the logo's colours. It follows
// the system's light or dark setting unless mode forces one of them.
type pangolinTheme struct {
	fyne.Theme
	mode string
}

func newTheme(mode string) fyne.Theme {
	return pangolinTheme{Theme: theme.DefaultTheme(), mode: mode}
}

func (t pangolinTheme) variant(system fyne.ThemeVariant) fyne.ThemeVariant {
	switch t.mode {
	case themeLight:
		return theme.VariantLight
	case themeDark:
		return theme.VariantDark
	}
	return system
}

func (t pangolinTheme) Color(name fyne.ThemeColorName, system fyne.ThemeVariant) color.Color {
	variant := t.variant(system)
	p := darkPalette
	if variant == theme.VariantLight {
		p = lightPalette
	}
	switch name {
	case theme.ColorNameBackground:
		return p.background
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return p.surface
	case theme.ColorNameButton:
		return p.raised
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		// Also the colour of secondary text, so it has to stay readable.
		return p.muted
	case theme.ColorNamePrimary:
		return colorAmber
	case theme.ColorNameForegroundOnPrimary:
		return colorNight
	case theme.ColorNameFocus, theme.ColorNameSelection:
		return color.NRGBA{R: colorAmber.R, G: colorAmber.G, B: colorAmber.B, A: 0x55}
	}
	return t.Theme.Color(name, variant)
}

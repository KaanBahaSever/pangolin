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

// Colours Pangolin adds to the Fyne theme.
const (
	colorNameSidebar    fyne.ThemeColorName = "pangolinSidebar"
	colorNameCard       fyne.ThemeColorName = "pangolinCard"
	colorNameCardBorder fyne.ThemeColorName = "pangolinCardBorder"
)

// palette holds the colours that differ between light and dark. Both are
// built around the amber and night blue of assets/logo.svg.
type palette struct {
	background color.NRGBA
	sidebar    color.NRGBA
	card       color.NRGBA // cards, dialogs, menus
	border     color.NRGBA // card outlines, separators
	input      color.NRGBA
	raised     color.NRGBA // buttons
	muted      color.NRGBA // secondary text, placeholders, disabled
}

var (
	colorNight = color.NRGBA{R: 0x14, G: 0x1A, B: 0x2E, A: 0xFF}
	colorAmber = color.NRGBA{R: 0xE8, G: 0xA2, B: 0x3E, A: 0xFF}

	darkPalette = palette{
		background: color.NRGBA{R: 0x11, G: 0x16, B: 0x2A, A: 0xFF},
		sidebar:    color.NRGBA{R: 0x16, G: 0x1C, B: 0x33, A: 0xFF},
		card:       color.NRGBA{R: 0x1C, G: 0x23, B: 0x40, A: 0xFF},
		border:     color.NRGBA{R: 0x29, G: 0x31, B: 0x50, A: 0xFF},
		input:      color.NRGBA{R: 0x25, G: 0x2D, B: 0x4C, A: 0xFF},
		raised:     color.NRGBA{R: 0x2A, G: 0x33, B: 0x52, A: 0xFF},
		muted:      color.NRGBA{R: 0x8F, G: 0x99, B: 0xB8, A: 0xFF},
	}
	lightPalette = palette{
		background: color.NRGBA{R: 0xF7, G: 0xF2, B: 0xE8, A: 0xFF},
		sidebar:    color.NRGBA{R: 0xEF, G: 0xE7, B: 0xD7, A: 0xFF},
		card:       color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		border:     color.NRGBA{R: 0xE6, G: 0xDC, B: 0xC9, A: 0xFF},
		input:      color.NRGBA{R: 0xF4, G: 0xEE, B: 0xE2, A: 0xFF},
		raised:     color.NRGBA{R: 0xEC, G: 0xE2, B: 0xCE, A: 0xFF},
		muted:      color.NRGBA{R: 0x69, G: 0x71, B: 0x8E, A: 0xFF},
	}
)

// pangolinTheme is the default Fyne theme in the logo's colours, with softer
// corners. It follows the system's light or dark setting unless mode forces
// one of them.
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
	case colorNameSidebar:
		return p.sidebar
	case colorNameCard, theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return p.card
	case colorNameCardBorder, theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return p.border
	case theme.ColorNameInputBackground:
		return p.input
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
	case theme.ColorNameHover:
		return color.NRGBA{R: colorAmber.R, G: colorAmber.G, B: colorAmber.B, A: 0x22}
	}
	return t.Theme.Color(name, variant)
}

func (t pangolinTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 10
	case theme.SizeNameInnerPadding:
		return 10
	}
	return t.Theme.Size(name)
}

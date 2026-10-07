package ui

import (
	"hash/fnv"
	"image/color"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// lockIcon is Material Design's "lock_outline" (Apache License 2.0), the
// icon set Fyne's own icons come from.
var lockIcon = theme.NewThemedResource(fyne.NewStaticResource("lock.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 17c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm6-9h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zM8.9 6c0-1.71 1.39-3.1 3.1-3.1s3.1 1.39 3.1 3.1v2H8.9V6zM18 20H6V10h12v10z"/></svg>`)))

// surface is a rounded rectangle that takes its colours from the current
// theme, so it follows a switch between light and dark without a rebuild.
type surface struct {
	widget.BaseWidget
	fill, stroke fyne.ThemeColorName // stroke may be empty
	radius       float32
}

func newSurface(fill, stroke fyne.ThemeColorName, radius float32) *surface {
	s := &surface{fill: fill, stroke: stroke, radius: radius}
	s.ExtendBaseWidget(s)
	return s
}

func (s *surface) CreateRenderer() fyne.WidgetRenderer {
	r := &surfaceRenderer{s: s, rect: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type surfaceRenderer struct {
	s    *surface
	rect *canvas.Rectangle
}

func (r *surfaceRenderer) Layout(size fyne.Size)        { r.rect.Resize(size) }
func (r *surfaceRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *surfaceRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *surfaceRenderer) Destroy()                     {}

func (r *surfaceRenderer) Refresh() {
	r.rect.FillColor = theme.Color(r.s.fill)
	r.rect.CornerRadius = r.s.radius
	if r.s.stroke != "" {
		r.rect.StrokeColor = theme.Color(r.s.stroke)
		r.rect.StrokeWidth = 1
	}
	r.rect.Refresh()
}

// card puts content on a rounded panel.
func card(content ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		newSurface(colorNameCard, colorNameCardBorder, 14),
		container.New(layout.NewCustomPaddedLayout(10, 10, 16, 12), container.NewVBox(content...)),
	)
}

// cardRows stacks rows in a card with a thin line between them.
func cardRows(rows ...fyne.CanvasObject) fyne.CanvasObject {
	var objects []fyne.CanvasObject
	for i, row := range rows {
		if i > 0 {
			objects = append(objects, widget.NewSeparator())
		}
		objects = append(objects, row)
	}
	return card(objects...)
}

// scrollPage stacks a page's parts from the top and scrolls when they do not
// fit, so a short page never leaves a gap in the middle. The right margin
// keeps card edges clear of the scroll bar.
func scrollPage(parts ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, 12, 0, 14), container.NewVBox(parts...)))
}

// caption is the small, muted label above a value or a field.
func caption(text string) *widget.Label {
	l := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameCaptionText
	l.Importance = widget.LowImportance
	return l
}

// muted is secondary text.
func muted(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Importance = widget.LowImportance
	return l
}

// heading is the title of a page.
func heading(text string) *widget.Label {
	l := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameSubHeadingText
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// flatButton is an icon button without a background, for actions in a row.
func flatButton(icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tapped)
	b.Importance = widget.LowImportance
	return b
}

// logoImage returns the logo, without its tile, at the given height.
func logoImage(size float32) *canvas.Image {
	img := canvas.NewImageFromResource(mark)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}

// Pastel avatar colours; the letter is always drawn in night blue, which
// reads on all of them in both themes.
var avatarColors = []color.NRGBA{
	{R: 0xF6, G: 0xBD, B: 0x60, A: 0xFF}, // amber
	{R: 0xF7, G: 0xA9, B: 0xA0, A: 0xFF}, // blush
	{R: 0x8F, G: 0xD3, B: 0xB6, A: 0xFF}, // mint
	{R: 0x8E, G: 0xC5, B: 0xF2, A: 0xFF}, // sky
	{R: 0xC3, G: 0xA6, B: 0xF2, A: 0xFF}, // lilac
	{R: 0xFF, G: 0xB3, B: 0x8A, A: 0xFF}, // peach
}

// avatar is a coloured circle with the first letter of title. The colour is
// derived from the title, so an entry keeps its colour.
func avatar(title string, size float32) fyne.CanvasObject {
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(title)))
	circle := canvas.NewRectangle(avatarColors[h.Sum32()%uint32(len(avatarColors))])
	circle.CornerRadius = size / 2
	circle.SetMinSize(fyne.NewSize(size, size))

	letter := "?"
	if r, _ := utf8.DecodeRuneInString(strings.TrimSpace(title)); r != utf8.RuneError {
		letter = strings.ToUpper(string(r))
	}
	text := canvas.NewText(letter, colorNight)
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.TextSize = size * 0.44
	text.Alignment = fyne.TextAlignCenter

	return container.NewCenter(container.NewStack(circle, container.NewCenter(text)))
}

// segmented is a row of mutually exclusive choices, the chosen one
// highlighted.
func segmented(options []string, selected int, changed func(int)) fyne.CanvasObject {
	buttons := make([]*widget.Button, len(options))
	paint := func() {
		for i, b := range buttons {
			if i == selected {
				b.Importance = widget.HighImportance
			} else {
				b.Importance = widget.LowImportance
			}
			b.Refresh()
		}
	}
	row := container.NewHBox()
	for i, option := range options {
		i := i
		buttons[i] = widget.NewButton(option, func() {
			if i == selected {
				return
			}
			selected = i
			paint()
			changed(i)
		})
		row.Add(buttons[i])
	}
	paint()
	return container.NewStack(
		newSurface(theme.ColorNameInputBackground, "", 10),
		container.New(layout.NewCustomPaddedLayout(2, 2, 2, 2), row),
	)
}

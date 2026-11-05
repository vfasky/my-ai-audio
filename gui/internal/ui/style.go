package ui

import "github.com/egoist/mygo/ui"

// Palette follows the dark voice-demo reference: near-black canvas, graphite
// cards, inset black wells, white primary capsules and a single orange accent
// reserved for live audio.
var (
	colCanvas  = ui.Hex("#0A0A0A")
	colSidebar = ui.Hex("#141414")
	colCard    = ui.Hex("#1C1C1C")
	// colHUD is the dictation capsule and its preview.
	colHUD          = ui.Hex("#121212")
	colWell         = ui.Hex("#0B0B0B")
	colPill         = ui.Hex("#262626")
	colPillHover    = ui.Hex("#303030")
	colPillPressed  = ui.Hex("#3A3A3A")
	colHairline     = ui.RGBA(255, 255, 255, 0.07)
	colRule         = ui.Hex("#2A2A2A")
	colText         = ui.Hex("#F5F5F7")
	colSubtle       = ui.Hex("#D1D1D6")
	colMuted        = ui.Hex("#8E8E93")
	colFaint        = ui.Hex("#5A5A5F")
	colOrange       = ui.Hex("#F0672A")
	colOrangeDeep   = ui.Hex("#8C3A18")
	colOrangeSoft   = ui.RGBA(240, 103, 42, 0.12)
	colGreen        = ui.Hex("#30D158")
	colRed          = ui.Hex("#FF453A")
	colRedSoft      = ui.RGBA(255, 69, 58, 0.14)
	colIdleDot      = ui.RGBA(255, 255, 255, 0.26)
	colWhitePressed = ui.Hex("#D4D4D8")
)

// theme is the forced dark theme handed to MyGo's stock widgets so switches,
// fields, segmented controls and the sidebar share the palette.
var theme = func() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = colCanvas
	t.Surface = ui.Hex("#1F1F1F")
	t.SurfaceHover = colPillHover
	t.SurfacePressed = colPillPressed
	t.Border = ui.Hex("#333336")
	t.Text = colText
	t.TextMuted = colMuted
	t.Accent = colOrange
	t.AccentHover = ui.Hex("#FF7A3D")
	t.AccentPressed = ui.Hex("#D45620")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = colRed
	t.Warning = ui.Hex("#FF9F0A")
	t.Success = colGreen
	t.Selection = ui.RGBA(240, 103, 42, 0.35)
	t.Focus = ui.RGBA(240, 103, 42, 0.6)
	t.Inverse = ui.Hex("#2C2C2E")
	t.InverseText = colText
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.22)
	t.Radius = 8
	return t
}()

func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

func solidIcon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor">` + shapes + `</svg>`))
}

var (
	iconWave     = icon(`<path d="M2 7v2M5 4.5v7M8 2.5v11M11 5v6M14 7v2"/>`)
	iconBook     = icon(`<path d="M3 3.5A1.5 1.5 0 0 1 4.5 2H13v10H4.5A1.5 1.5 0 0 0 3 13.5z"/><path d="M3 13.5A1.5 1.5 0 0 0 4.5 15H13v-3"/>`)
	iconSparkle  = icon(`<path d="M7 1.8l1.3 3.6 3.6 1.3-3.6 1.3L7 11.6 5.7 8 2.1 6.7l3.6-1.3z"/><path d="M12.5 10.3l.6 1.5 1.5.6-1.5.6-.6 1.5-.6-1.5-1.5-.6 1.5-.6z"/>`)
	iconChip     = icon(`<rect x="4" y="4" width="8" height="8" rx="1.5"/><path d="M6.5 1.8V4M9.5 1.8V4M6.5 12v2.2M9.5 12v2.2M1.8 6.5H4M1.8 9.5H4M12 6.5h2.2M12 9.5h2.2"/>`)
	iconPlay     = solidIcon(`<path d="M4.5 2.6v10.8L13 8z"/>`)
	iconArrow    = icon(`<path d="M4.5 11.5l7-7M5.5 4.5h6v6"/>`)
	iconClose    = icon(`<path d="M4.5 4.5l7 7M11.5 4.5l-7 7"/>`)
	iconWarn     = icon(`<path d="M8 2.2l6.2 11H1.8z"/><path d="M8 6.6v3.1M8 11.6v.1"/>`)
	iconLock     = icon(`<rect x="3.5" y="7" width="9" height="6.5" rx="1.5"/><path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2"/>`)
	iconBolt     = icon(`<path d="M5 5l6 6-3 2.5v-11L11 5l-6 6"/>`)
	iconPlus     = icon(`<path d="M8 3v10M3 8h10"/>`)
	iconKey      = icon(`<circle cx="5.5" cy="10.5" r="2.5"/><path d="M7.3 8.7L13 3M11 5l1.5 1.5"/>`)
	iconShield   = icon(`<path d="M8 1.8l5.2 2v4.1c0 3.1-2.2 5.4-5.2 6.3-3-0.9-5.2-3.2-5.2-6.3V3.8z"/><path d="M5.8 8l1.5 1.5 3-3"/>`)
	iconCursor   = icon(`<path d="M3.5 2.5l9 4.2-3.9 1.3-1.3 3.9z"/><path d="M8.6 8.6l3.6 3.6"/>`)
	iconCheck    = icon(`<path d="M3.5 8.4l3 3 6-6.8"/>`)
	iconInfo     = icon(`<circle cx="8" cy="8" r="6.2"/><path d="M8 7.2v4M8 4.9v.1"/>`)
	iconPower    = icon(`<path d="M8 2v5.5"/><path d="M4.8 4.2a5 5 0 1 0 6.4 0"/>`)
	iconDownload = icon(`<path d="M8 2.5v8M4.8 7.5L8 10.7l3.2-3.2M3 13.5h10"/>`)
	iconCaption  = icon(`<rect x="1.8" y="3" width="12.4" height="10" rx="2.2"/><path d="M4.5 7h3M9.5 7h2M4.5 10h2M8.5 10h3"/>`)
	iconChevron  = icon(`<path d="M4.5 6.2L8 9.8l3.5-3.6"/>`)
	iconChevronR = icon(`<path d="M6.2 4.5L9.8 8l-3.6 3.5"/>`)
	iconSliders  = icon(`<path d="M2.5 4.5h1.2M7.3 4.5h6.2M2.5 11.5h6.2M12.3 11.5h1.2"/><circle cx="5.5" cy="4.5" r="1.8"/><circle cx="10.5" cy="11.5" r="1.8"/>`)
)

type pillKind int

const (
	pillSecondary pillKind = iota
	pillPrimary
	pillGhost
	pillDanger
)

// pill is the capsule button of the reference: white primary, graphite
// secondary, an optional leading glyph and trailing arrow.
func pill(c *ui.Context, key, label string, kind pillKind, lead, trail *ui.SVG, disabled bool) bool {
	return capsule(c, key, label, kind, lead, trail, disabled, false)
}

// smallPill is the compact capsule used inside list rows.
func smallPill(c *ui.Context, key, label string, kind pillKind, lead *ui.SVG, disabled bool) bool {
	return capsule(c, key, label, kind, lead, nil, disabled, true)
}

func capsule(c *ui.Context, key, label string, kind pillKind, lead, trail *ui.SVG, disabled, small bool) bool {
	height, padX, size, gap := float32(40), float32(20), float32(14), float32(10)
	if small {
		height, padX, size, gap = 28, 12, 12.5, 6
	}
	b := ui.ButtonBase(c.Key(key)).Height(height).PaddingX(padX).Gap(gap).Radius(height / 2).
		Shrink(0).Justify(ui.Center).Label(label).Disabled(disabled)
	hover, pressed := b.Hovered() && !disabled, b.Pressed() && !disabled
	fg := colText
	switch kind {
	case pillPrimary:
		fg = colCanvas
		b.Background(pick(pressed, colWhitePressed, pick(hover, ui.Hex("#EBEBEF"), ui.Hex("#FFFFFF"))))
	case pillSecondary:
		b.Background(pick(pressed, colPillPressed, pick(hover, colPillHover, colPill)))
	case pillGhost:
		fg = pick(hover, colText, colMuted)
		b.Background(pick(pressed, colPillPressed, pick(hover, colPill, ui.Transparent)))
	case pillDanger:
		fg = ui.Hex("#FF6961")
		b.Background(pick(pressed, ui.RGBA(255, 69, 58, 0.26), pick(hover, ui.RGBA(255, 69, 58, 0.2), colRedSoft)))
	}
	if disabled {
		b.Opacity(0.38)
	}
	b.TextColor(fg).Children(func() {
		if lead != nil {
			ui.Icon(c, lead).Size(size*0.85, size*0.85)
		}
		ui.Text(c, label).FontSize(size).FontWeight(500).SingleLine()
		if trail != nil {
			ui.Icon(c, trail).Size(size, size)
		}
	})
	return b.Clicked() && !disabled
}

func pick(cond bool, a, b ui.Color) ui.Color {
	if cond {
		return a
	}
	return b
}

// group is a macOS 26 grouped-inset section: a quiet header, then rows on a
// rounded graphite surface separated by hairlines.
func group(c *ui.Context, key, title string, accessory func(), rows func()) {
	ui.Column(c.Key(key)).Gap(8).Children(func() {
		if title != "" || accessory != nil {
			ui.Row(c).PaddingX(4).Gap(8).MinHeight(24).Children(func() {
				ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(colSubtle).Grow(1)
				if accessory != nil {
					accessory()
				}
			})
		}
		ui.Column(c).Background(colCard).Radius(16).Border(1, colHairline).Clip().
			Dividers(1, colHairline).Children(rows)
	})
}

// row is one line of a group.
func row(c *ui.Context, key string, fn func()) ui.Element {
	return ui.Row(c.Key(key)).Padding(12, 16).Gap(12).MinHeight(48).Children(fn)
}

// notice is an inline callout tinted by its severity.
func notice(c *ui.Context, key, text string, warn bool) {
	fg, bg := colMuted, colCard
	if warn {
		fg, bg = ui.Hex("#FF9A62"), colOrangeSoft
	}
	ui.Row(c.Key(key)).Padding(12, 14).Gap(10).Radius(12).Background(bg).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, pickIcon(warn, iconWarn, iconInfo)).Size(15, 15).TextColor(fg).Margin(1, 0, 0, 0)
		ui.Text(c, text).FontSize(12.5).LineHeight(1.45).TextColor(pick(warn, ui.Hex("#FFC4A3"), colMuted)).Grow(1)
	})
}

// statusDot is a small filled circle.
func statusDot(c *ui.Context, color ui.Color) {
	ui.Box(c).Size(7, 7).Radius(4).Background(color).Shrink(0)
}

func pickIcon(cond bool, a, b *ui.SVG) *ui.SVG {
	if cond {
		return a
	}
	return b
}

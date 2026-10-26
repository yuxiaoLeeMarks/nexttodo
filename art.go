package main

import (
	"bytes"
	"image/png"

	"github.com/egoist/mygo/ui"
)

// drawAppIcon draws the app's icon filling r: an indigo square of macOS's
// rounded shape holding a white page of a calendar with a check on it.
func drawAppIcon(p *ui.Painter, r ui.Rect) {
	s := min(r.W, r.H)
	x, y := r.X+(r.W-s)/2, r.Y+(r.H-s)/2
	pt := func(fx, fy float32) (float32, float32) { return x + fx*s, y + fy*s }
	p.FillGradient(ui.Rect{X: x, Y: y, W: s, H: s},
		ui.LinearGradient{From: ui.Hex("#6d66f6"), To: ui.Hex("#3f35c9"), Angle: 180, Oklab: true}, s*0.225)
	// The page, with its shadow and the band along its top.
	page := ui.Rect{X: x + 0.2*s, Y: y + 0.24*s, W: 0.6 * s, H: 0.56 * s}
	p.Shadow(page, s*0.09, 0, s*0.02, s*0.05, 0, ui.RGBA(20, 10, 90, 0.35))
	p.Fill(page, ui.Hex("#ffffff"), s*0.09)
	p.Clip(page, s*0.09, func() {
		p.Fill(ui.Rect{X: page.X, Y: page.Y, W: page.W, H: 0.13 * s}, ui.Hex("#dcdcfd"), 0)
	})
	for _, fx := range []float32{0.36, 0.64} {
		cx, cy := pt(fx, 0.24)
		p.Fill(ui.Rect{X: cx - 0.022*s, Y: cy - 0.06*s, W: 0.044 * s, H: 0.11 * s}, ui.Hex("#2f27a8"), 0.022*s)
	}
	x0, y0 := pt(0.36, 0.585)
	x1, y1 := pt(0.46, 0.68)
	x2, y2 := pt(0.645, 0.48)
	check := (&ui.Path{}).MoveTo(x0, y0).LineTo(x1, y1).LineTo(x2, y2)
	p.StrokePath(check, 0.058*s, ui.Hex("#4f46e5"))
}

// trayIcon is the menu bar's icon: the app's mark, black on clear for
// macOS to tint, 18 points at twice the pixels.
func trayIcon() []byte {
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Box(c).Fill().Center().Children(func() {
			ui.Icon(c, iconLogo).Size(16, 16).TextColor(ui.RGB(0, 0, 0))
		})
	}, 18, 18, 2)
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

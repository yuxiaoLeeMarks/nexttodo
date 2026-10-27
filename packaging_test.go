package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The window of the disk image, and where packaging/dmg/settings.py puts
// its icons, which the background is drawn around.
const (
	dmgW, dmgH         = 660, 480
	dmgRowY            = 165 // the app's and Applications' centers
	dmgAppX, dmgAppsX  = 170, 490
	dmgCardY           = 292      // the card of what to do if macOS blocks the app
	dmgLinkX, dmgLinkY = 560, 368 // Privacy & Security, in the card
)

// TestDMGBackground draws the background of the disk image into
// packaging/dmg when NT_DMG is set, at one and two pixels a point: how to
// install the app, and what to do when macOS keeps it from opening, in
// English and Chinese.
func TestDMGBackground(t *testing.T) {
	if os.Getenv("NT_DMG") == "" {
		t.Skip("NT_DMG is not set")
	}
	p := lightPalette()
	view := func(c *ui.Context) {
		c.Root().Background(p.bg)
		ui.Box(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.FillGradient(r, ui.LinearGradient{From: ui.Hex("#f7f7f9"), To: ui.Hex("#ececf0"), Angle: 180}, 0)
			// An arrow from the app to Applications.
			y := float32(dmgRowY)
			x0, x1 := float32(dmgAppX+78), float32(dmgAppsX-78)
			pt.Line(x0, y, x1-2, y, 2, p.faint)
			head := (&ui.Path{}).MoveTo(x1-10, y-7).LineTo(x1, y).LineTo(x1-10, y+7)
			pt.StrokePath(head, 2, p.faint)
		})
		col := ui.Column(c).Absolute().Fill().AlignItems(ui.Center)
		col.Children(func() {
			ui.Column(c).Margin(26, 0, 0, 0).AlignItems(ui.Center).Gap(3).Children(func() {
				ui.Text(c, "Install NextTodo · 安装 NextTodo").FontSize(16).FontWeight(600).TextColor(p.fg)
				ui.Text(c, "Your research, planned. · 科研，有条不紊。").FontSize(12).TextColor(p.muted)
			})
			ui.Row(c).Absolute().Top(dmgRowY + 92).Left(0).Right(0).Justify(ui.Center).Gap(8).AlignItems(ui.Center).Children(func() {
				step(c, p, "1")
				ui.Text(c, "Drag NextTodo to Applications · 将 NextTodo 拖到「应用程序」").FontSize(12.5).FontWeight(500).TextColor(p.fg2)
			})
			card := ui.Column(c).Absolute().Top(dmgCardY).Left(28).Right(28).Bottom(18).Radius(14).
				Background(ui.RGBA(255, 255, 255, 0.82)).Border(1, p.line).Padding(18, 20).Gap(7)
			card.Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					step(c, p, "2")
					ui.Text(c, "macOS says it cannot open NextTodo? · 首次打开被 macOS 拦截？").FontSize(13).FontWeight(600).TextColor(p.fg)
				})
				line := func(s string) {
					ui.Text(c, s).FontSize(11.5).LineHeight(1.45).TextColor(p.fg2).MaxWidth(dmgLinkX - 28 - 20 - 70)
				}
				line("Click Done, then double-click Privacy & Security at right. Scroll down to Security and, beside NextTodo, click Open Anyway.")
				line("点「完成」，再双击右侧的 Privacy & Security（隐私与安全性），下滑到「安全性」，在 NextTodo 旁点「仍要打开」。")
				ui.Text(c, "Only once, as NextTodo is not yet notarized by Apple. · 只需一次：NextTodo 尚未经过 Apple 公证。").
					FontSize(10.5).TextColor(p.muted).Margin(2, 0, 0, 0).MaxWidth(dmgLinkX - 28 - 20 - 70)
			})
		})
	}
	for _, scale := range []float32{1, 2} {
		img := ui.Render(view, dmgW, dmgH, scale)
		name := "packaging/dmg/background.png"
		if scale == 2 {
			name = "packaging/dmg/background@2x.png"
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

// step is the number of a step, on an indigo disc.
func step(c *ui.Context, p palette, n string) {
	ui.Box(c).Size(20, 20).Radius(10).Background(p.accent).Center().Shrink(0).Children(func() {
		ui.Text(c, n).FontSize(11.5).FontWeight(700).TextColor(p.accentFg)
	})
}

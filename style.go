package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// palette is the quiet surface of magpie (github.com/yetone/magpie): a soft
// gray window, white cards with hairline edges, ink text in three grays,
// and one indigo accent used sparingly. The series colors tell kinds of
// things apart, as magpie's charts do.
type palette struct {
	dark                  bool
	bg, card, card2       ui.Color
	pill, pillHover       ui.Color
	segTrack, segThumb    ui.Color
	line, line2           ui.Color
	fg, fg2, muted, faint ui.Color
	ctl                   ui.Color // a control's quiet text
	accent, accentSoft    ui.Color
	accentFg, sel         ui.Color
	green, greenSoft      ui.Color
	red, redSoft          ui.Color
	amber, amberSoft      ui.Color
	series                [7]ui.Color
	shadow                ui.Color
}

func lightPalette() palette {
	return palette{
		bg: ui.Hex("#f4f4f6"), card: ui.Hex("#ffffff"), card2: ui.Hex("#f9f9fb"),
		pill: ui.Hex("#f1f1f4"), pillHover: ui.Hex("#e9e9ee"),
		segTrack: ui.Hex("#e9e9ed"), segThumb: ui.Hex("#ffffff"),
		line: ui.Hex("#e3e3e8"), line2: ui.Hex("#ececf0"),
		fg: ui.Hex("#1c1c21"), fg2: ui.Hex("#4b4b53"), muted: ui.Hex("#85858d"), faint: ui.Hex("#b9b9c2"),
		ctl:    ui.Hex("#63636b"),
		accent: ui.Hex("#4f46e5"), accentSoft: ui.Hex("#eeedfd"), accentFg: ui.Hex("#ffffff"), sel: ui.Hex("#ecebfb"),
		green: ui.Hex("#189a58"), greenSoft: ui.Hex("#e4f6ec"),
		red: ui.Hex("#d64545"), redSoft: ui.Hex("#fbe9e9"),
		amber: ui.Hex("#b45309"), amberSoft: ui.Hex("#fbf0df"),
		series: [7]ui.Color{ui.Hex("#4f46e5"), ui.Hex("#0ea5e9"), ui.Hex("#10b981"), ui.Hex("#f59e0b"),
			ui.Hex("#ec4899"), ui.Hex("#8b5cf6"), ui.Hex("#64748b")},
		shadow: ui.RGBA(20, 22, 30, 0.16),
	}
}

func darkPalette() palette {
	bg := ui.Hex("#1a1a1e")
	return palette{
		dark: true,
		bg:   bg, card: ui.Hex("#232327"), card2: ui.Hex("#1f1f23"),
		pill: ui.Hex("#2d2d33"), pillHover: ui.Hex("#36363d"),
		segTrack: ui.RGBA(255, 255, 255, 0.05).Over(bg), segThumb: ui.RGBA(255, 255, 255, 0.15).Over(bg),
		line: ui.Hex("#333339"), line2: ui.Hex("#2c2c32"),
		fg: ui.Hex("#ededf1"), fg2: ui.Hex("#b9b9c1"), muted: ui.Hex("#8b8b94"), faint: ui.Hex("#56565f"),
		ctl:    ui.Hex("#a1a1aa"),
		accent: ui.Hex("#a5b4fc"), accentSoft: ui.Hex("#2b2c47"), accentFg: ui.Hex("#14142a"), sel: ui.Hex("#33345a"),
		green: ui.Hex("#6fd99b"), greenSoft: ui.Hex("#1e3a2b"),
		red: ui.Hex("#f28b82"), redSoft: ui.Hex("#45272a"),
		amber: ui.Hex("#f2b544"), amberSoft: ui.Hex("#3f3220"),
		series: [7]ui.Color{ui.Hex("#a5b4fc"), ui.Hex("#7dd3fc"), ui.Hex("#6ee7b7"), ui.Hex("#fcd34d"),
			ui.Hex("#f9a8d4"), ui.Hex("#c4b5fd"), ui.Hex("#94a3b8")},
		shadow: ui.RGBA(0, 0, 0, 0.55),
	}
}

// theme gives MyGo's own widgets the palette: inputs, calendars, menus,
// tooltips, switches and alerts.
func (p *palette) theme(base *ui.Theme) *ui.Theme {
	t := *base
	t.Dark = p.dark
	t.Background = p.bg
	t.Surface = p.card
	t.SurfaceHover = p.pill
	t.SurfacePressed = p.pillHover
	t.Border = p.line
	t.Text = p.fg
	t.TextMuted = p.muted
	t.Accent = p.accent
	t.AccentHover = p.accent.Mix(p.fg, 0.12)
	t.AccentPressed = p.accent.Mix(p.fg, 0.22)
	t.AccentText = p.accentFg
	t.Danger = p.red
	t.Warning = p.amber
	t.Success = p.green
	t.Selection = p.accent.Alpha(0.25)
	t.Focus = p.accent.Alpha(0.45)
	t.Scrollbar = p.fg.Alpha(0.25)
	t.Radius = 6
	t.FontSize = 13
	return &t
}

// The colors of kinds of things.

func (a *app) kindColor(k EventKind) ui.Color {
	s := &a.p.series
	switch k {
	case KindMeeting:
		return s[0]
	case KindLab:
		return s[2]
	case KindSeminar:
		return s[3]
	case KindFocus:
		return s[5]
	}
	return s[6]
}

func (a *app) categoryColor(k Category) ui.Color {
	s := &a.p.series
	switch k {
	case CatIndustry:
		return s[1]
	case CatIndependent:
		return s[2]
	case CatService:
		return s[4]
	}
	return s[0]
}

func (a *app) statusColor(s Status) ui.Color {
	switch s {
	case StatusActive:
		return a.p.green
	case StatusPlanning:
		return a.p.amber
	case StatusDone:
		return a.p.accent
	}
	return a.p.faint
}

// tint is col washed over a card, as event blocks and icon discs fill.
func (a *app) tint(col ui.Color) ui.Color {
	if a.p.dark {
		return col.Alpha(0.2).Over(a.p.card)
	}
	return col.Alpha(0.12).Over(a.p.card)
}

// ink is col darkened enough to write in on its tint.
func (a *app) ink(col ui.Color) ui.Color {
	if a.p.dark {
		return col.Mix(a.p.fg, 0.35)
	}
	return col.Mix(a.p.fg, 0.45)
}

// Type.

// label is a caption in small capitals, as magpie labels its tiles.
func (a *app) label(c *ui.Context, s string) ui.Element {
	return ui.Text(c, strings.ToUpper(s)).FontSize(11).FontWeight(600).LetterSpacing(0.5).TextColor(a.p.muted).SingleLine()
}

func (a *app) title(c *ui.Context, s string, size float32) ui.Element {
	return ui.Text(c, s).FontSize(size).FontWeight(600).TextColor(a.p.fg)
}

// sub is the second line of a row.
func (a *app) sub(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(11.5).TextColor(a.p.muted).SingleLine()
}

func (a *app) muted(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(12).TextColor(a.p.muted)
}

// Surfaces.

// card is a white surface with a hairline edge.
func (a *app) card(c *ui.Context) ui.Element {
	return ui.Column(c).Background(a.p.card).Border(1, a.p.line).Radius(12).Clip()
}

// list is a card of rows with hairlines between them.
func (a *app) list(c *ui.Context, fn func()) ui.Element {
	return a.card(c).Dividers(1, a.p.line2).Children(fn)
}

// section heads a list: its title, a count or a note, and what it offers
// on the right.
func (a *app) section(c *ui.Context, title, aside string, right func()) {
	ui.Row(c).Padding(16, 4, 7).AlignItems(ui.Center).Gap(8).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(a.p.fg).SingleLine()
		if aside != "" {
			ui.Text(c, aside).FontSize(12).TextColor(a.p.faint).SingleLine().MinWidth(0)
		}
		ui.Spacer(c)
		if right != nil {
			right()
		}
	})
}

// empty is what an empty list says, in the middle of its card.
func (a *app) empty(c *ui.Context, title, sub string) ui.Element {
	return ui.Column(c).Padding(30, 20).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(a.p.fg).TextAlign(ui.Center)
		if sub != "" {
			ui.Text(c, sub).FontSize(12).TextColor(a.p.muted).TextAlign(ui.Center).MaxWidth(360)
		}
	})
}

// Controls.

type btnKind int

const (
	btnPrimary   btnKind = iota // the accent, for the main action
	btnSecondary                // a gray pill
	btnPlain                    // text, a pill when hovered
	btnDanger                   // red text
	btnLink                     // accent text
)

func (a *app) btn(c *ui.Context, label string, kind btnKind) ui.Element {
	p := &a.p
	b := ui.ButtonBase(c).Height(26).Padding(0, 11).Gap(5).AlignItems(ui.Center).Radius(7).Shrink(0)
	hover, pressed := b.Hovered(), b.Pressed()
	switch kind {
	case btnPrimary:
		bg := p.accent
		if pressed {
			bg = p.accent.Mix(p.fg, 0.22)
		} else if hover {
			bg = p.accent.Mix(p.fg, 0.12)
		}
		b.Background(bg).TextColor(p.accentFg)
	case btnSecondary:
		bg := p.pill
		if pressed {
			bg = p.pillHover.Mix(p.fg, 0.05)
		} else if hover {
			bg = p.pillHover
		}
		b.Background(bg).TextColor(p.fg)
	case btnPlain:
		b.Padding(0, 8).TextColor(p.ctl)
		if hover {
			b.Background(p.pill).TextColor(p.fg)
		}
	case btnDanger:
		b.Padding(0, 8).TextColor(p.red)
		if hover {
			b.Background(p.redSoft)
		}
	case btnLink:
		b.Padding(0, 6).TextColor(p.accent)
		if hover {
			b.Background(p.accentSoft)
		}
	}
	if label != "" {
		b.Children(func() {
			t := ui.Text(c, label).FontSize(12.5).SingleLine()
			if kind == btnPrimary {
				t.FontWeight(500)
			}
		})
	}
	return b
}

// iconLabel fills a button with an icon and a label.
func (a *app) iconLabel(c *ui.Context, b ui.Element, icon *ui.SVG, label string) ui.Element {
	b.Children(func() {
		ui.Icon(c, icon).Size(13, 13)
		ui.Text(c, label).FontSize(12.5).SingleLine()
	})
	return b
}

// iconBtn is a square button showing an icon, named by its tooltip.
func (a *app) iconBtn(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c).Size(26, 26).Center().Radius(7).Shrink(0).Label(label).Tooltip(label).TextColor(a.p.muted)
	if b.Hovered() {
		b.Background(a.p.pill).TextColor(a.p.fg)
	}
	b.Children(func() { ui.Icon(c, icon).Size(15, 15) })
	return b
}

// segTrack is a segmented control on a gray track, its choice on a white
// thumb, as magpie's in-page switches.
func (a *app) segTrack(c *ui.Context, key string, chosen *int, labels ...string) ui.Element {
	p := &a.p
	s := ui.SegmentedBase(c.Key(key), chosen, len(labels))
	s.Track.Padding(2).Gap(1).Radius(8).AlignItems(ui.Center).Shrink(0).Background(p.segTrack)
	s.Track.Children(func() {
		for i, l := range labels {
			seg := s.Segment(i).Height(24).Padding(0, 11).Radius(6).Center()
			on := i == *chosen
			hover := seg.Hovered()
			if on {
				seg.Background(p.segThumb).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.14)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.08))
			}
			seg.Children(func() {
				t := ui.Text(c, l).FontSize(12.5).SingleLine()
				switch {
				case on:
					t.TextColor(p.fg).FontWeight(500)
				case hover:
					t.TextColor(p.fg2)
				default:
					t.TextColor(p.ctl)
				}
			})
		}
	})
	return s.Track
}

// selectOf is a pop-up button choosing one of options, named by name:
// a gray pill with a chevron, and a floating menu below it.
func selectOf[T comparable](a *app, c *ui.Context, key string, chosen *T, options []T, name func(T) string) ui.Element {
	p := &a.p
	sel := ui.SelectBase(c.Key(key), chosen)
	sel.Trigger.Height(25).Padding(0, 6, 0, 9).Gap(5).Radius(6).AlignItems(ui.Center).Background(p.pill).MinWidth(0)
	if sel.Trigger.Hovered() {
		sel.Trigger.Background(p.pillHover)
	}
	sel.Trigger.Children(func() {
		ui.Text(c, name(*chosen)).FontSize(12.5).FontWeight(500).SingleLine().Grow(1).MinWidth(0).TextColor(p.fg)
		ui.Icon(c, iconChevronUpDown).Size(11, 11).TextColor(p.muted)
	})
	sel.Popup(func(panel ui.Element) {
		popover(a, panel).Margin(4, 0, 0, 0).Padding(4).MinWidth(180).Gap(1)
		for _, o := range options {
			item := sel.Item(o).Height(28).Padding(0, 8).Radius(7).Gap(8).AlignItems(ui.Center)
			if item.Highlighted() {
				item.Background(p.pill)
			}
			item.Children(func() {
				if o == *chosen {
					ui.Icon(c, iconCheck).Size(12, 12).TextColor(p.accent)
				} else {
					ui.Box(c).Size(12, 12)
				}
				ui.Text(c, name(o)).FontSize(12.5).TextColor(p.fg).SingleLine().Grow(1)
			})
		}
	})
	return sel.Trigger
}

// popover gives a floating panel magpie's look: a card with its soft
// shadow and a hairline ring.
func popover(a *app, panel ui.Element) ui.Element {
	p := &a.p
	return panel.Radius(11).Background(p.card).
		Shadow(0, 16, 44, 0, p.shadow).Shadow(0, 2, 8, 0, p.shadow.Alpha(0.06)).Border(0.5, p.fg.Alpha(0.12))
}

// chip is a short pill with a colored dot, as magpie's filters.
func (a *app) chip(c *ui.Context, text string, dot ui.Color) ui.Element {
	p := &a.p
	ch := ui.Row(c).Height(22).Padding(0, 9, 0, 8).Gap(6).Radius(999).Border(1, p.line).Background(p.card).
		AlignItems(ui.Center).Shrink(0)
	ch.Children(func() {
		if dot.A > 0 {
			ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		}
		ui.Text(c, text).FontSize(11.5).TextColor(p.fg2).SingleLine()
	})
	return ch
}

func (a *app) statusChip(c *ui.Context, s Status) ui.Element {
	return a.chip(c, a.statusName(s), a.statusColor(s))
}

// softChip is a pill in a tint of col, as a badge.
func (a *app) softChip(c *ui.Context, text string, col ui.Color) ui.Element {
	ch := ui.Row(c).Height(20).Padding(0, 8).Radius(999).Background(a.tint(col)).AlignItems(ui.Center).Shrink(0)
	ch.Children(func() { ui.Text(c, text).FontSize(11).FontWeight(500).TextColor(a.ink(col)).SingleLine() })
	return ch
}

// check is a round check box, as Reminders draws one: filled with the
// accent once done.
func (a *app) check(c *ui.Context, done *bool, label string) ui.Element {
	b := ui.CheckboxBase(c, done).Size(18, 18).Shrink(0).Label(label)
	a.paintCheck(b, b, *done)
	return b
}

// labeledCheck is a check box with its label beside it, which a click on
// toggles too.
func (a *app) labeledCheck(c *ui.Context, done *bool, label string) ui.Element {
	b := ui.CheckboxBase(c, done).Gap(8).AlignItems(ui.Center).Shrink(0)
	b.Children(func() {
		box := ui.Box(c).Size(18, 18).Shrink(0)
		a.paintCheck(b, box, *done)
		ui.Text(c, label).FontSize(12.5).TextColor(a.p.fg2).SingleLine()
	})
	return b
}

func (a *app) paintCheck(base, box ui.Element, done bool) {
	p := &a.p
	drawn := float32(0)
	if done {
		drawn = 1
	}
	drawn = base.Animate("check", drawn, 140*time.Millisecond)
	box.Radius(9)
	switch {
	case done:
		box.Background(p.accent)
	case base.Hovered():
		box.Border(1.5, p.fg2)
	default:
		box.Border(1.5, p.faint)
	}
	tick := p.accentFg
	box.DrawOver(func(pt *ui.Painter, r ui.Rect) {
		if drawn <= 0 {
			return
		}
		path := (&ui.Path{}).MoveTo(r.X+5, r.Y+9.4).LineTo(r.X+7.8, r.Y+12).LineTo(r.X+13, r.Y+6.2)
		pt.StrokePath(path.Trim(0, drawn), 1.7, tick)
	})
}

// star toggles whether a task is important; unless shown, it shows only
// while set or hovered.
func (a *app) star(c *ui.Context, on *bool, label string, shown bool) ui.Element {
	b := ui.ToggleBase(c, on).Size(24, 24).Center().Shrink(0).Radius(6).Label(label).Tooltip(label)
	color := a.p.faint
	switch {
	case *on:
		color = a.p.amber
	case b.Hovered():
		color = a.p.fg2
	}
	icon := iconStar
	if *on {
		icon = iconStarFilled
	}
	if !*on && !shown && !b.Hovered() {
		b.Opacity(0)
	}
	b.Children(func() { ui.Icon(c, icon).Size(14, 14).TextColor(color) })
	return b
}

// formRow is a row of a settings card: its label on the left, its control
// on the right, as System Settings lays them out.
func (a *app) formRow(c *ui.Context, label string, fn func()) ui.Element {
	return ui.Row(c.Key(label)).MinHeight(42).Padding(6, 10, 6, 14).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(a.p.fg).Grow(1).MinWidth(0)
		fn()
	})
}

// formRowHint is a form row with a line under its label.
func (a *app) formRowHint(c *ui.Context, label, hint string, fn func()) ui.Element {
	return ui.Row(c.Key(label)).MinHeight(50).Padding(7, 10, 7, 14).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(13).TextColor(a.p.fg)
			ui.Text(c, hint).FontSize(11.5).TextColor(a.p.muted)
		})
		fn()
	})
}

// input is a text field.
func (a *app) input(c *ui.Context, value *string, placeholder string) ui.Element {
	in := ui.TextInput(c, value).Placeholder(placeholder).Label(placeholder).Radius(7).Padding(4, 9)
	if in.Changed() {
		a.changed()
	}
	return in
}

// area is a text area without a box of its own, inside a card.
func (a *app) area(c *ui.Context, value *string, placeholder string) ui.Element {
	in := ui.TextAreaBase(c, value).Placeholder(placeholder).Label(placeholder).FontSize(13).LineHeight(1.5).
		TextColor(a.p.fg).Padding(10, 14).MinHeight(64).FillWidth()
	if in.Changed() {
		a.changed()
	}
	return in
}

// titleInput edits a title in place, wrapping as it grows. Return ends
// the editing rather than the line.
func (a *app) titleInput(c *ui.Context, value *string, placeholder string, size float32) ui.Element {
	in := ui.TextAreaBase(c, value).Lines(1, 4).Placeholder(placeholder).Label(placeholder).
		FontSize(size).FontWeight(600).TextColor(a.p.fg).FillWidth().
		OnTextKey(func(k ui.TextKey) bool { return k.Key == ui.KeyEnter })
	if in.Changed() {
		a.changed()
	}
	return in
}

// dateField edits an optional date: a button sets it, a cross clears it.
func (a *app) dateField(c *ui.Context, key string, date *time.Time) {
	ui.Row(c.Key(key)).Gap(2).AlignItems(ui.Center).Shrink(0).Children(func() {
		if date.IsZero() {
			if a.btn(c, a.tr.SetDate(), btnSecondary).Clicked() {
				*date = day(a.now())
				a.changed()
			}
			return
		}
		if ui.DateInput(c, date).Label(key).Width(132).Changed() {
			*date = day(*date)
			a.changed()
		}
		if a.iconBtn(c, iconClose, a.tr.Clear()).Clicked() {
			*date = time.Time{}
			a.changed()
		}
	})
}

// quickAdd is a row for adding something: a plus and a field. add gets the
// trimmed text on Return; handle, if not nil, is bound to the field.
func (a *app) quickAdd(c *ui.Context, draft *string, placeholder string, handle *ui.Handle, add func(string)) ui.Element {
	p := &a.p
	row := ui.Row(c).MinHeight(42).Padding(0, 14).Gap(10).AlignItems(ui.Center).Shrink(0)
	row.Children(func() {
		ui.Icon(c, iconPlus).Size(14, 14).TextColor(p.accent)
		in := ui.TextInputBase(c, draft).Placeholder(placeholder).Label(placeholder).Grow(1).FontSize(13).TextColor(p.fg)
		if handle != nil {
			in.Bind(handle)
		}
		if in.Submitted() {
			if s := strings.TrimSpace(*draft); s != "" {
				add(s)
			}
			*draft = ""
		}
	})
	return row
}

// modal shows a sheet over the dimmed window.
func (a *app) modal(c *ui.Context, open *bool, width float32, fn func()) ui.Element {
	return ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.2))
		popover(a, panel).Width(width).MaxWidth(width).Padding(18, 20).Gap(14).Radius(12)
		fn()
	})
}

// progressBar is a thin track, filled with the accent as far as frac.
func (a *app) progressBar(c *ui.Context, frac float32) ui.Element {
	track, fill := a.p.pill, a.p.accent
	return ui.Box(c).Height(4).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, track, 2)
		if frac > 0 {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.W*min(frac, 1), 4), H: r.H}, fill.Alpha(0.85), 2)
		}
	})
}

// dot is a small circle of col.
func (a *app) dot(c *ui.Context, col ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Radius(size / 2).Background(col).Shrink(0)
}

// disc is an icon on a tinted circle, as magpie shows a provider's logo.
func (a *app) disc(c *ui.Context, icon *ui.SVG, col ui.Color, size float32) ui.Element {
	d := ui.Box(c).Size(size, size).Radius(size / 2).Background(a.tint(col)).Center().Shrink(0)
	d.Children(func() { ui.Icon(c, icon).Size(size*0.5, size*0.5).TextColor(col) })
	return d
}

// priorityMarks writes a task's priority as Reminders does: !, !! or !!!.
func (a *app) priorityMarks(c *ui.Context, pr int) ui.Element {
	col := a.p.amber
	if pr >= 3 {
		col = a.p.red
	}
	return ui.Text(c, strings.Repeat("!", pr)).FontSize(13).FontWeight(700).TextColor(col).Shrink(0)
}

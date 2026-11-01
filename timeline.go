package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const timelineLabel = 260 // DIPs the projects' names take beside the timeline

// timelineSpan returns the months the timeline shows: from the earliest
// start, or a quarter before today, to the latest end, or nine months on.
func (a *app) timelineSpan(groups []projectGroup) (from, to time.Time) {
	now := a.now()
	from, to = now.AddDate(0, -3, 0), now.AddDate(0, 9, 0)
	for _, g := range groups {
		for _, p := range g.projects {
			if !p.Start.IsZero() && p.Start.Before(from) {
				from = p.Start
			}
			if !p.End.IsZero() && p.End.After(to) {
				to = p.End
			}
		}
	}
	from = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, now.Location())
	to = time.Date(to.Year(), to.Month()+1, 1, 0, 0, 0, 0, now.Location())
	return from, to
}

// timelineMonths returns the first of each month from from to to.
func timelineMonths(from, to time.Time) []time.Time {
	var out []time.Time
	for t := from; t.Before(to); t = t.AddDate(0, 1, 0) {
		out = append(out, t)
	}
	return out
}

// timelineSteps are how many months apart the timeline may mark them: each
// month, every other, each quarter, half year, year, two, five or ten.
var timelineSteps = []int{1, 2, 3, 6, 12, 24, 60, 120}

// timelineGap is the room a mark's label leaves: its padding and the space
// before the next.
const timelineGap = 14

// timelineStep returns how many months apart to mark n months in w DIPs,
// for labels of at most label DIPs: the fewest that leave each its room.
func timelineStep(n int, w, label float32) int {
	per := w / float32(max(n, 1))
	for _, s := range timelineSteps {
		if per*float32(s) >= label+timelineGap {
			return s
		}
	}
	return timelineSteps[len(timelineSteps)-1]
}

// marked says whether t, the first of a month, is marked every step months,
// which marks every January when step is a year or less.
func marked(t time.Time, step int) bool {
	return (t.Year()*12+int(t.Month())-1)%step == 0
}

// timeline shows the projects of groups as bars over the months they run,
// as a Gantt chart: each in its category's color, filled as far as its
// tasks are done, with a line at today.
func (a *app) timeline(c *ui.Context, groups []projectGroup) {
	p := &a.p
	from, to := a.timelineSpan(groups)
	span := float32(to.Sub(from))
	at := func(t time.Time) float32 { return max(0, min(100, float32(t.Sub(from))/span*100)) }
	months := timelineMonths(from, to)
	today := at(a.now())
	// The labels of the months, and the widest of them, which sets how far
	// apart the marks fall in the width there is.
	type label struct {
		span ui.Span
		w    float32
	}
	labels := make([]label, len(months))
	var widest float32
	for i, t := range months {
		s := ui.Span{Text: a.fmt(t, "MMM"), Size: 11, Color: p.muted, Features: "tnum"}
		if t.Month() == time.January {
			s.Text, s.Color, s.Weight = a.fmt(t, "y"), p.fg2, 600
		}
		w, _ := c.MeasureText(0, s)
		labels[i], widest = label{s, w}, max(widest, w)
	}
	step := func(r ui.Rect) int { return timelineStep(len(months), r.W, widest) }
	x := func(r ui.Rect, t time.Time) float32 { return r.X + r.W*at(t)/100 }
	grid := func(pt *ui.Painter, r ui.Rect) {
		n := step(r)
		for i, t := range months {
			if i == 0 || !marked(t, n) {
				continue
			}
			col := p.line2
			if t.Month() == time.January {
				col = p.line
			}
			pt.Fill(ui.Rect{X: x(r, t), Y: r.Y, W: 1, H: r.H}, col, 0)
		}
	}
	line := func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(ui.Rect{X: r.X + r.W*today/100 - 0.75, Y: r.Y, W: 1.5, H: r.H}, p.red.Alpha(0.7), 0)
	}
	// The months, over the bars: years at their Januaries, months between,
	// the first with its year if there is room, and today as a dot under
	// them, from which its line runs down.
	header := func(pt *ui.Painter, r ui.Rect) {
		grid(pt, r)
		n := step(r)
		first := true
		for i, t := range months {
			if !marked(t, n) {
				continue
			}
			l, x0 := labels[i], x(r, t)
			end := r.X + r.W
			for _, u := range months[i+1:] {
				if marked(u, n) {
					end = x(r, u)
					break
				}
			}
			if first && t.Month() != time.January {
				full := l.span
				full.Text = a.fmt(t, "yMMM")
				if w, _ := pt.MeasureText(0, full); x0+w+timelineGap <= end {
					l = label{full, w}
				}
			}
			first = false
			if x0+6+l.w > r.X+r.W+6 {
				continue // the last, past the end
			}
			pt.RichText(x0+6, r.Y+10, 0, l.span)
		}
		dot := r.Y + r.H - 5
		tx := r.X + r.W*today/100
		pt.Fill(ui.Rect{X: tx - 0.75, Y: dot, W: 1.5, H: r.Y + r.H - dot}, p.red.Alpha(0.7), 0)
		pt.Fill(ui.Rect{X: tx - 3, Y: dot - 3, W: 6, H: 6}, p.red, 3)
	}
	rows := func(pt *ui.Painter, r ui.Rect) {
		grid(pt, r)
		line(pt, r)
	}
	a.list(c, func() {
		ui.Row(c).Height(34).Shrink(0).Children(func() {
			ui.Box(c).Width(timelineLabel).Shrink(0)
			ui.Box(c).Grow(1).AlignSelf(ui.Stretch).Margin(0, 16, 0, 0).Draw(header)
		})
		for _, g := range groups {
			ui.Row(c.Key("group:"+g.key)).Height(30).Padding(0, 14).Gap(8).AlignItems(ui.Center).Background(p.card2).Children(func() {
				ui.Text(c, g.title).FontSize(12).FontWeight(600).TextColor(p.fg2).SingleLine()
				if g.hint != "" {
					ui.Text(c, g.hint).FontSize(11.5).TextColor(p.faint).SingleLine().MinWidth(0)
				}
			})
			for _, pr := range g.projects {
				a.timelineRow(c, pr, at, rows)
			}
		}
	})
}

// timelineRow is a project in the timeline: its name, and its bar, which
// opens it on a click.
func (a *app) timelineRow(c *ui.Context, pr *Project, at func(time.Time) float32, grid func(*ui.Painter, ui.Rect)) {
	p := &a.p
	title := orUntitled(pr.Title, a.tr.Untitled())
	row := ui.ButtonBase(c.Key(pr.ID)).Height(40).AlignItems(ui.Stretch).Radius(0).FocusRing(false).Label(title)
	if row.Hovered() {
		row.Background(p.card2)
	}
	_, col := a.categoryLook(pr.Category)
	done, total := a.data.progress(pr.ID)
	row.Children(func() {
		ui.Row(c).Width(timelineLabel).Shrink(0).Padding(0, 12, 0, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			a.dot(c, a.statusColor(pr.Status), 7)
			ui.Text(c, title).FontSize(13).TextColor(p.fg).SingleLine().MinWidth(0)
		})
		lane := ui.Box(c).Grow(1).Margin(0, 16, 0, 0).Draw(grid)
		if pr.Start.IsZero() && pr.End.IsZero() {
			lane.Children(func() {
				ui.Text(c, a.tr.NoDates()).Absolute().Top(12).Left(8).FontSize(11.5).TextColor(p.faint).SingleLine()
			})
			return
		}
		start := pr.Start
		if start.IsZero() {
			start = pr.Created
		}
		x0, x1 := at(start), float32(100)
		if !pr.End.IsZero() {
			x1 = at(pr.End.AddDate(0, 0, 1))
		}
		x1 = max(x1, x0+0.6)
		when := a.fmt(start, "yMMM") + " – "
		if !pr.End.IsZero() {
			when += a.fmt(pr.End, "yMMM")
		}
		faded := pr.Status == StatusPaused || pr.Status == StatusDone
		lane.Children(func() {
			bar := ui.Box(c).Absolute().Top(10).Height(20).LeftPercent(x0).WidthPercent(x1-x0).Radius(5).Clip().
				Background(a.tint(col)).Border(1, col.Alpha(0.45)).Tooltip(title + "\n" + when + " · " + a.tr.TasksProgress(done, total))
			if faded {
				bar.Opacity(0.6)
			}
			bar.Children(func() {
				if total > 0 && done > 0 {
					ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).WidthPercent(float32(done) / float32(total) * 100).Background(col.Alpha(0.28))
				}
			})
		})
		// The dates, in the bar if they fit, else after it, or before it.
		lane.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			s := ui.Span{Text: when, Size: 10.5, Weight: 500, Features: "tnum", Color: a.ink(col)}
			w, h := pt.MeasureText(0, s)
			b := ui.Rect{X: r.X + r.W*x0/100, Y: r.Y + 10, W: r.W * (x1 - x0) / 100, H: 20}
			x, y := b.X+7, b.Y+(b.H-h)/2
			clip := false
			switch {
			case w+14 <= b.W:
			case b.X+b.W+6+w <= r.X+r.W+16:
				x, s.Color = b.X+b.W+6, p.muted
			case b.X-6-w >= r.X:
				x, s.Color = b.X-6-w, p.muted
			default:
				clip = true
			}
			if faded {
				s.Color = s.Color.Alpha(0.6)
			}
			if clip {
				pt.Clip(b, 5, func() { pt.RichText(x, y, 0, s) })
			} else {
				pt.RichText(x, y, 0, s)
			}
		})
	})
	if row.Clicked() {
		a.openProject(pr.ID)
	}
}

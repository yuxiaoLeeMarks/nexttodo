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

// timelineTicks returns the months the timeline marks: each month, each
// quarter or each year, as many as fit.
func timelineTicks(from, to time.Time) []time.Time {
	months := (to.Year()-from.Year())*12 + int(to.Month()-from.Month())
	step := 1
	switch {
	case months > 48:
		step = 12
	case months > 18:
		step = 3
	}
	var out []time.Time
	for t := from; t.Before(to); t = t.AddDate(0, 1, 0) {
		if (int(t.Month())-1)%step == 0 {
			out = append(out, t)
		}
	}
	return out
}

// timeline shows the projects of groups as bars over the months they run,
// as a Gantt chart: each in its category's color, filled as far as its
// tasks are done, with a line at today.
func (a *app) timeline(c *ui.Context, groups []projectGroup) {
	p := &a.p
	from, to := a.timelineSpan(groups)
	span := float32(to.Sub(from))
	at := func(t time.Time) float32 { return max(0, min(100, float32(t.Sub(from))/span*100)) }
	ticks := timelineTicks(from, to)
	today := at(a.now())
	grid := func(pt *ui.Painter, r ui.Rect) {
		for i, t := range ticks {
			if i == 0 && t.Equal(from) {
				continue
			}
			col := p.line2
			if t.Month() == time.January {
				col = p.line
			}
			pt.Fill(ui.Rect{X: r.X + r.W*at(t)/100, Y: r.Y, W: 1, H: r.H}, col, 0)
		}
		pt.Fill(ui.Rect{X: r.X + r.W*today/100 - 0.75, Y: r.Y, W: 1.5, H: r.H}, p.red.Alpha(0.7), 0)
	}
	a.list(c, func() {
		// The months, over the bars.
		ui.Row(c).Height(34).Shrink(0).Children(func() {
			ui.Box(c).Width(timelineLabel).Shrink(0)
			ui.Box(c).Grow(1).AlignSelf(ui.Stretch).Margin(0, 16, 0, 0).Draw(grid).Children(func() {
				// Years at their Januaries, months between.
				for i, t := range ticks {
					label, col, weight := a.fmt(t, "MMM"), p.muted, 400
					switch {
					case t.Month() == time.January:
						label, col, weight = a.fmt(t, "y"), p.fg2, 600
					case i == 0:
						label = a.fmt(t, "yMMM")
					}
					ui.Text(c.Key(t.Format("2006-01")), label).Absolute().LeftPercent(at(t)).Top(10).Padding(0, 6).
						FontSize(11).FontWeight(weight).FontFeatures("tnum").TextColor(col).SingleLine()
				}
			})
		})
		for _, g := range groups {
			ui.Row(c.Key("group:"+g.key)).Height(30).Padding(0, 14).Gap(8).AlignItems(ui.Center).Background(p.card2).Children(func() {
				ui.Text(c, g.title).FontSize(12).FontWeight(600).TextColor(p.fg2).SingleLine()
				if g.hint != "" {
					ui.Text(c, g.hint).FontSize(11.5).TextColor(p.faint).SingleLine().MinWidth(0)
				}
			})
			for _, pr := range g.projects {
				a.timelineRow(c, pr, at, grid)
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
		ui.Box(c).Grow(1).Margin(0, 16, 0, 0).Draw(grid).Children(func() {
			if pr.Start.IsZero() && pr.End.IsZero() {
				ui.Text(c, a.tr.NoDates()).Absolute().Top(12).Left(8).FontSize(11.5).TextColor(p.faint).SingleLine()
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
			when := a.fmt(start, "yMMM") + " – "
			if !pr.End.IsZero() {
				when += a.fmt(pr.End, "yMMM")
			}
			bar := ui.Box(c).Absolute().Top(10).Height(20).LeftPercent(x0).WidthPercent(max(x1-x0, 0.6)).Radius(5).Clip().
				Background(a.tint(col)).Border(1, col.Alpha(0.45)).Tooltip(title + "\n" + when + " · " + a.tr.TasksProgress(done, total))
			if pr.Status == StatusPaused || pr.Status == StatusDone {
				bar.Opacity(0.6)
			}
			bar.Children(func() {
				if total > 0 && done > 0 {
					ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).WidthPercent(float32(done) / float32(total) * 100).Background(col.Alpha(0.28))
				}
				ui.Text(c, when).Absolute().Top(3).Left(7).FontSize(10.5).FontWeight(500).FontFeatures("tnum").
					TextColor(a.ink(col)).SingleLine()
			})
		})
	})
	if row.Clicked() {
		a.openProject(pr.ID)
	}
}

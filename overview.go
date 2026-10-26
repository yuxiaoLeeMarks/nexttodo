package main

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) greeting(now time.Time) string {
	switch h := now.Hour(); {
	case h < 5:
		return a.tr.GreetNight()
	case h < 12:
		return a.tr.GreetMorning()
	case h < 18:
		return a.tr.GreetAfternoon()
	case h < 23:
		return a.tr.GreetEvening()
	}
	return a.tr.GreetNight()
}

// nextEvent returns the event of today going on now or starting next.
func (a *app) nextEvent() *Event {
	now := a.now()
	for _, e := range a.data.eventsBetween(day(now), addDays(day(now), 1)) {
		if !e.AllDay && e.End.After(now) {
			return e
		}
	}
	return nil
}

// overview is the day at a glance: what is left of it and what comes
// next, today's agenda, the deadlines ahead and the projects in motion.
func (a *app) overview(c *ui.Context) {
	now := a.now()
	today := day(now)
	wk := weekStart(today, a.firstWeekday(c))

	var myDay []*Task
	for _, t := range a.data.Tasks {
		if a.inList(t, listMyDay) {
			myDay = append(myDay, t)
		}
	}
	sortTasks(myDay)
	var overdue, dueWeek int
	for _, t := range a.data.Tasks {
		if t.Done || t.Due.IsZero() || a.data.shelved(t) {
			continue
		}
		if d := daysBetween(today, t.Due); d < 0 {
			overdue++
		} else if day(t.Due).Before(addDays(wk, 7)) {
			dueWeek++
		}
	}
	todays := a.data.eventsBetween(today, addDays(today, 1))
	var active []*Project
	for _, pr := range a.data.Projects {
		if pr.Status == StatusActive && !pr.Archived {
			active = append(active, pr)
		}
	}

	a.pageBody(c, nil, func() {
		a.overviewHero(c, now, wk, a.openCount(myDay), len(todays), overdue, dueWeek, len(active))
		ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(3).Basis(0).MinWidth(0).Children(func() {
				a.section(c, a.tr.OverviewToday(), a.fmt(now, "MMMEd"), func() {
					if a.btn(c, a.tr.ViewAll(), btnLink).Clicked() {
						a.schedAnchor, a.schedMode, a.schedScrolled = today, 0, false
						a.navigate(PageSchedule)
					}
				})
				a.list(c, func() {
					if len(todays) == 0 && len(myDay) == 0 {
						a.empty(c, a.tr.OverviewTodayEmpty(), a.tr.OverviewTodayEmptySub())
						return
					}
					for _, e := range todays {
						a.eventRow(c, e)
					}
					for _, t := range myDay {
						a.taskRow(c, t, rowElsewhere)
					}
				})
			})
			ui.Column(c).Grow(2).Basis(0).MinWidth(0).Children(func() { a.deadlines(c, today) })
		})
		a.section(c, a.tr.OverviewActive(), itoa(len(active)), func() {
			if a.btn(c, a.tr.ViewAll(), btnLink).Clicked() {
				a.navigate(PageProjects)
			}
		})
		a.list(c, func() {
			if len(active) == 0 {
				a.empty(c, a.tr.OverviewActiveEmpty(), "")
				return
			}
			for _, pr := range active {
				a.projectRow(c, pr)
			}
		})
	})
}

// overviewHero is the card across the top: the date, the count of what is
// left today with the day's figures under it, the week's load, and the
// event coming next.
func (a *app) overviewHero(c *ui.Context, now, wk time.Time, left, events, overdue, dueWeek, active int) {
	p := &a.p
	a.card(c).Padding(18, 22, 20).Margin(4, 0, 0, 0).Children(func() {
		ui.Row(c).Gap(28).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, a.fmt(now, "MMMMEEEEd")).FontSize(15).FontWeight(600).TextColor(p.fg)
				ui.Text(c, a.greeting(now)).FontSize(12.5).TextColor(p.muted)
				ui.Row(c).AlignItems(ui.Baseline).Gap(10).Margin(10, 0, 0, 0).Children(func() {
					if left > 0 {
						ui.Text(c, itoa(left)).FontSize(64).FontWeight(200).LineHeight(1).FontFeatures("tnum").TextColor(p.fg)
					}
					ui.Text(c, a.tr.LeftToday(left)).FontSize(14).TextColor(p.fg2)
				})
				ui.Row(c).Gap(22).Wrap().Margin(12, 0, 0, 0).Children(func() {
					stat := func(n int, label string, alarm bool, open func()) {
						b := ui.ButtonBase(c.Key(label)).Gap(5).AlignItems(ui.Baseline).Padding(2, 4).Margin(0, -4).Radius(5)
						if b.Hovered() {
							b.Background(p.pill)
						}
						b.Children(func() {
							v := ui.Text(c, itoa(n)).FontSize(13).FontWeight(600).FontFeatures("tnum").TextColor(p.fg)
							if alarm {
								v.TextColor(p.red)
							}
							ui.Text(c, label).FontSize(12.5).TextColor(p.muted)
						})
						if b.Clicked() {
							open()
						}
					}
					stat(events, a.tr.StatEventsToday(), false, func() {
						a.schedAnchor, a.schedMode, a.schedScrolled = day(now), 0, false
						a.navigate(PageSchedule)
					})
					stat(overdue, a.tr.StatOverdue(), overdue > 0, func() { a.taskList = listMyDay; a.navigate(PageTasks) })
					stat(dueWeek, a.tr.StatDueWeek(), false, func() { a.taskList = listPlanned; a.navigate(PageTasks) })
					stat(active, a.tr.StatActive(), false, func() { a.navigate(PageProjects) })
				})
			})
			a.weekChart(c, now, wk)
			ui.Box(c).Width(1).Background(p.line2).Margin(4, 0)
			a.nextUp(c)
		})
	})
}

// weekChart draws the week's load, events and tasks due each day, as bars,
// today's in the accent; a click opens the day.
func (a *app) weekChart(c *ui.Context, now, wk time.Time) {
	p := &a.p
	var load [7]int
	most := 1
	for i := range load {
		d := addDays(wk, i)
		load[i] = len(a.data.eventsBetween(d, addDays(d, 1))) + len(a.data.tasksDueOn(d))
		most = max(most, load[i])
	}
	ui.Column(c).Width(248).Shrink(0).Gap(6).Children(func() {
		a.label(c, a.tr.ThisWeek())
		ui.Row(c).Height(78).Gap(6).AlignItems(ui.End).Margin(4, 0, 0, 0).Children(func() {
			for i, n := range load {
				d := addDays(wk, i)
				today := sameDay(d, now)
				col := ui.ButtonBase(c.Key(i)).Grow(1).Basis(0).FillHeight().Column().Justify(ui.End).AlignItems(ui.Stretch).
					Radius(3).FocusRing(false).Label(a.fmt(d, "MMMEd")).Tooltip(a.fmt(d, "MMMEd") + " · " + itoa(n))
				bar := p.pillHover
				switch {
				case today:
					bar = p.accent
				case col.Hovered():
					bar = p.faint
				}
				h := 4 + 70*float32(n)/float32(most)
				col.Children(func() { ui.Box(c).Height(h).Radius(3).Background(bar) })
				if col.Clicked() {
					a.schedAnchor, a.schedMode, a.schedScrolled = d, 0, false
					a.navigate(PageSchedule)
				}
			}
		})
		ui.Row(c).Gap(6).Children(func() {
			for i := range load {
				d := addDays(wk, i)
				l := ui.Text(c, a.fmt(d, "EEEEE")).FontSize(10.5).TextColor(p.muted).Grow(1).Basis(0).TextAlign(ui.Center).SingleLine()
				if sameDay(d, now) {
					l.TextColor(p.accent).FontWeight(600)
				}
			}
		})
	})
}

// nextUp is the event going on now, or starting next, today.
func (a *app) nextUp(c *ui.Context) {
	p := &a.p
	e := a.nextEvent()
	b := ui.ButtonBase(c.Key("next")).Width(230).Shrink(0).Column().AlignItems(ui.Start).Justify(ui.Start).Gap(4).
		Padding(2, 6).Margin(0, -6).Radius(8).FocusRing(false).Label(a.tr.NextUp())
	if e != nil && b.Hovered() {
		b.Background(p.card2)
	}
	b.Children(func() {
		a.label(c, a.tr.NextUp())
		if e == nil {
			ui.Text(c, a.tr.NothingNext()).FontSize(12.5).TextColor(p.muted).Margin(6, 0, 0, 0)
			return
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
			a.dot(c, a.kindColor(e.Kind), 8)
			ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(15).FontWeight(600).TextColor(p.fg).MaxLines(2)
		})
		ui.Text(c, a.clock(e.Start)+" – "+a.clock(e.End)).FontSize(12.5).FontFeatures("tnum").TextColor(p.fg2)
		if e.Location != "" {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconPin).Size(12, 12).TextColor(p.muted)
				ui.Text(c, e.Location).FontSize(12).TextColor(p.muted).SingleLine()
			})
		}
		if !e.Start.After(a.now()) {
			a.softChip(c, a.tr.HappeningNow(), p.green).Margin(4, 0, 0, 0)
		} else {
			a.softChip(c, a.until(e.Start), p.accent).Margin(4, 0, 0, 0)
		}
	})
	if e != nil {
		if b.Clicked() {
			a.editEvent(e)
		}
		c.After(time.Minute)
	}
}

// eventRow is an event in a list: its time, its kind's color, its title
// and where it is.
func (a *app) eventRow(c *ui.Context, e *Event) {
	p := &a.p
	now := a.now()
	row := ui.ButtonBase(c.Key(e.key())).MinHeight(46).Padding(6, 14).Gap(12).AlignItems(ui.Center).Radius(0).FocusRing(false).
		Label(orUntitled(e.Title, a.tr.Untitled()))
	if row.Hovered() {
		row.Background(p.card2)
	}
	if !e.AllDay && e.End.Before(now) {
		row.Opacity(0.5)
	}
	row.Children(func() {
		when := a.tr.AllDay()
		if !e.AllDay {
			when = a.clock(e.Start) + " – " + a.clock(e.End)
		}
		ui.Text(c, when).FontSize(12).FontFeatures("tnum").TextColor(p.muted).Width(118).Shrink(0).SingleLine()
		ui.Box(c).Width(3).Height(28).Radius(1.5).Background(a.kindColor(e.Kind)).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(13).FontWeight(500).TextColor(p.fg).SingleLine()
			sub := a.kindName(e.Kind)
			if e.series != nil {
				sub += " · " + a.repeatName(e.Repeat)
			}
			if e.Location != "" {
				sub += " · " + e.Location
			}
			if e.Project != "" {
				sub += " · " + a.projectName(e.Project)
			}
			a.sub(c, sub)
		})
		if !e.AllDay && !e.Start.After(now) && e.End.After(now) {
			a.softChip(c, a.tr.HappeningNow(), p.green)
		}
	})
	a.eventMenu(row, e)
	if row.Clicked() {
		a.editEvent(e)
	}
}

// deadlines lists what falls due in the next two weeks, the late first, and
// the projects ending within two months.
func (a *app) deadlines(c *ui.Context, today time.Time) {
	p := &a.p
	type item struct {
		date    time.Time
		title   string
		sub     string
		task    *Task
		project *Project
	}
	var items []item
	for _, t := range a.data.Tasks {
		if t.Done || t.Due.IsZero() || daysBetween(today, t.Due) > 14 || a.data.shelved(t) {
			continue
		}
		sub := a.tr.NoProject()
		if t.Project != "" {
			sub = a.projectName(t.Project)
		}
		items = append(items, item{date: day(t.Due), title: orUntitled(t.Title, a.tr.Untitled()), sub: sub, task: t})
	}
	for _, pr := range a.data.Projects {
		if pr.End.IsZero() || pr.Status == StatusDone || pr.Archived {
			continue
		}
		if d := daysBetween(today, pr.End); d >= 0 && d <= 60 {
			items = append(items, item{date: day(pr.End), title: orUntitled(pr.Title, a.tr.Untitled()), sub: a.tr.OverviewProjectEnd(), project: pr})
		}
	}
	slices.SortStableFunc(items, func(x, y item) int { return x.date.Compare(y.date) })
	a.section(c, a.tr.OverviewDeadlines(), itoa(len(items)), func() {
		if a.btn(c, a.tr.ViewAll(), btnLink).Clicked() {
			a.taskList = listPlanned
			a.navigate(PageTasks)
		}
	})
	a.list(c, func() {
		if len(items) == 0 {
			a.empty(c, a.tr.OverviewDeadlinesEmpty(), "")
			return
		}
		for i, it := range items {
			if i == 8 {
				break
			}
			key := "p:" + itoa(i)
			if it.task != nil {
				key = it.task.ID
			}
			b := ui.ButtonBase(c.Key(key)).MinHeight(46).Padding(6, 14).Gap(12).AlignItems(ui.Center).Radius(0).FocusRing(false).Label(it.title)
			if b.Hovered() {
				b.Background(p.card2)
			}
			n := daysBetween(today, it.date)
			urgent := n <= 1
			b.Children(func() {
				ui.Column(c).Width(34).Shrink(0).AlignItems(ui.Center).Gap(0).Children(func() {
					m := ui.Text(c, a.fmt(it.date, "MMM")).FontSize(9.5).FontWeight(600).TextColor(p.muted).SingleLine()
					d := ui.Text(c, itoa(it.date.Day())).FontSize(17).FontWeight(600).FontFeatures("tnum").TextColor(p.fg)
					if urgent {
						m.TextColor(p.red)
						d.TextColor(p.red)
					}
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, it.title).FontSize(13).FontWeight(500).TextColor(p.fg).SingleLine()
					a.sub(c, it.sub)
				})
				r := ui.Text(c, a.relDay(it.date)).FontSize(11.5).TextColor(p.muted).SingleLine().Shrink(0)
				if urgent {
					r.TextColor(p.red)
				}
			})
			if b.Clicked() {
				if it.task != nil {
					a.openTask(it.task.ID)
				} else {
					a.openProject(it.project.ID)
				}
			}
		}
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

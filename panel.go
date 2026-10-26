package main

import (
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// quickDraft is what the menu bar's panel is adding.
type quickDraft struct {
	mode    int // 0 a task to My Day, 1 an event today
	title   string
	start   time.Time
	minutes int
	added   string // the title just added, which the panel shows a moment
	field   ui.Handle
}

// nextHalfHour is the half hour after t.
func nextHalfHour(t time.Time) time.Time {
	s := at(t, t.Hour(), 0)
	if t.Minute() >= 30 {
		return s.Add(time.Hour)
	}
	return s.Add(30 * time.Minute)
}

// quickWhen reads what the panel's field says of its time: an event's day
// and hours, or a task's due day.
func (a *app) quickWhen(first time.Weekday) when {
	q := &a.quick
	return a.readWhen(&q.title, q.title, first, a.quickReads())
}

// quickReads is what the panel's line is read for: an event's time and
// length, or a task's reminder.
func (a *app) quickReads() int {
	if a.quick.mode == 1 {
		return readSpan
	}
	return readClock
}

// quickAddNow adds what the panel's field says: a task to My Day, or an
// event at the time the panel shows, which the line may have set, on the
// day the line names.
func (a *app) quickAddNow(first time.Weekday) {
	q := &a.quick
	w := a.quickWhen(first)
	if w.title == "" {
		return
	}
	if q.mode == 0 {
		a.addTaskWhen(w, "", listMyDay)
	} else {
		// The panel's time is on its day, tomorrow's after 23:30.
		start := q.start
		if !w.day.IsZero() {
			start = at(w.day, q.start.Hour(), q.start.Minute())
		}
		a.data.Events = append(a.data.Events, &Event{ID: newID(), Title: w.title, Kind: KindMeeting,
			Start: start, End: start.Add(time.Duration(q.minutes) * time.Minute)})
		a.changed()
	}
	q.title, q.added = "", w.title
	delete(a.literal, &q.title)
}

// panelView is the menu bar's panel, as magpie's: what is left of today and
// a field to add a task or an event, over the popover's material.
func (a *app) panelView(c *ui.Context) {
	a.begin(c)
	p := &a.p
	q := &a.quick
	now := a.now()
	first := a.firstWeekday(c)
	c.Root().Background(ui.Transparent)
	box := ui.Column(c).Fill()
	if c.Vibrancy() {
		// magpie's panel is the window's gray, thinly, over the material.
		box.Background(p.bg.Alpha(0.72))
	} else {
		box.Background(p.bg).Radius(12).Border(1, p.line)
	}
	box.Padding(12, 12, 8).Gap(10).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Padding(0, 2).Children(func() {
			ui.Icon(c, iconLogo).Size(16, 16).TextColor(p.fg)
			ui.Text(c, a.tr.AppName()).FontSize(13).FontWeight(600).TextColor(p.fg)
			ui.Spacer(c)
			ui.Text(c, a.fmt(now, "MMMEd")).FontSize(12).TextColor(p.muted)
			if a.iconBtn(c, iconOpen, a.tr.PanelOpen()).Clicked() {
				a.showMain()
			}
		})
		a.card(c).Padding(10).Gap(8).Shrink(0).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.segTrack(c, "quickMode", &q.mode, a.tr.PanelTask(), a.tr.PanelEvent()).Changed() {
					q.field.Focus()
				}
				ui.Spacer(c)
				if w := a.quickWhen(first); w.said() {
					a.whenChip(c, &q.title, w, a.quickReads())
				} else if q.added != "" {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconCheck).Size(12, 12).TextColor(p.green)
						ui.Text(c, a.tr.PanelAdded()).FontSize(12).TextColor(p.green)
					})
				}
			})
			placeholder := a.tr.PanelAddTask()
			if q.mode == 1 {
				placeholder = a.tr.PanelAddEvent()
			}
			in := ui.TextInput(c, &q.title).Placeholder(placeholder).Label(placeholder).AutoFocus().Bind(&q.field).
				Radius(7).Padding(5, 9)
			if in.Changed() {
				q.added = ""
				// The time the line says shows in the pickers, to change.
				if w := a.quickWhen(first); q.mode == 1 {
					if w.clock {
						q.start = w.start(now)
					}
					if w.minutes > 0 {
						q.minutes = w.minutes
					}
				}
			}
			if in.Submitted() {
				a.quickAddNow(first)
			}
			if q.mode == 1 {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconClock).Size(13, 13).TextColor(p.muted)
					ui.TimeInput(c, &q.start).Label(a.tr.FieldStarts())
					ui.Spacer(c)
					lengths := []int{30, 60, 90, 120, 180}
					if !slices.Contains(lengths, q.minutes) {
						lengths = append(lengths, q.minutes)
						slices.Sort(lengths)
					}
					selectOf(a, c, "quickMinutes", &q.minutes, lengths, a.length).Label(a.tr.Duration())
				})
			}
		})
		ui.Row(c).Padding(2, 4, 0).Children(func() {
			a.label(c, a.tr.OverviewToday())
			ui.Spacer(c)
			if n := a.myDayOpen(); n > 0 {
				ui.Text(c, itoa(n)+" "+a.tr.LeftToday(n)).FontSize(11.5).TextColor(p.muted)
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			a.list(c, func() {
				var events []*Event
				for _, e := range a.data.eventsBetween(day(now), addDays(day(now), 1)) {
					if e.AllDay || e.End.After(now) {
						events = append(events, e)
					}
				}
				var tasks []*Task
				for _, t := range a.data.Tasks {
					if !t.Done && a.inList(t, listMyDay) {
						tasks = append(tasks, t)
					}
				}
				sortTasks(tasks)
				if len(events) == 0 && len(tasks) == 0 {
					a.empty(c, a.tr.PanelNothing(), "")
					return
				}
				for _, e := range events {
					a.panelEventRow(c, e)
				}
				for _, t := range tasks {
					a.panelTaskRow(c, t)
				}
			})
		})
		ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c)
			if a.btn(c, a.tr.PanelOpen(), btnPlain).Clicked() {
				a.showMain()
			}
			if a.btn(c, a.tr.PanelQuit(), btnPlain).Clicked() {
				mygo.App.Quit()
			}
		})
	})
	if c.Shortcut(0, ui.KeyEscape) {
		a.hidePanel()
	}
}

func (a *app) panelEventRow(c *ui.Context, e *Event) {
	p := &a.p
	now := a.now()
	b := ui.ButtonBase(c.Key(e.key())).MinHeight(38).Padding(5, 12).Gap(10).AlignItems(ui.Center).Radius(0).FocusRing(false).
		Label(orUntitled(e.Title, a.tr.Untitled()))
	if b.Hovered() {
		b.Background(p.card2)
	}
	b.Children(func() {
		when := a.tr.AllDay()
		if !e.AllDay {
			when = a.clock(e.Start)
		}
		ui.Text(c, when).FontSize(11.5).FontFeatures("tnum").TextColor(p.muted).Width(60).Shrink(0).SingleLine()
		ui.Box(c).Width(3).Height(20).Radius(1.5).Background(a.kindColor(e.Kind)).Shrink(0)
		ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(12.5).TextColor(p.fg).Grow(1).MinWidth(0).SingleLine()
		if !e.AllDay && !e.Start.After(now) {
			a.softChip(c, a.tr.HappeningNow(), p.green)
		} else if !e.AllDay && e.Start.Sub(now) < 2*time.Hour {
			ui.Text(c, a.until(e.Start)).FontSize(11).TextColor(p.accent).SingleLine()
		}
	})
	if b.Clicked() {
		a.showMain()
		a.openEvent(e.key())
	}
}

func (a *app) panelTaskRow(c *ui.Context, t *Task) {
	p := &a.p
	ui.Row(c.Key(t.ID)).MinHeight(38).Padding(5, 12).Gap(10).AlignItems(ui.Center).Children(func() {
		done := t.Done
		if a.check(c, &done, a.tr.MarkDone()+": "+orUntitled(t.Title, a.tr.Untitled())).Changed() {
			t.setDone(done, a.now())
			a.changed()
		}
		if t.Priority > 0 {
			a.priorityMarks(c, t.Priority).FontSize(12)
		}
		ui.Text(c, orUntitled(t.Title, a.tr.Untitled())).FontSize(12.5).TextColor(p.fg).Grow(1).MinWidth(0).SingleLine()
		switch {
		case !t.Due.IsZero() && day(t.Due).Before(day(a.now())):
			ui.Text(c, a.relDay(t.Due)).FontSize(11).TextColor(p.red).SingleLine()
		case !t.Remind.IsZero() && t.Remind.After(a.now()):
			ui.Row(c).Gap(3).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Icon(c, iconBell).Size(10, 10).TextColor(p.muted)
				ui.Text(c, a.remindText(t.Remind)).FontSize(11).FontFeatures("tnum").TextColor(p.muted).SingleLine()
			})
		}
	})
}

package main

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	hourHeight  = 46 // DIPs an hour takes in the week
	gutter      = 66 // the column of hours
	snapMinutes = 15 // what a drag moves events by
)

// eventDraft is an event being edited in the dialog: a new one, or a copy
// of one saved.
type eventDraft struct {
	id       string // "" for a new event
	title    string
	kind     KindID
	project  string
	location string
	notes    string
	date     time.Time
	start    time.Time
	end      time.Time
	allDay   bool
	alert    int // minutes before, alertNone, or alertDefault
	repeat   Repeat
	until    time.Time
	// occ is the start of the occurrence of a repeating event opened, and
	// all whether a change goes to the whole series rather than to it.
	occ time.Time
	all int
}

// endDay returns the day the draft ends on: an all-day event's last day.
func (d *eventDraft) endDay() time.Time {
	span := daysBetween(d.start, d.end)
	if d.allDay && d.end.Equal(day(d.end)) {
		span--
	}
	return addDays(d.date, max(span, 0))
}

// setEndDay ends the draft on the day t, at the time it ends, keeping it
// from ending before it starts.
func (d *eventDraft) setEndDay(t time.Time) {
	span := max(daysBetween(d.date, t), 0)
	if d.allDay {
		d.end = addDays(day(d.start), span+1)
		return
	}
	d.end = at(addDays(day(d.start), span), d.end.Hour(), d.end.Minute())
	if !d.end.After(d.start) {
		d.end = d.start.Add(time.Hour)
	}
}

// alertDefault, in a draft, follows the settings.
const alertDefault = -2

// alertChoices are the alerts the dialog offers.
var alertChoices = []int{alertDefault, alertNone, 0, 5, 10, 15, 30, 60, 120, 1440}

// eventDrag is a drag of an event in the week: moving it, or resizing it
// from its bottom edge.
type eventDrag struct {
	id     string
	resize bool
	dx, dy float32 // how far the pointer moved since it pressed
}

// newEvent opens the dialog for an event at start, an hour long.
func (a *app) newEvent(start time.Time, project string) {
	if a.page != PageSchedule && a.page != PageProjects && a.page != PageOverview {
		a.navigate(PageSchedule)
	}
	a.evt = eventDraft{date: day(start), start: start, end: start.Add(time.Hour), kind: a.data.newKind(), project: project, alert: alertDefault}
	a.evtOpen = true
}

// editEvent opens the dialog for an event, or for an occurrence of a
// repeating one, which changes it alone unless the user says all.
func (a *app) editEvent(e *Event) {
	if e.Sub != "" {
		a.info, a.infoOpen = e, true // read only: its calendar has it
		return
	}
	alert := alertDefault
	if e.Alert != nil {
		alert = *e.Alert
	}
	a.evt = eventDraft{id: e.ID, title: e.Title, kind: e.Kind, project: e.Project, location: e.Location, notes: e.Notes,
		date: day(e.Start), start: e.Start, end: e.End, allDay: e.AllDay, alert: alert, repeat: e.Repeat, until: e.Until}
	if e.series != nil {
		a.evt.occ = e.Start
	}
	a.evtOpen = true
}

// saveEvent writes the draft to its event, or adds it. For an occurrence
// of a repeating event, it changes that occurrence alone, which leaves the
// series as an event of its own, or the whole series, moved by as many
// days as the occurrence was.
func (a *app) saveEvent() {
	d := &a.evt
	e := a.data.event(d.id)
	series := e != nil && e.repeating() && !d.occ.IsZero()
	switch {
	case e == nil:
		e = &Event{ID: newID()}
		a.data.Events = append(a.data.Events, e)
	case series && d.all == 0:
		a.data.skip(&Event{Start: d.occ, series: e})
		e = &Event{ID: newID()}
		a.data.Events = append(a.data.Events, e)
		d.repeat, d.until = RepeatNone, time.Time{}
	case series:
		// The series moves as the occurrence did, its skipped days and its
		// end too.
		n := daysBetween(d.occ, d.date)
		d.date = addDays(day(e.Start), n)
		for i := range e.Skip {
			e.Skip[i] = addDays(e.Skip[i], n)
		}
		if !e.Until.IsZero() && d.until.Equal(e.Until) {
			d.until = addDays(d.until, n)
		}
	}
	e.Repeat, e.Until = d.repeat, d.until
	if e.Repeat == RepeatNone {
		e.Until, e.Skip = time.Time{}, nil
	}
	e.Title, e.Kind, e.Project = strings.TrimSpace(d.title), d.kind, d.project
	e.Location, e.Notes, e.AllDay = strings.TrimSpace(d.location), d.notes, d.allDay
	e.Alert = nil
	if d.alert != alertDefault {
		n := d.alert
		e.Alert = &n
	}
	// An event keeps the days it goes over.
	span := daysBetween(d.start, d.end)
	if d.allDay {
		if d.end.Hour() != 0 || d.end.Minute() != 0 {
			span++ // a timed event made all-day takes its last day
		}
		e.Start, e.End = day(d.date), addDays(day(d.date), max(span, 1))
	} else {
		e.Start = at(d.date, d.start.Hour(), d.start.Minute())
		e.End = at(addDays(d.date, max(span, 0)), d.end.Hour(), d.end.Minute())
		if !e.End.After(e.Start) {
			// 22:00 to 1:00 ends the next day.
			if next := addDays(e.End, 1); span <= 0 && next.Sub(e.Start) <= 12*time.Hour {
				e.End = next
			} else {
				e.End = e.Start.Add(time.Hour)
			}
		}
	}
	a.evtOpen = false
	a.changed()
}

// moveToDay moves an event to the day d, keeping its times: an occurrence
// of a repeating event goes alone.
func (a *app) moveToDay(e *Event, d time.Time) {
	n := daysBetween(e.Start, d)
	if n == 0 {
		return
	}
	e = a.data.detach(e)
	e.Start, e.End = addDays(e.Start, n), addDays(e.End, n)
	a.changed()
}

func (a *app) repeatName(r Repeat) string {
	switch r {
	case RepeatDaily:
		return a.tr.RepeatDaily()
	case RepeatWeekdays:
		return a.tr.RepeatWeekdays()
	case RepeatWeekly:
		return a.tr.RepeatWeekly()
	case RepeatBiweekly:
		return a.tr.RepeatBiweekly()
	case RepeatMonthly:
		return a.tr.RepeatMonthly()
	}
	return a.tr.RepeatNever()
}

func (a *app) alertName(n int) string {
	switch {
	case n == alertDefault:
		return a.tr.AlertDefault(a.alertName(a.data.Prefs.AlertMinutes))
	case n < 0:
		return a.tr.AlertNone()
	case n == 0:
		return a.tr.AlertAtStart()
	case n == 1440:
		return a.tr.AlertDay()
	case n >= 60 && n%60 == 0:
		return a.tr.AlertHours(n / 60)
	}
	return a.tr.AlertMinutes(n)
}

func (a *app) schedulePage(c *ui.Context) {
	p := &a.p
	first := a.firstWeekday(c)
	var title, sub string
	if a.schedMode == 0 {
		start := weekStart(a.schedAnchor, first)
		title = a.fmt(start, "yMMMM")
		sub = a.fmt(start, "MMMd") + " – " + a.fmt(addDays(start, 6), "MMMd")
	} else {
		title = a.fmt(a.schedAnchor, "yMMMM")
	}
	ui.Row(c).Padding(2, 18, 10).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(600).TextColor(p.fg).SingleLine()
		if sub != "" {
			ui.Text(c, sub).FontSize(13).TextColor(p.muted).SingleLine().Margin(0, 0, 0, 4)
		}
		ui.Spacer(c)
		// The arrows and Today, also with ⌘←, ⌘→ and ⌘T, as in Calendar.
		if a.iconBtn(c, iconChevronLeft, a.tr.Previous()+"  ⌘←").Clicked() || c.Shortcut(ui.Cmd, ui.KeyLeft) {
			a.shiftSchedule(-1)
		}
		if a.btn(c, a.tr.Today(), btnSecondary).Tooltip("⌘T").Clicked() || c.Shortcut(ui.Cmd, ui.KeyT) {
			a.schedAnchor = day(a.now())
			a.schedScrolled = false
		}
		if a.iconBtn(c, iconChevronRight, a.tr.Next()+"  ⌘→").Clicked() || c.Shortcut(ui.Cmd, ui.KeyRight) {
			a.shiftSchedule(1)
		}
		ui.Box(c).Width(6)
		a.segTrack(c, "mode", &a.schedMode, a.tr.ViewWeek(), a.tr.ViewMonth())
		ui.Box(c).Width(6)
		if a.iconLabel(c, a.btn(c, "", btnPrimary), iconPlus, a.tr.NewEvent()).Clicked() {
			a.newEvent(a.nextSlot(), "")
		}
	})
	ui.Column(c).Grow(1).MinHeight(0).Padding(0, 18, 18).Children(func() {
		a.card(c).Grow(1).MinHeight(0).Children(func() {
			if a.schedMode == 0 {
				a.weekView(c, weekStart(a.schedAnchor, first))
			} else {
				a.monthView(c, first)
			}
		})
	})
}

func (a *app) shiftSchedule(n int) {
	if a.schedMode == 0 {
		a.schedAnchor = addDays(a.schedAnchor, 7*n)
		return
	}
	y, m, _ := a.schedAnchor.Date()
	a.schedAnchor = time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, a.schedAnchor.Location())
}

// nextSlot is the next half hour from now; on another day the schedule
// shows, 9 in the morning.
func (a *app) nextSlot() time.Time {
	now := a.now()
	if a.page == PageSchedule && !sameDay(day(a.schedAnchor), day(now)) {
		return at(a.schedAnchor, 9, 0)
	}
	t := at(now, now.Hour(), 0)
	if now.Minute() >= 30 {
		return t.Add(time.Hour)
	}
	return t.Add(30 * time.Minute)
}

// sketch is a new event being drawn by a drag down a day's empty hours,
// from where the pointer pressed to where it is.
type sketch struct {
	day   time.Time
	y0, y float32
}

// sketchRange returns the times the sketch of the day d covers, in steps
// of snapMinutes, at least one long, and whether there is one.
func (a *app) sketchRange(d time.Time) (from, to time.Time, ok bool) {
	s := a.sketch
	if !sameDay(s.day, d) || abs(s.y-s.y0) < 6 {
		return time.Time{}, time.Time{}, false
	}
	step := func(y float32) time.Time {
		n := int(math.Round(float64(y) / hourHeight * 60 / snapMinutes))
		return d.Add(time.Duration(max(0, min(n*snapMinutes, 24*60))) * time.Minute)
	}
	from, to = step(min(s.y0, s.y)), step(max(s.y0, s.y))
	if !to.After(from) {
		to = from.Add(snapMinutes * time.Minute)
	}
	return from, to, true
}

// placed is an event's part in one day, in the lane it takes among those
// it overlaps.
type placed struct {
	e           *Event
	from, to    time.Time
	lane, lanes int
}

// layoutDay places the timed events of the day starting at d in lanes, so
// that those overlapping share the width.
func layoutDay(events []*Event, d time.Time) []placed {
	next := addDays(d, 1)
	var ps []placed
	for _, e := range events {
		if e.AllDay || !e.Start.Before(next) || !e.End.After(d) {
			continue
		}
		ps = append(ps, placed{e: e, from: maxTime(e.Start, d), to: minTime(e.End, next)})
	}
	slices.SortStableFunc(ps, func(x, y placed) int {
		if c := x.from.Compare(y.from); c != 0 {
			return c
		}
		return y.to.Compare(x.to)
	})
	for i := 0; i < len(ps); {
		// A cluster runs while events overlap the ones before them.
		j, end := i, ps[i].to
		var lanes []time.Time
		for ; j < len(ps) && (j == i || ps[j].from.Before(end)); j++ {
			lane := -1
			for l, free := range lanes {
				if !ps[j].from.Before(free) {
					lane = l
					break
				}
			}
			if lane < 0 {
				lane = len(lanes)
				lanes = append(lanes, time.Time{})
			}
			lanes[lane] = ps[j].to
			ps[j].lane = lane
			end = maxTime(end, ps[j].to)
		}
		for k := i; k < j; k++ {
			ps[k].lanes = len(lanes)
		}
		i = j
	}
	return ps
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// dragShift returns how many days and minutes the drag in progress moves
// its event by, in whole days and steps of snapMinutes.
func (a *app) dragShift() (days, minutes int) {
	if !a.drag.resize && a.colW > 0 {
		days = int(math.Round(float64(a.drag.dx / a.colW)))
	}
	minutes = int(math.Round(float64(a.drag.dy)/hourHeight*60/snapMinutes)) * snapMinutes
	return days, minutes
}

// dragged moves or resizes e as the drag in progress says, and ends it:
// an occurrence of a repeating event goes alone.
func (a *app) dragged(e *Event) {
	days, minutes := a.dragShift()
	if days != 0 || minutes != 0 {
		e = a.data.detach(e)
	}
	shift := time.Duration(minutes) * time.Minute
	if a.drag.resize {
		e.End = maxTime(e.End.Add(shift), e.Start.Add(snapMinutes*time.Minute))
	} else {
		e.Start, e.End = addDays(e.Start, days).Add(shift), addDays(e.End, days).Add(shift)
	}
	a.drag = eventDrag{}
	a.changed()
}

// weekView is seven columns of hours from start, with the all-day events
// and the tasks due above them.
func (a *app) weekView(c *ui.Context, start time.Time) {
	p := &a.p
	now := a.now()
	events := a.data.eventsBetween(start, addDays(start, 7))
	days := make([]time.Time, 7)
	for i := range days {
		days[i] = addDays(start, i)
	}
	// The days.
	ui.Row(c).Shrink(0).PaddingY(8).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(gutter).Shrink(0)
		for _, d := range days {
			today := sameDay(d, now)
			ui.Row(c.Key(d.Unix())).Grow(1).Basis(0).MinWidth(0).Gap(6).Padding(0, 8).AlignItems(ui.Center).Children(func() {
				wd := ui.Text(c, a.fmt(d, "EEE")).FontSize(12).TextColor(p.muted).SingleLine()
				n := ui.Text(c, itoa(d.Day())).FontSize(15).FontWeight(500).FontFeatures("tnum").TextColor(p.fg)
				switch {
				case today:
					wd.TextColor(p.accent)
					n.Background(p.accent).TextColor(p.accentFg).Radius(11).Padding(1, 7).FontWeight(600)
				case d.Weekday() == time.Saturday || d.Weekday() == time.Sunday:
					n.TextColor(p.muted)
				}
			})
		}
	})
	// All-day events and the tasks due, which take drops.
	ui.Row(c).Shrink(0).AlignItems(ui.Stretch).Background(p.card2).BorderWidth(1, 0, 1, 0).BorderColor(p.line2).Children(func() {
		ui.Column(c).Width(gutter).Shrink(0).PaddingY(8).Children(func() {
			ui.Text(c, a.tr.AllDay()).FontSize(10.5).TextColor(p.muted).TextAlign(ui.End).Margin(0, 8, 0, 0).SingleLine()
		})
		for _, d := range days {
			cell := ui.Column(c.Key(d.Unix())).Grow(1).Basis(0).MinWidth(0).Padding(4).Gap(2).MinHeight(32).
				BorderWidth(0, 0, 0, 1).BorderColor(p.line2)
			a.dropDay(cell, d)
			cell.Children(func() {
				n := 0
				for _, e := range events {
					if e.AllDay && e.Start.Before(addDays(d, 1)) && e.End.After(d) {
						if n < 3 {
							a.eventChip(c, e, false)
						}
						n++
					}
				}
				for _, t := range a.data.tasksDueOn(d) {
					if n < 3 {
						a.dueChip(c, t)
					}
					n++
				}
				if n > 3 {
					a.muted(c, a.tr.MoreEvents(n-3)).FontSize(10.5).Padding(0, 4)
				}
			})
		}
	})
	// The hours.
	if !a.schedScrolled {
		// To the time now, in the week holding it, else to the morning.
		a.schedScroll.Y = 7.5 * hourHeight
		if !now.Before(start) && now.Before(addDays(start, 7)) {
			a.schedScroll.Y = max(0, float32(now.Hour())+float32(now.Minute())/60-1.5) * hourHeight
		}
		a.schedScrolled = true
	}
	ui.Scroll(c).TrackScroll(&a.schedScroll).Grow(1).Children(func() {
		ui.Row(c).Height(24*hourHeight + 12).Shrink(0).AlignItems(ui.Stretch).PaddingY(6).Children(func() {
			muted := p.muted
			ui.Box(c).Width(gutter).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
				for h := 1; h < 24; h++ {
					span := ui.Span{Text: a.clock(at(start, h, 0)), Size: 10.5, Color: muted, Features: "tnum"}
					w, th := pt.MeasureText(0, span)
					pt.RichText(r.X+r.W-w-8, r.Y+float32(h)*hourHeight-th/2, 0, span)
				}
			})
			for i, d := range days {
				a.dayColumn(c, i, d, events, now)
			}
		})
	})
}

// dayColumn is a day's hours and its events laid out in them, with the
// time now when it is today. A double click adds an event there; events
// move by dragging, and grow or shrink from their bottom edge.
func (a *app) dayColumn(c *ui.Context, i int, d time.Time, events []*Event, now time.Time) {
	p := &a.p
	today := sameDay(d, now)
	col := ui.Box(c.Key(d.Unix())).Grow(1).Basis(0).MinWidth(0).BorderWidth(0, 0, 0, 1).BorderColor(p.line2)
	line := p.line2
	col.Draw(func(pt *ui.Painter, r ui.Rect) {
		if i == 0 {
			a.colW = r.W // as wide as every day, for dragging across them
		}
		for h := 1; h < 24; h++ {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + float32(h)*hourHeight, W: r.W, H: 1}, line, 0)
		}
	})
	if col.DoubleClicked() {
		if _, y, ok := col.PointerPosition(); ok {
			half := int(y / (hourHeight / 2))
			a.newEvent(at(d, min(half/2, 23), (half%2)*30), "")
		}
	}
	// A drag down empty hours sketches a new event over them, as Calendar
	// does, which the dialog opens on as the pointer lets go.
	if _, dy, ok := col.Dragged(); ok && dy != 0 && a.drag.id == "" {
		_, y, _ := col.PointerPosition()
		if !sameDay(a.sketch.day, d) {
			a.sketch = sketch{day: d, y0: y - dy}
		}
		a.sketch.y = y
	}
	from, to, sketching := a.sketchRange(d)
	if _, _, ended := col.DragEnded(); ended && sketching {
		a.sketch = sketch{}
		a.newEvent(from, "")
		a.evt.end = to
	}
	// A task or an all-day event dragged down from above into the hours
	// takes an hour there: for a task, an hour blocked to work on it.
	var block time.Time
	var blockTitle string
	if v, over := ui.DragOver[dated](col); over {
		if _, y, ok := col.PointerPosition(); ok {
			block, blockTitle = blockStart(d, y), titleOf(v)
		}
	}
	if v, ok := ui.Drop[dated](col); ok {
		if _, y, ok := col.PointerPosition(); ok {
			a.timeBlock(v, blockStart(d, y))
		}
	}
	col.Children(func() {
		for _, pl := range layoutDay(events, d) {
			a.eventBlock(c, pl, d)
		}
		if !block.IsZero() {
			top := float32(block.Sub(d).Minutes()) / 60 * hourHeight
			ui.Column(c).Absolute().Top(top+1).Left(2).Right(2).Height(hourHeight-2).Padding(3, 7).Gap(1).Radius(5).PassThrough().
				Background(a.p.accent.Alpha(0.16)).Border(1, a.p.accent.Alpha(0.6)).Children(func() {
				ui.Text(c, orUntitled(blockTitle, a.tr.Untitled())).FontSize(11).FontWeight(600).TextColor(a.ink(a.p.accent)).SingleLine()
				ui.Text(c, a.clock(block)+" – "+a.clock(block.Add(time.Hour))).FontSize(10.5).FontFeatures("tnum").TextColor(a.ink(a.p.accent))
			})
		}
		if sketching {
			top := float32(from.Sub(d).Minutes()) / 60 * hourHeight
			h := float32(to.Sub(from).Minutes()) / 60 * hourHeight
			ui.Column(c).Absolute().Top(top+1).Left(2).Right(2).Height(h-2).Padding(3, 7).Radius(5).PassThrough().
				Background(p.accent.Alpha(0.16)).Border(1, p.accent.Alpha(0.6)).Children(func() {
				ui.Text(c, a.clock(from)+" – "+a.clock(to)).FontSize(10.5).FontWeight(500).FontFeatures("tnum").TextColor(a.ink(p.accent))
			})
		}
		if today {
			y := float32(now.Sub(d).Minutes()) / 60 * hourHeight
			ui.Box(c).Absolute().Top(y - 0.75).Left(0).Right(0).Height(1.5).Background(p.red).PassThrough()
			ui.Box(c).Absolute().Top(y-4).Left(-4).Size(8, 8).Radius(4).Background(p.red).PassThrough()
			c.After(time.Minute)
		}
	})
}

// eventBlock is an event's part in a day of the week: a tinted block with
// a bar of its kind's color, which a drag moves and its bottom edge
// resizes, both showing where it will go.
func (a *app) eventBlock(c *ui.Context, pl placed, d time.Time) {
	e := pl.e
	col := a.kindColor(e.Kind)
	from, to := pl.from, pl.to
	top := float32(from.Sub(d).Minutes()) / 60 * hourHeight
	h := max(float32(to.Sub(from).Minutes())/60*hourHeight, 18)
	w := 100 / float32(pl.lanes)
	key := e.key() // one occurrence of a series drags alone
	b := ui.ButtonBase(c.Key(key)).Absolute().Top(top+1).LeftPercent(w*float32(pl.lane)).WidthPercent(w).
		Height(h-2).Padding(0, 2).FocusRing(false).Cursor(ui.CursorPointer).Label(orUntitled(e.Title, a.tr.Untitled()))
	// A drag of the block moves the event, unless a subscription has it.
	if dx, dy, ok := b.Dragged(); ok && (dx != 0 || dy != 0) && !(a.drag.id == key && a.drag.resize) && e.Sub == "" {
		if a.drag.id != key {
			a.drag = eventDrag{id: key}
		}
		a.drag.dx += dx
		a.drag.dy += dy
	}
	dragging := a.drag.id == key
	if dragging {
		days, minutes := a.dragShift()
		shift := time.Duration(minutes) * time.Minute
		if a.drag.resize {
			to = maxTime(to.Add(shift), from.Add(snapMinutes*time.Minute))
			b.Height(float32(to.Sub(from).Minutes())/60*hourHeight - 2)
		} else {
			from, to = from.Add(shift), to.Add(shift)
			b.Translate(float32(days)*a.colW, float32(minutes)/60*hourHeight)
		}
	}
	b.Children(func() {
		inner := ui.Column(c).Fill().Padding(3, 6, 3, 7).Gap(0).Clip().Radius(5).Background(a.tint(col)).
			BorderWidth(0, 0, 0, 3).BorderColor(col)
		switch {
		case dragging:
			inner.Shadow(0, 6, 16, 0, ui.RGBA(0, 0, 0, 0.18)).Background(a.tint(col).Mix(col, 0.12))
		case b.Hovered():
			inner.Background(a.tint(col).Mix(col, 0.06))
		}
		fg := a.ink(col)
		inner.Children(func() {
			ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(12).FontWeight(500).TextColor(fg).MaxLines(2)
			if h > 34 || dragging {
				ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
					ui.Text(c, a.clock(from)+" – "+a.clock(to)).FontSize(10.5).FontFeatures("tnum").TextColor(fg.Alpha(0.75)).SingleLine()
					if e.series != nil {
						ui.Icon(c, iconRepeat).Size(10, 10).TextColor(fg.Alpha(0.6))
					}
				})
			}
			if e.Location != "" && h > 62 {
				ui.Text(c, e.Location).FontSize(10.5).TextColor(fg.Alpha(0.75)).SingleLine()
			}
		})
		// The bottom edge resizes the event.
		if e.Sub != "" {
			return
		}
		edge := ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(7).Cursor(ui.CursorResizeNS)
		if _, dy, ok := edge.Dragged(); ok && dy != 0 {
			if a.drag.id != key || !a.drag.resize {
				a.drag = eventDrag{id: key, resize: true}
			}
			a.drag.dy += dy
		}
		if _, _, ended := edge.DragEnded(); ended && a.drag.id == key {
			a.dragged(e)
		}
	})
	_, _, ended := b.DragEnded()
	if ended && a.drag.id == key {
		if abs(a.drag.dx) < 4 && abs(a.drag.dy) < 4 {
			// A press that hardly moved is a click.
			a.drag = eventDrag{}
			a.editEvent(e)
		} else {
			a.dragged(e)
		}
	}
	a.eventMenu(c, b, e)
	if b.Clicked() && !ended && !dragging {
		a.editEvent(e)
	}
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// dated is what drags onto a day: an event, or a task due then.
type dated interface{ dated() }

func (*Event) dated() {}
func (*Task) dated()  {}

// blockStart is the quarter hour of the day d at y in its column, where a
// block of an hour dropped there starts.
func blockStart(d time.Time, y float32) time.Time {
	m := int(y/hourHeight*60) / snapMinutes * snapMinutes
	return d.Add(time.Duration(max(0, min(m, 23*60))) * time.Minute)
}

func titleOf(v dated) string {
	switch v := v.(type) {
	case *Event:
		return v.Title
	case *Task:
		return v.Title
	}
	return ""
}

// timeBlock gives an hour from start to what was dropped in the hours: a
// task gets an event of focus on it, of its project; an all-day event
// becomes that hour, an occurrence of a series alone.
func (a *app) timeBlock(v dated, start time.Time) {
	switch v := v.(type) {
	case *Task:
		kind := kindFocus
		if a.data.kind(kind) == nil {
			kind = a.data.newKind()
		}
		a.data.Events = append(a.data.Events, &Event{ID: newID(), Title: v.Title, Kind: kind, Project: v.Project,
			Start: start, End: start.Add(time.Hour)})
	case *Event:
		if v.Sub != "" {
			return
		}
		e := a.data.detach(v)
		e.AllDay, e.Start, e.End = false, start, start.Add(time.Hour)
	}
	a.changed()
}

// dropDay makes cell take events and tasks dropped on it, for the day d:
// an event moves there keeping its times, a task falls due then. (An
// element takes values of one type, which both are.)
func (a *app) dropDay(cell ui.Element, d time.Time) {
	if _, over := ui.DragOver[dated](cell); over {
		cell.Background(a.p.accentSoft)
	}
	switch v, _ := ui.Drop[dated](cell); v := v.(type) {
	case *Event:
		if v.Sub == "" {
			a.moveToDay(v, d)
		}
	case *Task:
		v.Due = day(d)
		a.changed()
	}
}

func (a *app) eventMenu(c *ui.Context, b ui.Element, e *Event) {
	b.ContextMenu(func(m *ui.Menu) {
		if m.Item(a.tr.Edit()).Chosen() {
			a.editEvent(e)
		}
		if e.Sub != "" {
			return // read only
		}
		m.Separator()
		if e.series != nil {
			if m.Item(a.tr.DeleteThisEvent()).Chosen() {
				a.deleteEvent(c, e, false)
			}
			if m.Item(a.tr.DeleteAllEvents()).Chosen() {
				a.deleteEvent(c, e, true)
			}
			return
		}
		if m.Item(a.tr.Delete()).Chosen() {
			a.deleteEvent(c, e, true)
		}
	})
}

// eventChip is an event on one line, as the all-day row and the month
// show them, which drags to another day.
func (a *app) eventChip(c *ui.Context, e *Event, withTime bool) {
	p := &a.p
	b := ui.ButtonBase(c.Key(e.key())).Height(20).Gap(5).Padding(0, 5).AlignItems(ui.Center).Justify(ui.Start).AlignSelf(ui.Stretch).
		Radius(4).FocusRing(false).Label(orUntitled(e.Title, a.tr.Untitled()))
	if e.Sub == "" {
		b.Drag(e) // to another day; a subscription's stays
	}
	col := a.kindColor(e.Kind)
	switch {
	case e.AllDay:
		b.Background(a.tint(col))
	case b.Hovered():
		b.Background(p.pill)
	}
	if b.Dragging() {
		b.Opacity(0.4)
	}
	b.Children(func() {
		a.dot(c, col, 7)
		if withTime && !e.AllDay {
			ui.Text(c, a.clock(e.Start)).FontSize(10.5).FontFeatures("tnum").TextColor(p.muted).SingleLine().Shrink(0)
		}
		ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(11.5).TextColor(p.fg).SingleLine().MinWidth(0)
	})
	a.eventMenu(c, b, e)
	if b.Clicked() {
		a.editEvent(e)
	}
}

// dueChip is a task due that day, a round box before its title, which
// drags to another day.
func (a *app) dueChip(c *ui.Context, t *Task) {
	p := &a.p
	b := ui.ButtonBase(c.Key("task:"+t.ID)).Height(20).Gap(5).Padding(0, 5).AlignItems(ui.Center).Justify(ui.Start).AlignSelf(ui.Stretch).
		Radius(4).FocusRing(false).Drag(t).Label(orUntitled(t.Title, a.tr.Untitled())).Tooltip(a.tr.DueLabel())
	if b.Hovered() {
		b.Background(p.pill)
	}
	if b.Dragging() {
		b.Opacity(0.4)
	}
	col := p.fg2
	if t.overdue(a.now()) {
		col = p.red
	}
	b.Children(func() {
		mark := ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Border(1.2, col)
		if t.Done {
			mark.Background(p.faint).Border(1.2, p.faint)
		}
		tt := ui.Text(c, orUntitled(t.Title, a.tr.Untitled())).FontSize(11.5).TextColor(col).SingleLine().MinWidth(0)
		if t.Done {
			tt.Strikethrough().TextColor(p.muted)
		}
	})
	if b.Clicked() {
		a.openTask(t.ID)
	}
}

// monthView is the month of the anchor in six weeks. A click chooses a
// day, a double click adds an event, the day's number opens its week, and
// events and tasks drag between days.
func (a *app) monthView(c *ui.Context, first time.Weekday) {
	p := &a.p
	now := a.now()
	y, m, _ := a.schedAnchor.Date()
	month := time.Date(y, m, 1, 0, 0, 0, 0, a.schedAnchor.Location())
	start := weekStart(month, first)
	events := a.data.eventsBetween(start, addDays(start, 42))
	ui.Row(c).Shrink(0).Padding(8, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.line2).Children(func() {
		for i := 0; i < 7; i++ {
			ui.Text(c, a.fmt(addDays(start, i), "EEE")).FontSize(12).TextColor(p.muted).Grow(1).Basis(0).Padding(0, 10).SingleLine()
		}
	})
	tracks := make([]ui.Track, 6)
	for i := range tracks {
		tracks[i] = ui.Fr(1)
	}
	ui.Grid(c).Columns(7).RowTracks(tracks...).Grow(1).MinHeight(0).Children(func() {
		for i := 0; i < 42; i++ {
			d := addDays(start, i)
			inMonth := d.Month() == m
			cell := ui.ButtonBase(c.Key(d.Unix())).Column().Padding(5, 5).Gap(1).AlignItems(ui.Stretch).Justify(ui.Start).
				MinHeight(0).Clip().Radius(0).FocusRing(false).Label(a.fmt(d, "yMMMd"))
			if i%7 != 6 {
				cell.BorderWidth(0, 1, 0, 0).BorderColor(p.line2)
			}
			if i >= 7 {
				cell.BorderWidth(1, 0, 0, 0).BorderColor(p.line2)
				if i%7 != 6 {
					cell.BorderWidth(1, 1, 0, 0)
				}
			}
			switch {
			case sameDay(d, a.schedAnchor):
				cell.Background(p.sel.Alpha(0.6))
			case !inMonth:
				cell.Background(p.card2)
			}
			a.dropDay(cell, d)
			cell.Children(func() {
				ui.Row(c).Padding(0, 0, 2).Children(func() {
					open := ui.ButtonBase(c).Radius(11).FocusRing(false).Label(a.fmt(d, "yMMMMd")).Tooltip(a.tr.ViewWeek())
					open.Children(func() {
						n := ui.Text(c, itoa(d.Day())).FontSize(12.5).FontWeight(500).FontFeatures("tnum").Padding(1, 6).Radius(10).TextColor(p.fg)
						switch {
						case sameDay(d, now):
							n.Background(p.accent).TextColor(p.accentFg).FontWeight(600)
						case open.Hovered():
							n.Background(p.pill)
						case !inMonth:
							n.TextColor(p.faint)
						}
					})
					if open.Clicked() {
						a.schedAnchor, a.schedMode, a.schedScrolled = d, 0, false
					}
				})
				shown, more := 0, 0
				for _, e := range events {
					if e.Start.Before(addDays(d, 1)) && e.End.After(d) || sameDay(e.Start, d) {
						if shown < 3 {
							a.eventChip(c, e, true)
							shown++
						} else {
							more++
						}
					}
				}
				for _, t := range a.data.tasksDueOn(d) {
					if shown < 3 {
						a.dueChip(c, t)
						shown++
					} else {
						more++
					}
				}
				if more > 0 {
					m := ui.ButtonBase(c.Key("more")).Radius(4).Padding(1, 5).AlignSelf(ui.Start).FocusRing(false)
					if m.Hovered() {
						m.Background(p.pill)
					}
					m.Children(func() { ui.Text(c, a.tr.MoreEvents(more)).FontSize(10.5).TextColor(p.muted) })
					if m.Clicked() {
						a.schedAnchor, a.schedMode, a.schedScrolled = d, 0, false
					}
				}
			})
			if cell.DoubleClicked() {
				a.newEvent(at(d, 9, 0), "")
			} else if cell.Clicked() {
				a.schedAnchor = d
			}
		}
	})
}

// eventDialog edits a.evt while a.evtOpen.
func (a *app) eventDialog(c *ui.Context) {
	p := &a.p
	d := &a.evt
	a.modal(c, &a.evtOpen, 500, func() {
		label := a.tr.NewEvent()
		if d.id != "" {
			label = a.tr.EditEvent()
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(p.fg).Grow(1)
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.evtOpen = false
			}
		})
		ui.TextInput(c, &d.title).Placeholder(a.tr.EventTitlePlaceholder()).Label(a.tr.FieldTitle()).AutoFocus().
			FontSize(15).Radius(8).Padding(7, 10)
		// The tags, of which a click chooses one, or none chosen again.
		ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			for _, k := range a.data.Kinds {
				col := a.kindColor(k.ID)
				on := d.kind == k.ID
				b := ui.ButtonBase(c.Key(k.ID)).Height(26).Gap(6).Padding(0, 10).AlignItems(ui.Center).Radius(999).
					Role(ui.RoleRadio).Selected(on).Label(orUntitled(k.Name, a.tr.Untitled()))
				switch {
				case on:
					b.Background(a.tint(col)).Border(1, col.Alpha(0.6))
				case b.Hovered():
					b.Background(p.pillHover)
				default:
					b.Background(p.pill)
				}
				b.Children(func() {
					a.dot(c, col, 8)
					t := ui.Text(c, orUntitled(k.Name, a.tr.Untitled())).FontSize(12.5).TextColor(p.fg2)
					if on {
						t.TextColor(a.ink(col)).FontWeight(500)
					}
				})
				if b.Clicked() {
					if on {
						d.kind = ""
					} else {
						d.kind = k.ID
					}
				}
			}
			if a.iconBtn(c, iconTag, a.tr.EditKinds()).Clicked() {
				a.kindsOpen = true
			}
		})
		ui.Column(c).Background(p.card2).Border(1, p.line2).Radius(10).Dividers(1, p.line2).Gap(1).Children(func() {
			a.formRow(c, a.tr.FieldAllDay(), func() {
				// An all-day event made timed starts at nine, for an hour.
				if ui.Switch(c, &d.allDay).Label(a.tr.FieldAllDay()).Changed() && !d.allDay && d.start.Equal(day(d.start)) {
					d.start = at(d.date, 9, 0)
					d.end = d.start.Add(time.Hour)
				}
			})
			// Starts and ends, each on its day, as in Calendar.
			a.formRow(c, a.tr.FieldStarts(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.DateInput(c, &d.date).Label(a.tr.FieldStartDate()).Width(140)
					if !d.allDay && ui.TimeInput(c, &d.start).Label(a.tr.FieldStarts()).Changed() && !d.end.After(d.start) {
						d.end = d.start.Add(time.Hour)
					}
				})
			})
			a.formRow(c, a.tr.FieldEnds(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					end := d.endDay()
					if ui.DateInput(c, &end).Label(a.tr.FieldEndDate()).Width(140).Changed() {
						d.setEndDay(end)
					}
					if !d.allDay {
						ui.TimeInput(c, &d.end).Label(a.tr.FieldEnds())
					}
				})
			})
			if !d.allDay {
				a.formRow(c, a.tr.FieldAlert(), func() {
					selectOf(a, c, "alert", &d.alert, alertChoices, a.alertName).Label(a.tr.FieldAlert())
				})
			}
			if d.occ.IsZero() || d.all == 1 {
				a.formRow(c, a.tr.FieldRepeat(), func() {
					selectOf(a, c, "repeat", &d.repeat, repeats, a.repeatName).Label(a.tr.FieldRepeat())
				})
				if d.repeat != RepeatNone {
					a.formRow(c, a.tr.FieldRepeatEnd(), func() { a.dateField(c, "until", &d.until) })
				}
			}
			a.formRow(c, a.tr.FieldProject(), func() {
				selectOf(a, c, "evtProject", &d.project, a.projectChoices(d.project), a.projectName).MaxWidth(220).Label(a.tr.FieldProject())
			})
			a.formRow(c, a.tr.FieldLocation(), func() {
				ui.TextInput(c, &d.location).Placeholder(a.tr.EventLocationPlaceholder()).Label(a.tr.FieldLocation()).Width(220).Radius(7).Padding(4, 9)
			})
		})
		ui.TextArea(c, &d.notes).Placeholder(a.tr.FieldNotes()).Label(a.tr.FieldNotes()).MinHeight(64).Radius(8)
		if !d.occ.IsZero() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconRepeat).Size(14, 14).TextColor(p.muted)
				ui.Text(c, a.repeatName(d.repeat)).FontSize(12.5).TextColor(p.fg2).Grow(1)
				ui.Text(c, a.tr.ApplyTo()).FontSize(12.5).TextColor(p.muted)
				a.segTrack(c, "scope", &d.all, a.tr.ApplyThis(), a.tr.ApplyAll())
			})
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if d.id != "" && a.btn(c, a.tr.Delete(), btnDanger).Clicked() {
				if e := a.data.event(d.id); e != nil && d.occ.IsZero() {
					a.evtOpen = false
					a.deleteEvent(c, e, true)
				} else {
					a.askEvent = true
				}
			}
			ui.Spacer(c)
			if a.btn(c, a.tr.Cancel(), btnSecondary).Clicked() {
				a.evtOpen = false
			}
			label := a.tr.Create()
			if d.id != "" {
				label = a.tr.Save()
			}
			if a.btn(c, label, btnPrimary).Clicked() {
				a.saveEvent()
			}
		})
		if c.Shortcut(ui.Cmd, ui.KeyEnter) {
			a.saveEvent()
		}
	})
	if !d.occ.IsZero() {
		// A repeating event: this occurrence, or all of them.
		choice := ui.AlertDialog(c, &a.askEvent, a.tr.EventDeleteRepeat(), orUntitled(d.title, a.tr.Untitled()),
			a.tr.Cancel(), a.tr.DeleteAllEvents(), a.tr.DeleteThisEvent())
		if e := a.data.event(d.id); e != nil && choice >= 1 {
			a.evtOpen = false
			a.deleteEvent(c, &Event{ID: e.ID, Title: e.Title, Start: d.occ, series: e}, choice == 1)
		}
	}
}

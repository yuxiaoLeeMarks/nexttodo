package main

import (
	"strings"
	"testing"
	"time"
)

// A series set to end before it begins comes once, rather than never; and
// its end, set, starts on its day, not today.
func TestSeriesEndBeforeStart(t *testing.T) {
	a := testApp("en")
	a.newEvent(at(addDays(monday, 14), 9, 0), "")
	a.evt.title, a.evt.repeat, a.evt.until = "Reading group", RepeatWeekly, day(monday)
	a.saveEvent()
	e := eventTitled(a, "Reading group")
	if occ := e.occurrences(day(monday), addDays(day(monday), 120)); len(occ) != 1 {
		t.Errorf("%d occurrences, ending %v", len(occ), e.Until)
	}
}

// Making a series once, from a later occurrence, leaves it on that
// occurrence's day, not the series' first, which was deleted.
func TestSeriesMadeOnceFromLater(t *testing.T) {
	a := testApp("en")
	series := eventTitled(a, "Group meeting")
	series.Skip = []time.Time{day(series.Start)}
	later := series.occurrences(addDays(day(monday), 7), addDays(day(monday), 8))[0]
	a.editEvent(later)
	a.evt.all, a.evt.repeat = 1, RepeatNone
	a.saveEvent()
	if !sameDay(series.Start, later.Start) || series.Repeat != RepeatNone {
		t.Errorf("made once on %v", series.Start)
	}
}

// A press that hardly moves leaves no sketch where the next begins.
func TestSketchAfterJiggle(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	tt := newTester(t, a, "en")
	r, _ := tt.Find("Hangxin weekly sync") // Tuesday, 10:00
	top, x := r.Y-4, r.X+r.W/2
	y12 := top + 2*hourHeight + 4 // free, as 17:00 is
	tt.Press(x, y12)
	tt.Move(x, y12+3)
	tt.Release(x, y12+3)
	tt.Frame()
	if a.evtOpen {
		t.Fatal("the jiggle opened an event")
	}
	drag(tt, x, top+7*hourHeight+4, 0, hourHeight)
	tue := addDays(monday, 1)
	if !a.evtOpen || !a.evt.start.Equal(at(tue, 17, 0)) || !a.evt.end.Equal(at(tue, 18, 0)) {
		t.Errorf("sketched %v – %v", a.evt.start.Format("15:04"), a.evt.end.Format("15:04"))
	}
}

// An all-day event of days, dragged by its last day to the next, moves a
// day, not by as far as its first day is from there.
func TestDragAllDayByLaterDay(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	e := &Event{ID: "trip", Title: "Field trip", AllDay: true, Start: day(monday), End: addDays(day(monday), 3)}
	a.data.Events = append(a.data.Events, e)
	tt := newTester(t, a, "en")
	r, ok := tt.Find("Field trip") // Monday's
	if !ok || a.colW == 0 {
		t.Fatal("no trip in the week")
	}
	wed := r.X + r.W/2 + 2*a.colW
	drag(tt, wed, r.Y+r.H/2, a.colW, 0)
	if !sameDay(e.Start, addDays(monday, 1)) || !sameDay(addDays(e.End, -1), addDays(monday, 3)) {
		t.Errorf("moved to %v – %v, want Tue – Thu", e.Start.Format("Mon"), addDays(e.End, -1).Format("Mon"))
	}
}

// The part of an event going on into the next day has no edge resizing
// it, which would move where the event ends, not where the part does.
func TestNoResizeOfPartGoingOn(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	e := &Event{ID: "night", Title: "Night run", Start: at(monday, 22, 0), End: at(addDays(monday, 1), 2, 0)}
	a.data.Events = append(a.data.Events, e)
	a.schedScroll.Y, a.schedScrolled = 16*hourHeight, true
	tt := newTester(t, a, "en")
	r, ok := tt.Find("Night run")
	if !ok {
		t.Fatal("no night run")
	}
	bottom := r.Y - 4 + 2*hourHeight - 3 // Monday's part ends at midnight
	drag(tt, r.X+r.W/2, bottom, 0, -hourHeight)
	if e.End.Sub(e.Start) != 4*time.Hour {
		t.Errorf("a drag at the bottom of Monday's part made it %v – %v", e.Start, e.End)
	}
}

// The hours of a day are those of the clock, the day the clocks change too:
// 9:00 is at the 9:00 line, and the 9:00 line is 9:00.
func TestWeekOnClockChange(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	for _, d := range []time.Time{time.Date(2026, 3, 8, 0, 0, 0, 0, ny), time.Date(2026, 11, 1, 0, 0, 0, 0, ny)} {
		nine := time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, ny)
		if m := dayMinutes(nine, d); m != 9*60 {
			t.Errorf("%v: 9:00 at %v minutes", d.Format("Jan 2"), m)
		}
		if got := blockStart(d, 9*hourHeight+1); !got.Equal(nine) {
			t.Errorf("%v: the 9:00 line is %v", d.Format("Jan 2"), got.Format("15:04"))
		}
		if got := addMinutes(nine, 60); got.Hour() != 10 {
			t.Errorf("%v: an hour after 9:00 is %v", d.Format("Jan 2"), got.Format("15:04"))
		}
		if dayMinutes(addDays(d, 1), d) != 24*60 || dayMinutes(addDays(d, -1), d) != 0 {
			t.Error("the day's ends")
		}
	}
}

// A sketch begun in a day's last minutes stays in the day.
func TestSketchAtDayEnd(t *testing.T) {
	a := testApp("en")
	d := day(monday)
	a.sketch = sketch{day: d, y0: 24*hourHeight - 2, y: 24*hourHeight + 20}
	from, to, ok := a.sketchRange(d)
	if !ok || !sameDay(from, d) || to.After(addDays(d, 1)) || !to.After(from) {
		t.Errorf("sketched %v – %v", from, to)
	}
}

// A file imported leaves the events of a calendar subscribed to as they
// are, though it has their UIDs; the next refresh does not double them.
func TestImportBesideSubscription(t *testing.T) {
	a := testApp("en")
	sub := &Event{ID: "c", UID: "colloquium@dept", Title: "Colloquium", Sub: "s", Start: at(monday, 16, 0), End: at(monday, 17, 0)}
	a.data.Events = append(a.data.Events, sub)
	mine := &Event{UID: "colloquium@dept", Title: "Colloquium (my notes)", Start: at(monday, 16, 0), End: at(monday, 17, 0)}
	a.data.importEvents([]*Event{mine})
	if sub.Sub != "s" || sub.Title != "Colloquium" {
		t.Errorf("the subscribed event became %+v", sub)
	}
}

// An occurrence, alerted, then changed on its own, does not alert again;
// nor does an event of a calendar without UIDs at each refresh; and an
// alert three days ahead comes then.
func TestAlertsOnce(t *testing.T) {
	a := testApp("en")
	a.useTestLanguage("en")
	series := eventTitled(a, "Group meeting") // Mondays 9:30, the default 10 minutes before
	a.alertedTo = at(monday, 9, 0)
	if got := a.dueAlerts(at(monday, 9, 20)); len(got) != 1 {
		t.Fatalf("alerts at 9:20: %+v", got)
	}
	occ := series.occurrences(day(monday), addDays(day(monday), 1))[0]
	a.editEvent(occ)
	a.evt.notes = "Bring the poster"
	a.saveEvent()
	if got := a.dueAlerts(at(monday, 9, 21)); len(got) != 0 {
		t.Errorf("the occurrence changed alerted again: %+v", got)
	}

	b := testApp("en")
	b.useTestLanguage("en")
	s := &Subscription{ID: "s", Name: "Dept", Alerts: true}
	b.data.Subscriptions = []*Subscription{s}
	feed := func() icsImport {
		return icsImport{events: []*Event{{ID: newID(), Title: "Talk", Start: at(monday, 15, 0), End: at(monday, 16, 0)}}}
	}
	b.data.applySubscription(s, feed(), monday)
	b.alertedTo = at(monday, 14, 0)
	n := len(b.dueAlerts(at(monday, 14, 50)))
	b.data.applySubscription(s, feed(), monday)
	if again := b.dueAlerts(at(monday, 14, 52)); n != 1 || len(again) != 0 {
		t.Errorf("%d alerts, then %d after a refresh", n, len(again))
	}

	c := testApp("en")
	c.useTestLanguage("en")
	lead := 3 * 24 * 60
	far := &Event{ID: "far", Title: "Grant deadline", Start: at(addDays(monday, 4), 12, 0), End: at(addDays(monday, 4), 13, 0), Alert: &lead}
	c.data.Events = []*Event{far}
	c.alertedTo = at(addDays(monday, 1), 11, 0)
	if got := c.dueAlerts(at(addDays(monday, 1), 12, 0)); len(got) != 1 || !strings.Contains(got[0].subtitle, "4,320") && !strings.Contains(got[0].subtitle, "4320") {
		t.Errorf("three days ahead: %+v", got)
	}
}

// The alert of an occurrence gone opens its week, not the series to edit.
func TestOpenAlertOfOccurrenceGone(t *testing.T) {
	a := testApp("en")
	series := eventTitled(a, "Group meeting")
	occ := series.occurrences(day(monday), addDays(day(monday), 1))[0]
	ref := occ.key()
	a.data.skip(occ)
	a.openEvent(ref)
	if a.evtOpen || a.page != PageSchedule || !sameDay(a.schedAnchor, monday) {
		t.Errorf("opened %v on %v", a.evtOpen, a.schedAnchor)
	}
}

// An event of days, read only, says the day it ends.
func TestInfoOfDays(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	a.info = &Event{ID: "conf", Title: "Conference", Sub: "s", Start: at(monday, 9, 0), End: at(addDays(monday, 2), 17, 0)}
	a.infoOpen = true
	tt := newTester(t, a, "en")
	found := false
	for _, s := range tt.Texts() {
		if strings.Contains(s, "Oct 28") {
			found = true
		}
	}
	if !found {
		t.Errorf("no end day: %q", tt.Texts())
	}
}

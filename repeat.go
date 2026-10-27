package main

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// Repeat is how an event repeats: "" not at all.
type Repeat string

const (
	RepeatNone     Repeat = ""
	RepeatDaily    Repeat = "daily"
	RepeatWeekdays Repeat = "weekdays"
	RepeatWeekly   Repeat = "weekly"
	RepeatBiweekly Repeat = "biweekly"
	RepeatMonthly  Repeat = "monthly"
)

var repeats = []Repeat{RepeatNone, RepeatDaily, RepeatWeekdays, RepeatWeekly, RepeatBiweekly, RepeatMonthly}

// repeating reports whether e repeats in a way this version knows: one it
// does not, from a later version, is taken as once.
func (e *Event) repeating() bool { return e.Repeat != RepeatNone && slices.Contains(repeats, e.Repeat) }

// root is the event an occurrence repeats, or e itself.
func (e *Event) root() *Event {
	if e.series != nil {
		return e.series
	}
	return e
}

// key names e among the events of a list: an occurrence by its series and
// its start, as one series shows many times.
func (e *Event) key() string {
	if e.series == nil {
		return e.ID
	}
	return e.ID + "@" + strconv.FormatInt(e.Start.Unix(), 10)
}

// skipped reports whether the occurrence starting on t's day was removed.
func (e *Event) skipped(t time.Time) bool {
	return slices.ContainsFunc(e.Skip, func(s time.Time) bool { return sameDay(s, t) })
}

// nth returns the start of e's nth occurrence, and whether there is one:
// a monthly event on the 31st skips the shorter months.
func (e *Event) nth(n int) (time.Time, bool) {
	switch e.Repeat {
	case RepeatDaily:
		return addDays(e.Start, n), true
	case RepeatWeekly:
		return addDays(e.Start, 7*n), true
	case RepeatBiweekly:
		return addDays(e.Start, 14*n), true
	case RepeatMonthly:
		y, m, d := e.Start.Date()
		t := time.Date(y, m+time.Month(n), d, e.Start.Hour(), e.Start.Minute(), 0, 0, e.Start.Location())
		return t, t.Day() == d
	}
	return e.Start, n == 0
}

// occurrences returns the occurrences of a repeating event overlapping
// [from, to), as copies of it pointing back to it.
func (e *Event) occurrences(from, to time.Time) []*Event {
	if !e.repeating() {
		return nil
	}
	dur := e.End.Sub(e.Start)
	// An all-day event lasts days, which a change of clocks makes 23 or 25
	// hours long.
	end := func(start time.Time) time.Time {
		if e.AllDay {
			return addDays(start, daysBetween(e.Start, e.End))
		}
		return start.Add(dur)
	}
	// Start from about where the range does, rather than from the first.
	n := 0
	switch e.Repeat {
	case RepeatDaily, RepeatWeekdays:
		n = daysBetween(e.Start, from.Add(-dur)) - 1
	case RepeatWeekly:
		n = daysBetween(e.Start, from.Add(-dur))/7 - 1
	case RepeatBiweekly:
		n = daysBetween(e.Start, from.Add(-dur))/14 - 1
	case RepeatMonthly:
		n = (from.Year()-e.Start.Year())*12 + int(from.Month()-e.Start.Month()) - 1
	}
	var out []*Event
	for n = max(n, 0); ; n++ {
		var start time.Time
		ok := true
		if e.Repeat == RepeatWeekdays {
			start = addDays(e.Start, n)
			ok = start.Weekday() != time.Saturday && start.Weekday() != time.Sunday
		} else {
			start, ok = e.nth(n)
		}
		if !start.Before(to) || !e.Until.IsZero() && day(start).After(day(e.Until)) {
			break
		}
		if !ok || !end(start).After(from) && !start.Equal(from) || e.skipped(start) {
			continue
		}
		occ := *e
		occ.Start, occ.End, occ.series = start, end(start), e
		occ.Skip = nil
		out = append(out, &occ)
	}
	return out
}

// skip removes the occurrence occ from its series.
func (d *Data) skip(occ *Event) {
	s := occ.root()
	if !s.skipped(occ.Start) {
		s.Skip = append(s.Skip, day(occ.Start))
	}
}

// detach makes the occurrence occ an event of its own, out of its series,
// which it returns for changing.
func (d *Data) detach(occ *Event) *Event {
	if occ.series == nil {
		return occ
	}
	d.skip(occ)
	e := *occ
	e.ID, e.series, e.Repeat, e.Until, e.Skip, e.UID = newID(), nil, RepeatNone, time.Time{}, nil, ""
	d.Events = append(d.Events, &e)
	return &e
}

// nextDay returns the day after d on which a thing repeating as r comes
// again: a monthly one on the same day of the month, or its last.
func nextDay(r Repeat, d time.Time) time.Time {
	d = day(d)
	switch r {
	case RepeatDaily:
		return addDays(d, 1)
	case RepeatWeekdays:
		d = addDays(d, 1)
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = addDays(d, 1)
		}
		return d
	case RepeatWeekly:
		return addDays(d, 7)
	case RepeatBiweekly:
		return addDays(d, 14)
	case RepeatMonthly:
		y, m, dd := d.Date()
		last := time.Date(y, m+2, 0, 0, 0, 0, 0, d.Location()).Day()
		return time.Date(y, m+1, min(dd, last), 0, 0, 0, 0, d.Location())
	}
	return d
}

// repeatTask adds the next of a repeating task, done now: a copy of it
// due the next of its days still to come, reminding as many days later,
// which it returns. The task done stops repeating, its next going on.
func (d *Data) repeatTask(t *Task, now time.Time) *Task {
	due := t.Due
	if due.IsZero() {
		due = day(now)
	}
	next := nextDay(t.Repeat, due)
	for next.Before(day(now)) {
		next = nextDay(t.Repeat, next)
	}
	n := *t
	n.ID, n.Created, n.Done, n.DoneAt, n.MyDay, n.Due = newID(), now, false, time.Time{}, time.Time{}, next
	if !t.Remind.IsZero() {
		n.Remind = t.Remind.AddDate(0, 0, daysBetween(due, next))
	}
	t.Repeat = RepeatNone
	d.Tasks = append(d.Tasks, &n)
	return &n
}

// occurrenceRef names an occurrence for a notification: its series and
// its start.
func occurrenceRef(ref string) (id string, start time.Time) {
	id, unix, ok := strings.Cut(ref, "@")
	if !ok {
		return id, time.Time{}
	}
	n, _ := strconv.ParseInt(unix, 10, 64)
	return id, time.Unix(n, 0)
}

package main

import (
	"strings"
	"testing"
	"time"
)

const sampleICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Google Inc//Google Calendar 70.9054//EN\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:Asia/Shanghai\r\nEND:VTIMEZONE\r\n" +
	// A lecture on Mondays and Wednesdays, ten times, but not on one Monday.
	"BEGIN:VEVENT\r\nUID:lecture@example.com\r\nDTSTART;TZID=Asia/Shanghai:20261026T140000\r\nDTEND;TZID=Asia/Shanghai:20261026T153000\r\n" +
	"RRULE:FREQ=WEEKLY;COUNT=10;BYDAY=MO,WE\r\nEXDATE;TZID=Asia/Shanghai:20261102T140000\r\n" +
	"SUMMARY:Deep learning\\, lecture\r\nLOCATION:Room 3\\; East\r\n" +
	"DESCRIPTION:Bring the slides\\nand the notes. This line is long enough that a calendar folds it o\r\n ver two lines.\r\n" +
	"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\nEND:VEVENT\r\n" +
	// One lecture moved.
	"BEGIN:VEVENT\r\nUID:lecture@example.com\r\nRECURRENCE-ID;TZID=Asia/Shanghai:20261028T140000\r\n" +
	"DTSTART;TZID=Asia/Shanghai:20261029T100000\r\nDTEND;TZID=Asia/Shanghai:20261029T113000\r\nSUMMARY:Deep learning (moved)\r\nEND:VEVENT\r\n" +
	// A deadline at a moment, in UTC.
	"BEGIN:VEVENT\r\nUID:icml@deadlines\r\nDTSTART:20270130T115900Z\r\nSUMMARY:ICML abstract deadline\r\nEND:VEVENT\r\n" +
	// A conference over three days.
	"BEGIN:VEVENT\r\nUID:cvpr\r\nDTSTART;VALUE=DATE:20270610\r\nDTEND;VALUE=DATE:20270613\r\nSUMMARY:CVPR\r\nEND:VEVENT\r\n" +
	// Monthly on the second Tuesday: once only.
	"BEGIN:VEVENT\r\nUID:board\r\nDTSTART:20261110T090000Z\r\nDURATION:PT1H\r\nRRULE:FREQ=MONTHLY;BYDAY=2TU\r\nSUMMARY:Board\r\nEND:VEVENT\r\n" +
	// Cancelled.
	"BEGIN:VEVENT\r\nUID:gone\r\nDTSTART:20261110T090000Z\r\nSTATUS:CANCELLED\r\nSUMMARY:Gone\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestReadICS(t *testing.T) {
	sh, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skip(err)
	}
	got, err := readICS([]byte(sampleICS), sh)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string][]*Event{}
	for _, e := range got.events {
		byTitle[e.Title] = append(byTitle[e.Title], e)
	}
	if len(got.events) != 6 || got.simple != 1 || byTitle["Gone"] != nil {
		t.Fatalf("%d events, %d simplified: %v", len(got.events), got.simple, byTitle)
	}
	lec := byTitle["Deep learning, lecture"]
	if len(lec) != 2 {
		t.Fatalf("%d lecture series", len(lec))
	}
	mon, wed := lec[0], lec[1]
	if mon.Repeat != RepeatWeekly || !mon.Start.Equal(time.Date(2026, 10, 26, 14, 0, 0, 0, sh)) ||
		!wed.Start.Equal(time.Date(2026, 10, 28, 14, 0, 0, 0, sh)) || wed.End.Sub(wed.Start) != 90*time.Minute {
		t.Errorf("lectures %v %v – %v", mon.Start, wed.Start, wed.End)
	}
	if mon.Location != "Room 3; East" || !strings.Contains(mon.Notes, "slides\nand the notes") || !strings.Contains(mon.Notes, "folds it over two") {
		t.Errorf("location %q, notes %q", mon.Location, mon.Notes)
	}
	if mon.Alert == nil || *mon.Alert != 15 {
		t.Errorf("alert %v", mon.Alert)
	}
	// Ten lectures from Oct 26: the last on Nov 25. One Monday skipped, one
	// Wednesday moved.
	if !sameDay(mon.Until, time.Date(2026, 11, 25, 0, 0, 0, 0, sh)) || !sameDay(wed.Until, mon.Until) {
		t.Errorf("until %v %v", mon.Until, wed.Until)
	}
	var n int
	for _, x := range lec {
		n += len(x.occurrences(x.Start, addDays(x.Start, 60)))
	}
	if n != 8 {
		t.Errorf("%d lectures, want 10 less one skipped and one moved", n)
	}
	moved := byTitle["Deep learning (moved)"]
	if len(moved) != 1 || moved[0].Start.Day() != 29 || moved[0].UID == mon.UID {
		t.Errorf("moved %+v", moved)
	}
	dl := byTitle["ICML abstract deadline"][0]
	if !dl.Start.Equal(time.Date(2027, 1, 30, 19, 59, 0, 0, sh)) || dl.End.Sub(dl.Start) != 30*time.Minute {
		t.Errorf("deadline %v – %v", dl.Start, dl.End)
	}
	cvpr := byTitle["CVPR"][0]
	if !cvpr.AllDay || daysBetween(cvpr.Start, cvpr.End) != 3 {
		t.Errorf("cvpr %v – %v", cvpr.Start, cvpr.End)
	}
	if b := byTitle["Board"][0]; b.Repeat != RepeatNone || b.End.Sub(b.Start) != time.Hour {
		t.Errorf("board %+v", b)
	}
}

// What NextTodo writes, it reads back the same; imported again, events
// change in place.
func TestWriteICS(t *testing.T) {
	d := sampleData(monday, "en")
	b := writeICS(d.Events, monday)
	for _, l := range strings.Split(string(b), "\r\n") {
		if len(l) > 75 {
			t.Fatalf("a line of %d bytes", len(l))
		}
	}
	got, err := readICS(b, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.events) != len(d.Events) || got.simple != 0 {
		t.Fatalf("read back %d of %d events, %d simplified", len(got.events), len(d.Events), got.simple)
	}
	for i, e := range d.Events {
		x := got.events[i]
		if x.Title != e.Title || !x.Start.Equal(e.Start) || !x.End.Equal(e.End) || x.Repeat != e.Repeat || x.Location != e.Location {
			t.Errorf("%q came back as %+v", e.Title, x)
		}
	}
	// Back where it came from, it changes its events; on another Mac, it
	// adds them, and then changes them.
	n := len(d.Events)
	if added, changed := d.importEvents(got.events); added != 0 || changed != n || d.Events[0].UID != "" {
		t.Errorf("imported back: %d added, %d changed", added, changed)
	}
	other := newData()
	got, _ = readICS(b, time.Local)
	if added, changed := other.importEvents(got.events); added != n || changed != 0 {
		t.Errorf("imported elsewhere: %d added, %d changed", added, changed)
	}
	got, _ = readICS(b, time.Local)
	if added, changed := other.importEvents(got.events); added != 0 || changed != n || len(other.Events) != n {
		t.Errorf("imported again: %d added, %d changed, %d events", added, changed, len(other.Events))
	}
}

// Events added from a calendar file can be undone.
func TestAddCalendar(t *testing.T) {
	a := testApp("en")
	tt := newTester(t, a, "en")
	n := len(a.data.Events)
	got, err := readICS([]byte(sampleICS), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	a.addCalendar(got)
	tt.Frame()
	if len(a.data.Events) != n+6 || !tt.HasText("6 events imported · 1 repeating event kept its first time only") {
		t.Fatalf("%d events; %q", len(a.data.Events), tt.Texts())
	}
	tt.Click("Undo")
	if len(a.data.Events) != n {
		t.Errorf("%d events after Undo, want %d", len(a.data.Events), n)
	}
}

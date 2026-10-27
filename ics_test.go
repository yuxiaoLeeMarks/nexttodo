package main

import (
	"context"
	"errors"
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
	if !sameDay(mon.Until, time.Date(2026, 11, 23, 0, 0, 0, 0, sh)) || !sameDay(wed.Until, time.Date(2026, 11, 25, 0, 0, 0, 0, sh)) {
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
	b := writeICS(d.Events, monday, func(KindID) string { return "" })
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

func icsEvent(lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

// What a second review found in reading calendars.
func TestReadICSEdges(t *testing.T) {
	sh, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skip(err)
	}
	read := func(s string) icsImport {
		t.Helper()
		got, err := readICS([]byte(s), sh)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// A Monday evening call in New York is a Tuesday morning one here.
	got := read(icsEvent("UID:call", "DTSTART;TZID=America/New_York:20261019T210000", "DTEND;TZID=America/New_York:20261019T220000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO", "SUMMARY:Call"))
	if e := got.events[0]; e.Start.Weekday() != time.Tuesday || e.Start.Hour() != 9 || e.Repeat != RepeatWeekly {
		t.Errorf("the call is %v, %v", e.Start, e.Repeat)
	}
	// Every other week on Mondays and Wednesdays, from a Wednesday: the
	// Mondays start the week after next.
	got = read(icsEvent("UID:bw", "DTSTART:20261028T020000Z", "DURATION:PT1H", "RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE", "SUMMARY:BW"))
	var starts []string
	for _, e := range got.events {
		starts = append(starts, e.Start.Format("01-02"))
	}
	if strings.Join(starts, " ") != "10-28 11-09" || got.events[0].Repeat != RepeatBiweekly {
		t.Errorf("biweekly series start %v", starts)
	}
	// DURATION before DTSTART; a zone of Windows; a byte-order mark.
	got = read("\ufeff" + icsEvent("UID:d", "DURATION:PT2H", `DTSTART;TZID="Pacific Standard Time":20261026T090000`, "SUMMARY:D"))
	if e := got.events[0]; e.End.Sub(e.Start) != 2*time.Hour || !e.Start.Equal(time.Date(2026, 10, 27, 0, 0, 0, 0, sh)) {
		t.Errorf("d %v – %v", e.Start, e.End)
	}
	// A start that cannot be read leaves out that event alone.
	bad := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:x\r\nDTSTART:20261026T0200\r\nSUMMARY:Bad\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:y\r\nDTSTART:20261026T020000Z\r\nSUMMARY:Good\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if got = read(bad); len(got.events) != 1 || got.events[0].Title != "Good" {
		t.Errorf("events %+v", got.events)
	}
	// A cancelled occurrence is skipped; one changed alone, imported after
	// its series, takes the place of its day there.
	d := newData()
	d.importEvents(read(icsEvent("UID:s", "DTSTART:20261026T020000Z", "DURATION:PT1H", "RRULE:FREQ=WEEKLY", "SUMMARY:S")).events)
	cancel := read(icsEvent("UID:s", "RECURRENCE-ID:20261102T020000Z", "DTSTART:20261102T020000Z", "STATUS:CANCELLED", "SUMMARY:S"))
	d.importEvents(cancel.events)
	d.skipImported(cancel.skips)
	moved := read(icsEvent("UID:s", "RECURRENCE-ID:20261109T020000Z", "DTSTART:20261110T020000Z", "DURATION:PT1H", "SUMMARY:S moved"))
	d.importEvents(moved.events)
	d.skipImported(moved.skips)
	var days []string
	for _, e := range d.eventsBetween(time.Date(2026, 10, 26, 0, 0, 0, 0, sh), time.Date(2026, 11, 17, 0, 0, 0, 0, sh)) {
		days = append(days, e.Start.In(sh).Format("01-02"))
	}
	if strings.Join(days, " ") != "10-26 11-10 11-16" {
		t.Errorf("the series falls on %v", days)
	}
}

// A repeating all-day event goes out with a date to end on, as its start.
func TestWriteICSAllDayUntil(t *testing.T) {
	e := &Event{ID: "a", Title: "Retreat", AllDay: true, Start: day(monday), End: addDays(day(monday), 1),
		Repeat: RepeatWeekly, Until: addDays(day(monday), 21)}
	if s := string(writeICS([]*Event{e}, monday, func(KindID) string { return "" })); !strings.Contains(s, "RRULE:FREQ=WEEKLY;UNTIL="+e.Until.Format("20060102")+"\r\n") {
		t.Errorf("%s", s)
	}
}

// An event's tag goes out as its category, and comes back to the tag of
// that name.
func TestICSTags(t *testing.T) {
	a := testApp("en")
	b := writeICS(a.data.Events, monday, a.kindName)
	if !strings.Contains(string(b), "CATEGORIES:Meeting\r\n") {
		t.Fatal("no categories")
	}
	got, err := readICS(b, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	other := newData()
	other.Kinds = defaultKinds("en")
	other.importCalendar(got)
	for i, e := range other.Events {
		if e.Kind != a.data.Events[i].Kind {
			t.Errorf("%q came back tagged %q, not %q", e.Title, e.Kind, a.data.Events[i].Kind)
		}
	}
}

// A calendar on the web, followed: its events come, read only, come again
// as it changes, keep their IDs, stay when it cannot be had, and go with
// it.
func TestSubscriptions(t *testing.T) {
	a := testApp("en")
	feed := "BEGIN:VCALENDAR\r\nX-WR-CALNAME:Department\r\n" +
		"BEGIN:VEVENT\r\nUID:talk\r\nDTSTART:20261028T060000Z\r\nDURATION:PT1H\r\nSUMMARY:Talk\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:retreat\r\nDTSTART;VALUE=DATE:20261030\r\nSUMMARY:Retreat\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	var fail error
	a.fetch = func(_ context.Context, u string) ([]byte, error) {
		if u != "https://example.edu/dept.ics" {
			t.Errorf("fetched %q", u)
		}
		return []byte(feed), fail
	}
	if err := a.subscribe("webcal://example.edu/dept.ics"); err != nil {
		t.Fatal(err)
	}
	s := a.data.Subscriptions[0]
	talk := eventTitled(a, "Talk")
	if s.Name != "Department" || talk == nil || talk.Sub != s.ID || talk.Kind != a.data.newKind() || s.Updated.IsZero() {
		t.Fatalf("subscription %+v, talk %+v", s, talk)
	}
	// Changed there, changed here, the same event.
	feed = strings.Replace(feed, "SUMMARY:Talk", "SUMMARY:Talk moved", 1)
	a.refreshSubscription(s)
	if moved := eventTitled(a, "Talk moved"); moved == nil || moved.ID != talk.ID || eventTitled(a, "Talk") != nil {
		t.Fatalf("after a refresh: %+v", moved)
	}
	// Unreachable, it keeps what it had and says why.
	fail = errors.New("offline")
	a.refreshSubscription(s)
	if eventTitled(a, "Talk moved") == nil || a.subErr[s.ID] != "offline" {
		t.Errorf("failed refresh lost events, or no error: %q", a.subErr[s.ID])
	}
	fail = nil
	// Read only: a click shows it, its alerts wait for a yes, it is not
	// exported.
	e := eventTitled(a, "Talk moved")
	a.editEvent(e)
	if a.evtOpen || !a.infoOpen || a.info != e {
		t.Error("a subscription's event opened for editing")
	}
	if _, ok := a.alertLead(e); ok {
		t.Error("a subscription's event alerts unasked")
	}
	if strings.Contains(string(writeICS(a.data.Events, monday, a.kindName)), "Talk moved") {
		t.Error("a subscription's event was exported")
	}
	// Unsubscribed, its events go, until Undo.
	n := len(a.data.Events)
	step := a.remember()
	a.data.unsubscribe(s.ID)
	a.settle(step)
	if len(a.data.Events) != n-2 || len(a.data.Subscriptions) != 0 {
		t.Fatalf("%d events left of %d", len(a.data.Events), n)
	}
	a.restore(step)
	if len(a.data.Events) != n || len(a.data.Subscriptions) != 1 {
		t.Errorf("after Undo, %d events, %d subscriptions", len(a.data.Events), len(a.data.Subscriptions))
	}
	if _, err := calendarURL("not a url at all"); err == nil {
		t.Error("an address of nothing was taken")
	}
}

// Categories: split at the commas not escaped, the first named tag wins,
// and a tag the user gave an event stays as the file comes again.
func TestICSCategories(t *testing.T) {
	if got := icsList(`Lab\, Room 2,Seminar`); len(got) != 2 || got[0] != "Lab, Room 2" || got[1] != "Seminar" {
		t.Fatalf("list %q", got)
	}
	d := freshData("en")
	d.Kinds = append(d.Kinds, &Kind{ID: "room", Name: "Lab, Room 2"}, &Kind{ID: "dup", Name: "Meeting"})
	file := icsEvent("UID:x", "DTSTART:20261028T060000Z", `CATEGORIES:Work,Seminar`, "SUMMARY:X") +
		icsEvent("UID:y", "DTSTART:20261028T080000Z", `CATEGORIES:Lab\, Room 2`, "SUMMARY:Y") +
		icsEvent("UID:z", "DTSTART:20261028T090000Z", `CATEGORIES:Meeting`, "SUMMARY:Z")
	got, err := readICS([]byte(file), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	d.importCalendar(got)
	kinds := map[string]KindID{}
	for _, e := range d.Events {
		kinds[e.Title] = e.Kind
	}
	if kinds["X"] != "seminar" || kinds["Y"] != "room" || kinds["Z"] != "meeting" {
		t.Fatalf("tags %v", kinds)
	}
	// Retagged by the user, kept as the file comes again.
	for _, e := range d.Events {
		if e.Title == "X" {
			e.Kind = "focus"
		}
	}
	got, _ = readICS([]byte(file), time.Local)
	d.importCalendar(got)
	for _, e := range d.Events {
		if e.Title == "X" && e.Kind != "focus" {
			t.Errorf("import took back the user's tag: %q", e.Kind)
		}
	}
}

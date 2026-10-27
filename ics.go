package main

import (
	"bytes"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Calendar files (iCalendar, RFC 5545), as Calendar, Google Calendar and
// Outlook export them and conference sites publish deadlines in.

// icsLine is a content line: NAME;PARAM=V:VALUE.
type icsLine struct {
	name   string
	params map[string]string
	value  string
}

// icsLines unfolds the lines of a calendar file and splits each.
func icsLines(b []byte) []icsLine {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n ", "")
	s = strings.ReplaceAll(s, "\n\t", "")
	var out []icsLine
	for _, raw := range strings.Split(s, "\n") {
		if raw == "" {
			continue
		}
		// The value starts at the first colon outside quotes.
		quoted, at := false, -1
		for i, r := range raw {
			if r == '"' {
				quoted = !quoted
			} else if r == ':' && !quoted {
				at = i
				break
			}
		}
		if at < 0 {
			continue
		}
		head := strings.Split(raw[:at], ";")
		l := icsLine{name: strings.ToUpper(head[0]), params: map[string]string{}, value: raw[at+1:]}
		for _, p := range head[1:] {
			if k, v, ok := strings.Cut(p, "="); ok {
				l.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
			}
		}
		out = append(out, l)
	}
	return out
}

// icsText reads an escaped text value.
func icsText(s string) string {
	r := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return strings.TrimSpace(r.Replace(s))
}

// icsTime reads a date or a date and time: in its zone, UTC or the one
// its TZID names, or else in loc; and whether it is a date alone.
func icsTime(l icsLine, loc *time.Location) (time.Time, bool, error) {
	v := strings.TrimSpace(l.value)
	if l.params["VALUE"] == "DATE" || len(v) == 8 {
		t, err := time.ParseInLocation("20060102", v, loc)
		return t, true, err
	}
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse("20060102T150405Z", v)
		return t, false, err
	}
	zone := loc
	if id := l.params["TZID"]; id != "" {
		zone = icsZone(id, loc)
	}
	t, err := time.ParseInLocation("20060102T150405", v, zone)
	return t, false, err
}

// windowsZones names in IANA's way the zones Outlook names in Windows'.
var windowsZones = map[string]string{
	"China Standard Time": "Asia/Shanghai", "Taipei Standard Time": "Asia/Taipei", "Tokyo Standard Time": "Asia/Tokyo",
	"Korea Standard Time": "Asia/Seoul", "Singapore Standard Time": "Asia/Singapore", "India Standard Time": "Asia/Kolkata",
	"Pacific Standard Time": "America/Los_Angeles", "Mountain Standard Time": "America/Denver",
	"Central Standard Time": "America/Chicago", "Eastern Standard Time": "America/New_York",
	"GMT Standard Time": "Europe/London", "W. Europe Standard Time": "Europe/Berlin", "Romance Standard Time": "Europe/Paris",
	"Central Europe Standard Time": "Europe/Budapest", "Central European Standard Time": "Europe/Warsaw",
	"AUS Eastern Standard Time": "Australia/Sydney", "New Zealand Standard Time": "Pacific/Auckland",
	"UTC": "UTC", "Coordinated Universal Time": "UTC", "Greenwich Standard Time": "Atlantic/Reykjavik",
}

// icsZone finds the zone a TZID names: by IANA's name, Windows' name, or
// the end of a path as /mozilla.org/20050126_1/America/New_York; else loc.
func icsZone(id string, loc *time.Location) *time.Location {
	if z, err := time.LoadLocation(id); err == nil && id != "Local" {
		return z
	}
	if name, ok := windowsZones[id]; ok {
		if z, err := time.LoadLocation(name); err == nil {
			return z
		}
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '/' {
			if z, err := time.LoadLocation(id[i+1:]); err == nil && id[i+1:] != "" {
				return z
			}
		}
	}
	return loc
}

// icsDuration reads a duration: P1D, PT1H30M, P1W.
func icsDuration(s string) time.Duration {
	s = strings.TrimPrefix(strings.TrimPrefix(strings.ToUpper(s), "+"), "-")
	s = strings.TrimPrefix(s, "P")
	var d time.Duration
	n := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			n = n*10 + int(r-'0')
		case r == 'W':
			d += time.Duration(n) * 7 * 24 * time.Hour
			n = 0
		case r == 'D':
			d += time.Duration(n) * 24 * time.Hour
			n = 0
		case r == 'H':
			d += time.Duration(n) * time.Hour
			n = 0
		case r == 'M':
			d += time.Duration(n) * time.Minute
			n = 0
		case r == 'S':
			d += time.Duration(n) * time.Second
			n = 0
		}
	}
	return d
}

// icsImport is what a calendar file holds: its events, the occurrences
// of series changed or cancelled on their own, which those series skip
// even if imported before, and how many repeat in a way NextTodo cannot,
// which keep their first time.
type icsImport struct {
	events []*Event
	skips  []icsSkip
	simple int
	tags   map[*Event]string // the categories of events, which name their tags
}

// icsSkip is a day the series of a UID skips.
type icsSkip struct {
	uid string
	day time.Time
}

// readICS reads the events of a calendar file, in loc. An event whose
// start cannot be read is left out.
func readICS(b []byte, loc *time.Location) (icsImport, error) {
	var out icsImport
	lines := icsLines(bytes.TrimPrefix(b, []byte("\ufeff")))
	if len(lines) == 0 || lines[0].name != "BEGIN" || !strings.EqualFold(lines[0].value, "VCALENDAR") {
		return out, errors.New("not a calendar file")
	}
	type vevent struct {
		e          *Event
		start, end *icsLine
		duration   string
		rule       string
		exdate     []time.Time
		recur      time.Time // the occurrence a changed one replaces
		cancelled  bool
		tag        string
	}
	var all []*vevent
	var cur *vevent
	depth := 0 // within a VALARM or another part of an event
	for _, l := range lines {
		switch {
		case l.name == "BEGIN" && strings.EqualFold(l.value, "VEVENT"):
			cur = &vevent{e: &Event{ID: newID()}}
			continue
		case l.name == "END" && strings.EqualFold(l.value, "VEVENT"):
			if cur != nil && cur.start != nil {
				all = append(all, cur)
			}
			cur, depth = nil, 0
			continue
		case cur == nil:
			continue
		case l.name == "BEGIN":
			depth++
			continue
		case l.name == "END":
			depth--
			continue
		}
		e := cur.e
		if depth > 0 {
			// An alarm's trigger before the start is the event's alert.
			if l.name == "TRIGGER" && strings.HasPrefix(l.value, "-") && l.params["VALUE"] != "DATE-TIME" && e.Alert == nil {
				n := int(icsDuration(l.value) / time.Minute)
				e.Alert = &n
			}
			continue
		}
		switch l.name {
		case "UID":
			e.UID = strings.TrimSpace(l.value)
		case "SUMMARY":
			e.Title = icsText(l.value)
		case "LOCATION":
			e.Location = icsText(l.value)
		case "DESCRIPTION":
			e.Notes = icsText(l.value)
		case "STATUS":
			cur.cancelled = strings.EqualFold(l.value, "CANCELLED")
		case "CATEGORIES":
			cur.tag, _, _ = strings.Cut(icsText(l.value), ",")
		case "DTSTART":
			cur.start = &l
		case "DTEND":
			cur.end = &l
		case "DURATION":
			cur.duration = l.value
		case "RRULE":
			cur.rule = l.value
		case "EXDATE":
			for _, v := range strings.Split(l.value, ",") {
				if t, _, err := icsTime(icsLine{params: l.params, value: v}, loc); err == nil {
					cur.exdate = append(cur.exdate, t.In(loc))
				}
			}
		case "RECURRENCE-ID":
			if t, _, err := icsTime(l, loc); err == nil {
				cur.recur = t.In(loc)
			}
		}
	}
	series := map[string][]*Event{} // by UID, a series and those it split into
	for _, v := range all {
		e := v.e
		if !v.recur.IsZero() {
			out.skips = append(out.skips, icsSkip{uid: e.UID, day: day(v.recur)})
		}
		var err error
		if e.Start, e.AllDay, err = icsTime(*v.start, loc); err != nil || v.cancelled {
			continue
		}
		if v.end != nil {
			e.End, _, _ = icsTime(*v.end, loc)
		} else if v.duration != "" {
			e.End = e.Start.Add(icsDuration(v.duration))
		}
		switch {
		case e.AllDay && !e.End.After(e.Start):
			e.End = addDays(e.Start, 1)
		case !e.End.After(e.Start):
			e.End = e.Start.Add(30 * time.Minute) // a deadline, at a moment
		}
		// The rule repeats in the event's own zone; then the series take
		// the days and the times here.
		group := []*Event{e}
		if v.recur.IsZero() && v.rule != "" {
			more, ok := e.repeatAs(v.rule)
			if !ok {
				out.simple++
			}
			group = append(group, more...)
		}
		for _, x := range group {
			if !x.AllDay {
				x.Start, x.End = x.Start.In(loc), x.End.In(loc)
			}
			if !x.Until.IsZero() {
				x.Until = day(x.Until.In(loc))
			}
			for _, t := range v.exdate {
				x.Skip = append(x.Skip, day(t))
			}
		}
		if v.recur.IsZero() {
			series[e.UID] = group
		} else {
			e.UID += "/" + v.recur.Format("20060102")
		}
		out.events = append(out.events, group...)
		if v.tag != "" {
			if out.tags == nil {
				out.tags = map[*Event]string{}
			}
			for _, x := range group {
				out.tags[x] = strings.TrimSpace(v.tag)
			}
		}
	}
	// An occurrence changed or cancelled on its own, which its series skips.
	for _, sk := range out.skips {
		for _, x := range series[sk.uid] {
			if !x.skipped(sk.day) {
				x.Skip = append(x.Skip, sk.day)
			}
		}
	}
	return out, nil
}

// repeatAs makes e repeat as rule says, and returns the events more it
// takes, as on Mondays and Wednesdays, which repeat weekly each: false
// when NextTodo cannot repeat so, and e happens once.
func (e *Event) repeatAs(rule string) ([]*Event, bool) {
	parts := map[string]string{}
	for _, p := range strings.Split(strings.ToUpper(rule), ";") {
		if k, v, ok := strings.Cut(p, "="); ok {
			parts[k] = v
		}
	}
	interval := 1
	if n, err := strconv.Atoi(parts["INTERVAL"]); err == nil && n > 0 {
		interval = n
	}
	var days []time.Weekday
	codes := map[string]time.Weekday{"SU": 0, "MO": 1, "TU": 2, "WE": 3, "TH": 4, "FR": 5, "SA": 6}
	for _, d := range strings.Split(parts["BYDAY"], ",") {
		if wd, ok := codes[d]; ok {
			days = append(days, wd)
		} else if d != "" {
			return nil, false // the second Tuesday of the month
		}
	}
	if parts["BYSETPOS"] != "" || parts["BYMONTH"] != "" || parts["BYMONTHDAY"] != "" && parts["BYMONTHDAY"] != strconv.Itoa(e.Start.Day()) {
		return nil, false
	}
	weekdays := []time.Weekday{1, 2, 3, 4, 5}
	length := func(x *Event, start time.Time) {
		if x.AllDay {
			x.Start, x.End = start, addDays(start, daysBetween(x.Start, x.End))
		} else {
			x.Start, x.End = start, start.Add(x.End.Sub(x.Start))
		}
	}
	var more []*Event
	switch {
	case parts["FREQ"] == "DAILY" && interval == 1 && len(days) == 0:
		e.Repeat = RepeatDaily
	case parts["FREQ"] == "DAILY" && interval == 1 && slices.Equal(sorted(days), weekdays),
		parts["FREQ"] == "WEEKLY" && interval == 1 && slices.Equal(sorted(days), weekdays):
		e.Repeat = RepeatWeekdays
	case parts["FREQ"] == "WEEKLY" && (interval == 1 || interval == 2):
		e.Repeat = RepeatWeekly
		if interval == 2 {
			e.Repeat = RepeatBiweekly
		}
		// A series for each day of the week, from its first in the week
		// of the start, or, gone by then, in the next week it repeats.
		wkst := time.Monday
		if wd, ok := codes[parts["WKST"]]; ok {
			wkst = wd
		}
		week := addDays(e.Start, -((int(e.Start.Weekday()) - int(wkst) + 7) % 7))
		var starts []time.Time
		for _, wd := range days {
			s := addDays(week, (int(wd)-int(wkst)+7)%7)
			if day(s).Before(day(e.Start)) {
				s = addDays(s, 7*interval)
			}
			starts = append(starts, s)
		}
		slices.SortFunc(starts, time.Time.Compare)
		starts = slices.CompactFunc(starts, time.Time.Equal)
		if len(starts) > 0 {
			base := *e
			length(e, starts[0])
			for _, s := range starts[1:] {
				x := base
				x.ID = newID()
				if x.UID != "" {
					x.UID += "#" + s.Weekday().String()
				}
				length(&x, s)
				more = append(more, &x)
			}
		}
	case parts["FREQ"] == "MONTHLY" && interval == 1 && len(days) == 0:
		e.Repeat = RepeatMonthly
	default:
		return nil, false
	}
	// Until a moment, or so many times: each series ends with its last
	// time by then, which Until keeps until the series takes the days here.
	series := append([]*Event{e}, more...)
	var last time.Time
	if u := parts["UNTIL"]; u != "" {
		if t, date, err := icsTime(icsLine{params: map[string]string{}, value: u}, e.Start.Location()); err == nil {
			if last = t; date {
				last = addDays(t, 1).Add(-time.Second)
			}
		}
	}
	if n, err := strconv.Atoi(parts["COUNT"]); err == nil && n > 0 {
		// The nth time of all the series together.
		var times []time.Time
		for _, x := range series {
			for _, o := range x.occurrences(x.Start, addDays(x.Start, 3700)) {
				times = append(times, o.Start)
			}
		}
		slices.SortFunc(times, time.Time.Compare)
		if len(times) >= n {
			last = times[n-1]
		}
	}
	if !last.IsZero() {
		for _, x := range series {
			until := x.Start
			for _, o := range x.occurrences(x.Start, last.Add(time.Second)) {
				until = o.Start
			}
			x.Until = until
		}
	}
	return more, true
}

func sorted(days []time.Weekday) []time.Weekday {
	days = slices.Clone(days)
	slices.Sort(days)
	return days
}

// writeICS writes events as a calendar file, in the time of this Mac.
func writeICS(events []*Event, now time.Time, tag func(KindID) string) []byte {
	var b bytes.Buffer
	line := func(s string) {
		// Lines of at most 75 bytes, folded, not within a character.
		for len(s) > 75 {
			i := 75
			for i > 0 && s[i]&0xC0 == 0x80 {
				i--
			}
			b.WriteString(s[:i] + "\r\n")
			s = " " + s[i:]
		}
		b.WriteString(s + "\r\n")
	}
	text := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`)
	stamp := now.UTC().Format("20060102T150405Z")
	when := func(name string, t time.Time, date bool) string {
		if date {
			return name + ";VALUE=DATE:" + t.Format("20060102")
		}
		return name + ":" + t.Format("20060102T150405")
	}
	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//NextTodo//NextTodo//EN")
	line("CALSCALE:GREGORIAN")
	for _, e := range events {
		line("BEGIN:VEVENT")
		uid := e.UID
		if uid == "" {
			uid = e.ID + "@nexttodo"
		}
		line("UID:" + uid)
		line("DTSTAMP:" + stamp)
		line(when("DTSTART", e.Start, e.AllDay))
		line(when("DTEND", e.End, e.AllDay))
		line("SUMMARY:" + text.Replace(e.Title))
		if e.Location != "" {
			line("LOCATION:" + text.Replace(e.Location))
		}
		if e.Notes != "" {
			line("DESCRIPTION:" + text.Replace(e.Notes))
		}
		if name := tag(e.Kind); name != "" {
			line("CATEGORIES:" + text.Replace(name))
		}
		if e.repeating() {
			rule := map[Repeat]string{
				RepeatDaily: "FREQ=DAILY", RepeatWeekdays: "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", RepeatWeekly: "FREQ=WEEKLY",
				RepeatBiweekly: "FREQ=WEEKLY;INTERVAL=2", RepeatMonthly: "FREQ=MONTHLY",
			}[e.Repeat]
			switch {
			case !e.Until.IsZero() && e.AllDay:
				rule += ";UNTIL=" + e.Until.Format("20060102") // as the start, a date
			case !e.Until.IsZero():
				rule += ";UNTIL=" + addDays(day(e.Until), 1).Add(-time.Second).UTC().Format("20060102T150405Z")
			}
			line("RRULE:" + rule)
			for _, s := range e.Skip {
				line(when("EXDATE", at(s, e.Start.Hour(), e.Start.Minute()), e.AllDay))
			}
		}
		line("END:VEVENT")
	}
	line("END:VCALENDAR")
	return b.Bytes()
}

// importEvents adds the events of a calendar file, those imported or
// exported before changed in place by their UIDs, and returns how many it
// added and changed.
func (d *Data) importEvents(events []*Event) (added, changed int) {
	byUID := map[string]*Event{}
	for _, e := range d.Events {
		byUID[e.ID+"@nexttodo"] = e
		if e.UID != "" {
			byUID[e.UID] = e
		}
	}
	for _, e := range events {
		if old := byUID[e.UID]; e.UID != "" && old != nil {
			// What is NextTodo's own stays: its project and alert, and its
			// tag unless the file names one.
			e.ID, e.Project = old.ID, old.Project
			if e.Kind == "" {
				e.Kind = old.Kind
			}
			if strings.HasSuffix(e.UID, "@nexttodo") && old.UID == "" {
				e.UID = "" // its own, back
			}
			if e.Alert == nil {
				e.Alert = old.Alert
			}
			*old = *e
			changed++
			continue
		}
		d.Events = append(d.Events, e)
		added++
	}
	return added, changed
}

// importCalendar adds the events of a calendar file as importEvents does,
// each new one with the tag its category names, if there is one, and has
// the series imported before skip what the file changed on its own.
func (d *Data) importCalendar(got icsImport) (added, changed int) {
	for e, name := range got.tags {
		for _, k := range d.Kinds {
			if strings.EqualFold(k.Name, name) {
				e.Kind = k.ID
			}
		}
	}
	added, changed = d.importEvents(got.events)
	d.skipImported(got.skips)
	return added, changed
}

// skipImported makes the series of earlier imports skip the occurrences a
// calendar file changed or cancelled on their own.
func (d *Data) skipImported(skips []icsSkip) {
	for _, sk := range skips {
		for _, e := range d.Events {
			if sk.uid != "" && (e.UID == sk.uid || strings.HasPrefix(e.UID, sk.uid+"#")) && !e.skipped(sk.day) {
				e.Skip = append(e.Skip, sk.day)
			}
		}
	}
}

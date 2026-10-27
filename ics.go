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

// icsList reads a list of texts, split at the commas not escaped.
func icsList(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case ',':
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	var texts []string
	for _, v := range out {
		if v = icsText(v); v != "" {
			texts = append(texts, v)
		}
	}
	return texts
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
	tags   map[*Event][]string // the categories of events, which name their tags
	name   string              // the calendar's own, as X-WR-CALNAME says
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
		tags       []string
	}
	var all []*vevent
	var cur *vevent
	depth := 0 // within a VALARM or another part of an event
	for _, l := range lines {
		switch {
		case l.name == "BEGIN" && strings.EqualFold(l.value, "VEVENT"):
			cur = &vevent{e: &Event{ID: newID()}}
			continue
		case cur == nil && l.name == "X-WR-CALNAME":
			out.name = icsText(l.value)
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
			cur.tags = append(cur.tags, icsList(l.value)...)
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
			if more, ok := e.repeatAs(v.rule); ok {
				group = append(group, more...)
			} else if starts, ok := expandRule(e, v.rule, icsNow()); ok {
				// A rule NextTodo cannot repeat by, as the fourth Thursday of
				// November, comes as its times, from a year ago to two on.
				group = group[:0]
				for _, st := range starts {
					if slices.ContainsFunc(v.exdate, func(t time.Time) bool { return sameDay(t, st.In(loc)) }) {
						continue
					}
					x := *e
					x.ID, x.UID = newID(), e.UID+"@"+st.Format("20060102")
					if e.AllDay {
						x.Start, x.End = st, addDays(st, daysBetween(e.Start, e.End))
					} else {
						x.Start, x.End = st, st.Add(e.End.Sub(e.Start))
					}
					group = append(group, &x)
				}
			} else {
				out.simple++
			}
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
		if len(v.tags) > 0 {
			if out.tags == nil {
				out.tags = map[*Event][]string{}
			}
			for _, x := range group {
				out.tags[x] = v.tags
			}
		}
	}
	// An occurrence changed or cancelled on its own, which its series skips,
	// or which goes, of a rule come as its times.
	gone := map[*Event]bool{}
	for _, sk := range out.skips {
		for _, x := range series[sk.uid] {
			switch {
			case x.Repeat == RepeatNone && sameDay(x.Start, sk.day):
				gone[x] = true
			case x.Repeat != RepeatNone && !x.skipped(sk.day):
				x.Skip = append(x.Skip, sk.day)
			}
		}
	}
	out.events = slices.DeleteFunc(out.events, func(x *Event) bool { return gone[x] })
	return out, nil
}

// icsNow is the time that expanded rules are around: from a year before it
// to two after.
var icsNow = time.Now

// expandRule returns the starts of the times of e by a rule NextTodo
// cannot repeat by, from a year before now to two after, in e's zone: by
// month, day of the month, day of the week (the fourth Thursday, the last
// Friday), position in the set, every so many days, weeks, months or
// years, so many times or until a moment. False for one it cannot read.
func expandRule(e *Event, rule string, now time.Time) ([]time.Time, bool) {
	parts := map[string]string{}
	for _, p := range strings.Split(strings.ToUpper(rule), ";") {
		if k, v, ok := strings.Cut(p, "="); ok {
			parts[k] = v
		}
	}
	for _, k := range []string{"BYWEEKNO", "BYYEARDAY", "BYHOUR", "BYMINUTE", "BYSECOND"} {
		if parts[k] != "" {
			return nil, false
		}
	}
	freq := parts["FREQ"]
	if freq != "DAILY" && freq != "WEEKLY" && freq != "MONTHLY" && freq != "YEARLY" {
		return nil, false
	}
	ints := func(s string) ([]int, bool) {
		var out []int
		for _, v := range strings.Split(s, ",") {
			if v == "" {
				continue
			}
			n, err := strconv.Atoi(v)
			if err != nil || n == 0 {
				return nil, false
			}
			out = append(out, n)
		}
		return out, true
	}
	interval := 1
	if n, err := strconv.Atoi(parts["INTERVAL"]); err == nil && n > 0 {
		interval = n
	}
	months, ok1 := ints(parts["BYMONTH"])
	monthDays, ok2 := ints(parts["BYMONTHDAY"])
	setPos, ok3 := ints(parts["BYSETPOS"])
	if !ok1 || !ok2 || !ok3 {
		return nil, false
	}
	type byDay struct {
		n  int // the nth of the month, from its end if below 0; 0 for every
		wd time.Weekday
	}
	codes := map[string]time.Weekday{"SU": 0, "MO": 1, "TU": 2, "WE": 3, "TH": 4, "FR": 5, "SA": 6}
	var days []byDay
	for _, v := range strings.Split(parts["BYDAY"], ",") {
		if v == "" {
			continue
		}
		wd, ok := codes[v[max(len(v)-2, 0):]]
		n, err := strconv.Atoi(v[:len(v)-2])
		if !ok || err != nil && len(v) > 2 {
			return nil, false
		}
		days = append(days, byDay{n, wd})
	}
	if freq == "YEARLY" && len(months) == 0 && slices.ContainsFunc(days, func(d byDay) bool { return d.n != 0 }) {
		return nil, false // the 20th Monday of the year
	}
	count, _ := strconv.Atoi(parts["COUNT"])
	var until time.Time
	if u := parts["UNTIL"]; u != "" {
		t, date, err := icsTime(icsLine{params: map[string]string{}, value: u}, e.Start.Location())
		if err != nil {
			return nil, false
		}
		if until = t; date {
			until = addDays(t, 1).Add(-time.Second)
		}
	}
	from, to := now.AddDate(-1, 0, 0), now.AddDate(2, 0, 0)
	loc, start := e.Start.Location(), day(e.Start)
	inMonths := func(m time.Month) bool { return len(months) == 0 || slices.Contains(months, int(m)) }
	weekdayOK := func(t time.Time) bool {
		return len(days) == 0 || slices.ContainsFunc(days, func(d byDay) bool { return d.wd == t.Weekday() })
	}
	// The days a month holds of the rule.
	ofMonth := func(y int, m time.Month) []time.Time {
		last := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
		var out []time.Time
		switch {
		case len(monthDays) > 0:
			for _, md := range monthDays {
				if md < 0 {
					md = last + 1 + md
				}
				if t := time.Date(y, m, md, 0, 0, 0, 0, loc); md >= 1 && md <= last && weekdayOK(t) {
					out = append(out, t)
				}
			}
		case len(days) > 0:
			for _, d := range days {
				var all []time.Time
				for md := 1; md <= last; md++ {
					if t := time.Date(y, m, md, 0, 0, 0, 0, loc); t.Weekday() == d.wd {
						all = append(all, t)
					}
				}
				switch {
				case d.n == 0:
					out = append(out, all...)
				case d.n > 0 && d.n <= len(all):
					out = append(out, all[d.n-1])
				case d.n < 0 && -d.n <= len(all):
					out = append(out, all[len(all)+d.n])
				}
			}
		case start.Day() <= last:
			out = append(out, time.Date(y, m, start.Day(), 0, 0, 0, 0, loc))
		}
		return out
	}
	var out []time.Time
	n := 0
	for i := 0; i < 20000; i++ {
		var set []time.Time
		var begins time.Time
		switch freq {
		case "YEARLY":
			y := start.Year() + i*interval
			begins = time.Date(y, 1, 1, 0, 0, 0, 0, loc)
			ms := months
			if len(ms) == 0 {
				ms = []int{int(start.Month())}
			}
			for _, m := range ms {
				set = append(set, ofMonth(y, time.Month(m))...)
			}
		case "MONTHLY":
			begins = time.Date(start.Year(), start.Month()+time.Month(i*interval), 1, 0, 0, 0, 0, loc)
			if inMonths(begins.Month()) {
				set = ofMonth(begins.Year(), begins.Month())
			}
		case "WEEKLY":
			begins = addDays(weekStart(start, time.Monday), 7*i*interval)
			if len(days) == 0 {
				set = []time.Time{addDays(begins, (int(start.Weekday())+6)%7)}
			}
			for _, d := range days {
				set = append(set, addDays(begins, (int(d.wd)+6)%7))
			}
			set = slices.DeleteFunc(set, func(t time.Time) bool { return !inMonths(t.Month()) })
		case "DAILY":
			begins = addDays(start, i*interval)
			if inMonths(begins.Month()) && weekdayOK(begins) &&
				(len(monthDays) == 0 || slices.Contains(monthDays, begins.Day())) {
				set = []time.Time{begins}
			}
		}
		if begins.After(to) || !until.IsZero() && begins.After(until) {
			break
		}
		slices.SortFunc(set, time.Time.Compare)
		set = slices.CompactFunc(set, time.Time.Equal)
		if len(setPos) > 0 {
			var picked []time.Time
			for _, p := range setPos {
				switch {
				case p > 0 && p <= len(set):
					picked = append(picked, set[p-1])
				case p < 0 && -p <= len(set):
					picked = append(picked, set[len(set)+p])
				}
			}
			slices.SortFunc(picked, time.Time.Compare)
			set = picked
		}
		for _, d := range set {
			t := time.Date(d.Year(), d.Month(), d.Day(), e.Start.Hour(), e.Start.Minute(), 0, 0, loc)
			if t.Before(e.Start) {
				continue
			}
			if n++; count > 0 && n > count || !until.IsZero() && t.After(until) || t.After(to) {
				return out, true
			}
			if !t.Before(from) {
				out = append(out, t)
			}
		}
	}
	return out, true
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
	for _, k := range []string{"BYWEEKNO", "BYYEARDAY", "BYHOUR", "BYMINUTE", "BYSECOND"} {
		if parts[k] != "" {
			return nil, false
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
	// Every year on its day, as most holidays and birthdays.
	yearly := parts["FREQ"] == "YEARLY" && interval == 1 && len(days) == 0 && parts["BYSETPOS"] == "" &&
		(parts["BYMONTH"] == "" || parts["BYMONTH"] == strconv.Itoa(int(e.Start.Month()))) &&
		(parts["BYMONTHDAY"] == "" || parts["BYMONTHDAY"] == strconv.Itoa(e.Start.Day()))
	if !yearly && (parts["BYSETPOS"] != "" || parts["BYMONTH"] != "" || parts["BYMONTHDAY"] != "" && parts["BYMONTHDAY"] != strconv.Itoa(e.Start.Day())) {
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
	case yearly:
		e.Repeat = RepeatYearly
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
		if e.Sub != "" {
			continue // the calendar it comes from has it
		}
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
				RepeatBiweekly: "FREQ=WEEKLY;INTERVAL=2", RepeatMonthly: "FREQ=MONTHLY", RepeatYearly: "FREQ=YEARLY",
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
	byUID := d.byUID()
	for _, e := range events {
		if old := byUID[e.UID]; e.UID != "" && old != nil {
			// What is NextTodo's own stays: its project, its alert and the
			// tag the user gave it.
			e.ID, e.Project = old.ID, old.Project
			if old.Kind != "" || e.Kind == "" {
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
// each new one, or untagged yet, with the first tag its categories name,
// and has the series imported before skip what the file changed on its
// own. A tag the user gave an event stays.
func (d *Data) importCalendar(got icsImport) (added, changed int) {
	known := d.byUID()
	for e, names := range got.tags {
		if old := known[e.UID]; e.UID == "" || old == nil || old.Kind == "" {
			e.Kind = d.kindNamed(names)
		}
	}
	added, changed = d.importEvents(got.events)
	d.skipImported(got.skips)
	return added, changed
}

// kindNamed is the first tag of the first of names there is one of.
func (d *Data) kindNamed(names []string) KindID {
	for _, name := range names {
		for _, k := range d.Kinds {
			if strings.EqualFold(k.Name, name) {
				return k.ID
			}
		}
	}
	return ""
}

// byUID finds the events by the UIDs calendar files know them by: those
// imported by theirs, and NextTodo's own by ID@nexttodo.
func (d *Data) byUID() map[string]*Event {
	m := map[string]*Event{}
	for _, e := range d.Events {
		m[e.ID+"@nexttodo"] = e
		if e.UID != "" {
			m[e.UID] = e
		}
	}
	return m
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

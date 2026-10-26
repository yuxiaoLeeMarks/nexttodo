package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// when is what a line typed to add a task or an event says of its time,
// as Fantastical reads "Group meeting fri 3-4pm" or "明天下午3点组会".
type when struct {
	title   string    // the line without what it says of time
	day     time.Time // the day it names, or zero
	clock   bool      // whether it names a time of day, hour:minute
	hour    int
	minute  int
	minutes int // how long, 0 when it does not say
}

// said reports whether the line says anything of its time.
func (w when) said() bool { return !w.day.IsZero() || w.clock || w.minutes > 0 }

// start returns when the line says a thing starts, on its day or today.
func (w when) start(now time.Time) time.Time {
	d := w.day
	if d.IsZero() {
		d = day(now)
	}
	return at(d, w.hour, w.minute)
}

// rule finds one way of saying a time: apply reads a match into w, or
// returns false to leave it in the title.
type rule struct {
	re    *regexp.Regexp
	kind  int // ruleDay, ruleClock or ruleLength: the first match of a kind wins
	apply func(m []string, w *when, s *parseState) bool
}

const (
	ruleDay = iota
	ruleClock
	ruleLength
)

// parseState is what the rules read besides the match.
type parseState struct {
	now     time.Time
	first   time.Weekday
	evening bool // "tonight", "今晚": a bare hour is in the evening
}

// The words joining a time to the rest, which go with it.
const (
	onEN = `(?:\b(?:on|by|due)\s+)?`
	atEN = `(?:\b(?:at|from)\s+)?`
	// A time in Chinese: 下午3点半, 15:30, 三点.
	timeZH = `(上午|早上|早晨|中午|下午|傍晚|晚上|凌晨)?\s*(\d{1,2}|[一二两三四五六七八九十]{1,3})(?:[:：](\d{2})|点(?:(半)|(\d{1,2})分?|钟)?)`
)

var weekdaysEN = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

var weekdaysZH = map[string]time.Weekday{
	"日": time.Sunday, "天": time.Sunday, "一": time.Monday, "二": time.Tuesday, "三": time.Wednesday,
	"四": time.Thursday, "五": time.Friday, "六": time.Saturday,
}

var monthsEN = map[string]time.Month{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

const monthNames = `(jan|january|feb|february|mar|march|apr|april|may|jun|june|jul|july|aug|august|sep|sept|september|oct|october|nov|november|dec|december)`

// rules are tried in order, ranges before the times they hold.
var rules = []rule{
	// Days.
	{regexp.MustCompile(`(?i)` + onEN + `\b(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		return w.setDate(atoi(m[1]), atoi(m[2]), atoi(m[3]), s.now)
	}},
	{regexp.MustCompile(`(?i)` + onEN + `\b` + monthNames + `\.?\s+(\d{1,2})(?:st|nd|rd|th)?\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		return w.setDate(0, int(monthsEN[strings.ToLower(m[1][:3])]), atoi(m[2]), s.now)
	}},
	{regexp.MustCompile(`(?i)` + onEN + `\b(\d{1,2})(?:st|nd|rd|th)?\s+` + monthNames + `\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		return w.setDate(0, int(monthsEN[strings.ToLower(m[2][:3])]), atoi(m[1]), s.now)
	}},
	{regexp.MustCompile(`(?i)` + onEN + `\b(\d{1,2})/(\d{1,2})\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		return w.setDate(0, atoi(m[1]), atoi(m[2]), s.now)
	}},
	{regexp.MustCompile(`(\d{1,2})月(\d{1,2})[日号]?`), ruleDay, func(m []string, w *when, s *parseState) bool {
		return w.setDate(0, atoi(m[1]), atoi(m[2]), s.now)
	}},
	{regexp.MustCompile(`(?i)` + onEN + `\b(today|tonight|tomorrow|tmrw|tmr)\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		switch strings.ToLower(m[1]) {
		case "today":
			w.day = day(s.now)
		case "tonight":
			w.day, s.evening = day(s.now), true
		default:
			w.day = addDays(day(s.now), 1)
		}
		return true
	}},
	{regexp.MustCompile(`(大后天|后天|今天|今日|今晚|明天|明日|明早|明晚)`), ruleDay, func(m []string, w *when, s *parseState) bool {
		n := map[string]int{"今天": 0, "今日": 0, "今晚": 0, "明天": 1, "明日": 1, "明早": 1, "明晚": 1, "后天": 2, "大后天": 3}[m[1]]
		w.day = addDays(day(s.now), n)
		s.evening = strings.HasSuffix(m[1], "晚")
		return true
	}},
	{regexp.MustCompile(`(?i)` + onEN + `\b(?:(next|this)\s+)?(monday|mon|tuesday|tues|tue|wednesday|wed|thursday|thurs|thur|thu|friday|fri|saturday|sunday)\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		w.day = weekday(s, weekdaysEN[strings.ToLower(m[2][:3])], strings.ToLower(m[1]))
		return true
	}},
	{regexp.MustCompile(`(下下|下个?|本|这个?)?(?:周|星期|礼拜)([一二三四五六日天])`), ruleDay, func(m []string, w *when, s *parseState) bool {
		which := map[string]string{"下下": "next2", "下": "next", "下个": "next", "本": "this", "这": "this", "这个": "this"}[m[1]]
		w.day = weekday(s, weekdaysZH[m[2]], which)
		return true
	}},
	{regexp.MustCompile(`(?i)\bin\s+(\d+|a|one|two|three)\s+(days?|weeks?)\b`), ruleDay, func(m []string, w *when, s *parseState) bool {
		n := map[string]int{"a": 1, "one": 1, "two": 2, "three": 3}[strings.ToLower(m[1])]
		if n == 0 {
			n = atoi(m[1])
		}
		if strings.HasPrefix(strings.ToLower(m[2]), "week") {
			n *= 7
		}
		w.day = addDays(day(s.now), n)
		return true
	}},
	{regexp.MustCompile(`(\d+|[一二两三四五六七八九十]+)天后`), ruleDay, func(m []string, w *when, s *parseState) bool {
		w.day = addDays(day(s.now), number(m[1]))
		return true
	}},
	{regexp.MustCompile(`(?i)\bnext\s+week\b|下个?(?:周|星期|礼拜)`), ruleDay, func(m []string, w *when, s *parseState) bool {
		w.day = addDays(weekStart(s.now, s.first), 7)
		return true
	}},

	// Times of day, from and to.
	{regexp.MustCompile(`(?i)` + atEN + `\b(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*(?:-|–|—|~|\bto\b|\buntil\b|\btill\b)\s*(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\b`), ruleClock, func(m []string, w *when, s *parseState) bool {
		if m[2] == "" && m[3] == "" && m[5] == "" && m[6] == "" {
			return false // 3-4: pages, not hours
		}
		p1, p2 := strings.ToLower(m[3]), strings.ToLower(m[6])
		if p1 == "" && p2 != "" {
			// 3-4pm, but 11-1pm.
			p1 = p2
			if h1, h2 := atoi(m[1]), atoi(m[4]); p2 == "pm" && h1 != 12 && (h1 > h2 || h2 == 12) {
				p1 = "am"
			}
		}
		return w.setRange(atoi(m[1]), atoi(m[2]), p1, atoi(m[4]), atoi(m[5]), p2, s)
	}},
	{regexp.MustCompile(timeZH + `\s*(?:到|至|-|–|—|~)\s*` + timeZH), ruleClock, func(m []string, w *when, s *parseState) bool {
		h1, m1, ok1 := clockZH(m[1:6])
		p2 := m[6]
		if p2 == "" {
			p2 = m[1] // 下午3点到5点
		}
		h2, m2, ok2 := clockZH(append([]string{p2}, m[7:11]...))
		if !ok1 || !ok2 {
			return false
		}
		return w.setRange(h1, m1, zhPeriod(m[1]), h2, m2, zhPeriod(p2), s)
	}},
	{regexp.MustCompile(`(?i)` + atEN + `\b(\d{1,2})(?::(\d{2}))?\s*(am|pm)\b`), ruleClock, func(m []string, w *when, s *parseState) bool {
		return w.setClock(atoi(m[1]), atoi(m[2]), strings.ToLower(m[3]), s)
	}},
	{regexp.MustCompile(`(?i)` + atEN + `\b(\d{1,2}):(\d{2})\b`), ruleClock, func(m []string, w *when, s *parseState) bool {
		return w.setClock(atoi(m[1]), atoi(m[2]), "", s)
	}},
	{regexp.MustCompile(`(?i)\bat\s+(\d{1,2})\b`), ruleClock, func(m []string, w *when, s *parseState) bool {
		return w.setClock(atoi(m[1]), 0, "", s)
	}},
	{regexp.MustCompile(`(?i)` + atEN + `\b(noon|midnight)\b`), ruleClock, func(m []string, w *when, s *parseState) bool {
		if strings.ToLower(m[1]) == "noon" {
			return w.setClock(12, 0, "pm", s)
		}
		return w.setClock(0, 0, "", s)
	}},
	{regexp.MustCompile(timeZH), ruleClock, func(m []string, w *when, s *parseState) bool {
		h, min, ok := clockZH(m[1:6])
		return ok && w.setClock(h, min, zhPeriod(m[1]), s)
	}},

	// How long.
	{regexp.MustCompile(`(?i)(?:\bfor\s+)?\b(\d+)\s*h\s*(\d{1,2})\s*(?:m|min|mins)?\b`), ruleLength, func(m []string, w *when, s *parseState) bool {
		return w.setLength(atoi(m[1])*60 + atoi(m[2]))
	}},
	{regexp.MustCompile(`(?i)(?:\bfor\s+)?\b(\d+(?:\.\d+)?)\s*(h|hr|hrs|hours?|mins?|minutes?)\b`), ruleLength, func(m []string, w *when, s *parseState) bool {
		n, _ := strconv.ParseFloat(m[1], 64)
		if strings.HasPrefix(strings.ToLower(m[2]), "h") {
			n *= 60
		}
		return w.setLength(int(n + 0.5))
	}},
	{regexp.MustCompile(`(?i)\bfor\s+(\d+)\s*m\b`), ruleLength, func(m []string, w *when, s *parseState) bool {
		return w.setLength(atoi(m[1]))
	}},
	{regexp.MustCompile(`(?i)\bfor\s+(?:(an|one)\s+hour|half\s+an\s+hour)\b`), ruleLength, func(m []string, w *when, s *parseState) bool {
		if m[1] == "" {
			return w.setLength(30)
		}
		return w.setLength(60)
	}},
	{regexp.MustCompile(`(\d+(?:\.\d+)?|[一二两三四五六七八九十]+)\s*个?(半)?(?:小时|钟头)`), ruleLength, func(m []string, w *when, s *parseState) bool {
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			n = float64(number(m[1]))
		}
		if m[2] != "" {
			n += 0.5
		}
		return w.setLength(int(n*60 + 0.5))
	}},
	{regexp.MustCompile(`半个?小时`), ruleLength, func(m []string, w *when, s *parseState) bool {
		return w.setLength(30)
	}},
	{regexp.MustCompile(`(\d+)\s*分钟`), ruleLength, func(m []string, w *when, s *parseState) bool {
		return w.setLength(atoi(m[1]))
	}},
}

// parseWhen reads what line says of a day and, when times, of a time of
// day and how long. A line that is nothing but a time is a title.
func parseWhen(line string, now time.Time, first time.Weekday, times bool) when {
	s := &parseState{now: now, first: first}
	w := when{}
	done := map[int]bool{}
	text := line
	for _, r := range rules {
		if done[r.kind] || r.kind != ruleDay && !times {
			continue
		}
		for _, loc := range r.re.FindAllStringSubmatchIndex(text, -1) {
			if possessive(text, loc[1]) {
				continue // Friday's paper
			}
			m := make([]string, len(loc)/2)
			for i := range m {
				if loc[2*i] >= 0 {
					m[i] = text[loc[2*i]:loc[2*i+1]]
				}
			}
			if r.apply(m, &w, s) {
				text = text[:loc[0]] + " " + text[loc[1]:]
				done[r.kind] = true
				break
			}
		}
	}
	w.title = tidyTitle(text)
	if w.title == "" || !w.said() {
		return when{title: strings.TrimSpace(line)}
	}
	return w
}

// possessive reports whether text goes on at i with an apostrophe.
func possessive(text string, i int) bool {
	r, _ := utf8.DecodeRuneInString(text[i:])
	return r == '\'' || r == '’'
}

// tidyTitle closes the gaps the times left.
func tidyTitle(s string) string {
	return strings.Trim(strings.Join(strings.Fields(s), " "), " ,，、;；-–—:：")
}

// setDate sets the day y-m-d, this year or, gone, the next when y is 0.
func (w *when) setDate(y, m, d int, now time.Time) bool {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return false
	}
	guess := y == 0
	if guess {
		y = now.Year()
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, now.Location())
	if t.Day() != d {
		return false // 2/30
	}
	if guess && t.Before(day(now)) {
		t = t.AddDate(1, 0, 0)
	}
	w.day = t
	return true
}

// weekday returns the day wd: the coming one, or of this week, the next
// or the one after, as which says.
func weekday(s *parseState, wd time.Weekday, which string) time.Time {
	today := day(s.now)
	if which == "" {
		return addDays(today, (int(wd)-int(today.Weekday())+7)%7)
	}
	d := addDays(weekStart(today, s.first), (int(wd)-int(s.first)+7)%7)
	switch which {
	case "next":
		d = addDays(d, 7)
	case "next2":
		d = addDays(d, 14)
	}
	return d
}

// hour24 reads h in the period said: "pm", "am" or "" for none, when an
// hour from 1 to 6 is in the afternoon, as meetings are, or the evening
// said earlier.
func hour24(h int, period string, s *parseState) int {
	switch period {
	case "pm":
		if h < 12 {
			h += 12
		}
	case "am":
		if h == 12 {
			h = 0
		}
	case "noon":
		if h < 11 {
			h += 12
		}
	case "morning": // 上午12点 is noon
	default:
		if h >= 1 && h <= 6 || s.evening && h < 12 {
			h += 12
		}
	}
	return h
}

func (w *when) setClock(h, m int, period string, s *parseState) bool {
	if h > 23 || m > 59 || period != "" && (h > 12 || h == 0) {
		return false
	}
	w.clock, w.hour, w.minute = true, hour24(h, period, s), m
	return true
}

func (w *when) setRange(h1, m1 int, p1 string, h2, m2 int, p2 string, s *parseState) bool {
	if !w.setClock(h1, m1, p1, s) {
		return false
	}
	end := h2*60 + m2
	if h2 <= 23 && m2 <= 59 {
		end = hour24(h2, p2, s)*60 + m2
		if p2 == "" && end <= w.hour*60+w.minute && h2 < 12 {
			end += 12 * 60 // 10:00-1:00
		}
	}
	if n := end - (w.hour*60 + w.minute); n > 0 && n < 24*60 {
		w.minutes = n
	}
	return true
}

func (w *when) setLength(n int) bool {
	if n <= 0 || n > 24*60 {
		return false
	}
	if w.minutes == 0 {
		w.minutes = n
	}
	return true
}

// clockZH reads a Chinese time's period, hour and minutes, as timeZH
// groups them.
func clockZH(g []string) (h, m int, ok bool) {
	h = number(g[1])
	switch {
	case g[2] != "":
		m = atoi(g[2])
	case g[3] != "":
		m = 30
	case g[4] != "":
		m = atoi(g[4])
	}
	return h, m, h >= 0 && h <= 24 && m <= 59
}

func zhPeriod(p string) string {
	switch p {
	case "下午", "傍晚", "晚上":
		return "pm"
	case "中午":
		return "noon"
	case "上午", "早上", "早晨":
		return "morning"
	case "凌晨":
		return "am"
	}
	return ""
}

// number reads a number written in digits or in Chinese, to ninety-nine.
func number(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	digits := map[rune]int{'一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	tens, ones := 0, 0
	for _, r := range s {
		if r == '十' {
			tens, ones = max(ones, 1), 0
			continue
		}
		ones = digits[r]
	}
	return tens*10 + ones
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

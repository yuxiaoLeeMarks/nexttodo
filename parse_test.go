package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 15, 0, 0, time.Local) // a Monday
	d := func(m time.Month, dd int) time.Time { return time.Date(2026, m, dd, 0, 0, 0, 0, time.Local) }
	cases := []struct {
		in      string
		reads   int
		title   string
		day     time.Time
		clock   string // "15:04", or "" for none
		minutes int
	}{
		{"Group meeting fri 3-4pm", readSpan, "Group meeting", d(10, 30), "15:00", 60},
		{"Call Lin tomorrow at 9", readSpan, "Call Lin", d(10, 27), "09:00", 0},
		{"Review draft tomorrow", readDay, "Review draft", d(10, 27), "", 0},
		{"Seminar next Wednesday 14:00-15:30", readSpan, "Seminar", d(11, 4), "14:00", 90},
		{"Lunch with Bob at noon", readSpan, "Lunch with Bob", time.Time{}, "12:00", 0},
		{"Submit camera-ready by Nov 3", readDay, "Submit camera-ready", d(11, 3), "", 0},
		{"Deadline 10/20", readDay, "Deadline", time.Date(2027, 10, 20, 0, 0, 0, 0, time.Local), "", 0},
		{"Writing 7pm for 2h", readSpan, "Writing", time.Time{}, "19:00", 120},
		{"Retreat 2026-11-12", readDay, "Retreat", d(11, 12), "", 0},
		{"Dinner tonight at 7", readSpan, "Dinner", d(10, 26), "19:00", 0},
		{"Standup 11-1pm", readSpan, "Standup", time.Time{}, "11:00", 120},
		{"Mon", readDay, "Mon", time.Time{}, "", 0},
		{"Read Friday's paper", readDay, "Read Friday's paper", time.Time{}, "", 0},
		{"Read chapters 3-4", readSpan, "Read chapters 3-4", time.Time{}, "", 0},
		{"Sync in 2 weeks", readDay, "Sync", d(11, 9), "", 0},
		{"明天下午3点组会", readSpan, "组会", d(10, 27), "15:00", 0},
		{"组会 周五下午3点到5点", readSpan, "组会", d(10, 30), "15:00", 120},
		{"下周三 上午10点半 开题报告", readSpan, "开题报告", d(11, 4), "10:30", 0},
		{"今晚8点 写论文 两个小时", readSpan, "写论文", d(10, 26), "20:00", 120},
		{"读文献 一个半小时 3点", readSpan, "读文献", time.Time{}, "15:00", 90},
		{"提交终稿 11月3日", readDay, "提交终稿", d(11, 3), "", 0},
		{"上午10点到12点 实验", readSpan, "实验", time.Time{}, "10:00", 120},
		{"三天后 复查数据", readDay, "复查数据", d(10, 29), "", 0},
		{"组会 15:00", readDay, "组会 15:00", time.Time{}, "", 0},
		// Weeks said every way.
		{"Review Friday next week", readDay, "Review", d(11, 6), "", 0},
		{"next week Friday review", readDay, "review", d(11, 6), "", 0},
		{"Plan this friday", readDay, "Plan", d(10, 30), "", 0},
		{"Hike next weekend", readDay, "Hike", d(11, 7), "", 0},
		{"下周末 爬山", readDay, "爬山", d(11, 7), "", 0},
		{"这周末 整理数据", readDay, "整理数据", d(10, 31), "", 0},
		// Not times, nor dates.
		{"多读一点文献", readClock, "多读一点文献", time.Time{}, "", 0},
		{"回复审稿人第2点意见", readClock, "回复审稿人第2点意见", time.Time{}, "", 0},
		{"注意两点", readClock, "注意两点", time.Time{}, "", 0},
		{"Dilute antibody 1:50", readClock, "Dilute antibody 1:50", time.Time{}, "", 0},
		{"Read 1/3 of the book", readClock, "Read 1/3 of the book", time.Time{}, "", 0},
		{"Reviewer 2 may object", readClock, "Reviewer 2 may object", time.Time{}, "", 0},
		{"下午三点 组会", readSpan, "组会", time.Time{}, "15:00", 0},
		{"三点半 组会", readSpan, "组会", time.Time{}, "15:30", 0},
		// Night and the 24-hour clock.
		{"晚上12点 提交", readClock, "提交", time.Time{}, "00:00", 0},
		{"Call home tonight at 12", readClock, "Call home", d(10, 26), "00:00", 0},
		{"Server check tonight at 1", readClock, "Server check", d(10, 26), "01:00", 0},
		{"Flight 05:30", readClock, "Flight", time.Time{}, "05:30", 0},
		{"Backup 03:00", readClock, "Backup", time.Time{}, "03:00", 0},
		{"Deploy at midnight", readClock, "Deploy", time.Time{}, "00:00", 0},
		// Over midnight.
		{"Observing 晚上11点到凌晨1点", readSpan, "Observing", time.Time{}, "23:00", 120},
		{"Party 10pm-2am", readSpan, "Party", time.Time{}, "22:00", 240},
		{"Gala 8pm-12am", readSpan, "Gala", time.Time{}, "20:00", 240},
		{"晚上10点到12点 跑实验", readSpan, "跑实验", time.Time{}, "22:00", 120},
		{"Shift 23:00-01:00", readSpan, "Shift", time.Time{}, "23:00", 120},
		// Years, and what joins the time.
		{"2026年10月30日 答辩", readDay, "答辩", d(10, 30), "", 0},
		{"2028年3月1日 结题", readDay, "结题", time.Date(2028, 3, 1, 0, 0, 0, 0, time.Local), "", 0},
		{"Deadline 10/30/2026", readDay, "Deadline", d(10, 30), "", 0},
		{"Visa Nov 3rd 2027", readDay, "Visa", time.Date(2027, 11, 3, 0, 0, 0, 0, time.Local), "", 0},
		{"Submit report by 5pm", readClock, "Submit report", time.Time{}, "17:00", 0},
		{"下午5点前 提交报告", readClock, "提交报告", time.Time{}, "17:00", 0},
		{"Meeting 9:30", readSpan, "Meeting", time.Time{}, "09:30", 0},
	}
	for _, c := range cases {
		w := parseWhen(c.in, now, time.Monday, c.reads)
		clock := ""
		if w.clock {
			clock = w.start(now).Format("15:04")
		}
		if w.title != c.title || !w.day.Equal(c.day) || clock != c.clock || w.minutes != c.minutes {
			t.Errorf("%q: got %q %v %q %d, want %q %v %q %d", c.in, w.title, w.day.Format(time.DateOnly), clock, w.minutes,
				c.title, c.day.Format(time.DateOnly), c.clock, c.minutes)
		}
	}
}

func TestQuickAddReadsTime(t *testing.T) {
	a := testApp("en")
	a.page = PageTasks
	tt := newTester(t, a, "en")
	tt.Click("Add a task")
	tt.Type("Write the rebuttal fri")
	friday := addDays(day(monday), 4)
	if !tt.HasText(a.fmt(friday, "MMMEd")) {
		t.Fatalf("no chip for the day; texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	added := taskTitled(a, "Write the rebuttal")
	if added == nil || !added.Due.Equal(friday) || !added.inMyDay(monday) {
		t.Fatalf("task %+v", added)
	}

	// The × adds the line as typed.
	tt.Type("Ship v2 tomorrow")
	if err := tt.Click("Add as typed, without the date"); err != nil {
		t.Fatal(err)
	}
	tt.Click("Add a task")
	tt.Key(0, ui.KeyEnter)
	if x := taskTitled(a, "Ship v2 tomorrow"); x == nil || !x.Due.IsZero() {
		t.Fatalf("task %+v", x)
	}
	if len(a.literal) != 0 {
		t.Error("the field still adds lines as typed")
	}

	// The panel's events take their day and hours from the line.
	pt := ui.NewTester(a.panelView, 340, 480)
	pt.SetLocale("en")
	pt.Frame()
	pt.Click("Event")
	pt.Click("Add an event today")
	pt.Type("Coffee with Ada tomorrow 3-4:30pm")
	if a.quick.start.Hour() != 15 || a.quick.minutes != 90 {
		t.Errorf("the pickers show %v for %d minutes", a.quick.start, a.quick.minutes)
	}
	pt.Key(0, ui.KeyEnter)
	e := eventTitled(a, "Coffee with Ada")
	tomorrow := addDays(day(monday), 1)
	if e == nil || !e.Start.Equal(at(tomorrow, 15, 0)) || !e.End.Equal(at(tomorrow, 16, 30)) {
		t.Fatalf("event %+v", e)
	}
}

func taskTitled(a *app, title string) *Task {
	for _, x := range a.data.Tasks {
		if x.Title == title {
			return x
		}
	}
	return nil
}

func TestQuickEventLate(t *testing.T) {
	a := testApp("en")
	late := at(monday, 23, 40)
	a.now = func() time.Time { return late }
	a.quick.start, a.quick.mode = nextHalfHour(late), 1
	a.quick.title = "Telescope time"
	a.quickAddNow(time.Monday)
	e := eventTitled(a, "Telescope time")
	if e == nil || !e.Start.Equal(at(addDays(monday, 1), 0, 0)) {
		t.Fatalf("an event added at 23:40 starts %v", e)
	}
}

// A time gone today, said with no day, is tomorrow's; the chip says so.
func TestWhenRolls(t *testing.T) {
	evening := time.Date(2026, 10, 26, 21, 0, 0, 0, time.Local)
	w := parseWhen("Call Lin at 9", evening, time.Monday, readClock)
	if got := w.start(evening); !got.Equal(time.Date(2026, 10, 27, 9, 0, 0, 0, time.Local)) {
		t.Errorf("reminds %v", got)
	}
	a := testApp("en")
	a.now = func() time.Time { return evening }
	if s := a.whenText(w); s != "Tomorrow · "+a.clock(w.start(evening)) {
		t.Errorf("the chip says %q", s)
	}
	if w := parseWhen("Call home tonight at 12", evening, time.Monday, readClock); !w.start(evening).Equal(time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local)) {
		t.Errorf("tonight at 12 is %v", w.start(evening))
	}
}

// The panel's next line does not take the day of the one before.
func TestPanelNextLine(t *testing.T) {
	a := testApp("en")
	pt := ui.NewTester(a.panelView, 340, 480)
	pt.SetLocale("en")
	a.quick.start = nextHalfHour(monday)
	pt.Frame()
	pt.Click("Event")
	pt.Click("Add an event today")
	pt.Type("Coffee with Ada tomorrow 3pm")
	pt.Key(0, ui.KeyEnter)
	pt.Type("Call Bob")
	pt.Key(0, ui.KeyEnter)
	ada, bob := eventTitled(a, "Coffee with Ada"), eventTitled(a, "Call Bob")
	if ada == nil || !ada.Start.Equal(at(addDays(monday, 1), 15, 0)) {
		t.Fatalf("Ada %+v", ada)
	}
	if bob == nil || !sameDay(bob.Start, monday) {
		t.Fatalf("Bob %+v, want today", bob)
	}
}

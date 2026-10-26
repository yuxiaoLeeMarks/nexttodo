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
	}
	for _, c := range cases {
		w := parseWhen(c.in, now, time.Monday, c.reads)
		clock := ""
		if w.clock {
			clock = at(now, w.hour, w.minute).Format("15:04")
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

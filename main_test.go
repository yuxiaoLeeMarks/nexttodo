package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"nexttodo/locales"
)

// monday is the morning the tests run in: Monday, 26 October 2026.
var monday = time.Date(2026, 10, 26, 10, 20, 0, 0, time.Local)

func testApp(lang string) *app {
	a := newApp(sampleData(monday, lang), "")
	a.now = func() time.Time { return monday }
	a.schedAnchor = day(monday)
	a.alertedTo = monday.Add(-time.Minute)
	return a
}

func newTester(t *testing.T, a *app, lang string) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(a.view, 1280, 820)
	tt.SetLocale(lang)
	tt.Frame()
	return tt
}

func eventTitled(a *app, title string) *Event {
	for _, e := range a.data.Events {
		if e.Title == title {
			return e
		}
	}
	return nil
}

func TestWeekStart(t *testing.T) {
	if got := weekStart(monday.Add(48*time.Hour), time.Monday); !sameDay(got, monday) {
		t.Errorf("week of Wednesday starts %v", got)
	}
	if got := weekStart(monday, time.Sunday); got.Weekday() != time.Sunday || daysBetween(got, monday) != 1 {
		t.Errorf("Sunday week of Monday starts %v", got)
	}
	if n := daysBetween(monday, monday.Add(-30*time.Hour)); n != -1 {
		t.Errorf("days to yesterday, before 10 am: %d", n)
	}
}

func TestSortTasks(t *testing.T) {
	d := day(monday)
	ts := []*Task{
		{ID: "done", Done: true},
		{ID: "undated"},
		{ID: "later", Due: addDays(d, 3)},
		{ID: "soon-low", Due: d, Priority: 1},
		{ID: "soon-high", Due: d, Priority: 3},
	}
	sortTasks(ts)
	var ids []string
	for _, x := range ts {
		ids = append(ids, x.ID)
	}
	if got := strings.Join(ids, " "); got != "soon-high soon-low later undated done" {
		t.Errorf("order %s", got)
	}
}

func TestLayoutDay(t *testing.T) {
	d := day(monday)
	ev := func(id string, h0, m0, h1, m1 int) *Event {
		return &Event{ID: id, Start: at(d, h0, m0), End: at(d, h1, m1)}
	}
	ps := layoutDay([]*Event{ev("a", 9, 0, 11, 0), ev("b", 10, 0, 12, 0), ev("c", 10, 30, 11, 30), ev("d", 13, 0, 14, 0)}, d)
	lanes := map[string][2]int{}
	for _, p := range ps {
		lanes[p.e.ID] = [2]int{p.lane, p.lanes}
	}
	want := map[string][2]int{"a": {0, 3}, "b": {1, 3}, "c": {2, 3}, "d": {0, 1}}
	for id, w := range want {
		if lanes[id] != w {
			t.Errorf("%s: lane %v, want %v", id, lanes[id], w)
		}
	}
}

func TestBackupRoundTrip(t *testing.T) {
	d := sampleData(monday, "en")
	ten := 10
	d.Events[0].Alert = &ten
	b, err := d.encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := decodeData(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Projects) != len(d.Projects) || len(back.Tasks) != len(d.Tasks) || len(back.Events) != len(d.Events) {
		t.Errorf("round trip lost data: %d/%d projects", len(back.Projects), len(d.Projects))
	}
	if back.Events[0].Alert == nil || *back.Events[0].Alert != 10 || back.Prefs.AlertMinutes != 10 {
		t.Error("alerts were not kept")
	}
	if _, err := decodeData([]byte(`{"format":"other"}`)); err == nil {
		t.Error("a file of another app decoded")
	}
	if d, _ := decodeData([]byte(`{"format":"nexttodo"}`)); d.Prefs.AlertMinutes != 10 || d.Prefs.WeekStart != -1 {
		t.Errorf("an old file's settings %+v", d.Prefs)
	}
}

func TestMigrateCategories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	old := `{"format":"nexttodo","version":1,"projects":[{"id":"p","title":"Grant","category":1,"status":1,"created":"2026-01-01T00:00:00Z"}],"prefs":{"weekStart":-1}}`
	os.WriteFile(path, []byte(old), 0o644)
	d := loadData(path, monday, "zh-Hans")
	if len(d.Categories) != 4 || d.Categories[1].Name != "横向课题" || d.Version != dataVersion {
		t.Fatalf("categories %+v, version %d", d.Categories, d.Version)
	}
	if p := d.project("p"); p == nil || p.Category != "industry" || d.category(p.Category) == nil {
		t.Errorf("the project's category is %q", p.Category)
	}
	b, _ := d.encode()
	again, err := decodeData(b)
	if err != nil || again.project("p").Category != "industry" || len(again.Categories) != 4 {
		t.Errorf("round trip: %v", err)
	}
}

func TestCategories(t *testing.T) {
	a := testApp("en")
	a.page = PageProjects
	tt := newTester(t, a, "en")
	if err := tt.Click("Edit categories"); err != nil {
		t.Fatal(err)
	}
	if !a.catOpen {
		t.Fatal("the categories did not open")
	}
	tt.Click("New category")
	tt.Type("Teaching")
	tt.Key(0, ui.KeyEnter)
	k := a.data.Categories[len(a.data.Categories)-1]
	if k.Name != "Teaching" || len(a.data.Categories) != 5 {
		t.Fatalf("categories %+v", a.data.Categories)
	}
	// Dragged by its grip above the first, it comes first. Rows are found
	// by their first look button, a row apart.
	look, _ := tt.Find("Color and icon")
	row := float32(55)
	drag(tt, look.X-16, look.Y+look.H/2+4*row, 0, -4*row-look.H/2+2)
	if a.data.Categories[0] != k {
		t.Errorf("order %q", []string{a.data.Categories[0].Name, a.data.Categories[1].Name})
	}
	// A project in it groups under it.
	a.data.Projects[0].Category = k.ID
	a.catOpen = false
	tt.Frame()
	if !tt.HasText("Teaching") {
		t.Error("the projects page has no Teaching group")
	}
	// Deleting a category in use asks, and leaves its project uncategorized.
	a.catOpen = true
	tt.Frame()
	r, _ := tt.Find(a.tr.ProjectsCount(1))
	tt.ClickAt(r.X+r.W+23, r.Y+r.H/2)
	if a.askCat != k.ID {
		t.Fatalf("no question before deleting a category in use (asked %q)", a.askCat)
	}
	tt.Key(0, ui.KeyEnter) // the alert's Delete
	if a.data.category(k.ID) != nil || a.data.Projects[0].Category != "" {
		t.Error("the category was not deleted")
	}
	a.catOpen = false
	tt.Frame()
	if !tt.HasText("Uncategorized") {
		t.Error("no Uncategorized group")
	}
}

func TestArchive(t *testing.T) {
	a := testApp("en")
	pr := a.data.Projects[0] // the NSFC grant, with tasks in My Day
	a.openProject(pr.ID)
	tt := newTester(t, a, "en")
	if err := tt.Click("Archive"); err != nil {
		t.Fatal(err)
	}
	if !pr.Archived || a.projOpen != "" {
		t.Fatalf("archived %v, open %q", pr.Archived, a.projOpen)
	}
	for _, x := range a.tasksIn(listMyDay) {
		if x.Project == pr.ID {
			t.Errorf("%q of an archived project is in My Day", x.Title)
		}
	}
	if _, listed := tt.Find(pr.Title); listed {
		t.Errorf("the archived project is still listed: %q", tt.Texts())
	}
	// Undo, from the toast, brings it back.
	if err := tt.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	if pr.Archived {
		t.Fatal("Undo did not bring the project back")
	}
	// Archived again, it shows under Archived, where it opens to be taken
	// out.
	a.openProject(pr.ID)
	tt.Frame()
	tt.Click("Archive")
	tt.Click("Archived")
	if _, listed := tt.Find(pr.Title); !listed {
		t.Fatal("the archived section does not list it")
	}
	tt.Click(pr.Title)
	if err := tt.Click("Unarchive"); err != nil {
		t.Fatal(err)
	}
	if pr.Archived {
		t.Error("the project is still archived")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	d := loadData(path, monday, "zh-Hans")
	if len(d.Projects) == 0 {
		t.Fatal("a first launch has no sample data")
	}
	a := newApp(d, path)
	a.data.Tasks = append(a.data.Tasks, &Task{ID: "x", Title: "written"})
	a.changed()
	if again := loadData(path, monday, "en"); again.task("x") == nil {
		t.Error("the change was not saved")
	}
}

func TestMyDay(t *testing.T) {
	a := testApp("en")
	n := 0
	for _, x := range a.tasksIn(listMyDay) {
		if !x.Done {
			n++
		}
	}
	if n != a.myDayOpen() {
		t.Errorf("My Day lists %d open tasks, counts %d", n, a.myDayOpen())
	}
	for _, x := range a.data.Tasks {
		if !x.Done && !x.Due.IsZero() && !day(x.Due).After(day(monday)) && !a.inList(x, listMyDay) {
			t.Errorf("%q is due but not in My Day", x.Title)
		}
	}
}

func TestAlerts(t *testing.T) {
	a := testApp("en")
	a.useTestLanguage("en")
	start := at(monday, 11, 0)
	ev := &Event{ID: "talk", Title: "Talk", Start: start, End: start.Add(time.Hour), Location: "Hall"}
	quiet := &Event{ID: "quiet", Title: "Quiet", Start: start, End: start.Add(time.Hour), Alert: new(int)}
	*quiet.Alert = alertNone
	a.data.Events = []*Event{ev, quiet, {ID: "allday", AllDay: true, Start: day(monday), End: addDays(day(monday), 1)}}
	a.alertedTo = at(monday, 10, 40)

	if got := a.dueAlerts(at(monday, 10, 49)); len(got) != 0 {
		t.Fatalf("alerts before their time: %+v", got)
	}
	got := a.dueAlerts(at(monday, 10, 50))
	if len(got) != 1 || got[0].id != "event:talk" || got[0].subtitle != "Starts in 10 minutes" || !strings.Contains(got[0].body, "Hall") {
		t.Fatalf("alerts at 10:50: %+v", got)
	}
	if again := a.dueAlerts(at(monday, 10, 52)); len(again) != 0 {
		t.Errorf("the alert came twice: %+v", again)
	}
	// Moved to later, it alerts again before its new time.
	ev.Start, ev.End = at(monday, 12, 0), at(monday, 13, 0)
	if got := a.dueAlerts(at(monday, 11, 50)); len(got) != 1 {
		t.Errorf("a moved event did not alert: %+v", got)
	}
	// Alerts of a while ago, as after a sleep, still come while the event
	// has not ended.
	a.data.Prefs.AlertMinutes = 0
	late := &Event{ID: "late", Title: "Late", Start: at(monday, 12, 30), End: at(monday, 14, 0)}
	a.data.Events = append(a.data.Events, late)
	if got := a.dueAlerts(at(monday, 13, 0)); len(got) != 1 || got[0].id != "event:late" {
		t.Errorf("an alert missed while asleep: %+v", got)
	}
}

func TestAddAndFinishTask(t *testing.T) {
	a := testApp("en")
	a.page = PageTasks
	tt := newTester(t, a, "en")
	if err := tt.Click("Add a task"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Write the rebuttal")
	tt.Key(0, ui.KeyEnter)
	var added *Task
	for _, x := range a.data.Tasks {
		if x.Title == "Write the rebuttal" {
			added = x
		}
	}
	if added == nil {
		t.Fatalf("no task added; texts %q", tt.Texts())
	}
	if !added.inMyDay(monday) || !tt.HasText("Write the rebuttal") {
		t.Error("a task added to My Day is not in it")
	}
	tt.Advance(time.Second) // its row grows in
	if err := tt.Click("Write the rebuttal"); err != nil {
		t.Fatal(err)
	}
	if a.taskSel != added.ID {
		t.Fatal("a click did not choose the task")
	}
	if err := tt.Click("Mark as done"); err != nil {
		t.Fatal(err)
	}
	if !added.Done {
		t.Error("the task is not done")
	}
}

func TestUndoDelete(t *testing.T) {
	a := testApp("en")
	a.page = PageTasks
	tt := newTester(t, a, "en")
	n := len(a.data.Tasks)
	tt.Click("Sign the data-sharing agreement")
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	if len(a.data.Tasks) != n-1 || a.taskSel != "" {
		t.Fatalf("tasks %d after deleting one of %d", len(a.data.Tasks), n)
	}
	if err := tt.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	if len(a.data.Tasks) != n {
		t.Fatalf("Undo brought back %d of %d tasks", len(a.data.Tasks), n)
	}
	// An event deleted from its dialog comes back with ⌘Z.
	a.page = PageSchedule
	e := eventTitled(a, "GPU cluster: alignment runs")
	a.editEvent(e)
	tt.Frame()
	tt.Click("Delete")
	if eventTitled(a, e.Title) != nil || a.evtOpen {
		t.Fatal("the event was not deleted")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if eventTitled(a, e.Title) == nil {
		t.Error("⌘Z did not bring the event back")
	}
}

func TestSearch(t *testing.T) {
	a := testApp("en")
	tt := newTester(t, a, "en")
	tt.Key(ui.Cmd, ui.KeyF)
	if !a.searchOpen {
		t.Fatal("⌘F did not open the search")
	}
	tt.Type("tpami")
	hits := a.searchHits(a.searchQuery)
	if len(hits) < 2 || hits[0].group != "Tasks" || !strings.Contains(hits[0].title, "TPAMI") {
		t.Fatalf("hits %+v", hits)
	}
	// The second, an event, opens on Return.
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if a.searchOpen || !a.evtOpen && a.page != PageTasks {
		t.Errorf("Return: search open %v, page %v", a.searchOpen, a.page)
	}
	// A project found by its journal.
	found := false
	for _, h := range a.searchHits("spectroscopy") {
		found = found || h.group == "Projects"
	}
	if !found {
		t.Error("no project found by its journal")
	}
}

func TestCreateEvent(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	tt := newTester(t, a, "en")
	n := len(a.data.Events)
	if err := tt.Click("New event"); err != nil {
		t.Fatal(err)
	}
	if !a.evtOpen {
		t.Fatal("the event dialog did not open")
	}
	tt.Type("Thesis defense")
	if err := tt.Click("Create"); err != nil {
		t.Fatal(err)
	}
	if len(a.data.Events) != n+1 || a.evtOpen {
		t.Fatalf("events %d, dialog open %v", len(a.data.Events), a.evtOpen)
	}
	if e := a.data.Events[n]; e.Title != "Thesis defense" || !e.End.After(e.Start) || e.Alert != nil {
		t.Errorf("event %+v", e)
	}
	if !tt.HasText("Thesis defense") {
		t.Error("the week does not show the new event")
	}
}

// drag presses at (x, y), moves by (dx, dy) in steps, and lets go.
func drag(tt *ui.Tester, x, y, dx, dy float32) {
	tt.Press(x, y)
	for i := 1; i <= 6; i++ {
		tt.Move(x+dx*float32(i)/6, y+dy*float32(i)/6)
	}
	tt.Release(x+dx, y+dy)
	tt.Frame()
}

func TestDragEvent(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	tt := newTester(t, a, "en")
	e := eventTitled(a, "GPU cluster: alignment runs")
	start, end := e.Start, e.End
	r, ok := tt.Find(e.Title)
	if !ok {
		t.Fatal("no GPU cluster in the week")
	}
	// An hour down, and a day to the right.
	drag(tt, r.X+r.W/2, r.Y+8, a.colW, hourHeight)
	if want := addDays(start, 1).Add(time.Hour); !e.Start.Equal(want) || !e.End.Equal(addDays(end, 1).Add(time.Hour)) {
		t.Fatalf("moved to %v – %v, want %v", e.Start, e.End, want)
	}
	if a.evtOpen {
		t.Error("the drag opened the event as a click")
	}
	// Its bottom edge, dragged down half an hour, makes it end later.
	start, end = e.Start, e.End
	r, _ = tt.Find(e.Title)
	edge := r.Y + float32(end.Sub(start).Minutes())/60*hourHeight - 7
	drag(tt, r.X+r.W/2, edge, 0, hourHeight/2)
	if !e.Start.Equal(start) || !e.End.Equal(end.Add(30*time.Minute)) {
		t.Errorf("resized to %v – %v, want it to end at %v", e.Start, e.End, end.Add(30*time.Minute))
	}
	// A click still opens it.
	r, _ = tt.Find(e.Title)
	tt.ClickAt(r.X+r.W/2, r.Y+8)
	if !a.evtOpen || a.evt.id != e.ID {
		t.Error("a click did not open the event")
	}
}

func TestRepeat(t *testing.T) {
	d := day(monday)
	count := func(e *Event, from, to time.Time) []string {
		var out []string
		for _, o := range e.occurrences(from, to) {
			out = append(out, o.Start.Format("Jan 2 15:04"))
		}
		return out
	}
	weekly := &Event{ID: "w", Start: at(d, 9, 30), End: at(d, 10, 30), Repeat: RepeatWeekly}
	if got := count(weekly, addDays(d, -7), addDays(d, 21)); strings.Join(got, ", ") != "Oct 26 09:30, Nov 2 09:30, Nov 9 09:30" {
		t.Errorf("weekly: %q", got)
	}
	weekly.Until, weekly.Skip = addDays(d, 7), []time.Time{d}
	if got := count(weekly, d, addDays(d, 21)); strings.Join(got, ", ") != "Nov 2 09:30" {
		t.Errorf("weekly until Nov 2, without Oct 26: %q", got)
	}
	weekdays := &Event{ID: "d", Start: at(addDays(d, 4), 8, 0), End: at(addDays(d, 4), 8, 30), Repeat: RepeatWeekdays}
	if got := count(weekdays, addDays(d, 4), addDays(d, 9)); strings.Join(got, ", ") != "Oct 30 08:00, Nov 2 08:00, Nov 3 08:00" {
		t.Errorf("weekdays: %q", got)
	}
	jan31 := time.Date(2027, 1, 31, 14, 0, 0, 0, time.Local)
	monthly := &Event{ID: "m", Start: jan31, End: jan31.Add(time.Hour), Repeat: RepeatMonthly}
	if got := count(monthly, jan31, jan31.AddDate(0, 3, 1)); strings.Join(got, ", ") != "Jan 31 14:00, Mar 31 14:00" {
		t.Errorf("monthly on the 31st: %q", got)
	}
	// Far from its start, the expansion starts near the range.
	if got := count(weekly, addDays(d, 700), addDays(d, 707)); len(got) != 0 {
		t.Errorf("after its end: %q", got)
	}
}

func TestEditOccurrence(t *testing.T) {
	a := testApp("en")
	series := eventTitled(a, "Group meeting")
	next := a.data.eventsBetween(addDays(day(monday), 7), addDays(day(monday), 8))
	var occ *Event
	for _, e := range next {
		if e.series == series {
			occ = e
		}
	}
	if occ == nil {
		t.Fatal("no group meeting next Monday")
	}
	// This event only: it leaves the series.
	a.editEvent(occ)
	a.evt.title = "Group meeting (moved)"
	a.saveEvent()
	if series.Title != "Group meeting" || !series.skipped(occ.Start) || eventTitled(a, "Group meeting (moved)") == nil {
		t.Fatalf("this event: series %q, skips %v", series.Title, series.Skip)
	}
	// All events, moved a day later: the series moves.
	occ = a.data.eventsBetween(day(monday), addDays(day(monday), 1))[0]
	a.editEvent(occ)
	a.evt.all, a.evt.title, a.evt.date = 1, "Lab meeting", addDays(day(monday), 1)
	a.saveEvent()
	if series.Title != "Lab meeting" || !sameDay(series.Start, addDays(monday, 1)) || series.Repeat != RepeatWeekly {
		t.Errorf("all events: %q on %v", series.Title, series.Start)
	}
}

func TestSketchEvent(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	tt := newTester(t, a, "en")
	// Tuesday's column, where the Hangxin sync starts at 10:00, an hour
	// tall: 13:00 is three hours below its top.
	r, _ := tt.Find("Hangxin weekly sync")
	top := r.Y - 4 + 3*hourHeight
	drag(tt, r.X+r.W/2, top+4, 0, 2*hourHeight)
	if !a.evtOpen || !a.evt.start.Equal(at(addDays(monday, 1), 13, 0)) || !a.evt.end.Equal(at(addDays(monday, 1), 15, 0)) {
		t.Fatalf("sketched %v – %v, dialog %v", a.evt.start, a.evt.end, a.evtOpen)
	}
}

func TestTaskKeys(t *testing.T) {
	a := testApp("en")
	a.page = PageTasks
	tt := newTester(t, a, "en")
	tasks := a.tasksIn(listMyDay)
	tt.Key(0, ui.KeyDown)
	if a.taskSel != tasks[0].ID {
		t.Fatalf("Down chose %q", a.taskSel)
	}
	tt.Key(0, ui.KeyDown)
	if a.taskSel != tasks[1].ID {
		t.Fatalf("Down again chose %q", a.taskSel)
	}
	tt.Key(ui.Cmd, ui.KeyK)
	if !tasks[1].Done {
		t.Error("⌘K did not mark the task done")
	}
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Key(0, ui.KeyBackspace)
	if a.data.task(tasks[1].ID) != nil || a.taskSel != tasks[2].ID {
		t.Errorf("Delete: task left %v, chosen %q", a.data.task(tasks[1].ID) != nil, a.taskSel)
	}
}

func TestDragOccurrence(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	tt := newTester(t, a, "en")
	series := eventTitled(a, "Group meeting") // weekly, on Mondays
	start := series.Start
	r, _ := tt.Find("Group meeting")
	drag(tt, r.X+r.W/2, r.Y+8, 0, hourHeight)
	if !series.Start.Equal(start) || len(series.Skip) != 1 || !sameDay(series.Skip[0], monday) {
		t.Fatalf("the series moved: %v, skips %v", series.Start, series.Skip)
	}
	var moved []*Event
	for _, e := range a.data.eventsBetween(day(monday), addDays(day(monday), 14)) {
		if e.Title == "Group meeting" {
			moved = append(moved, e)
		}
	}
	if len(moved) != 2 || !moved[0].Start.Equal(start.Add(time.Hour)) || moved[0].series != nil || moved[1].series != series {
		t.Errorf("two weeks of group meetings: %+v", moved)
	}
}

func TestDropOnDay(t *testing.T) {
	a := testApp("en")
	a.page, a.schedMode = PageSchedule, 1
	tt := newTester(t, a, "en")
	e := eventTitled(a, "Faculty retreat")
	from, ok := tt.Find("Faculty retreat")
	to, ok2 := tt.Find("Oct 14, 2026")
	if !ok || !ok2 {
		t.Fatal("no chip or day to drop on")
	}
	drag(tt, from.X+from.W/2, from.Y+from.H/2, to.X+to.W/2-(from.X+from.W/2), to.Y+to.H/2-(from.Y+from.H/2))
	if !sameDay(e.Start, at(monday, 0, 0).AddDate(0, 0, -12)) {
		t.Errorf("dropped on Oct 14, the retreat is on %v", e.Start)
	}
}

func TestMonth(t *testing.T) {
	a := testApp("en")
	a.page, a.schedMode = PageSchedule, 1
	tt := newTester(t, a, "en")
	tt.Click("Oct 14, 2026")
	tt.Click("Oct 14, 2026")
	if !a.evtOpen || !sameDay(a.evt.date, addDays(monday, -12)) || a.evt.start.Hour() != 9 {
		t.Fatalf("double click: dialog %v, date %v", a.evtOpen, a.evt.date)
	}
	a.evtOpen = false
	tt.Frame()
	if err := tt.Click("October 30, 2026"); err != nil {
		t.Fatal(err)
	}
	if a.schedMode != 0 || !sameDay(a.schedAnchor, addDays(monday, 4)) {
		t.Errorf("mode %d, anchor %v", a.schedMode, a.schedAnchor)
	}
}

func TestNewProject(t *testing.T) {
	a := testApp("en")
	a.page = PageProjects
	tt := newTester(t, a, "en")
	if !tt.HasText("Weakly supervised cross-modal alignment") {
		t.Fatal("the projects are not listed")
	}
	if err := tt.Click("New project"); err != nil {
		t.Fatal(err)
	}
	if a.projOpen == "" {
		t.Fatal("the new project did not open")
	}
	tt.Type("Protein folding")
	if p := a.data.project(a.projOpen); p == nil || p.Title != "Protein folding" {
		t.Fatalf("project %+v", p)
	}
	tt.Click("Add a task to this project")
	tt.Type("Gather the structures")
	tt.Key(0, ui.KeyEnter)
	if done, total := a.data.progress(a.projOpen); total != 1 || done != 0 {
		t.Errorf("progress %d/%d", done, total)
	}
}

func TestPages(t *testing.T) {
	a := testApp("en")
	tt := newTester(t, a, "en")
	for _, pg := range []string{"Tasks", "Schedule", "Projects", "Directions", "Overview"} {
		if err := tt.Click(pg); err != nil {
			t.Fatal(err)
		}
		if a.navLabel(a.page) != pg {
			t.Errorf("clicked %s, shows %s", pg, a.navLabel(a.page))
		}
	}
	tt.Click("Settings")
	if a.page != PageSettings {
		t.Error("the settings did not open")
	}
}

func TestPanel(t *testing.T) {
	a := testApp("en")
	tt := ui.NewTester(a.panelView, 340, 480)
	tt.SetLocale("en")
	tt.Frame()
	if !tt.HasText("Group meeting") || !tt.HasText("Book flights for the December workshop") {
		t.Fatalf("the panel does not show today: %q", tt.Texts())
	}
	tt.Click("Add a task to My Day")
	tt.Type("Call the lab")
	tt.Key(0, ui.KeyEnter)
	var task *Task
	for _, x := range a.data.Tasks {
		if x.Title == "Call the lab" {
			task = x
		}
	}
	if task == nil || !task.inMyDay(monday) || !tt.HasText("Added") {
		t.Fatalf("no task added to My Day: %+v", task)
	}
	tt.Click("Event")
	a.quick.start = at(monday, 15, 0)
	a.quick.minutes = 90
	tt.Click("Add an event today")
	tt.Type("Coffee with Ada")
	tt.Key(0, ui.KeyEnter)
	e := eventTitled(a, "Coffee with Ada")
	if e == nil || !e.Start.Equal(at(monday, 15, 0)) || e.End.Sub(e.Start) != 90*time.Minute {
		t.Fatalf("event %+v", e)
	}
}

func TestChinese(t *testing.T) {
	a := testApp("zh-Hans")
	tt := newTester(t, a, "zh-Hans")
	for _, s := range []string{"总览", "早上好", "今日", "即将截止", "进行中的项目"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	tt.Click("项目")
	if !tt.HasText("纵向课题") || !tt.HasText("横向课题") {
		t.Error("the categories are not in Chinese")
	}
}

// useTestLanguage sets the language as a build would, for code outside it.
func (a *app) useTestLanguage(lang string) {
	a.lang, a.locale, a.tr = lang, lang, locales.In(lang)
}

// TestSnapshots writes a picture of every page, in both appearances and
// languages, to $NT_SNAPSHOTS.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("NT_SNAPSHOTS")
	if dir == "" {
		t.Skip("NT_SNAPSHOTS is not set")
	}
	os.MkdirAll(dir, 0o755)
	type shot struct {
		name string
		set  func(a *app)
		w, h int
	}
	shots := []shot{
		{"overview", func(a *app) { a.page = PageOverview }, 1280, 820},
		{"tasks", func(a *app) {
			a.page = PageTasks
			for _, x := range a.data.Tasks {
				if x.Priority == 3 && !x.Done {
					a.taskSel = x.ID
					break
				}
			}
		}, 1280, 820},
		{"schedule", func(a *app) { a.page = PageSchedule }, 1280, 820},
		{"month", func(a *app) { a.page, a.schedMode = PageSchedule, 1 }, 1280, 820},
		{"projects", func(a *app) { a.page = PageProjects }, 1280, 820},
		{"project", func(a *app) { a.openProject(a.data.Projects[0].ID) }, 1280, 820},
		{"directions", func(a *app) { a.page = PageDirections }, 1280, 820},
		{"settings", func(a *app) { a.page = PageSettings }, 1280, 900},
		{"event", func(a *app) { a.page = PageSchedule; a.editEvent(a.data.Events[1]) }, 1280, 820},
		{"categories", func(a *app) { a.page, a.catOpen = PageProjects, true }, 1280, 820},
		{"archived", func(a *app) {
			a.page, a.projShowArchived = PageProjects, true
			a.data.Projects[4].Archived, a.data.Projects[4].ArchivedAt = true, monday
		}, 1280, 820},
		{"archivedproject", func(a *app) {
			a.data.Projects[4].Archived, a.data.Projects[4].ArchivedAt = true, monday
			a.openProject(a.data.Projects[4].ID)
		}, 1280, 820},
		{"eventrepeat", func(a *app) {
			a.page = PageSchedule
			a.editEvent(a.data.eventsBetween(day(monday), addDays(day(monday), 1))[0])
		}, 1280, 900},
		{"sketch", func(a *app) {
			a.page = PageSchedule
			a.sketch = sketch{day: addDays(day(monday), 2), y0: 12.6 * hourHeight, y: 14.4 * hourHeight}
		}, 1280, 820},
		{"search", func(a *app) { a.page, a.searchOpen, a.searchQuery = PageTasks, true, "re" }, 1280, 820},
		{"panel", nil, 340, 480},
	}
	only := os.Getenv("NT_PAGE")
	for _, lang := range []string{"en", "zh-Hans"} {
		for _, dark := range []bool{false, true} {
			for _, s := range shots {
				if only != "" && !strings.Contains(only, s.name) {
					continue
				}
				a := testApp(lang)
				view := a.view
				if s.set == nil {
					view = a.panelView
					a.quick.start = at(monday, 11, 0)
				} else {
					s.set(a)
				}
				tt := ui.NewTester(view, s.w, s.h)
				tt.SetLocale(lang)
				tt.SetScale(2)
				tt.SetDark(dark)
				tt.Frame()
				tt.Advance(time.Second)
				mode := "light"
				if dark {
					mode = "dark"
				}
				f, err := os.Create(filepath.Join(dir, s.name+"-"+lang+"-"+mode+".png"))
				if err != nil {
					t.Fatal(err)
				}
				png.Encode(f, tt.Image())
				f.Close()
			}
		}
	}
}

// TestIcon draws the app icon into resources/icon.png when NT_ICON is set,
// on macOS's icon grid: an 824-pixel body in 1024, with its shadow.
func TestIcon(t *testing.T) {
	if os.Getenv("NT_ICON") == "" {
		t.Skip("NT_ICON is not set")
	}
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			body := ui.Rect{X: 100, Y: 100, W: 824, H: 824}
			p.Shadow(body, 185, 0, 10, 24, 0, ui.RGBA(0, 0, 40, 0.3))
			drawAppIcon(p, body)
		})
	}, 1024, 1024, 1)
	f, err := os.Create("resources/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

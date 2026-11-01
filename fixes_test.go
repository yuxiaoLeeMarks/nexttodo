package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Undoing a deletion keeps what was done after it.
func TestUndoKeepsLaterWork(t *testing.T) {
	a := testApp("en")
	a.page = PageTasks
	tt := newTester(t, a, "en")
	tt.Click("Sign the data-sharing agreement")
	tt.Click("Delete")
	a.addTask("Added after", "", listAll)
	edited := taskTitled(a, "Book flights for the December workshop")
	edited.Title = "Book flights to Vancouver"
	tt.Key(ui.Cmd, ui.KeyZ)
	if taskTitled(a, "Sign the data-sharing agreement") == nil || taskTitled(a, "Added after") == nil ||
		taskTitled(a, "Book flights to Vancouver") == nil {
		t.Fatalf("⌘Z lost work: %d tasks", len(a.data.Tasks))
	}

	// A project deleted comes back with its tasks, but a task changed since
	// stays as it is now.
	pr := a.data.Projects[0]
	tasks := a.data.projectTasks(pr.ID)
	step := a.remember()
	a.data.deleteProject(pr.ID)
	a.settle(step)
	tasks[0].Title = "Changed since"
	a.restore(step)
	if a.data.project(pr.ID) == nil || tasks[1].Project != "" && a.data.task(tasks[1].ID).Project != pr.ID {
		t.Fatal("the project or its tasks did not come back")
	}
	if x := a.data.task(tasks[0].ID); x.Title != "Changed since" {
		t.Errorf("undo undid a later change: %q", x.Title)
	}

	// Erasing is undone, but not what was added after.
	n := len(a.data.Tasks)
	a.askErase = true
	d := newData()
	a.replaceData(d, "Erased")
	a.addTask("After erasing", "", listAll)
	a.restore(a.replaced)
	if len(a.data.Tasks) != n+1 || taskTitled(a, "After erasing") == nil {
		t.Errorf("%d tasks after undoing the erase, want %d", len(a.data.Tasks), n+1)
	}
}

// Saving an event that goes over midnight, or over days, keeps its end.
func TestSaveLongEvent(t *testing.T) {
	a := testApp("en")
	late := &Event{ID: "late", Title: "Observation", Start: at(monday, 22, 0), End: at(addDays(monday, 1), 1, 0)}
	long := &Event{ID: "long", Title: "Retreat", Start: at(monday, 9, 0), End: at(addDays(monday, 2), 17, 0)}
	days := &Event{ID: "days", Title: "Conference", AllDay: true, Start: day(monday), End: addDays(day(monday), 3)}
	a.data.Events = append(a.data.Events, late, long, days)
	for _, e := range []*Event{late, long, days} {
		start, end := e.Start, e.End
		a.editEvent(e)
		a.evt.title += "!"
		a.saveEvent()
		if !e.Start.Equal(start) || !e.End.Equal(end) {
			t.Errorf("%s: %v – %v, want %v – %v", e.Title, e.Start, e.End, start, end)
		}
	}
	// A new event ending after midnight ends the next day.
	a.newEvent(at(monday, 22, 0), "")
	a.evt.title, a.evt.end = "Night run", at(monday, 1, 30)
	a.saveEvent()
	if e := eventTitled(a, "Night run"); e == nil || !e.End.Equal(at(addDays(monday, 1), 1, 30)) {
		t.Errorf("night run %+v", e)
	}
}

// Moving a whole series moves its skipped days and its end with it.
func TestMoveSeries(t *testing.T) {
	a := testApp("en")
	start := at(addDays(monday, -21), 9, 0)
	e := &Event{ID: "s", Title: "Standup", Start: start, End: start.Add(30 * time.Minute), Repeat: RepeatDaily,
		Skip: []time.Time{addDays(day(start), 2)}, Until: addDays(day(start), 10)}
	a.data.Events = append(a.data.Events, e)
	occ := e.occurrences(addDays(day(start), 3), addDays(day(start), 4))[0]
	a.editEvent(occ)
	a.evt.date, a.evt.all = addDays(a.evt.date, 1), 1
	a.saveEvent()
	if !e.Start.Equal(addDays(start, 1)) || !sameDay(e.Skip[0], addDays(start, 3)) || !sameDay(e.Until, addDays(start, 11)) {
		t.Errorf("series from %v, skipping %v, until %v", e.Start, e.Skip, e.Until)
	}
}

// A repeating all-day event takes one day each time, over a change of
// clocks too.
func TestAllDayRepeatOverDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, ny)
	e := &Event{ID: "w", AllDay: true, Start: start, End: start.AddDate(0, 0, 1), Repeat: RepeatWeekly}
	for _, o := range e.occurrences(start, start.AddDate(0, 0, 21)) {
		if !o.End.Equal(addDays(o.Start, 1)) {
			t.Errorf("an occurrence from %v ends %v", o.Start, o.End)
		}
	}
}

// A way of repeating this version does not know is once, not a hang.
func TestUnknownRepeat(t *testing.T) {
	d := newData()
	d.Events = append(d.Events, &Event{ID: "y", Start: at(monday, 9, 0), End: at(monday, 10, 0), Repeat: "hourly"})
	done := make(chan []*Event)
	go func() { done <- d.eventsBetween(day(monday), addDays(day(monday), 7)) }()
	select {
	case got := <-done:
		if len(got) != 1 {
			t.Errorf("%d events", len(got))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("eventsBetween hangs on an unknown repeat")
	}
}

// An event made within its alert's lead alerts at once.
func TestLateAlert(t *testing.T) {
	a := testApp("en")
	now := at(monday, 14, 50)
	a.alertedTo = now
	a.data.Events = append(a.data.Events, &Event{ID: "soon", Title: "Soon", Start: at(monday, 14, 55), End: at(monday, 15, 30)})
	got := a.dueAlerts(now.Add(time.Second))
	found := false
	for _, x := range got {
		found = found || x.title == "Soon"
	}
	if !found {
		t.Fatalf("no alert for an event starting in 5 minutes: %+v", got)
	}
	if again := a.dueAlerts(now.Add(time.Minute)); len(again) != 0 {
		t.Errorf("alerted twice: %+v", again)
	}
}

// The day's backup is the data as the day began, not after its first save.
func TestBackupBeforeSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	a := newApp(sampleData(monday, "en"), path)
	a.now = func() time.Time { return monday }
	b, _ := a.data.encode()
	writeFile(path, b)
	n := len(a.data.Tasks)
	a.data.Tasks = nil
	a.changed()
	a.flush()
	raw, err := os.ReadFile(backups(path)[0].path)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := decodeData(raw)
	if len(d.Tasks) != n {
		t.Errorf("the backup has %d tasks, want the %d of the morning", len(d.Tasks), n)
	}
}

// Groups of the same name do not share a key.
func TestSameNamedGroups(t *testing.T) {
	a := testApp("en")
	a.data.Categories[1].Name = a.data.Categories[0].Name
	a.page = PageProjects
	newTester(t, a, "en")
	a.projGroup = groupByDirection
	a.data.Directions[0].Title, a.data.Directions[1].Title = "", ""
	newTester(t, a, "en")
}

// A project of a status this version does not know does not crash.
func TestBadStatus(t *testing.T) {
	d := sampleData(monday, "en")
	d.Projects[0].Status = 9
	b, _ := d.encode()
	d, err := decodeData(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.Projects[0].Status != StatusActive {
		t.Errorf("status %d", d.Projects[0].Status)
	}
}

// The dialog ends an event on another day, and says when it ends.
func TestEventEndDay(t *testing.T) {
	a := testApp("en")
	a.newEvent(at(monday, 9, 0), "")
	a.evt.title = "Retreat"
	if !a.evt.endDay().Equal(day(monday)) {
		t.Fatalf("a new event ends %v", a.evt.endDay())
	}
	a.evt.setEndDay(addDays(day(monday), 2))
	a.evt.end = at(a.evt.end, 17, 0)
	a.saveEvent()
	e := eventTitled(a, "Retreat")
	if !e.End.Equal(at(addDays(monday, 2), 17, 0)) {
		t.Fatalf("ends %v", e.End)
	}
	// Moved to another day, it keeps its days.
	a.editEvent(e)
	if !a.evt.endDay().Equal(addDays(day(monday), 2)) {
		t.Fatalf("the dialog ends it %v", a.evt.endDay())
	}
	a.evt.date = addDays(a.evt.date, 1)
	if !a.evt.endDay().Equal(addDays(day(monday), 3)) {
		t.Errorf("moved, the dialog ends it %v", a.evt.endDay())
	}
	a.saveEvent()
	if !e.Start.Equal(at(addDays(monday, 1), 9, 0)) || !e.End.Equal(at(addDays(monday, 3), 17, 0)) {
		t.Errorf("moved: %v – %v", e.Start, e.End)
	}
	// All day, over three days.
	a.newEvent(at(monday, 9, 0), "")
	a.evt.title, a.evt.allDay = "Conference", true
	a.evt.setEndDay(addDays(day(monday), 2))
	if !a.evt.endDay().Equal(addDays(day(monday), 2)) {
		t.Errorf("all day, the dialog ends it %v", a.evt.endDay())
	}
	a.saveEvent()
	if e := eventTitled(a, "Conference"); !e.Start.Equal(day(monday)) || !e.End.Equal(addDays(day(monday), 3)) {
		t.Errorf("conference %v – %v", e.Start, e.End)
	}
}

// A task reminds of itself at its time, once, and a time typed with it
// sets that time.
func TestTaskReminder(t *testing.T) {
	a := testApp("en")
	a.page = PageTasks
	tt := newTester(t, a, "en")
	tt.Click("Add a task")
	tt.Type("Call Lin tomorrow 3pm")
	tt.Key(0, ui.KeyEnter)
	x := taskTitled(a, "Call Lin")
	tomorrow := addDays(day(monday), 1)
	if x == nil || !x.Due.Equal(tomorrow) || !x.Remind.Equal(at(tomorrow, 15, 0)) {
		t.Fatalf("task %+v", x)
	}
	a.alertedTo = at(tomorrow, 14, 59)
	got := a.dueAlerts(at(tomorrow, 15, 0))
	found := false
	for _, al := range got {
		found = found || al.id == "task:"+x.ID && al.title == "Call Lin"
	}
	if !found {
		t.Fatalf("no reminder: %+v", got)
	}
	if again := a.dueAlerts(at(tomorrow, 15, 1)); len(again) != 0 {
		t.Errorf("reminded twice: %+v", again)
	}
	// Done, it reminds no more.
	x.Remind = at(tomorrow, 16, 0)
	x.setDone(true, monday)
	if got := a.dueAlerts(at(tomorrow, 16, 0)); len(got) != 0 {
		t.Errorf("a done task reminded: %+v", got)
	}
	// The detail sets a reminder, which the row shows.
	x.setDone(false, monday)
	x.Remind = time.Time{}
	a.taskList, a.taskSel = listAll, x.ID
	tt.Frame()
	if err := tt.Click("Add time"); err != nil {
		t.Fatal(err)
	}
	if !x.Remind.Equal(at(tomorrow, 9, 0)) {
		t.Errorf("reminds %v, want at nine on its day", x.Remind)
	}
	tt.Frame()
	if !tt.HasText("Tomorrow " + a.clock(x.Remind)) {
		t.Errorf("the row does not show the reminder: %q", tt.Texts())
	}
}

// The projects show as a timeline, remembered, whose rows open them.
func TestTimeline(t *testing.T) {
	a := testApp("en")
	a.page = PageProjects
	tt := newTester(t, a, "en")
	if err := tt.Click("Timeline"); err != nil {
		t.Fatal(err)
	}
	if !a.data.Prefs.Timeline || !tt.HasText("No dates") {
		t.Fatalf("no timeline: %q", tt.Texts())
	}
	pr := a.data.Projects[0]
	if err := tt.Click(pr.Title); err != nil {
		t.Fatal(err)
	}
	if a.projOpen != pr.ID {
		t.Error("a row did not open its project")
	}
	from, to := a.timelineSpan(a.projectGroups())
	if from.After(pr.Start) || to.Before(pr.End) || from.Day() != 1 || to.Day() != 1 {
		t.Errorf("the timeline runs %v – %v", from, to)
	}
	months := timelineMonths(time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local), time.Date(2030, 1, 1, 0, 0, 0, 0, time.Local))
	step := timelineStep(len(months), 600, 30)
	n := 0
	for _, m := range months {
		if marked(m, step) {
			n++
		}
	}
	if step != 12 || n != 9 {
		t.Errorf("%d marks every %d months from March 2020 to 2030 in 600 DIPs, want each January", n, step)
	}
}

// The months of the timeline leave room for their labels, however narrow
// it is: fewer marks, never one over another.
func TestTimelineSteps(t *testing.T) {
	for _, c := range []struct {
		n        int
		w, label float32
		want     int
	}{
		{13, 1000, 26, 1},
		{13, 520, 26, 1},
		{13, 400, 26, 2},
		{13, 300, 26, 2},
		{13, 200, 26, 3},
		{13, 150, 26, 6},
		{13, 60, 30, 12},
		{240, 600, 30, 24},
		{240, 100, 30, 120},
		{1, 0, 30, 120},
	} {
		if got := timelineStep(c.n, c.w, c.label); got != c.want {
			t.Errorf("%d months in %v DIPs, labels %v wide: every %d, want %d", c.n, c.w, c.label, got, c.want)
		}
	}
	jan := time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)
	for _, step := range timelineSteps[:5] {
		if !marked(jan, step) {
			t.Errorf("January not marked every %d months", step)
		}
	}
	if marked(jan.AddDate(0, 1, 0), 2) || !marked(jan.AddDate(0, 2, 0), 2) || !marked(jan.AddDate(0, 3, 0), 3) {
		t.Error("months marked out of step")
	}
}

// A task dragged into the hours of the week blocks an hour for it.
func TestTimeBlock(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	a.schedScroll.Y, a.schedScrolled = 0, true
	tt := newTester(t, a, "en")
	task := taskTitled(a, "Quantize the encoder to INT4 and measure WER")
	from, ok := tt.Find(task.Title)
	sync, ok2 := tt.Find("Hangxin weekly sync") // on Tuesday at 10:00
	if !ok || !ok2 {
		t.Fatalf("no chip for the task: %q", tt.Texts())
	}
	// Down into Tuesday's hours, at noon.
	x, y := from.X+from.W/2, from.Y+from.H/2
	drag(tt, x, y, 0, sync.Y+2*hourHeight+4-y)
	var block *Event
	for _, e := range a.data.Events {
		if e.Title == task.Title {
			block = e
		}
	}
	if block == nil || block.Kind != kindFocus || block.Project != task.Project || !sameDay(block.Start, task.Due) ||
		block.Start.Hour() != 12 || block.End.Sub(block.Start) != time.Hour {
		t.Fatalf("block %+v", block)
	}
	if task.Done || task.Due.IsZero() {
		t.Error("the task changed")
	}
}

// ⌘←, ⌘→ and ⌘T move the schedule, as in Calendar.
func TestScheduleKeys(t *testing.T) {
	a := testApp("en")
	a.page = PageSchedule
	tt := newTester(t, a, "en")
	tt.Key(ui.Cmd, ui.KeyRight)
	if !sameDay(a.schedAnchor, addDays(monday, 7)) {
		t.Fatalf("⌘→ went to %v", a.schedAnchor)
	}
	tt.Key(ui.Cmd, ui.KeyLeft)
	tt.Key(ui.Cmd, ui.KeyLeft)
	if !sameDay(a.schedAnchor, addDays(monday, -7)) {
		t.Fatalf("⌘← went to %v", a.schedAnchor)
	}
	tt.Key(ui.Cmd, ui.KeyT)
	if !sameDay(a.schedAnchor, monday) {
		t.Errorf("⌘T went to %v", a.schedAnchor)
	}
}

// Undo puts back a field the deletion changed, though the thing changed
// since in another; and the days a series skips, one by one.
func TestUndoFieldByField(t *testing.T) {
	a := testApp("en")
	pr := a.data.Projects[0]
	x := a.data.projectTasks(pr.ID)[0]
	step := a.remember()
	a.data.deleteProject(pr.ID)
	a.settle(step)
	x.setDone(true, monday)
	a.restore(step)
	if x = a.data.task(x.ID); x.Project != pr.ID || !x.Done {
		t.Errorf("task after undo: project %q, done %v", x.Project, x.Done)
	}

	// Delete one occurrence, drag another, undo the first.
	var series *Event
	for _, e := range a.data.Events {
		if e.repeating() {
			series = e
			break
		}
	}
	occ := series.occurrences(series.Start, addDays(series.Start, 60))
	step = a.remember()
	a.data.skip(occ[1])
	a.settle(step)
	a.moveToDay(occ[2], addDays(occ[2].Start, 1))
	a.restore(step)
	if series.skipped(occ[1].Start) || !series.skipped(occ[2].Start) {
		t.Errorf("series skips %v", series.Skip)
	}
}

// Undoing the erase of many events is quick.
func TestUndoMany(t *testing.T) {
	a := testApp("en")
	for i := range 10000 {
		a.data.Events = append(a.data.Events, &Event{ID: "e" + itoa(i), Title: "x", Start: monday, End: monday.Add(time.Hour)})
	}
	a.replaceData(newData(), "Erased")
	begin := time.Now()
	a.restore(a.replaced)
	if took := time.Since(begin); len(a.data.Events) < 10000 || took > 300*time.Millisecond {
		t.Errorf("%d events back in %v", len(a.data.Events), took)
	}
}

// The pages, and the lists of tasks, change as the pointer goes down.
func TestSwitchOnPress(t *testing.T) {
	a := testApp("en")
	tt := newTester(t, a, "en")
	r, ok := tt.Find("Tasks")
	if !ok {
		t.Fatal("no Tasks tab")
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if a.page != PageTasks {
		t.Fatalf("pressed, the page is %v", a.page)
	}
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	r, ok = tt.Find("Important  3")
	if !ok {
		t.Fatalf("no Important list: %q", tt.Texts())
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if a.taskList != listImportant {
		t.Errorf("pressed, the list is %v", a.taskList)
	}
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
}

// Events' tags are the user's: data of 0.4 gets the five it had, and
// tags are added, renamed, recolored, put first and deleted, with Undo.
func TestKinds(t *testing.T) {
	old := `{"format":"nexttodo","version":2,"categories":[],"events":[
		{"id":"a","title":"Sync","kind":0,"start":"2026-10-26T10:00:00Z","end":"2026-10-26T11:00:00Z"},
		{"id":"b","title":"Talk","kind":2,"start":"2026-10-27T10:00:00Z","end":"2026-10-27T11:00:00Z"}]}`
	d, err := decodeData([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	d.migrate("zh-Hans")
	if len(d.Kinds) != 5 || d.Kinds[0].Name != "会议" || d.Events[0].Kind != "meeting" || d.Events[1].Kind != "seminar" {
		t.Fatalf("migrated tags %+v, events %q %q", d.Kinds, d.Events[0].Kind, d.Events[1].Kind)
	}

	a := testApp("en")
	a.page, a.kindsOpen = PageSchedule, true
	tt := newTester(t, a, "en")
	if !tt.HasText("Event tags") || !tt.HasText("New events") {
		t.Fatalf("no sheet: %q", tt.Texts())
	}
	tt.Click("New tag")
	tt.Type("Teaching")
	tt.Key(0, ui.KeyEnter)
	k := a.data.Kinds[len(a.data.Kinds)-1]
	if k.Name != "Teaching" {
		t.Fatalf("added %+v", k)
	}
	// First, it tags new events.
	a.data.moveKind(k, a.data.Kinds[0])
	a.kindsOpen = false
	a.newEvent(at(monday, 9, 0), "")
	if a.evt.kind != k.ID {
		t.Errorf("a new event is tagged %q", a.evt.kind)
	}
	a.evtOpen = false
	// Deleting a tag of events leaves them untagged, until Undo.
	// So does a calendar subscribed to, which would give it to its events
	// again.
	meeting := a.data.kind("meeting")
	n := a.eventsTagged("meeting")
	sub := &Subscription{ID: "s", Name: "Dept", Kind: "meeting"}
	a.data.Subscriptions = append(a.data.Subscriptions, sub)
	step := a.remember()
	a.data.deleteKind(meeting.ID)
	a.settle(step)
	if a.data.kind("meeting") != nil || a.eventsTagged("meeting") != 0 || n == 0 || sub.Kind != "" {
		t.Fatalf("deleted: %d events still tagged, the calendar's tag %q", a.eventsTagged("meeting"), sub.Kind)
	}
	a.restore(step)
	if a.data.kind("meeting") == nil || a.eventsTagged("meeting") != n || a.data.Subscriptions[0].Kind != "meeting" {
		t.Errorf("after Undo, %d of %d events tagged, the calendar's tag %q", a.eventsTagged("meeting"), n, a.data.Subscriptions[0].Kind)
	}
}

// A first launch welcomes with where to begin; the sample data comes and
// goes on a click, leaving what the user made.
func TestWelcomeAndSample(t *testing.T) {
	a := newApp(freshData("en"), "")
	a.now = func() time.Time { return monday }
	tt := newTester(t, a, "en")
	for _, s := range []string{"Welcome to NextTodo", "Add your first task", "Plan an event", "Start a project", "Import a calendar"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click("Add your first task"); err != nil {
		t.Fatal(err)
	}
	if a.page != PageTasks {
		t.Fatalf("the first task's tile went to %v", a.page)
	}
	a.navigate(PageOverview)
	tt.Frame()
	tt.Click("Look around with sample data first")
	if a.data.empty() || !a.data.hasSample() {
		t.Fatal("no sample data")
	}
	tt.Frame()
	if !tt.HasText("This is sample data, to show what NextTodo keeps.") {
		t.Fatalf("no notice of the sample data: %q", tt.Texts())
	}
	// Things of one's own stay as the sample goes, and a sample project
	// one put a task in, with the task; Undo brings it back.
	adopted := a.data.Projects[0]
	mine := a.addTask("Mine", adopted.ID, listAll)
	own := &Event{ID: "own", Title: "组会", Start: at(monday, 9, 0), End: at(monday, 10, 0)}
	a.data.Events = append(a.data.Events, own)
	tt.Click("Clear sample data")
	if a.data.hasSample() || len(a.data.Tasks) != 1 || a.data.Tasks[0] != mine || mine.Project != adopted.ID ||
		len(a.data.Projects) != 1 || len(a.data.Events) != 1 || a.data.Events[0] != own || len(a.data.Kinds) != 5 {
		t.Fatalf("after clearing: %d projects, %d tasks, %d events", len(a.data.Projects), len(a.data.Tasks), len(a.data.Events))
	}
	a.restore(a.undo[len(a.undo)-1])
	if !a.data.hasSample() || a.data.task(mine.ID) == nil {
		t.Error("Undo did not bring the sample back")
	}
}

// A repeating task done comes again, due the next time, and undone at
// once takes that back.
func TestRepeatingTask(t *testing.T) {
	a := testApp("en")
	friday := addDays(day(monday), 4)
	x := a.addTask("Weekly report", "", listAll)
	x.Due, x.Repeat, x.Remind = friday, RepeatWeekly, at(friday, 9, 0)
	a.setDone(x, true)
	var next *Task
	for _, y := range a.data.Tasks {
		if y.Title == "Weekly report" && !y.Done {
			next = y
		}
	}
	if next == nil || !next.Due.Equal(addDays(friday, 7)) || next.Repeat != RepeatWeekly || !next.Remind.Equal(at(addDays(friday, 7), 9, 0)) {
		t.Fatalf("next %+v", next)
	}
	if !x.Done || x.Repeat != RepeatNone {
		t.Errorf("the one done: %+v", x)
	}
	a.setDone(x, false)
	if a.data.task(next.ID) != nil || x.Repeat != RepeatWeekly || x.Done {
		t.Errorf("undone, the next stays or the repeat is lost: %+v", x)
	}
	// Late, it comes again on the next of its days still to come.
	x.Due = addDays(day(monday), -7)
	a.setDone(x, true)
	for _, y := range a.data.Tasks {
		if y.Title == "Weekly report" && !y.Done && !y.Due.Equal(day(monday)) {
			t.Errorf("a late weekly task comes again %v", y.Due)
		}
	}
	// A monthly one on the 31st comes on the last of a shorter month, and
	// on the 31st again after it; a yearly one of February 29 in leap years.
	jan31 := &Task{ID: "m", Title: "Rent", Due: time.Date(2027, 1, 31, 0, 0, 0, 0, time.Local), Repeat: RepeatMonthly}
	var dues []string
	for x := jan31; len(dues) < 4; {
		x = a.data.repeatTask(x, x.Due)
		dues = append(dues, x.Due.Format("01-02"))
	}
	if strings.Join(dues, " ") != "02-28 03-31 04-30 05-31" {
		t.Errorf("monthly from Jan 31: %v", dues)
	}
	leap := &Task{ID: "y", Title: "Birthday", Due: time.Date(2028, 2, 29, 0, 0, 0, 0, time.Local), Repeat: RepeatYearly}
	dues = nil
	for x := leap; len(dues) < 4; {
		x = a.data.repeatTask(x, x.Due)
		dues = append(dues, x.Due.Format("2006-01-02"))
	}
	if strings.Join(dues, " ") != "2029-02-28 2030-02-28 2031-02-28 2032-02-29" {
		t.Errorf("yearly from Feb 29: %v", dues)
	}
	// Late and done, a monthly one of the 31st comes on the next 31st or
	// last day still to come.
	late := &Task{ID: "l", Title: "Late", Due: time.Date(2027, 1, 31, 0, 0, 0, 0, time.Local), Repeat: RepeatMonthly}
	if x := a.data.repeatTask(late, time.Date(2027, 4, 15, 10, 0, 0, 0, time.Local)); x.Due.Day() != 30 {
		t.Errorf("caught up to %v, want April 30", x.Due)
	}
	// One moved off the end of its month by hand keeps its new day.
	moved := &Task{ID: "v", Title: "Moved", Due: time.Date(2027, 2, 15, 0, 0, 0, 0, time.Local), Repeat: RepeatMonthly, RepeatDay: 31}
	if x := a.data.repeatTask(moved, moved.Due); x.Due.Day() != 15 {
		t.Errorf("moved by hand, comes again on %v", x.Due)
	}
	// Typed: due Friday, every week.
	a.page = PageTasks
	tt := newTester(t, a, "en")
	tt.Click("Add a task")
	tt.Type("Group seminar every friday")
	tt.Key(0, ui.KeyEnter)
	if y := taskTitled(a, "Group seminar"); y == nil || y.Repeat != RepeatWeekly || !y.Due.Equal(friday) {
		t.Errorf("typed: %+v", y)
	}
}

// A task repeating in a way of a later version is done once, rather than
// hanging the app as it looked for its next day.
func TestUnknownTaskRepeat(t *testing.T) {
	a := testApp("en")
	x := a.addTask("Quarterly review", "", listAll)
	x.Due, x.Repeat = addDays(day(monday), -7), Repeat("quarterly")
	n := len(a.data.Tasks)
	done := make(chan bool)
	go func() {
		a.setDone(x, true)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hung")
	}
	if !x.Done || len(a.data.Tasks) != n {
		t.Errorf("done %v, %d tasks", x.Done, len(a.data.Tasks))
	}
}

// Undoing a repeating task's done takes its next back, unless the next was
// done or changed since: then the next goes on, and the first stays once.
func TestUndoRepeatingTaskLater(t *testing.T) {
	a := testApp("en")
	r := a.addTask("Weekly report", "", listAll)
	r.Due, r.Repeat = addDays(day(monday), 4), RepeatWeekly
	a.setDone(r, true)
	n1 := a.data.Tasks[len(a.data.Tasks)-1]
	a.setDone(n1, true)
	n2 := a.data.Tasks[len(a.data.Tasks)-1]
	a.setDone(r, false)
	open := 0
	for _, x := range a.data.Tasks {
		if x.Title == "Weekly report" && !x.Done && x.Repeat != RepeatNone {
			open++
		}
	}
	if open != 1 || r.Repeat != RepeatNone || a.data.task(n2.ID) == nil {
		t.Errorf("%d series open, the first repeats %q", open, r.Repeat)
	}
	// Changed since, the next stays with what was added to it.
	s := a.addTask("Monthly bills", "", listAll)
	s.Due, s.Repeat = addDays(day(monday), 4), RepeatMonthly
	a.setDone(s, true)
	next := a.data.Tasks[len(a.data.Tasks)-1]
	next.Notes = "Pay the lab's cloud bill too"
	a.setDone(s, false)
	if a.data.task(next.ID) == nil || s.Repeat != RepeatNone {
		t.Errorf("the next, changed, went: %v; the first repeats %q", a.data.task(next.ID) == nil, s.Repeat)
	}
}

// A task's checklist: steps added one after another, ticked off, counted
// in its row, found by search, and fresh again in a repeating task's next.
func TestTaskSteps(t *testing.T) {
	a := testApp("en")
	x := taskTitled(a, "Book flights for the December workshop")
	a.page, a.taskList, a.taskSel = PageTasks, listAll, x.ID
	tt := newTester(t, a, "en")
	if err := tt.Click("Add a step"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Compare fares")
	tt.Key(0, ui.KeyEnter)
	tt.Type("Ask for the visa letter")
	tt.Key(0, ui.KeyEnter)
	if len(x.Steps) != 2 || x.Steps[1].Title != "Ask for the visa letter" {
		t.Fatalf("steps %+v", x.Steps)
	}
	if err := tt.Click("Mark as done: Compare fares"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !x.Steps[0].Done || !tt.HasText("1/2") {
		t.Errorf("a step ticked off: %+v, texts %q", x.Steps[0], tt.Texts())
	}
	if hits := a.searchHits("visa letter"); len(hits) == 0 || hits[0].key != "t"+x.ID {
		t.Errorf("search %+v", hits)
	}
	x.Repeat = RepeatMonthly
	a.setDone(x, true)
	for _, y := range a.data.Tasks {
		if y.Title == x.Title && !y.Done && (len(y.Steps) != 2 || y.stepsDone() != 0) {
			t.Errorf("the next has steps %+v", y.Steps)
		}
	}
}

// A right-click, or a Control-click, on a tab or a switch opens its menu,
// not the page or the choice.
func TestRightClickDoesNotSwitch(t *testing.T) {
	a := testApp("en")
	tt := newTester(t, a, "en")
	tt.RightClick("Tasks")
	tt.ClickWith(ui.Ctrl, "Schedule")
	if a.page != PageOverview {
		t.Errorf("a right-click went to %v", a.page)
	}
	tt.Click("Tasks")
	if a.page != PageTasks {
		t.Fatal("a click did not go to the tasks")
	}
	tt.Frame()
	tt.RightClick("Important  3")
	tt.ClickWith(ui.Ctrl, "Planned  10")
	if a.taskList != listMyDay {
		t.Errorf("a right-click chose list %v", a.taskList)
	}
}

// Undo of a deletion made with the event's dialog open keeps the dialog
// and what is typed in it.
func TestUndoKeepsDialog(t *testing.T) {
	a := testApp("en")
	a.newEvent(at(monday, 9, 0), "")
	a.evt.title = "Half typed"
	step := a.remember()
	a.data.deleteKind("seminar")
	a.settle(step)
	a.restore(step)
	if !a.evtOpen || a.evt.title != "Half typed" || a.data.kind("seminar") == nil {
		t.Errorf("dialog open %v, title %q", a.evtOpen, a.evt.title)
	}
}

// Data of 0.6 and before had its sample unmarked: it is marked as such
// only if there is enough of it, and one's own 组会 alone is not.
func TestMarkSample(t *testing.T) {
	d := sampleData(monday, "zh-Hans")
	own := &Task{ID: "own", Title: "我的待办"}
	d.Tasks = append(d.Tasks, own)
	// A sample project with an entry of one's own, and one with a task of
	// one's own, and a sample direction with a project of one's own: all
	// stay as the sample goes.
	noted, tasked := d.Projects[0], d.Projects[1]
	noted.Journal = append(noted.Journal, Entry{ID: "mine", Date: monday, Text: "与审稿人沟通了补充实验的范围。"})
	mineToo := &Task{ID: "own2", Title: "我自己的实验", Project: tasked.ID}
	d.Tasks = append(d.Tasks, mineToo)
	myProject := &Project{ID: "mp", Title: "我的项目", Direction: d.Directions[0].ID}
	d.Projects = append(d.Projects, myProject)
	for _, x := range d.Tasks {
		x.Sample = false
	}
	for _, x := range d.Events {
		x.Sample = false
	}
	for _, x := range d.Projects {
		x.Sample = false
	}
	for _, x := range d.Directions {
		x.Sample = false
	}
	d.Version = 3
	d.migrate("zh-Hans")
	if !d.hasSample() || own.Sample {
		t.Errorf("legacy sample: marked %v, own %v", d.hasSample(), own.Sample)
	}
	d.clearSample()
	if d.project(noted.ID) == nil || len(noted.Journal) == 0 || d.project(tasked.ID) == nil || mineToo.Project != tasked.ID ||
		myProject.Direction == "" || d.direction(myProject.Direction) == nil {
		t.Errorf("clearing took one's own: noted %v, tasked %v, my direction %q",
			d.project(noted.ID) != nil, d.project(tasked.ID) != nil, myProject.Direction)
	}
	if d.hasSample() {
		t.Error("the sample stayed")
	}
	alone := freshData("zh-Hans")
	alone.Events = append(alone.Events, &Event{ID: "g", Title: "组会"})
	alone.Directions = append(alone.Directions, &Direction{ID: "d", Title: "科学智能"})
	alone.Version = 3
	alone.migrate("zh-Hans")
	if alone.hasSample() {
		t.Error("one's own 组会 was taken for the sample")
	}
}

// A day of the month shows the events that fit it, and how many more
// there are, never one cut in half.
func TestMonthFit(t *testing.T) {
	for _, c := range []struct {
		h       float32
		n, want int
	}{
		{120, 3, 3}, // room 89: three take 63
		{120, 4, 4}, // four take 84
		{120, 5, 3}, // five do not: three and "2 more" take 80
		{76, 2, 2},  // room 45: two take 42
		{76, 4, 1},  // one and "3 more" take 38
		{40, 3, 0},  // no room but for "3 more"
		{200, 0, 0},
	} {
		if got := monthFit(c.h)(c.n); got != c.want {
			t.Errorf("%d in a day %v tall: shows %d, want %d", c.n, c.h, got, c.want)
		}
		if got := monthFit(c.h)(c.n); got < c.n {
			room := c.h - 1 - 2*monthPad - monthDayH
			if used := float32(got)*(monthChipH+monthGap) + monthGap + monthMoreH; used > room && got > 0 {
				t.Errorf("%d in a day %v tall: %d and more take %v of %v", c.n, c.h, got, used, room)
			}
		}
	}
}

// Holidays subscribed to before anything else leave the welcome, and the
// sample comes with them, rather than in their place.
func TestWelcomeWithHolidays(t *testing.T) {
	a := newApp(freshData("en"), "")
	a.now = func() time.Time { return monday }
	sub := &Subscription{ID: "h", Name: "Holidays", URL: "https://example.com/h.ics"}
	hol := &Event{ID: "x", Title: "Holiday", Sub: "h", AllDay: true, Start: day(monday), End: addDays(day(monday), 1)}
	a.data.Subscriptions, a.data.Events = []*Subscription{sub}, []*Event{hol}
	tt := newTester(t, a, "en")
	if !a.data.empty() || !tt.HasText("Welcome to NextTodo") {
		t.Fatalf("no welcome beside the holidays: %q", tt.Texts())
	}
	tt.Click("Look around with sample data first")
	if !a.data.hasSample() || len(a.data.Subscriptions) != 1 || a.data.event("x") == nil {
		t.Fatalf("the sample took the holidays' place: %d calendars", len(a.data.Subscriptions))
	}
	a.clearSample()
	if !a.data.empty() || a.data.event("x") == nil {
		t.Error("clearing the sample took the holidays")
	}
}

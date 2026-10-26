package main

import (
	"os"
	"path/filepath"
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
	d.Events = append(d.Events, &Event{ID: "y", Start: at(monday, 9, 0), End: at(monday, 10, 0), Repeat: "yearly"})
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
	if n := len(timelineTicks(time.Date(2020, 3, 1, 0, 0, 0, 0, time.Local), time.Date(2030, 1, 1, 0, 0, 0, 0, time.Local))); n != 9 {
		t.Errorf("%d ticks from March 2020 to 2030, want each January", n)
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
	if block == nil || block.Kind != KindFocus || block.Project != task.Project || !sameDay(block.Start, task.Due) ||
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

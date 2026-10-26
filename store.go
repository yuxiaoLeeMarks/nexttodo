package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// CatID names a category of projects. NextTodo 0.2 wrote a number, one of
// four fixed categories, which reads as the ID of its default category.
type CatID string

// The default categories: the 纵向 / 横向 split of Chinese research, with
// self-directed work and academic service.
var defaultCatIDs = []CatID{"funded", "industry", "independent", "service"}

func (c *CatID) UnmarshalJSON(b []byte) error {
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*c = ""
		if n >= 0 && n < len(defaultCatIDs) {
			*c = defaultCatIDs[n]
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*c = CatID(s)
	return nil
}

// Category is a kind of project the user names, describes, colors and
// orders: by how projects are funded, as 纵向课题 and 横向课题, or any way
// that suits them.
type Category struct {
	ID    CatID  `json:"id"`
	Name  string `json:"name"`
	Note  string `json:"note,omitempty"`
	Color int    `json:"color"` // a color of the palette's series
	Icon  string `json:"icon,omitempty"`
}

// Status is where a project stands.
type Status int

const (
	StatusPlanning Status = iota
	StatusActive
	StatusPaused
	StatusDone
)

var statuses = []Status{StatusPlanning, StatusActive, StatusPaused, StatusDone}

// EventKind is what an event is, drawn as a different engraving fill in the
// calendar: solid ink, hatching, an outline or a light wash.
type EventKind int

const (
	KindMeeting EventKind = iota
	KindLab
	KindSeminar
	KindFocus
	KindOther
)

var eventKinds = []EventKind{KindMeeting, KindLab, KindSeminar, KindFocus, KindOther}

// Direction is a research direction (研究方向) that projects belong to.
type Direction struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Summary string    `json:"summary,omitempty"`
	Primary bool      `json:"primary,omitempty"`
	Created time.Time `json:"created"`
}

// Entry is a dated line of a project's journal.
type Entry struct {
	ID   string    `json:"id"`
	Date time.Time `json:"date"`
	Text string    `json:"text"`
}

// Project is a research project.
type Project struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Code      string    `json:"code,omitempty"` // grant or contract number
	Category  CatID     `json:"category,omitempty"`
	Status    Status    `json:"status"`
	Direction string    `json:"direction,omitempty"`
	Lead      string    `json:"lead,omitempty"` // PI or role
	Funder    string    `json:"funder,omitempty"`
	Start     time.Time `json:"start,omitzero"`
	End       time.Time `json:"end,omitzero"`
	Summary   string    `json:"summary,omitempty"`
	Journal   []Entry   `json:"journal,omitempty"`
	Created   time.Time `json:"created"`
	// An archived project, and its tasks, are put away: out of the lists,
	// the overview and the pickers, kept as they were.
	Archived   bool      `json:"archived,omitempty"`
	ArchivedAt time.Time `json:"archivedAt,omitzero"`
}

// Task is a to-do. Due is a date (local midnight), zero for none; the task
// is in My Day while MyDay is today.
type Task struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Notes    string    `json:"notes,omitempty"`
	Project  string    `json:"project,omitempty"`
	Due      time.Time `json:"due,omitzero"`
	Remind   time.Time `json:"remind,omitzero"` // when a notification reminds of it
	Priority int       `json:"priority,omitempty"` // 0 none, 1 low, 2 medium, 3 high
	Starred  bool      `json:"starred,omitempty"`
	MyDay    time.Time `json:"myDay,omitzero"`
	Done     bool      `json:"done,omitempty"`
	DoneAt   time.Time `json:"doneAt,omitzero"`
	Created  time.Time `json:"created"`
}

// Event is something on the schedule.
type Event struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Kind     EventKind `json:"kind"`
	Project  string    `json:"project,omitempty"`
	Location string    `json:"location,omitempty"`
	Notes    string    `json:"notes,omitempty"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	AllDay   bool      `json:"allDay,omitempty"`
	// Alert is how many minutes before the start to send a notification,
	// alertNone for none, or nil for the default of the settings.
	Alert *int `json:"alert,omitempty"`
	// Repeat repeats the event from Start until the day Until, if set,
	// but on the days of Skip.
	Repeat Repeat      `json:"repeat,omitempty"`
	Until  time.Time   `json:"until,omitzero"`
	Skip   []time.Time `json:"skip,omitempty"`

	series *Event // the event an occurrence repeats
}

// alertNone is an alert that is never sent.
const alertNone = -1

// Prefs are the app's own settings; MyGo keeps the language too.
type Prefs struct {
	Language     string `json:"language,omitempty"`   // "" the system's, "en" or "zh-Hans"
	Appearance   string `json:"appearance,omitempty"` // "", "light" or "dark"
	WeekStart    int    `json:"weekStart"`            // -1 the region's, 0 Sunday, 1 Monday
	AlertMinutes int    `json:"alertMinutes"`         // the default alert, or alertNone
	HideMenuBar  bool   `json:"hideMenuBar,omitempty"`
	NoBadge      bool   `json:"noBadge,omitempty"`  // no count on the Dock icon
	NoHotkey     bool   `json:"noHotkey,omitempty"` // no global shortcut for the panel
	Page         int    `json:"page,omitempty"`     // the page shown last, which opens again
}

// Data is everything the app keeps, as written to disk and to backups.
type Data struct {
	Format     string       `json:"format"`
	Version    int          `json:"version"`
	Categories []*Category  `json:"categories"`
	Directions []*Direction `json:"directions"`
	Projects   []*Project   `json:"projects"`
	Tasks      []*Task      `json:"tasks"`
	Events     []*Event     `json:"events"`
	Prefs      Prefs        `json:"prefs"`
}

const (
	dataFormat  = "nexttodo"
	dataVersion = 2 // 2: categories of the user's own
)

func newData() *Data {
	return &Data{Format: dataFormat, Version: dataVersion, Prefs: Prefs{WeekStart: -1, AlertMinutes: 10}}
}

// migrate brings data written by an earlier version up to date: data of
// 0.2 gets the default categories its projects' numbers stood for.
func (d *Data) migrate(defaults func() []*Category) {
	if d.Version < 2 && len(d.Categories) == 0 {
		d.Categories = defaults()
	}
	d.Version = dataVersion
}

func newID() string {
	var b [6]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// decodeData reads data from a file or a backup.
func decodeData(b []byte) (*Data, error) {
	d := newData()
	if err := json.Unmarshal(b, d); err != nil {
		return nil, err
	}
	if d.Format != dataFormat {
		return nil, errors.New("not a NextTodo backup")
	}
	d.Categories = slices.DeleteFunc(d.Categories, func(x *Category) bool { return x == nil })
	d.Directions = slices.DeleteFunc(d.Directions, func(x *Direction) bool { return x == nil })
	d.Projects = slices.DeleteFunc(d.Projects, func(x *Project) bool { return x == nil })
	d.Tasks = slices.DeleteFunc(d.Tasks, func(x *Task) bool { return x == nil })
	d.Events = slices.DeleteFunc(d.Events, func(x *Event) bool { return x == nil })
	for _, p := range d.Projects {
		if !slices.Contains(statuses, p.Status) {
			p.Status = StatusActive
		}
	}
	return d, nil
}

func (d *Data) encode() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// writeFile writes b to path through a temporary file, so that a crash
// leaves either the old file or the new one.
func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Lookups.

func (d *Data) project(id string) *Project {
	if id == "" {
		return nil
	}
	for _, p := range d.Projects {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (d *Data) category(id CatID) *Category {
	if id == "" {
		return nil
	}
	for _, k := range d.Categories {
		if k.ID == id {
			return k
		}
	}
	return nil
}

// deleteCategory removes a category; its projects are left without one.
func (d *Data) deleteCategory(id CatID) {
	d.Categories = slices.DeleteFunc(d.Categories, func(k *Category) bool { return k.ID == id })
	for _, p := range d.Projects {
		if p.Category == id {
			p.Category = ""
		}
	}
}

// moveCategory puts the category moved before the one at, or last.
func (d *Data) moveCategory(moved, at *Category) {
	i := slices.Index(d.Categories, moved)
	if i < 0 || moved == at {
		return
	}
	d.Categories = slices.Delete(d.Categories, i, i+1)
	j := slices.Index(d.Categories, at)
	if j < 0 {
		j = len(d.Categories)
	}
	d.Categories = slices.Insert(d.Categories, j, moved)
}

func (d *Data) direction(id string) *Direction {
	if id == "" {
		return nil
	}
	for _, x := range d.Directions {
		if x.ID == id {
			return x
		}
	}
	return nil
}

func (d *Data) task(id string) *Task {
	if id == "" {
		return nil
	}
	for _, t := range d.Tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (d *Data) event(id string) *Event {
	if id == "" {
		return nil
	}
	for _, e := range d.Events {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (d *Data) projectTitle(id string) string {
	if p := d.project(id); p != nil {
		return p.Title
	}
	return ""
}

// Changes.

func (d *Data) deleteTask(id string) {
	d.Tasks = slices.DeleteFunc(d.Tasks, func(t *Task) bool { return t.ID == id })
}

func (d *Data) deleteEvent(id string) {
	d.Events = slices.DeleteFunc(d.Events, func(e *Event) bool { return e.ID == id })
}

// deleteProject removes a project and detaches its tasks and events, which
// stay on their own.
func (d *Data) deleteProject(id string) {
	d.Projects = slices.DeleteFunc(d.Projects, func(p *Project) bool { return p.ID == id })
	for _, t := range d.Tasks {
		if t.Project == id {
			t.Project = ""
		}
	}
	for _, e := range d.Events {
		if e.Project == id {
			e.Project = ""
		}
	}
}

func (d *Data) deleteDirection(id string) {
	d.Directions = slices.DeleteFunc(d.Directions, func(x *Direction) bool { return x.ID == id })
	for _, p := range d.Projects {
		if p.Direction == id {
			p.Direction = ""
		}
	}
}

func (t *Task) setDone(done bool, now time.Time) {
	t.Done = done
	if done {
		t.DoneAt = now
	} else {
		t.DoneAt = time.Time{}
	}
}

// Queries.

// shelved reports whether t belongs to an archived project.
func (d *Data) shelved(t *Task) bool {
	p := d.project(t.Project)
	return p != nil && p.Archived
}

func (t *Task) inMyDay(today time.Time) bool {
	return !t.MyDay.IsZero() && sameDay(t.MyDay, today)
}

func (t *Task) overdue(today time.Time) bool {
	return !t.Done && !t.Due.IsZero() && day(t.Due).Before(today)
}

// progress returns the project's finished and total tasks.
func (d *Data) progress(projectID string) (done, total int) {
	for _, t := range d.Tasks {
		if t.Project == projectID {
			total++
			if t.Done {
				done++
			}
		}
	}
	return done, total
}

func (d *Data) projectTasks(projectID string) []*Task {
	var out []*Task
	for _, t := range d.Tasks {
		if t.Project == projectID {
			out = append(out, t)
		}
	}
	sortTasks(out)
	return out
}

// eventsBetween returns the events overlapping [from, to), by start, with
// the occurrences of repeating ones.
func (d *Data) eventsBetween(from, to time.Time) []*Event {
	var out []*Event
	for _, e := range d.Events {
		if e.repeating() {
			out = append(out, e.occurrences(from, to)...)
			continue
		}
		if e.Start.Before(to) && e.End.After(from) || e.Start.Equal(from) {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b *Event) int { return a.Start.Compare(b.Start) })
	return out
}

func (d *Data) tasksDueOn(date time.Time) []*Task {
	var out []*Task
	for _, t := range d.Tasks {
		if !t.Due.IsZero() && sameDay(t.Due, date) && !d.shelved(t) {
			out = append(out, t)
		}
	}
	sortTasks(out)
	return out
}

// sortTasks orders tasks as lists show them: open before done, then by due
// date (dated first), priority, stars and creation.
func sortTasks(ts []*Task) {
	slices.SortStableFunc(ts, func(a, b *Task) int {
		if a.Done != b.Done {
			if a.Done {
				return 1
			}
			return -1
		}
		if a.Done {
			return b.DoneAt.Compare(a.DoneAt)
		}
		switch {
		case a.Due.IsZero() && !b.Due.IsZero():
			return 1
		case !a.Due.IsZero() && b.Due.IsZero():
			return -1
		case !a.Due.Equal(b.Due):
			return a.Due.Compare(b.Due)
		}
		if a.Priority != b.Priority {
			return b.Priority - a.Priority
		}
		if a.Starred != b.Starred {
			if a.Starred {
				return -1
			}
			return 1
		}
		return a.Created.Compare(b.Created)
	})
}

// matches reports whether s contains the query, ignoring case.
func matches(query string, fields ...string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// Dates.

// day returns the local midnight starting t's day.
func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func addDays(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+n, t.Hour(), t.Minute(), 0, 0, t.Location())
}

// weekStart returns the first day of t's week, which starts on first.
func weekStart(t time.Time, first time.Weekday) time.Time {
	t = day(t)
	back := (int(t.Weekday()) - int(first) + 7) % 7
	return addDays(t, -back)
}

// daysBetween counts whole days from a to b.
func daysBetween(a, b time.Time) int {
	a, b = day(a), day(b)
	return int(math.Round(b.Sub(a).Hours() / 24))
}

// at returns date at hour h and minute m.
func at(date time.Time, h, m int) time.Time {
	y, mo, d := date.Date()
	return time.Date(y, mo, d, h, m, 0, 0, date.Location())
}

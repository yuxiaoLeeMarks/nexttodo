package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"nexttodo/internal/mcp"
)

// The tools NextTodo gives its assistant, served over MCP to the pi it
// runs: what the assistant reads of the user's tasks, events, projects and
// directions, and how it adds to them and changes them. There is no tool
// that deletes. Each runs on the main thread, as the windows' changes do,
// and each change is a step of its own that Undo takes back.

// agentTools are the tools, as the MCP server serves them.
func (a *app) agentTools() []mcp.Tool {
	return []mcp.Tool{
		agentTool(a, "get_overview", "Overview", true,
			"The user's day at a glance: the local date and time, today's events, the tasks of My Day (added for today, due today or overdue), deadlines of the coming week, and the active projects. Call it first when a request is about the user's plans.",
			`{"type":"object","properties":{}}`,
			func(a *app, _ struct{}) (any, error) { return a.toolOverview(), nil }),
		agentTool(a, "search", "Search", true,
			"Find tasks, events, projects, project journal entries and research directions whose text contains every word of the query, ignoring case. Use it to find an item's ID before changing it.",
			`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`,
			(*app).toolSearch),
		agentTool(a, "list_tasks", "List tasks", true,
			"List tasks. filter: open (not done), today (My Day), overdue, upcoming (due in the next 14 days), done (recently finished), all. Tasks of archived projects are left out unless project_id names one.",
			`{"type":"object","properties":{"filter":{"type":"string","enum":["open","today","overdue","upcoming","done","all"]},"project_id":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":200}}}`,
			(*app).toolListTasks),
		agentTool(a, "list_events", "List events", true,
			"List the events between two dates, from the start of `from` to the end of `to` (both default to the coming week), with the occurrences of repeating events and those of subscribed calendars, which are read only.",
			`{"type":"object","properties":{"from":{"type":"string","description":"A date, as 2026-03-02 or tomorrow"},"to":{"type":"string"}}}`,
			(*app).toolListEvents),
		agentTool(a, "list_projects", "List projects", true,
			"List the projects, with their status, progress, category and research direction, and the categories and directions there are, with their IDs.",
			`{"type":"object","properties":{"include_done":{"type":"boolean"},"include_archived":{"type":"boolean"}}}`,
			(*app).toolListProjects),
		agentTool(a, "get_project", "Read project", true,
			"Everything of a project: its details and summary, its tasks, its journal (the latest entries first) and its events of the coming month.",
			`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			(*app).toolGetProject),
		agentTool(a, "create_task", "New task", false,
			"Add a task. Dates and times are local: 2026-03-02, 2026-03-02 15:00, or as said, as tomorrow, 下周五, fri 3pm. A due date with a time also sets the reminder to that time.",
			`{"type":"object","properties":{"title":{"type":"string"},"notes":{"type":"string"},"project_id":{"type":"string"},"due":{"type":"string","description":"The day it is due"},"remind":{"type":"string","description":"When to notify, a date and time"},"my_day":{"type":"boolean","description":"Add it to today's list"},"priority":{"type":"string","enum":["none","low","medium","high"]},"starred":{"type":"boolean"},"repeat":{"type":"string","enum":["none","daily","weekdays","weekly","biweekly","monthly","yearly"]},"steps":{"type":"array","items":{"type":"string"},"description":"A checklist of what it takes"}},"required":["title"]}`,
			(*app).toolCreateTask),
		agentTool(a, "update_task", "Change task", false,
			"Change a task: only the fields given change. done marks it done, or not; a repeating task done comes again, due the next time. An empty string clears due, remind or project_id.",
			`{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"notes":{"type":"string"},"project_id":{"type":"string"},"due":{"type":"string"},"remind":{"type":"string"},"my_day":{"type":"boolean"},"priority":{"type":"string","enum":["none","low","medium","high"]},"starred":{"type":"boolean"},"done":{"type":"boolean"},"add_steps":{"type":"array","items":{"type":"string"}}},"required":["id"]}`,
			(*app).toolUpdateTask),
		agentTool(a, "create_project", "New project", false,
			"Add a research project, with its first tasks if given, at once. Status defaults to planning; category and direction are IDs or names from list_projects.",
			`{"type":"object","properties":{"title":{"type":"string"},"summary":{"type":"string","description":"What it is about and what it aims at, a paragraph"},"status":{"type":"string","enum":["planning","active","paused","done"]},"category_id":{"type":"string"},"direction_id":{"type":"string"},"code":{"type":"string","description":"A grant or contract number"},"lead":{"type":"string","description":"PI or role"},"funder":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"tasks":{"type":"array","items":{"type":"object","properties":{"title":{"type":"string"},"due":{"type":"string"},"notes":{"type":"string"}},"required":["title"]}}},"required":["title"]}`,
			(*app).toolCreateProject),
		agentTool(a, "update_project", "Change project", false,
			"Change a project: only the fields given change. archived puts it away with its tasks, or takes it out again.",
			`{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"summary":{"type":"string"},"status":{"type":"string","enum":["planning","active","paused","done"]},"category_id":{"type":"string"},"direction_id":{"type":"string"},"code":{"type":"string"},"lead":{"type":"string"},"funder":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"archived":{"type":"boolean"}},"required":["id"]}`,
			(*app).toolUpdateProject),
		agentTool(a, "add_journal_entry", "Note in journal", false,
			"Add a dated entry to a project's journal: progress, a decision, notes of a meeting, an idea worth keeping.",
			`{"type":"object","properties":{"project_id":{"type":"string"},"text":{"type":"string"},"date":{"type":"string","description":"Defaults to today"}},"required":["project_id","text"]}`,
			(*app).toolAddEntry),
		agentTool(a, "create_event", "New event", false,
			"Add an event to the schedule. start is local, as 2026-03-02 14:00 or 明天下午3点; a date alone makes an all-day event. It lasts an hour unless end or duration_minutes says otherwise. tag_id is an event tag's ID or name.",
			`{"type":"object","properties":{"title":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"duration_minutes":{"type":"integer","minimum":1},"all_day":{"type":"boolean"},"location":{"type":"string"},"notes":{"type":"string"},"project_id":{"type":"string"},"tag_id":{"type":"string"},"repeat":{"type":"string","enum":["none","daily","weekdays","weekly","biweekly","monthly","yearly"]},"until":{"type":"string","description":"The last day a repeating event comes"}},"required":["title","start"]}`,
			(*app).toolCreateEvent),
		agentTool(a, "update_event", "Change event", false,
			"Change an event that does not repeat: only the fields given change. Moving start keeps its length unless end is given. Events of subscribed calendars are read only.",
			`{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"location":{"type":"string"},"notes":{"type":"string"},"project_id":{"type":"string"},"tag_id":{"type":"string"}},"required":["id"]}`,
			(*app).toolUpdateEvent),
		agentTool(a, "create_direction", "New direction", false,
			"Add a research direction (研究方向), which projects belong to.",
			`{"type":"object","properties":{"title":{"type":"string"},"summary":{"type":"string"}},"required":["title"]}`,
			(*app).toolCreateDirection),
	}
}

// agentTool makes a tool that reads its arguments into a T and runs on the
// main thread, its result written as JSON.
func agentTool[T any](a *app, name, title string, readOnly bool, desc, schema string, run func(*app, T) (any, error)) mcp.Tool {
	return mcp.Tool{
		Name: name, Title: title, Description: desc, Input: json.RawMessage(schema), ReadOnly: readOnly,
		Call: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args T
			if err := json.Unmarshal(raw, &args); err != nil {
				return "", fmt.Errorf("invalid arguments: %v", err)
			}
			var res any
			var err error
			if e := a.onMain(ctx, func() { res, err = run(a, args) }); e != nil {
				return "", e
			}
			if err != nil {
				return "", err
			}
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(res); err != nil {
				return "", err
			}
			return strings.TrimSpace(b.String()), nil
		},
	}
}

// onMain runs fn on the main thread and waits for it; tests, which have
// no main loop, run it at once.
func (a *app) onMain(ctx context.Context, fn func()) error {
	if !a.live {
		fn()
		return nil
	}
	done := make(chan struct{})
	mygo.RunOnMain(func() {
		defer close(done)
		fn()
	})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// What the tools say of things.

const (
	toolDate     = "2006-01-02 Mon"
	toolDateTime = "2006-01-02 Mon 15:04"
)

type taskOut struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Done      bool     `json:"done,omitempty"`
	DoneAt    string   `json:"done_at,omitempty"`
	Due       string   `json:"due,omitempty"`
	Overdue   bool     `json:"overdue,omitempty"`
	Remind    string   `json:"remind,omitempty"`
	Repeat    string   `json:"repeat,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	Starred   bool     `json:"starred,omitempty"`
	MyDay     bool     `json:"my_day,omitempty"`
	Project   string   `json:"project,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
	Notes     string   `json:"notes,omitempty"`
	Steps     []string `json:"steps,omitempty"`
}

func (a *app) taskOut(t *Task) taskOut {
	today := day(a.now())
	o := taskOut{ID: t.ID, Title: t.Title, Done: t.Done, Repeat: string(t.Repeat), Starred: t.Starred,
		MyDay: t.inMyDay(today), ProjectID: t.Project, Notes: clip(t.Notes, 600), Overdue: t.overdue(today)}
	if t.Done {
		o.DoneAt = t.DoneAt.Format(toolDateTime)
	}
	if !t.Due.IsZero() {
		o.Due = t.Due.Format(toolDate)
	}
	if !t.Remind.IsZero() {
		o.Remind = t.Remind.Format(toolDateTime)
	}
	if t.Priority > 0 {
		o.Priority = priorityNames[t.Priority]
	}
	if t.Project != "" {
		o.Project = a.data.projectTitle(t.Project)
	}
	for _, s := range t.Steps {
		mark := "[ ] "
		if s.Done {
			mark = "[x] "
		}
		o.Steps = append(o.Steps, mark+s.Title)
	}
	return o
}

type eventOut struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Start     string `json:"start"`
	End       string `json:"end"`
	AllDay    bool   `json:"all_day,omitempty"`
	Location  string `json:"location,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Project   string `json:"project,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Repeat    string `json:"repeat,omitempty"`
	Calendar  string `json:"calendar,omitempty"` // of a subscription, read only
}

func (a *app) eventOut(e *Event) eventOut {
	o := eventOut{ID: e.key(), Title: e.Title, AllDay: e.AllDay, Location: e.Location, Notes: clip(e.Notes, 400),
		ProjectID: e.Project, Repeat: string(e.root().Repeat)}
	if e.AllDay {
		o.Start, o.End = e.Start.Format(toolDate), addDays(e.End, -1).Format(toolDate)
	} else {
		o.Start, o.End = e.Start.Format(toolDateTime), e.End.Format(toolDateTime)
	}
	if e.Project != "" {
		o.Project = a.data.projectTitle(e.Project)
	}
	if k := a.data.kind(e.Kind); k != nil {
		o.Tag = k.Name
	}
	if e.Sub != "" {
		o.Calendar = "subscribed"
		for _, s := range a.data.Subscriptions {
			if s.ID == e.Sub && s.Name != "" {
				o.Calendar = s.Name
			}
		}
	}
	return o
}

type projectOut struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Progress  string `json:"progress,omitempty"` // tasks done of all
	Category  string `json:"category,omitempty"`
	Direction string `json:"direction,omitempty"`
	Code      string `json:"code,omitempty"`
	Lead      string `json:"lead,omitempty"`
	Funder    string `json:"funder,omitempty"`
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Archived  bool   `json:"archived,omitempty"`
}

func (a *app) projectOut(p *Project, summary int) projectOut {
	o := projectOut{ID: p.ID, Title: p.Title, Status: statusNames[p.Status], Code: p.Code, Lead: p.Lead,
		Funder: p.Funder, Summary: clip(p.Summary, summary), Archived: p.Archived}
	if done, total := a.data.progress(p.ID); total > 0 {
		o.Progress = strconv.Itoa(done) + "/" + strconv.Itoa(total)
	}
	if k := a.data.category(p.Category); k != nil {
		o.Category = k.Name
	}
	if d := a.data.direction(p.Direction); d != nil {
		o.Direction = d.Title
	}
	if !p.Start.IsZero() {
		o.Start = p.Start.Format(toolDate)
	}
	if !p.End.IsZero() {
		o.End = p.End.Format(toolDate)
	}
	return o
}

var (
	priorityNames = []string{"none", "low", "medium", "high"}
	statusNames   = map[Status]string{StatusPlanning: "planning", StatusActive: "active", StatusPaused: "paused", StatusDone: "done"}
)

// clip shortens s to about n characters.
func clip(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

// Reading.

func (a *app) toolOverview() any {
	now := a.now()
	today := day(now)
	var myDay, overdue, week []taskOut
	for _, t := range a.data.Tasks {
		if t.Done || a.data.shelved(t) {
			continue
		}
		switch {
		case t.overdue(today):
			overdue = append(overdue, a.taskOut(t))
		case a.inList(t, listMyDay):
			myDay = append(myDay, a.taskOut(t))
		case !t.Due.IsZero() && day(t.Due).Before(addDays(today, 8)):
			week = append(week, a.taskOut(t))
		}
	}
	var events []eventOut
	for _, e := range a.data.eventsBetween(today, addDays(today, 1)) {
		events = append(events, a.eventOut(e))
	}
	var active []projectOut
	for _, p := range a.data.Projects {
		if p.Status == StatusActive && !p.Archived {
			active = append(active, a.projectOut(p, 0))
		}
	}
	_, off := now.Zone()
	return map[string]any{
		"now":             now.Format(toolDateTime),
		"utc_offset":      utcOffset(off),
		"today_events":    events,
		"my_day":          myDay,
		"overdue":         overdue,
		"due_next_7_days": week,
		"active_projects": active,
	}
}

func utcOffset(secs int) string {
	sign := "+"
	if secs < 0 {
		sign, secs = "-", -secs
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, secs/3600, secs/60%60)
}

func (a *app) toolSearch(args struct {
	Query string `json:"query"`
}) (any, error) {
	words := strings.Fields(strings.ToLower(args.Query))
	if len(words) == 0 {
		return nil, errors.New("query is empty")
	}
	match := func(fields ...string) bool {
		text := strings.ToLower(strings.Join(fields, "\n"))
		for _, w := range words {
			if !strings.Contains(text, w) {
				return false
			}
		}
		return true
	}
	const limit = 25
	var tasks []taskOut
	for _, t := range a.data.Tasks {
		f := []string{t.Title, t.Notes}
		for _, s := range t.Steps {
			f = append(f, s.Title)
		}
		if len(tasks) < limit && match(f...) {
			tasks = append(tasks, a.taskOut(t))
		}
	}
	var events []eventOut
	for _, e := range a.data.Events {
		if len(events) < limit && match(e.Title, e.Location, e.Notes) {
			events = append(events, a.eventOut(e))
		}
	}
	type entryOut struct {
		ProjectID string `json:"project_id"`
		Project   string `json:"project"`
		Date      string `json:"date"`
		Text      string `json:"text"`
	}
	var projects []projectOut
	var entries []entryOut
	for _, p := range a.data.Projects {
		if len(projects) < limit && match(p.Title, p.Code, p.Summary, p.Lead, p.Funder) {
			projects = append(projects, a.projectOut(p, 300))
		}
		for _, e := range p.Journal {
			if len(entries) < limit && match(e.Text) {
				entries = append(entries, entryOut{p.ID, p.Title, e.Date.Format(toolDate), clip(e.Text, 400)})
			}
		}
	}
	type dirOut struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Summary string `json:"summary,omitempty"`
	}
	var dirs []dirOut
	for _, d := range a.data.Directions {
		if match(d.Title, d.Summary) {
			dirs = append(dirs, dirOut{d.ID, d.Title, clip(d.Summary, 300)})
		}
	}
	if len(tasks)+len(events)+len(projects)+len(entries)+len(dirs) == 0 {
		return map[string]any{"found": "nothing"}, nil
	}
	return map[string]any{"tasks": tasks, "events": events, "projects": projects, "journal": entries, "directions": dirs}, nil
}

func (a *app) toolListTasks(args struct {
	Filter    string `json:"filter"`
	ProjectID string `json:"project_id"`
	Limit     int    `json:"limit"`
}) (any, error) {
	today := day(a.now())
	if args.Limit <= 0 {
		args.Limit = 50
	}
	if args.ProjectID != "" {
		p, err := a.findProject(args.ProjectID)
		if err != nil {
			return nil, err
		}
		args.ProjectID = p.ID
	}
	var ts []*Task
	for _, t := range a.data.Tasks {
		if args.ProjectID != "" && t.Project != args.ProjectID || args.ProjectID == "" && a.data.shelved(t) {
			continue
		}
		var in bool
		switch args.Filter {
		case "today":
			in = a.inList(t, listMyDay) || a.data.shelved(t) && !t.Done && t.inMyDay(today)
		case "overdue":
			in = t.overdue(today)
		case "upcoming":
			in = !t.Done && !t.Due.IsZero() && !day(t.Due).Before(today) && day(t.Due).Before(addDays(today, 15))
		case "done":
			in = t.Done
		case "all":
			in = true
		case "", "open":
			in = !t.Done
		default:
			return nil, fmt.Errorf("unknown filter %q", args.Filter)
		}
		if in {
			ts = append(ts, t)
		}
	}
	sortTasks(ts)
	more := 0
	if len(ts) > args.Limit {
		more = len(ts) - args.Limit
		ts = ts[:args.Limit]
	}
	out := make([]taskOut, 0, len(ts))
	for _, t := range ts {
		out = append(out, a.taskOut(t))
	}
	res := map[string]any{"tasks": out}
	if more > 0 {
		res["more"] = more
	}
	return res, nil
}

func (a *app) toolListEvents(args struct {
	From string `json:"from"`
	To   string `json:"to"`
}) (any, error) {
	today := day(a.now())
	from, to := today, addDays(today, 6)
	if args.From != "" {
		t, _, err := a.readTime(args.From)
		if err != nil {
			return nil, err
		}
		from = day(t)
		if args.To == "" {
			to = addDays(from, 6)
		}
	}
	if args.To != "" {
		t, _, err := a.readTime(args.To)
		if err != nil {
			return nil, err
		}
		to = day(t)
	}
	if to.Before(from) {
		return nil, errors.New("to is before from")
	}
	if daysBetween(from, to) > 92 {
		return nil, errors.New("ask for at most three months at a time")
	}
	var out []eventOut
	for _, e := range a.data.eventsBetween(from, addDays(to, 1)) {
		out = append(out, a.eventOut(e))
		if len(out) == 300 {
			break
		}
	}
	return map[string]any{"from": from.Format(toolDate), "to": to.Format(toolDate), "events": out}, nil
}

func (a *app) toolListProjects(args struct {
	IncludeDone     bool `json:"include_done"`
	IncludeArchived bool `json:"include_archived"`
}) (any, error) {
	var ps []projectOut
	for _, p := range a.data.Projects {
		if p.Archived && !args.IncludeArchived || p.Status == StatusDone && !args.IncludeDone && !p.Archived {
			continue
		}
		ps = append(ps, a.projectOut(p, 200))
	}
	type named struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var cats, dirs, tags []named
	for _, k := range a.data.Categories {
		cats = append(cats, named{string(k.ID), k.Name})
	}
	for _, d := range a.data.Directions {
		dirs = append(dirs, named{d.ID, d.Title})
	}
	for _, k := range a.data.Kinds {
		tags = append(tags, named{string(k.ID), k.Name})
	}
	return map[string]any{"projects": ps, "categories": cats, "directions": dirs, "event_tags": tags}, nil
}

func (a *app) toolGetProject(args struct {
	ID string `json:"id"`
}) (any, error) {
	p, err := a.findProject(args.ID)
	if err != nil {
		return nil, err
	}
	var tasks []taskOut
	for _, t := range a.data.projectTasks(p.ID) {
		tasks = append(tasks, a.taskOut(t))
	}
	type entryOut struct {
		Date string `json:"date"`
		Text string `json:"text"`
	}
	journal := slices.Clone(p.Journal)
	slices.SortStableFunc(journal, func(x, y Entry) int { return y.Date.Compare(x.Date) })
	var entries []entryOut
	for i, e := range journal {
		if i == 30 {
			break
		}
		entries = append(entries, entryOut{e.Date.Format(toolDate), clip(e.Text, 800)})
	}
	today := day(a.now())
	var events []eventOut
	for _, e := range a.data.eventsBetween(today, addDays(today, 31)) {
		if e.Project == p.ID {
			events = append(events, a.eventOut(e))
		}
	}
	return map[string]any{"project": a.projectOut(p, 4000), "tasks": tasks, "journal": entries, "upcoming_events": events}, nil
}

// Changing.

func (a *app) toolCreateTask(args struct {
	Title     string   `json:"title"`
	Notes     string   `json:"notes"`
	ProjectID string   `json:"project_id"`
	Due       string   `json:"due"`
	Remind    string   `json:"remind"`
	MyDay     bool     `json:"my_day"`
	Priority  string   `json:"priority"`
	Starred   bool     `json:"starred"`
	Repeat    string   `json:"repeat"`
	Steps     []string `json:"steps"`
}) (any, error) {
	t := &Task{Title: strings.TrimSpace(args.Title), Notes: strings.TrimSpace(args.Notes), Starred: args.Starred}
	if t.Title == "" {
		return nil, errors.New("title is empty")
	}
	if args.ProjectID != "" {
		p, err := a.findProject(args.ProjectID)
		if err != nil {
			return nil, err
		}
		t.Project = p.ID
	}
	if err := a.setTaskTimes(t, &args.Due, &args.Remind); err != nil {
		return nil, err
	}
	var err error
	if t.Priority, err = readPriority(args.Priority); err != nil {
		return nil, err
	}
	if t.Repeat, err = readRepeat(args.Repeat); err != nil {
		return nil, err
	}
	if t.Repeat != RepeatNone && t.Due.IsZero() {
		t.Due = day(a.now())
	}
	for _, s := range args.Steps {
		if s = strings.TrimSpace(s); s != "" {
			t.Steps = append(t.Steps, &Step{Title: s})
		}
	}
	t.ID, t.Created = newID(), a.now()
	a.agentChange(func() {
		if args.MyDay {
			t.MyDay = day(a.now())
		}
		if p := a.data.project(t.Project); p != nil {
			p.Sample = false
		}
		a.data.Tasks = append(a.data.Tasks, t)
	}, change{kind: "task", id: t.ID, verb: "created"})
	return map[string]any{"created": a.taskOut(t)}, nil
}

func (a *app) toolUpdateTask(args struct {
	ID        string   `json:"id"`
	Title     *string  `json:"title"`
	Notes     *string  `json:"notes"`
	ProjectID *string  `json:"project_id"`
	Due       *string  `json:"due"`
	Remind    *string  `json:"remind"`
	MyDay     *bool    `json:"my_day"`
	Priority  *string  `json:"priority"`
	Starred   *bool    `json:"starred"`
	Done      *bool    `json:"done"`
	AddSteps  []string `json:"add_steps"`
}) (any, error) {
	t := a.data.task(args.ID)
	if t == nil {
		return nil, fmt.Errorf("no task with id %q; find it with search or list_tasks", args.ID)
	}
	// Read everything first, so that a mistake changes nothing.
	next := *t
	next.Steps = slices.Clone(t.Steps)
	if args.Title != nil {
		if next.Title = strings.TrimSpace(*args.Title); next.Title == "" {
			return nil, errors.New("title is empty")
		}
	}
	if args.Notes != nil {
		next.Notes = strings.TrimSpace(*args.Notes)
	}
	if args.ProjectID != nil {
		next.Project = ""
		if *args.ProjectID != "" {
			p, err := a.findProject(*args.ProjectID)
			if err != nil {
				return nil, err
			}
			next.Project = p.ID
		}
	}
	if err := a.setTaskTimes(&next, args.Due, args.Remind); err != nil {
		return nil, err
	}
	if args.MyDay != nil {
		next.MyDay = time.Time{}
		if *args.MyDay {
			next.MyDay = day(a.now())
		}
	}
	if args.Priority != nil {
		var err error
		if next.Priority, err = readPriority(*args.Priority); err != nil {
			return nil, err
		}
	}
	if args.Starred != nil {
		next.Starred = *args.Starred
	}
	for _, s := range args.AddSteps {
		if s = strings.TrimSpace(s); s != "" {
			next.Steps = append(next.Steps, &Step{Title: s})
		}
	}
	verb := "updated"
	if args.Done != nil && *args.Done && !t.Done {
		verb = "completed"
	}
	a.agentChange(func() {
		done := t.Done
		*t = next
		if p := a.data.project(t.Project); p != nil {
			p.Sample = false
		}
		if args.Done != nil && *args.Done != done {
			t.Done = done
			a.setDone(t, *args.Done)
		}
	}, change{kind: "task", id: t.ID, verb: verb})
	res := map[string]any{"updated": a.taskOut(t)}
	if r, ok := a.repeated[t.ID]; ok && verb == "completed" {
		if n := a.data.task(r.next); n != nil {
			res["next"] = a.taskOut(n)
		}
	}
	return res, nil
}

// setTaskTimes reads a task's due day and reminder, as given: nil leaves
// one as it is, "" clears it. A due date said with a time reminds then,
// unless a reminder is given too.
func (a *app) setTaskTimes(t *Task, due, remind *string) error {
	if due != nil {
		t.Due = time.Time{}
		if s := strings.TrimSpace(*due); s != "" {
			d, clock, err := a.readTime(s)
			if err != nil {
				return fmt.Errorf("due: %v", err)
			}
			t.Due = day(d)
			if clock && (remind == nil || strings.TrimSpace(*remind) == "") {
				t.Remind = d
			}
		}
	}
	if remind != nil {
		if s := strings.TrimSpace(*remind); s == "" {
			if due == nil || !t.Remind.IsZero() && !sameDay(t.Remind, t.Due) {
				t.Remind = time.Time{}
			}
		} else {
			r, clock, err := a.readTime(s)
			if err != nil {
				return fmt.Errorf("remind: %v", err)
			}
			if !clock {
				r = at(r, 9, 0) // a day alone reminds in the morning
			}
			t.Remind = r
			if t.Due.IsZero() {
				t.Due = day(r)
			}
		}
	}
	return nil
}

func (a *app) toolCreateProject(args struct {
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	CategoryID  string `json:"category_id"`
	DirectionID string `json:"direction_id"`
	Code        string `json:"code"`
	Lead        string `json:"lead"`
	Funder      string `json:"funder"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Tasks       []struct {
		Title string `json:"title"`
		Due   string `json:"due"`
		Notes string `json:"notes"`
	} `json:"tasks"`
}) (any, error) {
	p := &Project{Title: strings.TrimSpace(args.Title), Summary: strings.TrimSpace(args.Summary),
		Code: strings.TrimSpace(args.Code), Lead: strings.TrimSpace(args.Lead), Funder: strings.TrimSpace(args.Funder)}
	if p.Title == "" {
		return nil, errors.New("title is empty")
	}
	if len(a.data.Categories) > 0 {
		p.Category = a.data.Categories[0].ID
	}
	if err := a.setProjectFields(p, &args.Status, &args.CategoryID, &args.DirectionID, &args.Start, &args.End); err != nil {
		return nil, err
	}
	var tasks []*Task
	for _, x := range args.Tasks {
		t := &Task{Title: strings.TrimSpace(x.Title), Notes: strings.TrimSpace(x.Notes)}
		if t.Title == "" {
			continue
		}
		if err := a.setTaskTimes(t, &x.Due, nil); err != nil {
			return nil, fmt.Errorf("task %q: %v", t.Title, err)
		}
		tasks = append(tasks, t)
	}
	p.ID = newID()
	a.agentChange(func() {
		now := a.now()
		p.Created = now
		a.data.Projects = append(a.data.Projects, p)
		for _, t := range tasks {
			t.ID, t.Created, t.Project = newID(), now, p.ID
			a.data.Tasks = append(a.data.Tasks, t)
		}
	}, change{kind: "project", id: p.ID, verb: "created"})
	out := make([]taskOut, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, a.taskOut(t))
	}
	return map[string]any{"created": a.projectOut(p, 4000), "tasks": out}, nil
}

func (a *app) toolUpdateProject(args struct {
	ID          string  `json:"id"`
	Title       *string `json:"title"`
	Summary     *string `json:"summary"`
	Status      *string `json:"status"`
	CategoryID  *string `json:"category_id"`
	DirectionID *string `json:"direction_id"`
	Code        *string `json:"code"`
	Lead        *string `json:"lead"`
	Funder      *string `json:"funder"`
	Start       *string `json:"start"`
	End         *string `json:"end"`
	Archived    *bool   `json:"archived"`
}) (any, error) {
	p, err := a.findProject(args.ID)
	if err != nil {
		return nil, err
	}
	next := *p
	if args.Title != nil {
		if next.Title = strings.TrimSpace(*args.Title); next.Title == "" {
			return nil, errors.New("title is empty")
		}
	}
	for _, f := range []struct {
		v  *string
		to *string
	}{{args.Summary, &next.Summary}, {args.Code, &next.Code}, {args.Lead, &next.Lead}, {args.Funder, &next.Funder}} {
		if f.v != nil {
			*f.to = strings.TrimSpace(*f.v)
		}
	}
	if err := a.setProjectFields(&next, args.Status, args.CategoryID, args.DirectionID, args.Start, args.End); err != nil {
		return nil, err
	}
	if args.Archived != nil && *args.Archived != p.Archived {
		next.Archived, next.ArchivedAt = *args.Archived, time.Time{}
		if next.Archived {
			next.ArchivedAt = a.now()
		}
	}
	a.agentChange(func() {
		*p = next
		p.Sample = false
		if p.Archived && a.projOpen == p.ID {
			a.projOpen = ""
		}
	}, change{kind: "project", id: p.ID, verb: "updated"})
	return map[string]any{"updated": a.projectOut(p, 4000)}, nil
}

// setProjectFields reads a project's status, category, direction and
// dates, as given: nil or "" leaves one as it is, but "none" takes the
// category or direction away.
func (a *app) setProjectFields(p *Project, status, cat, dir, start, end *string) error {
	if status != nil && *status != "" {
		i := slices.IndexFunc(statuses, func(s Status) bool { return statusNames[s] == *status })
		if i < 0 {
			return fmt.Errorf("unknown status %q", *status)
		}
		p.Status = statuses[i]
	}
	if cat != nil && *cat != "" {
		if strings.EqualFold(*cat, "none") {
			p.Category = ""
		} else {
			k := a.findCategory(*cat)
			if k == nil {
				return fmt.Errorf("no category %q; list_projects lists them", *cat)
			}
			p.Category = k.ID
		}
	}
	if dir != nil && *dir != "" {
		if strings.EqualFold(*dir, "none") {
			p.Direction = ""
		} else {
			d := a.findDirection(*dir)
			if d == nil {
				return fmt.Errorf("no direction %q; list_projects lists them, create_direction adds one", *dir)
			}
			p.Direction = d.ID
		}
	}
	for _, f := range []struct {
		s  *string
		to *time.Time
	}{{start, &p.Start}, {end, &p.End}} {
		if f.s == nil {
			continue
		}
		*f.to = time.Time{}
		if s := strings.TrimSpace(*f.s); s != "" {
			t, _, err := a.readTime(s)
			if err != nil {
				return err
			}
			*f.to = day(t)
		}
	}
	if !p.Start.IsZero() && !p.End.IsZero() && p.End.Before(p.Start) {
		return errors.New("end is before start")
	}
	return nil
}

func (a *app) toolAddEntry(args struct {
	ProjectID string `json:"project_id"`
	Text      string `json:"text"`
	Date      string `json:"date"`
}) (any, error) {
	p, err := a.findProject(args.ProjectID)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(args.Text)
	if text == "" {
		return nil, errors.New("text is empty")
	}
	date := a.now()
	if args.Date != "" {
		d, clock, err := a.readTime(args.Date)
		if err != nil {
			return nil, err
		}
		if date = d; !clock {
			date = at(d, a.now().Hour(), a.now().Minute())
		}
	}
	e := Entry{ID: newID(), Date: date, Text: text}
	a.agentChange(func() {
		p.Journal = append(p.Journal, e)
		p.Sample = false
	}, change{kind: "project", id: p.ID, verb: "noted"})
	return map[string]any{"added": map[string]string{"project": p.Title, "date": e.Date.Format(toolDate), "text": e.Text}}, nil
}

func (a *app) toolCreateEvent(args struct {
	Title     string `json:"title"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Duration  int    `json:"duration_minutes"`
	AllDay    *bool  `json:"all_day"`
	Location  string `json:"location"`
	Notes     string `json:"notes"`
	ProjectID string `json:"project_id"`
	TagID     string `json:"tag_id"`
	Repeat    string `json:"repeat"`
	Until     string `json:"until"`
}) (any, error) {
	e := &Event{Title: strings.TrimSpace(args.Title), Location: strings.TrimSpace(args.Location),
		Notes: strings.TrimSpace(args.Notes), Kind: a.data.newKind()}
	if e.Title == "" {
		return nil, errors.New("title is empty")
	}
	start, clock, err := a.readTime(args.Start)
	if err != nil {
		return nil, fmt.Errorf("start: %v", err)
	}
	e.AllDay = !clock
	if args.AllDay != nil {
		e.AllDay = *args.AllDay
	}
	if err := a.setEventTimes(e, start, clock, args.End, args.Duration); err != nil {
		return nil, err
	}
	if err := a.setEventLinks(e, &args.ProjectID, &args.TagID); err != nil {
		return nil, err
	}
	if e.Repeat, err = readRepeat(args.Repeat); err != nil {
		return nil, err
	}
	if args.Until != "" && e.Repeat != RepeatNone {
		u, _, err := a.readTime(args.Until)
		if err != nil {
			return nil, fmt.Errorf("until: %v", err)
		}
		if e.Until = day(u); e.Until.Before(day(e.Start)) {
			e.Until = day(e.Start)
		}
	}
	e.ID = newID()
	a.agentChange(func() { a.data.Events = append(a.data.Events, e) }, change{kind: "event", id: e.ID, verb: "created"})
	return map[string]any{"created": a.eventOut(e)}, nil
}

func (a *app) toolUpdateEvent(args struct {
	ID        string  `json:"id"`
	Title     *string `json:"title"`
	Start     *string `json:"start"`
	End       *string `json:"end"`
	Location  *string `json:"location"`
	Notes     *string `json:"notes"`
	ProjectID *string `json:"project_id"`
	TagID     *string `json:"tag_id"`
}) (any, error) {
	id, _ := occurrenceRef(args.ID)
	e := a.data.event(id)
	switch {
	case e == nil:
		return nil, fmt.Errorf("no event with id %q; find it with list_events or search", args.ID)
	case e.Sub != "":
		return nil, errors.New("this event comes from a subscribed calendar, which is read only")
	case e.repeating():
		return nil, errors.New("this event repeats; changing it here is not supported, so tell the user to change it in NextTodo's schedule")
	}
	next := *e
	if args.Title != nil {
		if next.Title = strings.TrimSpace(*args.Title); next.Title == "" {
			return nil, errors.New("title is empty")
		}
	}
	if args.Location != nil {
		next.Location = strings.TrimSpace(*args.Location)
	}
	if args.Notes != nil {
		next.Notes = strings.TrimSpace(*args.Notes)
	}
	if args.Start != nil || args.End != nil {
		start, allDay := e.Start, e.AllDay
		length := int(e.End.Sub(e.Start).Minutes())
		if args.Start != nil {
			t, hasDay, clock, err := a.readTimeFull(*args.Start)
			if err != nil {
				return nil, fmt.Errorf("start: %v", err)
			}
			switch {
			case clock && !hasDay:
				start = at(e.Start, t.Hour(), t.Minute()) // a time alone: the same day
			case clock:
				start = t
			case allDay:
				start = day(t)
			default:
				start = at(t, e.Start.Hour(), e.Start.Minute()) // a day alone: the same time
			}
			if clock && allDay {
				allDay, length = false, 60
			}
		}
		next.AllDay = allDay
		if allDay {
			days := max(daysBetween(e.Start, e.End), 1)
			next.Start, next.End = day(start), addDays(day(start), days)
			if args.End != nil {
				t, _, _, err := a.readTimeFull(*args.End)
				if err != nil {
					return nil, fmt.Errorf("end: %v", err)
				}
				if day(t).Before(next.Start) {
					return nil, errors.New("end is before start")
				}
				next.End = addDays(day(t), 1)
			}
		} else {
			next.Start, next.End = start, addMinutes(start, max(length, 15))
			if args.End != nil {
				t, err := a.eventEnd(start, *args.End)
				if err != nil {
					return nil, err
				}
				next.End = t
			}
		}
	}
	if err := a.setEventLinks(&next, args.ProjectID, args.TagID); err != nil {
		return nil, err
	}
	a.agentChange(func() { *e = next }, change{kind: "event", id: e.ID, verb: "updated"})
	return map[string]any{"updated": a.eventOut(e)}, nil
}

// setEventTimes sets when an event starts and ends: at start, until end
// or for minutes, an hour by default; all day, over the days to end's.
func (a *app) setEventTimes(e *Event, start time.Time, clock bool, end string, minutes int) error {
	end = strings.TrimSpace(end)
	if e.AllDay {
		e.Start, e.End = day(start), addDays(day(start), 1)
		if end != "" {
			t, _, _, err := a.readTimeFull(end)
			if err != nil {
				return fmt.Errorf("end: %v", err)
			}
			if day(t).Before(day(start)) {
				return errors.New("end is before start")
			}
			e.End = addDays(day(t), 1)
		}
		return nil
	}
	if !clock {
		start = at(start, 9, 0)
	}
	e.Start = start
	switch {
	case end != "":
		t, err := a.eventEnd(start, end)
		if err != nil {
			return err
		}
		e.End = t
	case minutes > 0:
		e.End = addMinutes(start, minutes)
	default:
		e.End = addMinutes(start, 60)
	}
	return nil
}

// eventEnd reads when an event starting at start ends: a time alone ends
// the day it starts, or the next, as 22:00 to 1:00 does.
func (a *app) eventEnd(start time.Time, s string) (time.Time, error) {
	t, hasDay, clock, err := a.readTimeFull(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("end: %v", err)
	}
	if !clock {
		return time.Time{}, errors.New("end needs a time of day, as start has one")
	}
	if !hasDay {
		t = at(start, t.Hour(), t.Minute())
		if !t.After(start) && addDays(t, 1).Sub(start) <= 12*time.Hour {
			t = addDays(t, 1)
		}
	}
	if !t.After(start) {
		return time.Time{}, errors.New("end is before start")
	}
	return t, nil
}

func (a *app) setEventLinks(e *Event, project, tag *string) error {
	if project != nil {
		if *project == "" {
			e.Project = ""
		} else {
			p, err := a.findProject(*project)
			if err != nil {
				return err
			}
			e.Project = p.ID
		}
	}
	if tag != nil && *tag != "" {
		k := a.findKind(*tag)
		if k == nil {
			return fmt.Errorf("no event tag %q; list_projects lists them", *tag)
		}
		e.Kind = k.ID
	}
	return nil
}

func (a *app) toolCreateDirection(args struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}) (any, error) {
	d := &Direction{Title: strings.TrimSpace(args.Title), Summary: strings.TrimSpace(args.Summary)}
	if d.Title == "" {
		return nil, errors.New("title is empty")
	}
	if x := a.findDirection(d.Title); x != nil {
		return nil, fmt.Errorf("there is a direction %q already, with id %s", x.Title, x.ID)
	}
	d.ID = newID()
	a.agentChange(func() {
		d.Created = a.now()
		a.data.Directions = append(a.data.Directions, d)
	}, change{kind: "direction", id: d.ID, verb: "created"})
	return map[string]any{"created": map[string]string{"id": d.ID, "title": d.Title, "summary": d.Summary}}, nil
}

// Finding what the agent names, by ID or, as models do, by name.

func (a *app) findProject(ref string) (*Project, error) {
	if p := a.data.project(ref); p != nil {
		return p, nil
	}
	var found *Project
	for _, p := range a.data.Projects {
		if strings.EqualFold(strings.TrimSpace(p.Title), strings.TrimSpace(ref)) && (found == nil || found.Archived) {
			found = p
		}
	}
	if found == nil {
		return nil, fmt.Errorf("no project with id %q; find it with list_projects or search", ref)
	}
	return found, nil
}

func (a *app) findCategory(ref string) *Category {
	for _, k := range a.data.Categories {
		if string(k.ID) == ref || strings.EqualFold(k.Name, strings.TrimSpace(ref)) {
			return k
		}
	}
	return nil
}

func (a *app) findDirection(ref string) *Direction {
	for _, d := range a.data.Directions {
		if d.ID == ref || strings.EqualFold(d.Title, strings.TrimSpace(ref)) {
			return d
		}
	}
	return nil
}

func (a *app) findKind(ref string) *Kind {
	for _, k := range a.data.Kinds {
		if string(k.ID) == ref || strings.EqualFold(k.Name, strings.TrimSpace(ref)) {
			return k
		}
	}
	return nil
}

func readPriority(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	if i := slices.Index(priorityNames, s); i >= 0 {
		return i, nil
	}
	return 0, fmt.Errorf("unknown priority %q", s)
}

func readRepeat(s string) (Repeat, error) {
	if s == "" || s == "none" {
		return RepeatNone, nil
	}
	if r := Repeat(s); r.known() {
		return r, nil
	}
	return RepeatNone, fmt.Errorf("unknown repeat %q", s)
}

// readTime reads a local date, with a time of day or not: as 2026-03-02,
// 2026-03-02 15:00 or 2026-03-02T15:00:00+08:00, or as people say it, as
// tomorrow 3pm or 下周五下午. clock reports whether it has a time of day.
func (a *app) readTime(s string) (t time.Time, clock bool, err error) {
	t, _, clock, err = a.readTimeFull(s)
	return t, clock, err
}

// readTimeFull reads a time as readTime does, and whether it names a day:
// a time of day alone is at its next coming.
func (a *app) readTimeFull(s string) (t time.Time, hasDay, clock bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false, false, errors.New("no date given")
	}
	now := a.now()
	loc := now.Location()
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04Z07:00"} {
		if t, err := time.Parse(l, s); err == nil {
			return t.In(loc), true, true, nil
		}
	}
	for _, l := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006/01/02 15:04", "2006-01-02 Mon 15:04"} {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, true, true, nil
		}
	}
	for _, l := range []string{"2006-01-02", "2006/01/02", "2006-01-02 Mon", "2006.01.02"} {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, true, false, nil
		}
	}
	// As said: the parser of quick add reads it, behind a title it leaves.
	const mark = "§"
	w := parseWhen(mark+" "+s, now, a.weekFirst(), readClock)
	if w.title == mark && (!w.day.IsZero() || w.clock) {
		if !w.clock {
			return w.day, true, false, nil
		}
		return w.start(now), !w.day.IsZero(), true, nil
	}
	return time.Time{}, false, false, fmt.Errorf("cannot read %q as a date; write it as 2026-03-02 or 2026-03-02 15:00", s)
}

// weekFirst is the first day of the user's week, as last drawn.
func (a *app) weekFirst() time.Weekday {
	if a.first == nil {
		return time.Monday
	}
	return *a.first
}

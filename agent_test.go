package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"nexttodo/internal/mcp"
	"nexttodo/internal/pi"
	"nexttodo/locales"
)

func found(tt *ui.Tester, text string) bool {
	_, ok := tt.Find(text)
	return ok
}

// callTool calls a tool of the assistant's, as pi does over MCP, and reads
// its result.
func callTool(t *testing.T, a *app, name string, args any) (map[string]json.RawMessage, error) {
	t.Helper()
	var tool *mcp.Tool
	for _, x := range a.agentTools() {
		if x.Name == name {
			tool = &x
		}
	}
	if tool == nil {
		t.Fatalf("no tool %s", name)
	}
	raw, _ := json.Marshal(args)
	text, err := tool.Call(context.Background(), raw)
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s answered what is not JSON: %s", name, text)
	}
	return out, nil
}

func mustTool(t *testing.T, a *app, name string, args any) map[string]json.RawMessage {
	t.Helper()
	out, err := callTool(t, a, name, args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

func TestToolsDeclared(t *testing.T) {
	a := testApp("en")
	names := map[string]bool{}
	for _, tool := range a.agentTools() {
		if names[tool.Name] {
			t.Errorf("%s twice", tool.Name)
		}
		names[tool.Name] = true
		var schema map[string]any
		if err := json.Unmarshal(tool.Input, &schema); err != nil || schema["type"] != "object" {
			t.Errorf("%s: schema %s: %v", tool.Name, tool.Input, err)
		}
		if strings.HasPrefix(tool.Name, "delete") || strings.Contains(tool.Name, "remove") {
			t.Errorf("the assistant deletes nothing: %s", tool.Name)
		}
		if tool.ReadOnly != (strings.HasPrefix(tool.Name, "get_") || strings.HasPrefix(tool.Name, "list_") || tool.Name == "search") {
			t.Errorf("%s: read only %v", tool.Name, tool.ReadOnly)
		}
	}
	if len(names) < 14 {
		t.Errorf("tools: %v", names)
	}
}

// The closed loop: an idea becomes a project with its first tasks, which
// the overview shows, and Undo takes it all back.
func TestProjectFromIdeaAndUndo(t *testing.T) {
	a := testApp("zh-Hans")
	projects, tasks := len(a.data.Projects), len(a.data.Tasks)
	a.ag.chat.running = true
	out := mustTool(t, a, "create_project", map[string]any{
		"title":   "检索增强的文献综述",
		"summary": "用 RAG 帮助快速梳理一个方向的文献。",
		"status":  "active",
		"tasks": []map[string]any{
			{"title": "收集 30 篇核心论文", "due": "下周五"},
			{"title": "搭一个检索原型", "notes": "先用本地向量库"},
		},
	})
	var created projectOut
	json.Unmarshal(out["created"], &created)
	pr := a.data.project(created.ID)
	if pr == nil || pr.Title != "检索增强的文献综述" || pr.Status != StatusActive || created.Progress != "0/2" {
		t.Fatalf("project: %+v", created)
	}
	read := taskTitled(a, "收集 30 篇核心论文")
	if read == nil || read.Project != pr.ID || read.Due.Weekday() != time.Friday || daysBetween(monday, read.Due) != 11 {
		t.Fatalf("task: %+v", read)
	}
	mustTool(t, a, "add_journal_entry", map[string]any{"project_id": pr.Title, "text": "和导师聊过，方向可行。"})
	if len(pr.Journal) != 1 {
		t.Fatal("the journal takes the entry, the project named by its title")
	}

	a.finishRun()
	items := a.ag.chat.items
	if len(items) != 1 || items[0].kind != itemChanges || len(items[0].run.changes) != 1 || len(items[0].run.steps) != 2 {
		t.Fatalf("the run's changes: %+v", items)
	}
	// The overview shows it among the active projects.
	tt := newTester(t, a, "zh-Hans")
	if !found(tt, "检索增强的文献综述") {
		t.Fatal("the overview shows the new project")
	}

	// Work of the user's since stays.
	other := taskTitled(a, "搭一个检索原型")
	other.Title = "搭一个检索原型（已开始）"
	a.undoRun(items[0].run)
	if len(a.data.Projects) != projects || len(pr.Journal) != 1 && a.data.project(pr.ID) != nil {
		t.Fatalf("undo takes the project back: %d projects", len(a.data.Projects))
	}
	if len(a.data.Tasks) != tasks+1 || taskTitled(a, "搭一个检索原型（已开始）") == nil {
		t.Fatalf("a task changed since stays: %d tasks", len(a.data.Tasks))
	}
}

func TestCreateTaskTimes(t *testing.T) {
	a := testApp("zh-Hans")
	out := mustTool(t, a, "create_task", map[string]any{"title": "交报销", "due": "明天下午3点", "my_day": true, "priority": "high"})
	var got taskOut
	json.Unmarshal(out["created"], &got)
	task := a.data.task(got.ID)
	tomorrow := addDays(day(monday), 1)
	if task == nil || !task.Due.Equal(tomorrow) || !task.Remind.Equal(at(tomorrow, 15, 0)) || task.Priority != 3 || !task.inMyDay(monday) {
		t.Fatalf("task: %+v", task)
	}
	if got.Due != "2026-10-27 Tue" || got.Remind != "2026-10-27 Tue 15:00" {
		t.Fatalf("as the tool says it: %+v", got)
	}
	// ISO, and a reminder alone sets the day.
	mustTool(t, a, "create_task", map[string]any{"title": "Send the review", "remind": "2026-10-30 09:30"})
	r := taskTitled(a, "Send the review")
	if !r.Remind.Equal(time.Date(2026, 10, 30, 9, 30, 0, 0, time.Local)) || !sameDay(r.Due, r.Remind) {
		t.Fatalf("reminder: %+v", r)
	}
	// Mistakes say why, and change nothing.
	n := len(a.data.Tasks)
	for _, args := range []map[string]any{
		{"title": ""},
		{"title": "x", "due": "someday maybe"},
		{"title": "x", "project_id": "nope"},
		{"title": "x", "priority": "urgent"},
		{"title": "x", "repeat": "hourly"},
	} {
		if _, err := callTool(t, a, "create_task", args); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
	if len(a.data.Tasks) != n || len(a.undo) != 2 {
		t.Fatalf("mistakes changed something: %d tasks, %d steps", len(a.data.Tasks), len(a.undo))
	}
}

func TestUpdateTask(t *testing.T) {
	a := testApp("en")
	mustTool(t, a, "create_task", map[string]any{"title": "Water the plants", "due": "today", "repeat": "weekly"})
	task := taskTitled(a, "Water the plants")
	out := mustTool(t, a, "update_task", map[string]any{"id": task.ID, "done": true})
	if !task.Done || out["next"] == nil {
		t.Fatalf("done, a repeating task comes again: %s", out["next"])
	}
	mustTool(t, a, "update_task", map[string]any{"id": task.ID, "title": "Water the ferns", "due": "", "add_steps": []string{"Fill the can"}})
	if task.Title != "Water the ferns" || !task.Due.IsZero() || len(task.Steps) != 1 {
		t.Fatalf("changed: %+v", task)
	}
	if _, err := callTool(t, a, "update_task", map[string]any{"id": "nope"}); err == nil || !strings.Contains(err.Error(), "search") {
		t.Fatalf("an unknown task says how to find one: %v", err)
	}
	before := *task
	if _, err := callTool(t, a, "update_task", map[string]any{"id": task.ID, "title": "Half", "due": "never"}); err == nil {
		t.Fatal("a bad date is an error")
	}
	if task.Title != before.Title {
		t.Fatal("an update that fails changes nothing")
	}
}

func TestEvents(t *testing.T) {
	a := testApp("en")
	out := mustTool(t, a, "create_event", map[string]any{"title": "Lab sync", "start": "2026-10-28 14:00", "end": "15:30", "location": "Room 4"})
	var ev eventOut
	json.Unmarshal(out["created"], &ev)
	e := a.data.event(ev.ID)
	if e == nil || !e.Start.Equal(time.Date(2026, 10, 28, 14, 0, 0, 0, time.Local)) || !e.End.Equal(time.Date(2026, 10, 28, 15, 30, 0, 0, time.Local)) {
		t.Fatalf("event: %+v", e)
	}
	mustTool(t, a, "create_event", map[string]any{"title": "Conference", "start": "2026-11-02", "end": "2026-11-04"})
	conf := eventTitled(a, "Conference")
	if !conf.AllDay || daysBetween(conf.Start, conf.End) != 3 {
		t.Fatalf("a date alone is all day, to its end's: %+v", conf)
	}
	mustTool(t, a, "create_event", map[string]any{"title": "Late run", "start": "2026-10-28 22:00", "end": "1:00 am"})
	if late := eventTitled(a, "Late run"); late.End.Sub(late.Start) != 3*time.Hour {
		t.Fatalf("over midnight: %v to %v", late.Start, late.End)
	}
	mustTool(t, a, "create_event", map[string]any{"title": "Seminar", "start": "2026-10-29 16:00", "duration_minutes": 90, "repeat": "weekly", "tag_id": "Seminar"})
	sem := eventTitled(a, "Seminar")
	if sem.End.Sub(sem.Start) != 90*time.Minute || sem.Repeat != RepeatWeekly {
		t.Fatalf("seminar: %+v", sem)
	}
	// Moving keeps the length; a repeating event is left to the app.
	mustTool(t, a, "update_event", map[string]any{"id": e.ID, "start": "2026-10-29 10:00"})
	if !e.Start.Equal(time.Date(2026, 10, 29, 10, 0, 0, 0, time.Local)) || e.End.Sub(e.Start) != 90*time.Minute {
		t.Fatalf("moved: %v to %v", e.Start, e.End)
	}
	mustTool(t, a, "update_event", map[string]any{"id": e.ID, "start": "2026-10-30"})
	if e.Start.Hour() != 10 || e.Start.Day() != 30 {
		t.Fatalf("a new day keeps the time: %v", e.Start)
	}
	if _, err := callTool(t, a, "update_event", map[string]any{"id": sem.ID, "title": "x"}); err == nil {
		t.Fatal("a repeating event is not changed here")
	}
	if _, err := callTool(t, a, "create_event", map[string]any{"title": "Bad", "start": "2026-10-28 14:00", "end": "2026-10-28 13:00"}); err == nil {
		t.Fatal("an end before the start")
	}
	// The week lists them, the occurrences of the repeating with them.
	out = mustTool(t, a, "list_events", map[string]any{"from": "2026-10-26", "to": "2026-11-08"})
	var evs []eventOut
	json.Unmarshal(out["events"], &evs)
	seminars := 0
	for _, x := range evs {
		if x.Title == "Seminar" {
			seminars++
			if !strings.Contains(x.ID, "@") {
				t.Errorf("an occurrence names its start: %s", x.ID)
			}
		}
	}
	if seminars != 2 {
		t.Fatalf("two seminars in two weeks, got %d", seminars)
	}
}

func TestReadTools(t *testing.T) {
	a := testApp("en")
	out := mustTool(t, a, "get_overview", map[string]any{})
	if !strings.Contains(string(out["now"]), "2026-10-26 Mon 10:20") || out["active_projects"] == nil {
		t.Fatalf("overview: %v", out)
	}
	pr := a.data.Projects[0]
	pr.Journal = append(pr.Journal, Entry{ID: "j", Date: monday, Text: "Reviewer 2 wants an ablation on noise"})
	out = mustTool(t, a, "search", map[string]any{"query": "ablation NOISE"})
	if !strings.Contains(string(out["journal"]), pr.ID) {
		t.Fatalf("search finds the journal, by every word: %s", out["journal"])
	}
	out = mustTool(t, a, "search", map[string]any{"query": "zzzz"})
	if string(out["found"]) != `"nothing"` {
		t.Fatalf("nothing: %v", out)
	}
	out = mustTool(t, a, "get_project", map[string]any{"id": pr.ID})
	if !strings.Contains(string(out["journal"]), "ablation") {
		t.Fatal("the project's journal")
	}
	out = mustTool(t, a, "list_projects", map[string]any{})
	if !strings.Contains(string(out["categories"]), `"id"`) || !strings.Contains(string(out["event_tags"]), "Seminar") {
		t.Fatalf("projects list their categories and tags: %s", out["event_tags"])
	}
	out = mustTool(t, a, "list_tasks", map[string]any{"filter": "today"})
	var ts []taskOut
	json.Unmarshal(out["tasks"], &ts)
	if len(ts) == 0 {
		t.Fatal("today's tasks")
	}
	if _, err := callTool(t, a, "list_tasks", map[string]any{"filter": "soon"}); err == nil {
		t.Fatal("an unknown filter")
	}
}

func TestReadTime(t *testing.T) {
	a := testApp("en")
	for _, c := range []struct {
		in            string
		want          time.Time
		hasDay, clock bool
	}{
		{"2026-11-02", time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local), true, false},
		{"2026-11-02 14:30", time.Date(2026, 11, 2, 14, 30, 0, 0, time.Local), true, true},
		{"2026-11-02T14:30:00", time.Date(2026, 11, 2, 14, 30, 0, 0, time.Local), true, true},
		{"tomorrow", day(addDays(monday, 1)), true, false},
		{"tomorrow 3pm", at(addDays(monday, 1), 15, 0), true, true},
		{"下周五", day(addDays(monday, 11)), true, false},
		{"15:00", at(monday, 15, 0), false, true},
	} {
		got, hasDay, clock, err := a.readTimeFull(c.in)
		if err != nil || !got.Equal(c.want) || hasDay != c.hasDay || clock != c.clock {
			t.Errorf("%q: %v day %v clock %v, %v; want %v", c.in, got, hasDay, clock, err, c.want)
		}
	}
	utc, _, _, err := a.readTimeFull("2026-11-02T06:30:00Z")
	if err != nil || !utc.Equal(time.Date(2026, 11, 2, 6, 30, 0, 0, time.UTC)) || utc.Location() != time.Local {
		t.Errorf("RFC 3339, in local time: %v %v", utc, err)
	}
	for _, bad := range []string{"", "someday", "the 45th"} {
		if _, _, _, err := a.readTimeFull(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
}

// What pi says becomes the rows of the conversation.
func TestChatFromEvents(t *testing.T) {
	a := testApp("en")
	feed := func(lines ...string) {
		for _, l := range lines {
			var head struct{ Type string }
			json.Unmarshal([]byte(l), &head)
			a.piEvent(pi.Event{Type: head.Type, Raw: json.RawMessage(l)})
		}
	}
	a.ag.chat.items = append(a.ag.chat.items, a.newItem(itemUser, "Plan my week"))
	a.ag.chat.running = true
	feed(
		`{"type":"agent_start"}`,
		`{"type":"message_start","message":{"role":"user","content":"<now>2026-10-26 Mon 10:20 UTC+08:00</now>\nPlan my week"}}`,
		`{"type":"message_start","message":{"role":"assistant","content":[],"stopReason":"pending"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"Look at the "}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"overview."}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Look at the overview."},{"type":"toolCall","id":"c1","name":"mcp__nexttodo__get_overview","arguments":{}}],"stopReason":"toolUse"}}`,
		`{"type":"tool_execution_start","toolCallId":"c1","toolName":"mcp__nexttodo__get_overview","args":{}}`,
	)
	items := a.ag.chat.items
	if len(items) != 3 || items[1].kind != itemAssistant || items[1].thinking != "Look at the overview." || items[2].kind != itemTool || items[2].done {
		t.Fatalf("rows: %+v", items)
	}
	feed(
		`{"type":"tool_execution_end","toolCallId":"c1","toolName":"mcp__nexttodo__get_overview","result":{"content":[{"type":"text","text":"{}"}]},"isError":false}`,
		`{"type":"message_start","message":{"role":"assistant","content":[],"stopReason":"pending"}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"You have "}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"two deadlines."}}`,
	)
	items = a.ag.chat.items
	last := items[len(items)-1]
	if !items[2].done || last.text != "You have two deadlines." || !last.streaming {
		t.Fatalf("streaming: %+v", last)
	}
	tt := newTester(t, a, "en")
	a.navigate(PageAssistant)
	tt.Frame()
	if !found(tt, "Looked at your day") {
		t.Fatal("the tool shows as what it did")
	}
	feed(
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"You have two deadlines this week."}],"stopReason":"stop"}}`,
		`{"type":"agent_end"}`,
		`{"type":"agent_settled","aborted":false}`,
	)
	if a.ag.chat.running || last.streaming || last.text != "You have two deadlines this week." {
		t.Fatalf("settled: running %v, %+v", a.ag.chat.running, last)
	}
	// An error and a stop say so.
	a.ag.chat.running = true
	feed(
		`{"type":"message_start","message":{"role":"assistant","content":[]}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"429 rate limited"}}`,
		`{"type":"agent_settled","aborted":true}`,
	)
	items = a.ag.chat.items
	if items[len(items)-2].kind != itemError || items[len(items)-2].text != "429 rate limited" || items[len(items)-1].kind != itemNote {
		t.Fatalf("error rows: %+v %+v", items[len(items)-2], items[len(items)-1])
	}
}

func TestItemsOfSession(t *testing.T) {
	a := testApp("en")
	var msgs []pi.Message
	json.Unmarshal([]byte(`[
		{"role":"user","content":"<now>2026-10-26 Mon 10:20 UTC+08:00</now>\nAdd a task to call Mei"},
		{"role":"assistant","content":[{"type":"text","text":"Adding it."},{"type":"toolCall","id":"c1","name":"mcp__nexttodo__create_task","arguments":{"title":"Call Mei"}}],"stopReason":"toolUse"},
		{"role":"toolResult","toolCallId":"c1","toolName":"mcp__nexttodo__create_task","content":[{"type":"text","text":"{\"created\":{}}"}],"isError":false},
		{"role":"assistant","content":[{"type":"text","text":"Done."}],"stopReason":"stop"}
	]`), &msgs)
	a.tr = locales.In("en")
	items := a.itemsOf(msgs)
	if len(items) != 4 || items[0].text != "Add a task to call Mei" || items[2].kind != itemTool || items[2].result == "" || !items[2].done {
		t.Fatalf("items: %+v", items)
	}
	if _, label := a.toolLabel(items[2]); label != "Add task “Call Mei”" {
		t.Fatalf("label: %q", label)
	}
}

func TestToolLabels(t *testing.T) {
	a := testApp("en")
	a.tr = locales.In("en")
	for _, c := range []struct {
		tool, args, want string
	}{
		{"codemode", `{"code":"const r = await tools.mcp__magpie__firecrawl__firecrawl_search({query: \"RAG survey 2026\"}); return r"}`, "Searched the web for “RAG survey 2026”"},
		{"codemode", `{"code":"return await tools.mcp__magpie__deepxiv__search_papers({query: 'diffusion policy'})"}`, "Searched papers for “diffusion policy”"},
		{"codemode", `{"code":"return 1+1"}`, "Ran a script"},
		{"mcp__other__lookup", `{}`, "Used lookup"},
		{"mcp__nexttodo__update_task", `{"id":"x","done":true}`, "Complete a task"},
	} {
		if _, got := a.toolLabel(&chatItem{tool: c.tool, args: json.RawMessage(c.args)}); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.tool, c.args, got, c.want)
		}
	}
}

func TestNowTag(t *testing.T) {
	a := testApp("en")
	tag := a.nowTag()
	if !strings.HasPrefix(tag, "<now>2026-10-26 Mon 10:20 UTC") || stripNow(tag+"Hello") != "Hello" {
		t.Fatalf("tag %q", tag)
	}
	if stripNow("No tag <now>x</now>") != "No tag <now>x</now>" {
		t.Fatal("only a leading tag goes")
	}
}

func TestTranslateHelpers(t *testing.T) {
	text, notes := splitNotes("Retrieval-augmented generation\n\n<notes>\n- 检索增强生成 → RAG\n</notes>")
	if text != "Retrieval-augmented generation" || notes != "- 检索增强生成 → RAG" {
		t.Fatalf("split: %q %q", text, notes)
	}
	if text, _ := splitNotes("Half a translation <no"); text != "Half a translation" {
		t.Fatalf("a tag being written is not shown: %q", text)
	}
	if mainScript("检索增强生成 (RAG) 是一种方法") != unicode.Han || mainScript("Hello 世界, hello") != unicode.Latin || mainScript("こんにちは世界") != unicode.Hiragana {
		t.Fatal("scripts")
	}
	a := testApp("zh-Hans")
	a.lang = "zh-Hans"
	if a.translateTarget("Attention is all you need") != "Simplified Chinese" || a.translateTarget("注意力就是一切") != "English" {
		t.Fatal("automatic: into Chinese, Chinese into English")
	}
	a.data.Prefs.TranslateTo = "Japanese"
	if a.translateTarget("注意力") != "Japanese" {
		t.Fatal("as chosen")
	}
	a.lang = "en"
	a.data.Prefs.TranslateTo = ""
	if a.translateTarget("Hello") != "Simplified Chinese" || a.translateTarget("Bonjour à tous") != "Simplified Chinese" || a.translateTarget("你好") != "English" {
		t.Fatal("automatic in English")
	}
}

func TestTranslatorKeepsFinalText(t *testing.T) {
	a := testApp("en")
	a.xl.busy = true
	for _, l := range []string{
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"The translation given up"}],"stopReason":"aborted"}}`,
		`{"type":"agent_settled","aborted":true}`,
		`{"type":"agent_start"}`,
		`{"type":"message_start","message":{"role":"assistant","content":[]}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Let me check a term."}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Let me check a term."},{"type":"toolCall","id":"c","name":"codemode","arguments":{}}],"stopReason":"toolUse"}}`,
		`{"type":"tool_execution_start","toolCallId":"c","toolName":"codemode","args":{"code":"return tools.mcp__magpie__firecrawl__firecrawl_search({query:'检索增强生成 英文'})"}}`,
	} {
		var head struct{ Type string }
		json.Unmarshal([]byte(l), &head)
		a.translatorEvent(pi.Event{Type: head.Type, Raw: json.RawMessage(l)})
	}
	if a.xl.status == "" {
		t.Fatal("looking a term up shows")
	}
	for _, l := range []string{
		`{"type":"tool_execution_end","toolCallId":"c","toolName":"codemode","result":{"content":[]}}`,
		`{"type":"message_start","message":{"role":"assistant","content":[]}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Retrieval-augmented"}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Retrieval-augmented generation"}],"stopReason":"stop"}}`,
		`{"type":"agent_settled"}`,
	} {
		var head struct{ Type string }
		json.Unmarshal([]byte(l), &head)
		a.translatorEvent(pi.Event{Type: head.Type, Raw: json.RawMessage(l)})
	}
	if a.xl.raw != "Retrieval-augmented generation" || a.xl.busy {
		t.Fatalf("the translation is the last message alone: %q", a.xl.raw)
	}
}

// The new pages draw, in both languages, with nothing running.
func TestAssistantPagesDraw(t *testing.T) {
	for _, lang := range []string{"en", "zh-Hans"} {
		a := testApp(lang)
		tt := newTester(t, a, lang)
		a.navigate(PageAssistant)
		tt.Frame()
		hello := "What can I help with?"
		if lang != "en" {
			hello = "有什么可以帮你？"
		}
		if !found(tt, hello) {
			t.Errorf("%s: the assistant's welcome", lang)
		}
		a.navigate(PageTranslate)
		tt.Frame()
		a.xl.src = "Attention is all you need"
		tt.Frame()
		a.navigate(PageOverview)
		tt.Frame()
	}
}

// scriptChat plays a conversation in which the assistant turns an idea
// into a project, as pi would: the tools really run.
func scriptChat(t *testing.T, a *app, done bool) {
	zh := strings.HasPrefix(a.lang, "zh") || a.data.Categories[0].Name == "纵向课题"
	ask, title, task1, task2 := "I have an idea: use LLMs to write structured abstracts of papers. Make it a project with two tasks, one due tomorrow 3 pm.",
		"Structured abstracts with LLMs", "Survey 10 papers on structured summaries", "Build a baseline on 20 abstracts"
	reply := "Done: **Structured abstracts with LLMs** is set up under *Independent research*, with two tasks:\n\n- Survey 10 papers on structured summaries, due **tomorrow 3 PM**, with a reminder\n- Build a baseline on 20 abstracts\n\nWant me to look for existing work, such as datasets of structured abstracts, and note it in the journal?"
	if zh {
		ask, title, task1, task2 = "我有个想法：用大语言模型自动给论文写结构化摘要。把它建成项目，加两条待办，其中一条明天下午三点前完成。",
			"用大语言模型生成论文结构化摘要", "梳理 10 篇结构化摘要的相关工作", "在 20 篇摘要上搭一个基线"
		reply = "项目已经建好：**用大语言模型生成论文结构化摘要**，归在「自主研究」下。两条待办：\n\n- 梳理 10 篇结构化摘要的相关工作，**明天 15:00** 前完成，已设提醒\n- 在 20 篇摘要上搭一个基线\n\n要不要我先搜一下这个方向已有的工作，比如结构化摘要的数据集，记进项目日志？"
	}
	feed := func(lines ...string) {
		for _, l := range lines {
			var head struct{ Type string }
			json.Unmarshal([]byte(l), &head)
			a.piEvent(pi.Event{Type: head.Type, Raw: json.RawMessage(l)})
		}
	}
	js := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	a.ag.state.Model = &pi.Model{ID: "deepseek-flash", Name: "DeepSeek-V4.1-Flash · DeepSeek", Provider: "magpie"}
	a.ag.state.ThinkingLevel = "high"
	a.ag.levels = []string{"off", "low", "medium", "high"}
	a.ag.chat.items = append(a.ag.chat.items, a.newItem(itemUser, ask))
	a.ag.chat.running, a.ag.chat.since = true, a.now()
	feed(`{"type":"agent_start"}`,
		`{"type":"message_start","message":{"role":"assistant","content":[]}}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"The user wants a project with two tasks. Check the categories first."}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"thinking","thinking":"The user wants a project with two tasks. Check the categories first."}],"stopReason":"toolUse"}}`,
		`{"type":"tool_execution_start","toolCallId":"c1","toolName":"mcp__nexttodo__list_projects","args":{}}`)
	out, _ := json.Marshal(mustTool(t, a, "list_projects", map[string]any{}))
	feed(`{"type":"tool_execution_end","toolCallId":"c1","toolName":"mcp__nexttodo__list_projects","result":{"content":[{"type":"text","text":` + js(string(out)) + `}]}}`)
	args := map[string]any{"title": title, "category_id": "independent", "summary": "…", "tasks": []map[string]any{{"title": task1, "due": "tomorrow 3pm"}, {"title": task2}}}
	feed(`{"type":"tool_execution_start","toolCallId":"c2","toolName":"mcp__nexttodo__create_project","args":` + js(args) + `}`)
	if !done {
		feed(`{"type":"message_start","message":{"role":"assistant","content":[]}}`)
		return
	}
	out, _ = json.Marshal(mustTool(t, a, "create_project", args))
	a.ag.chat.items[len(a.ag.chat.items)-1].took = 1200 * time.Millisecond
	feed(`{"type":"tool_execution_end","toolCallId":"c2","toolName":"mcp__nexttodo__create_project","result":{"content":[{"type":"text","text":`+js(string(out))+`}]}}`,
		`{"type":"message_start","message":{"role":"assistant","content":[]}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":`+js(reply)+`}],"stopReason":"stop"}}`,
		`{"type":"agent_settled","aborted":false}`)
	for _, it := range a.ag.chat.items {
		if it.kind == itemAssistant && it.thinking != "" {
			it.thought = 3 * time.Second
		}
	}
}

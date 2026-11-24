package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"nexttodo/internal/pi"
)

// The assistant's page: the conversations on the left, the one shown on
// the right, its messages over the field that writes the next.

// monoFont is the font of code and of what tools are given.
const monoFont = "SF Mono, Menlo, monospace"

var (
	iconSpark = icon(`<path d="M8 1.8l1.3 3.6c.2.6.7 1.1 1.3 1.3L14.2 8l-3.6 1.3c-.6.2-1.1.7-1.3 1.3L8 14.2l-1.3-3.6c-.2-.6-.7-1.1-1.3-1.3L1.8 8l3.6-1.3c.6-.2 1.1-.7 1.3-1.3z"/>`)
	iconSend  = icon(`<path d="M8 13V3.4M4 7.2L8 3.2l4 4"/>`)
	iconStop  = filled(`<rect x="4.5" y="4.5" width="7" height="7" rx="1.4"/>`)
	iconChat  = icon(`<path d="M3 3.5h10c.6 0 1 .4 1 1v6c0 .6-.4 1-1 1H8.2L5 14v-2.5H3c-.6 0-1-.4-1-1v-6c0-.6.4-1 1-1z"/>`)
	iconWarn  = icon(`<path d="M8 2.4l6 10.8H2zM8 6.6v3M8 11.4v.2"/>`)
	iconTool  = icon(`<path d="M10.2 2.6a3 3 0 0 0-3.4 4L2.6 10.8a1.3 1.3 0 0 0 1.8 1.8l4.2-4.2a3 3 0 0 0 4-3.4L10.8 6.8 9.4 6.6 9.2 5.2z"/>`)
	iconCopy  = icon(`<rect x="5.2" y="5.2" width="8" height="8.6" rx="1.6"/><path d="M10.8 5.2V3.8c0-.9-.7-1.6-1.6-1.6H4c-.9 0-1.6.7-1.6 1.6v5.4c0 .9.7 1.6 1.6 1.6h1.2"/>`)
	iconUndo  = icon(`<path d="M5.4 3.6L2.6 6.4l2.8 2.8M2.8 6.4h6.6a3.6 3.6 0 0 1 0 7.2H7"/>`)
	iconSwap  = icon(`<path d="M3 5.4h9.6M10.2 3l2.4 2.4-2.4 2.4M13 10.6H3.4M5.8 8.2l-2.4 2.4 2.4 2.4"/>`)
	iconTrans = icon(`<path d="M2.4 3.6h6.4M5.6 2.2v1.4M4 3.6c.4 2.3 1.9 4.2 4 5.2M7.4 3.6c-.5 2.6-2.3 4.7-4.8 5.6M8.8 13.8l2.6-6.4 2.6 6.4M9.6 12h3.6"/>`)
)

func (a *app) assistantPage(c *ui.Context) {
	if !a.ag.listed {
		a.ag.listed = true
		a.listSessions()
	}
	if a.ag.err == "" && !a.ag.missing {
		a.startAgent() // warm, so that the first message goes at once; not again once it failed
	}
	ui.Row(c).Grow(1).MinHeight(0).Padding(0, 14, 14, 10).Gap(10).AlignItems(ui.Stretch).Children(func() {
		a.agentSidebar(c)
		a.card(c).Grow(1).MinWidth(0).Children(func() { a.agentMain(c) })
	})
}

// The conversations.

func (a *app) agentSidebar(c *ui.Context) {
	p := &a.p
	ui.Column(c).Width(224).Shrink(0).Gap(1).Children(func() {
		b := ui.ButtonBase(c.Key("new")).Height(30).Padding(0, 10).Gap(8).Radius(8).AlignItems(ui.Center).
			Margin(0, 0, 8, 0).Background(p.card).Border(1, p.line).Label(a.tr.AgentNewChat())
		if b.Hovered() {
			b.Background(p.card2)
		}
		b.Children(func() {
			ui.Icon(c, iconEdit).Size(13, 13).TextColor(p.fg2)
			ui.Text(c, a.tr.AgentNewChat()).FontSize(12.5).FontWeight(500).TextColor(p.fg).Grow(1)
		})
		if b.Clicked() {
			a.newChat()
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				today := day(a.now())
				group := -1
				for _, s := range a.ag.sessions {
					g := 3
					switch n := daysBetween(s.Updated, today); {
					case n <= 0:
						g = 0
					case n == 1:
						g = 1
					case n < 7:
						g = 2
					}
					if g != group {
						group = g
						names := []string{a.tr.AgentToday(), a.tr.AgentYesterday(), a.tr.AgentThisWeek(), a.tr.AgentEarlier()}
						a.label(c, names[g]).Padding(12, 10, 4)
					}
					a.sessionRow(c, s)
				}
			})
		})
	})
}

func (a *app) sessionRow(c *ui.Context, s pi.Session) {
	p := &a.p
	c = c.Key(s.Path)
	if a.ag.renaming == s.Path {
		in := ui.TextInput(c, &a.ag.rename).AutoFocus().Height(28).FontSize(12.5)
		done := func() {
			a.renameSession(s.Path, a.ag.rename)
			a.ag.renaming = ""
		}
		in.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEscape {
				a.ag.renaming = ""
				a.redraw()
				return true
			}
			return false
		})
		switch focused := in.Focused(); {
		case in.Submitted():
			done()
		case a.ag.renameFocus && !focused:
			done() // clicked away
		default:
			a.ag.renameFocus = focused
		}
		return
	}
	on := s.Path == a.ag.chat.path
	b := ui.ButtonBase(c).Height(30).Padding(0, 10).Radius(7).AlignItems(ui.Center).Label(a.sessionTitle(s))
	switch {
	case on:
		b.Background(p.card).Shadow(0, 1, 2, 0, p.shadow.Alpha(0.08))
	case b.Hovered():
		b.Background(p.pill)
	}
	b.Children(func() {
		t := ui.Text(c, a.sessionTitle(s)).FontSize(12.5).SingleLine().Grow(1).MinWidth(0).TextColor(p.fg2)
		if on {
			t.TextColor(p.fg).FontWeight(500)
		}
	})
	if clicked(b) {
		a.openSession(s.Path)
	}
	b.ContextMenu(func(m *ui.Menu) {
		if m.Item(a.tr.AgentRename()).Chosen() {
			a.ag.renaming, a.ag.rename = s.Path, a.sessionTitle(s)
		}
		if m.Item(a.tr.ShowInFinder()).Chosen() {
			exec.Command("open", "-R", s.Path).Start()
		}
		m.Separator()
		if m.Item(a.tr.Delete()).Chosen() {
			if err := a.deleteSession(s.Path); err != nil {
				c.Toast(err.Error())
			}
		}
	})
}

// The conversation shown.

func (a *app) agentMain(c *ui.Context) {
	ch := &a.ag.chat
	if a.ag.client == nil && !a.ag.starting && a.ag.err != "" && len(ch.items) == 0 {
		a.agentSetup(c)
		return
	}
	a.ag.md.ShareSelection = false
	if a.ag.md.Theme == nil {
		a.ag.md.Theme = &markdown.Theme{FontSize: 13.5}
	}
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		switch {
		case ch.loading:
			ui.Column(c).Grow(1).Center().Children(func() { ui.Spinner(c).Label(a.tr.AgentLoading()) })
		case len(ch.items) == 0:
			a.agentWelcome(c)
		default:
			a.transcript(c)
		}
		a.composer(c)
	})
}

// transcript lists the conversation, following its end as it grows.
func (a *app) transcript(c *ui.Context) {
	ch := &a.ag.chat
	items := ch.items
	n := len(items) + 1 // and what goes on at the end
	a.ag.list.Key = func(i int) any {
		if i == len(items) {
			return "tail"
		}
		return items[i].id
	}
	a.ag.list.FollowEnd = true
	a.ag.list.FollowAnimation = &ui.Smooth
	ui.List(c, &a.ag.list, n, func(i int) {
		ui.Column(c).Padding(0, 28).MaxWidth(780).Margin(0, ui.Auto).FillWidth().Children(func() {
			if i == len(items) {
				a.transcriptTail(c)
				return
			}
			a.chatRow(c, items[i], i)
		})
	}).Grow(1).Padding(22, 0, 12).Gap(14)
}

// chatRow shows a row of the conversation.
func (a *app) chatRow(c *ui.Context, it *chatItem, i int) {
	p := &a.p
	switch it.kind {
	case itemUser:
		queued := a.ag.chat.running && containsText(a.ag.chat.queued, it.text)
		ui.Column(c).AlignItems(ui.End).Gap(3).Margin(6, 0, 0, 0).Children(func() {
			ui.Text(c, it.text).FontSize(13.5).LineHeight(1.5).TextColor(p.fg).Selectable().
				Padding(8, 13).Radius(14).Background(p.pill).MaxWidth(560)
			if queued {
				ui.Text(c, a.tr.AgentQueued()).FontSize(11).TextColor(p.faint)
			}
		})
	case itemAssistant:
		ui.Column(c).Gap(6).Children(func() {
			if it.thinking != "" {
				a.thinkingRow(c, it)
			}
			if it.text != "" {
				if it.streaming {
					markdown.Stream(c.Key(it.id), it.text, &a.ag.md)
				} else {
					markdown.View(c.Key(it.id), it.text, &a.ag.md)
				}
			}
			if !it.streaming && it.text != "" && a.lastReply(i) {
				a.replyActions(c, it)
			}
		})
	case itemTool:
		a.toolRow(c, it)
	case itemChanges:
		a.changesCard(c, it.run)
	case itemError:
		ui.Row(c).Padding(9, 12).Gap(9).Radius(10).Background(p.redSoft).AlignItems(ui.Start).Children(func() {
			ui.Icon(c, iconWarn).Size(14, 14).TextColor(p.red).Margin(1, 0, 0, 0)
			ui.Text(c, it.text).FontSize(12.5).TextColor(p.fg2).Selectable().Grow(1).MinWidth(0)
		})
	case itemNote:
		ui.Text(c, it.text).FontSize(11.5).TextColor(p.faint).TextAlign(ui.Center).AlignSelf(ui.Stretch)
	}
}

// lastReply reports whether the row i is the assistant's last words of
// the conversation, which offer to copy them.
func (a *app) lastReply(i int) bool {
	items := a.ag.chat.items
	for j := len(items) - 1; j > i; j-- {
		if items[j].kind == itemAssistant || items[j].kind == itemUser {
			return false
		}
	}
	return !a.ag.chat.running
}

func (a *app) replyActions(c *ui.Context, it *chatItem) {
	ui.Row(c).Gap(2).Margin(-4, 0, 0, -6).Children(func() {
		if a.iconBtn(c, iconCopy, a.tr.Copy()).Clicked() {
			mygo.Clipboard.WriteText(it.text)
			c.Toast(a.tr.Copied())
		}
	})
}

// thinkingRow is the model's thinking, folded: how long it thought,
// opened to read it.
func (a *app) thinkingRow(c *ui.Context, it *chatItem) {
	p := &a.p
	live := it.streaming && it.text == ""
	b := ui.ButtonBase(c.Key("think")).Gap(6).AlignItems(ui.Center).Padding(2, 6).Margin(0, -6).Radius(6).AlignSelf(ui.Start)
	if b.Hovered() {
		b.Background(p.pill)
	}
	b.Children(func() {
		ui.Icon(c, iconBulb).Size(13, 13).TextColor(p.muted)
		label := a.tr.AgentThinking()
		if !live {
			label = a.tr.AgentThought(max(int(it.thought.Seconds()+0.5), 1))
		}
		ui.Text(c, label).FontSize(12).TextColor(p.muted)
		chev := iconChevronRight
		if it.showThink {
			chev = iconChevronDown
		}
		ui.Icon(c, chev).Size(10, 10).TextColor(p.faint)
	})
	if b.Clicked() {
		it.showThink = !it.showThink
	}
	if it.showThink {
		ui.Text(c, strings.TrimSpace(it.thinking)).FontSize(12).LineHeight(1.5).TextColor(p.muted).Selectable().
			Padding(0, 0, 0, 12).BorderWidth(0, 0, 0, 2).BorderColor(p.line)
	}
}

// toolRow is a tool the assistant called: what it did, in a line, opened
// to read what it was given and what it answered.
func (a *app) toolRow(c *ui.Context, it *chatItem) {
	p := &a.p
	ic, label := a.toolLabel(it)
	ui.Column(c).Gap(6).Children(func() {
		b := ui.ButtonBase(c.Key("tool")).Gap(8).AlignItems(ui.Center).Padding(3, 6).Margin(0, -6).Radius(6).AlignSelf(ui.Start).MaxWidth(640)
		if b.Hovered() {
			b.Background(p.pill)
		}
		b.Children(func() {
			switch {
			case !it.done:
				ui.Spinner(c).Size(13, 13)
			case it.failed:
				ui.Icon(c, iconWarn).Size(13, 13).TextColor(p.amber)
			default:
				ui.Icon(c, ic).Size(13, 13).TextColor(p.muted)
			}
			ui.Text(c, label).FontSize(12.5).TextColor(p.fg2).SingleLine().MinWidth(0)
			if it.done && it.took >= time.Second {
				ui.Text(c, a.tr.AgentSeconds(int(it.took.Seconds()+0.5))).FontSize(11.5).TextColor(p.faint).Shrink(0)
			}
		})
		if b.Clicked() {
			it.open = !it.open
		}
		if it.open {
			ui.Column(c).Padding(10, 12).Gap(8).Radius(9).Background(p.card2).Border(1, p.line2).Children(func() {
				if s := prettyJSON(it.args, 2000); s != "" && s != "{}" {
					ui.Text(c, s).FontSize(11.5).Font(monoFont).TextColor(p.fg2).Selectable()
				}
				if it.result != "" {
					if len(it.args) > 2 {
						ui.Box(c).Height(1).Background(p.line2)
					}
					col := p.fg2
					if it.failed {
						col = p.red
					}
					ui.Text(c, clip(prettyText(it.result), 3000)).FontSize(11.5).Font(monoFont).TextColor(col).Selectable()
				}
			})
		}
	})
}

// transcriptTail is what goes on at the end: the assistant at work, and a
// question of an extension's.
func (a *app) transcriptTail(c *ui.Context) {
	p := &a.p
	ch := &a.ag.chat
	if req := a.ag.ask; req != nil {
		a.askCard(c, req)
	}
	if !ch.running || a.ag.ask != nil {
		ui.Box(c).Height(4)
		return
	}
	// Working, while nothing else shows it: no text streaming, no tool
	// turning.
	if n := len(ch.items); n > 0 {
		last := ch.items[n-1]
		if last.kind == itemAssistant && last.streaming || last.kind == itemTool && !last.done {
			ui.Box(c).Height(4)
			return
		}
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Height(22).Children(func() {
		ui.Spinner(c).Size(13, 13)
		label := a.tr.AgentWorking()
		if a.ag.client == nil {
			label = a.tr.AgentStarting()
		}
		ui.Text(c, label).FontSize(12.5).TextColor(p.muted)
	})
}

// askCard is an extension's question: a choice, a yes or no, or text.
func (a *app) askCard(c *ui.Context, req *pi.UIRequest) {
	p := &a.p
	a.card(c).Padding(14, 16).Gap(10).Children(func() {
		ui.Text(c, orUntitled(req.Title, a.tr.AgentQuestion())).FontSize(13).FontWeight(600).TextColor(p.fg)
		if req.Message != "" {
			ui.Text(c, req.Message).FontSize(12.5).TextColor(p.fg2).Selectable()
		}
		switch req.Method {
		case "select":
			ui.Column(c).Gap(4).Children(func() {
				for _, o := range req.Options {
					if a.btn(c.Key(o), o, btnSecondary).AlignSelf(ui.Start).Clicked() {
						v := o
						a.answerAsk(&v, nil)
					}
				}
			})
		case "input":
			in := a.input(c, &a.ag.askText, req.Placeholder).AutoFocus()
			if in.Submitted() {
				v := a.ag.askText
				a.answerAsk(&v, nil)
			}
		case "editor":
			ui.TextArea(c, &a.ag.askText).Lines(3, 12).AutoFocus()
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if a.btn(c, a.tr.Cancel(), btnPlain).Clicked() {
				a.answerAsk(nil, nil)
			}
			switch req.Method {
			case "confirm":
				if a.btn(c, a.tr.AgentNo(), btnSecondary).Clicked() {
					no := false
					a.answerAsk(nil, &no)
				}
				if a.btn(c, a.tr.AgentYes(), btnPrimary).Clicked() {
					yes := true
					a.answerAsk(nil, &yes)
				}
			case "input", "editor":
				if a.btn(c, a.tr.Done(), btnPrimary).Clicked() {
					v := a.ag.askText
					a.answerAsk(&v, nil)
				}
			}
		})
	})
}

// changesCard is what a run changed in NextTodo, each thing to open, and
// Undo for all of it.
func (a *app) changesCard(c *ui.Context, r *agentRun) {
	p := &a.p
	if r == nil {
		return
	}
	ui.Column(c).Radius(11).Border(1, p.line).Background(p.card2).Clip().Children(func() {
		ui.Row(c).Padding(9, 10, 9, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconSpark).Size(13, 13).TextColor(p.accent)
			ui.Text(c, a.tr.AgentChanged(len(r.changes))).FontSize(12.5).FontWeight(600).TextColor(p.fg).Grow(1)
			if r.undone {
				ui.Text(c, a.tr.AgentUndone()).FontSize(12).TextColor(p.muted)
			} else if a.iconLabel(c, a.btn(c, "", btnPlain), iconUndo, a.tr.Undo()).Clicked() {
				a.undoRun(r)
				c.Toast(a.tr.AgentUndoneToast())
			}
		})
		ui.Column(c).Dividers(1, p.line2).BorderWidth(1, 0, 0, 0).BorderColor(p.line2).Children(func() {
			for _, ch := range r.changes {
				a.changeRow(c.Key(ch.kind+ch.id), ch, r.undone)
			}
		})
	})
}

func (a *app) changeRow(c *ui.Context, ch change, undone bool) {
	p := &a.p
	title, sub, ic, open := a.changeLook(ch)
	gone := title == ""
	b := ui.ButtonBase(c).Padding(8, 14).Gap(10).AlignItems(ui.Center).Disabled(gone || undone)
	if b.Hovered() && !gone && !undone {
		b.Background(p.pill)
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(14, 14).TextColor(p.muted)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			t := ui.Text(c, orUntitled(title, a.tr.AgentGone())).FontSize(12.5).TextColor(p.fg).SingleLine()
			if gone || undone {
				t.TextColor(p.muted).Strikethrough()
			}
			if sub != "" {
				a.sub(c, sub)
			}
		})
		ui.Text(c, a.verbName(ch.verb)).FontSize(11.5).TextColor(p.muted).Shrink(0)
		if !gone && !undone {
			ui.Icon(c, iconChevronRight).Size(11, 11).TextColor(p.faint)
		}
	})
	if b.Clicked() && open != nil {
		open()
	}
}

// changeLook is how a change shows: the title of what changed ("" when
// it is gone), a line about it, its icon, and how to open it.
func (a *app) changeLook(ch change) (title, sub string, ic *ui.SVG, open func()) {
	switch ch.kind {
	case "task":
		ic = iconCheck
		if t := a.data.task(ch.id); t != nil {
			title = orUntitled(t.Title, a.tr.Untitled())
			var parts []string
			if !t.Due.IsZero() {
				parts = append(parts, a.relDay(t.Due))
			}
			if !t.Remind.IsZero() {
				parts = append(parts, a.clock(t.Remind))
			}
			if t.Project != "" {
				parts = append(parts, a.projectName(t.Project))
			}
			sub = strings.Join(parts, " · ")
			open = func() { a.openTask(ch.id) }
		}
	case "event":
		ic = iconCalendar
		if e := a.data.event(ch.id); e != nil {
			title = orUntitled(e.Title, a.tr.Untitled())
			sub = a.fmt(e.Start, "MMMEd")
			if !e.AllDay {
				sub += " " + a.clock(e.Start)
			}
			open = func() { a.openEvent(ch.id) }
		}
	case "project":
		ic = iconFolder
		if pr := a.data.project(ch.id); pr != nil {
			title = orUntitled(pr.Title, a.tr.Untitled())
			if done, total := a.data.progress(pr.ID); total > 0 {
				sub = a.tr.AgentTasksCount(total)
				_ = done
			}
			open = func() { a.openProject(ch.id) }
		}
	case "direction":
		ic = iconCompass
		if d := a.data.direction(ch.id); d != nil {
			title = orUntitled(d.Title, a.tr.Untitled())
			open = func() { a.navigate(PageDirections) }
		}
	}
	return
}

func (a *app) verbName(v string) string {
	switch v {
	case "created":
		return a.tr.AgentCreated()
	case "completed":
		return a.tr.AgentCompleted()
	case "noted":
		return a.tr.AgentNoted()
	}
	return a.tr.AgentUpdated()
}

// The tools, as people say what they do.

var (
	codeToolRE  = regexp.MustCompile(`tools\.(\w+)\s*\(`)
	codeQueryRE = regexp.MustCompile("(?:query|q|search|keywords?)\\s*:\\s*[\"'`]([^\"'`]{1,80})")
)

// toolLabel says what a tool call does, in a line, with an icon.
func (a *app) toolLabel(it *chatItem) (*ui.SVG, string) {
	var args map[string]any
	json.Unmarshal(it.args, &args)
	str := func(k string) string {
		s, _ := args[k].(string)
		return clip(strings.Join(strings.Fields(s), " "), 50)
	}
	name := it.tool
	if rest, ok := strings.CutPrefix(name, "mcp__nexttodo__"); ok {
		switch rest {
		case "get_overview":
			return iconSun, a.tr.AgentToolOverview()
		case "search":
			return iconSearch, a.tr.AgentToolSearch(str("query"))
		case "list_tasks":
			return iconChecklist, a.tr.AgentToolTasks()
		case "list_events":
			return iconCalendar, a.tr.AgentToolEvents()
		case "list_projects":
			return iconFolder, a.tr.AgentToolProjects()
		case "get_project":
			return iconFolder, a.tr.AgentToolProject()
		case "create_task":
			return iconCheck, a.tr.AgentToolNewTask(str("title"))
		case "update_task":
			if done, ok := args["done"].(bool); ok && done {
				return iconCheck, a.tr.AgentToolCompleteTask()
			}
			return iconEdit, a.tr.AgentToolEditTask()
		case "create_project":
			return iconFolder, a.tr.AgentToolNewProject(str("title"))
		case "update_project":
			return iconEdit, a.tr.AgentToolEditProject()
		case "add_journal_entry":
			return iconNote, a.tr.AgentToolNote()
		case "create_event":
			return iconCalendar, a.tr.AgentToolNewEvent(str("title"))
		case "update_event":
			return iconEdit, a.tr.AgentToolEditEvent()
		case "create_direction":
			return iconCompass, a.tr.AgentToolNewDirection(str("title"))
		}
	}
	switch name {
	case "codemode":
		code, _ := args["code"].(string)
		var tools []string
		for _, m := range codeToolRE.FindAllStringSubmatch(code, -1) {
			if t := shortTool(m[1]); !containsText(tools, t) {
				tools = append(tools, t)
			}
		}
		q := ""
		if m := codeQueryRE.FindStringSubmatch(code); m != nil {
			q = clip(m[1], 50)
		}
		return a.toolsLabel(tools, q)
	case "read":
		return iconNote, a.tr.AgentToolRead(str("path"))
	case "grep", "find", "ls":
		return iconSearch, a.tr.AgentToolFiles()
	case "tool_search":
		return iconSearch, a.tr.AgentToolFind()
	}
	return a.toolsLabel([]string{shortTool(name)}, str("query"))
}

// toolsLabel says what calls of other servers' tools do: searching the
// web or papers, reading a page, or else their names.
func (a *app) toolsLabel(tools []string, q string) (*ui.SVG, string) {
	all := func(f func(string) bool) bool {
		for _, t := range tools {
			if !f(t) {
				return false
			}
		}
		return len(tools) > 0
	}
	paper := func(t string) bool {
		return strings.Contains(t, "paper") || strings.Contains(t, "arxiv") || strings.Contains(t, "pmc")
	}
	search := func(t string) bool { return strings.Contains(t, "search") }
	read := func(t string) bool {
		return strings.Contains(t, "scrape") || strings.Contains(t, "fetch") || strings.Contains(t, "read")
	}
	switch {
	case all(paper) && q != "":
		return iconBook, a.tr.AgentToolPapers(q)
	case all(paper):
		return iconBook, a.tr.AgentToolPapersRead()
	case all(search) && q != "":
		return iconGlobe, a.tr.AgentToolWeb(q)
	case all(read):
		return iconGlobe, a.tr.AgentToolPage()
	case len(tools) == 0:
		return iconCode, a.tr.AgentToolScript()
	}
	return iconTool, a.tr.AgentToolUse(strings.Join(tools, ", "))
}

// shortTool names an MCP tool without its server: mcp__magpie__firecrawl__
// firecrawl_search is firecrawl_search.
func shortTool(name string) string {
	if i := strings.LastIndex(name, "__"); i >= 0 {
		return name[i+2:]
	}
	return name
}

func containsText(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func prettyJSON(raw json.RawMessage, n int) string {
	if len(raw) == 0 {
		return ""
	}
	var b bytes.Buffer
	if json.Indent(&b, raw, "", "  ") != nil {
		return clip(string(raw), n)
	}
	return clip(b.String(), n)
}

// prettyText indents a result that is JSON, and leaves the rest.
func prettyText(s string) string {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
		var b bytes.Buffer
		if json.Indent(&b, []byte(t), "", "  ") == nil {
			return b.String()
		}
	}
	return s
}

// The welcome, before the first message.

func (a *app) agentWelcome(c *ui.Context) {
	p := &a.p
	ui.Column(c).Grow(1).MinHeight(0).Center().Padding(20, 28).Gap(18).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(6).Children(func() {
			a.disc(c, iconSpark, p.accent, 40)
			ui.Text(c, a.tr.AgentHello()).FontSize(19).FontWeight(600).TextColor(p.fg).Margin(8, 0, 0, 0)
			ui.Text(c, a.tr.AgentHelloSub()).FontSize(12.5).TextColor(p.muted).TextAlign(ui.Center).MaxWidth(440)
		})
		ui.Row(c).Gap(8).Wrap().Justify(ui.Center).MaxWidth(620).Children(func() {
			for _, s := range []struct {
				icon       *ui.SVG
				label, ask string
			}{
				{iconSun, a.tr.AgentSuggestToday(), a.tr.AgentSuggestTodayAsk()},
				{iconBulb, a.tr.AgentSuggestIdea(), ""},
				{iconCalendar, a.tr.AgentSuggestWeek(), a.tr.AgentSuggestWeekAsk()},
				{iconBook, a.tr.AgentSuggestPapers(), ""},
				{iconNote, a.tr.AgentSuggestRemember(), ""},
			} {
				b := ui.ButtonBase(c.Key(s.label)).Height(30).Padding(0, 12).Gap(7).Radius(999).AlignItems(ui.Center).
					Border(1, p.line).Background(p.card)
				if b.Hovered() {
					b.Background(p.pill)
				}
				b.Children(func() {
					ui.Icon(c, s.icon).Size(13, 13).TextColor(p.muted)
					ui.Text(c, s.label).FontSize(12.5).TextColor(p.fg2)
				})
				if b.Clicked() {
					if s.ask != "" {
						a.askAgent(s.ask)
					} else {
						a.ag.draft = s.label + " "
						a.ag.field.Focus()
					}
				}
			}
		})
	})
}

// agentSetup is what the page shows without pi: how to get it.
func (a *app) agentSetup(c *ui.Context) {
	p := &a.p
	ui.Column(c).Grow(1).Center().Padding(28).Children(func() {
		ui.Column(c).MaxWidth(520).Gap(12).Children(func() {
			a.disc(c, iconSpark, p.accent, 36)
			ui.Text(c, a.tr.AgentSetupTitle()).FontSize(17).FontWeight(600).TextColor(p.fg)
			if a.ag.missing {
				ui.Text(c, a.tr.AgentSetupBody()).FontSize(12.5).LineHeight(1.55).TextColor(p.fg2)
				cmd := "npm install -g @earendil-works/pi-coding-agent"
				ui.Row(c).Padding(8, 8, 8, 12).Gap(8).Radius(8).Background(p.card2).Border(1, p.line).AlignItems(ui.Center).Children(func() {
					ui.Text(c, cmd).Font(monoFont).FontSize(12).TextColor(p.fg).Selectable().Grow(1)
					if a.iconBtn(c, iconCopy, a.tr.Copy()).Clicked() {
						mygo.Clipboard.WriteText(cmd)
						c.Toast(a.tr.Copied())
					}
				})
				ui.Text(c, a.tr.AgentSetupThen()).FontSize(12.5).LineHeight(1.55).TextColor(p.fg2)
			} else {
				ui.Text(c, a.tr.AgentStartFailed()).FontSize(12.5).TextColor(p.fg2)
				ui.Text(c, a.ag.err).FontSize(11.5).Font(monoFont).TextColor(p.red).Selectable().
					Padding(8, 12).Radius(8).Background(p.redSoft)
			}
			ui.Row(c).Gap(8).Margin(4, 0, 0, 0).Children(func() {
				if a.btn(c, a.tr.AgentTryAgain(), btnPrimary).Clicked() {
					a.ag.err = ""
					a.lookForPi()
					a.startAgent()
				}
				if a.btn(c, a.tr.AgentSettings(), btnSecondary).Clicked() {
					a.navigate(PageSettings)
				}
			})
		})
	})
}

// The field writing the next message.

func (a *app) composer(c *ui.Context) {
	p := &a.p
	ch := &a.ag.chat
	ui.Column(c).Padding(4, 24, 16).MaxWidth(808).Margin(0, ui.Auto).FillWidth().Shrink(0).Children(func() {
		box := ui.Column(c).Radius(14).Border(1, p.line).Background(p.card).Padding(10, 12, 8, 14).Gap(6).
			Shadow(0, 2, 10, 0, p.shadow.Alpha(0.08))
		box.Children(func() {
			in := ui.TextAreaBase(c, &a.ag.draft).Lines(1, 10).FontSize(13.5).LineHeight(1.5).
				Placeholder(a.tr.AgentPlaceholder()).Bind(&a.ag.field).Disabled(ch.loading)
			if a.page == PageAssistant && !in.Focused() && a.ag.ask == nil && a.ag.renaming == "" && len(ch.items) == 0 {
				in.AutoFocus()
			}
			in.OnTextKey(func(k ui.TextKey) bool {
				switch {
				case k.Key == ui.KeyEnter && k.Mods == 0:
					if strings.TrimSpace(k.Text()) != "" {
						a.askAgent(k.Text())
						k.SetText("")
					}
					a.redraw()
					return true
				case k.Key == ui.KeyEscape && ch.running && strings.TrimSpace(k.Text()) == "":
					a.stopRun()
					return true
				case k.Key == ui.KeyUp && k.Mods == 0 && k.Text() == "":
					if last := a.lastAsked(); last != "" {
						k.SetText(last)
						return true
					}
				}
				return false
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				a.modelMenu(c)
				a.levelMenu(c)
				ui.Spacer(c)
				if in.Focused() && a.ag.draft != "" {
					ui.Text(c, a.tr.AgentKeys()).FontSize(11).TextColor(p.faint).SingleLine()
				}
				empty := strings.TrimSpace(a.ag.draft) == ""
				stop := ch.running && empty
				b := ui.ButtonBase(c.Key("send")).Size(28, 28).Radius(14).Center().Shrink(0)
				switch {
				case stop:
					b.Background(p.fg).TextColor(p.card).Label(a.tr.AgentStop()).Tooltip(a.tr.AgentStop() + "  esc")
				case empty:
					b.Background(p.pill).TextColor(p.faint).Disabled(true).Label(a.tr.AgentSend())
				default:
					b.Background(p.accent).TextColor(p.accentFg).Label(a.tr.AgentSend())
					if b.Hovered() {
						b.Background(p.accent.Mix(p.fg, 0.12))
					}
				}
				b.Children(func() {
					if stop {
						ui.Icon(c, iconStop).Size(13, 13)
					} else {
						ui.Icon(c, iconSend).Size(15, 15)
					}
				})
				if b.Clicked() {
					if stop {
						a.stopRun()
					} else {
						a.askAgent(a.ag.draft)
						a.ag.draft = ""
					}
				}
			})
		})
	})
}

// lastAsked is the user's last message, for Up to bring back.
func (a *app) lastAsked() string {
	items := a.ag.chat.items
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].kind == itemUser {
			return items[i].text
		}
	}
	return ""
}

// modelMenu chooses the model, among those pi has.
func (a *app) modelMenu(c *ui.Context) {
	p := &a.p
	name := a.tr.AgentModel()
	if m := a.ag.state.Model; m != nil {
		name = orUntitled(m.Name, m.ID)
		if i := strings.Index(name, " · "); i > 0 {
			name = name[:i]
		}
	}
	b := ui.ButtonBase(c.Key("model")).Height(24).Padding(0, 6, 0, 8).Gap(4).Radius(6).AlignItems(ui.Center).
		MinWidth(0).Label(a.tr.AgentModel()).Disabled(a.ag.client == nil || a.ag.chat.running)
	if b.Hovered() || a.ag.modelOpen {
		b.Background(p.pill)
	}
	b.Children(func() {
		ui.Text(c, name).FontSize(12).TextColor(p.muted).SingleLine().MinWidth(0)
		ui.Icon(c, iconChevronDown).Size(9, 9).TextColor(p.faint)
	})
	if b.Clicked() {
		a.ag.modelOpen = !a.ag.modelOpen
		if a.ag.modelOpen {
			a.loadModels()
		}
	}
	ui.PopoverBase(c, b, &a.ag.modelOpen, func(panel ui.Element) {
		popover(a, panel).Margin(0, 0, 4, 0).Padding(4).MinWidth(240).MaxHeight(380).Gap(1)
		ui.Scroll(c).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				if len(a.ag.models) == 0 {
					ui.Row(c).Padding(8).Center().Children(func() { ui.Spinner(c) })
				}
				provider := ""
				for _, m := range a.ag.models {
					if m.Provider != provider {
						provider = m.Provider
						a.label(c, provider).Padding(8, 8, 3)
					}
					cur := a.ag.state.Model
					on := cur != nil && cur.ID == m.ID && cur.Provider == m.Provider
					it := ui.ButtonBase(c.Key(m.Provider+"/"+m.ID)).Height(28).Padding(0, 8).Gap(8).Radius(7).AlignItems(ui.Center)
					if it.Hovered() {
						it.Background(p.pill)
					}
					it.Children(func() {
						if on {
							ui.Icon(c, iconCheck).Size(12, 12).TextColor(p.accent)
						} else {
							ui.Box(c).Size(12, 12)
						}
						ui.Text(c, orUntitled(m.Name, m.ID)).FontSize(12.5).TextColor(p.fg).SingleLine().Grow(1).MinWidth(0)
					})
					if it.Clicked() {
						a.ag.modelOpen = false
						a.setModel(m)
					}
				}
			})
		})
	})
}

// levelMenu chooses how hard the model thinks, as it can.
func (a *app) levelMenu(c *ui.Context) {
	if len(a.ag.levels) < 2 {
		return
	}
	level := a.ag.state.ThinkingLevel
	chosen := level
	sel := selectOf(a, c, "level", &chosen, a.ag.levels, a.levelName)
	sel.Background(ui.Color{}).Height(24)
	if a.ag.client == nil || a.ag.chat.running {
		sel.Disabled(true)
	}
	if chosen != level {
		a.setThinking(chosen)
	}
}

func (a *app) levelName(l string) string {
	switch l {
	case "off":
		return a.tr.ThinkOff()
	case "minimal":
		return a.tr.ThinkMinimal()
	case "low":
		return a.tr.ThinkLow()
	case "medium":
		return a.tr.ThinkMedium()
	case "high":
		return a.tr.ThinkHigh()
	case "xhigh":
		return a.tr.ThinkXHigh()
	case "max":
		return a.tr.ThinkMax()
	}
	return l
}

// Asking from the overview.

// askFromAnywhere shows the assistant, ready for a message.
func (a *app) askFromAnywhere() {
	if a.page == PageOverview && !a.data.empty() {
		a.askField.Focus() // the overview's own field
		return
	}
	a.navigate(PageAssistant)
	a.ag.field.Focus()
}

// askBar is the overview's field asking the assistant: what it answers
// shows under it, as what it adds shows on the overview at once.
func (a *app) askBar(c *ui.Context) {
	p := &a.p
	ch := &a.ag.chat
	ui.Column(c).Gap(8).Margin(4, 0, 2, 0).Children(func() {
		row := ui.Row(c).Height(40).Padding(0, 10, 0, 14).Gap(10).AlignItems(ui.Center).Radius(12).
			Background(p.card).Border(1, p.line)
		row.Children(func() {
			ui.Icon(c, iconSpark).Size(15, 15).TextColor(p.accent)
			in := ui.TextInputBase(c, &a.askDraft).Placeholder(a.tr.AskPlaceholder()).Label(a.tr.NavAssistant()).
				Grow(1).FontSize(13).TextColor(p.fg).Bind(&a.askField)
			if in.Focused() {
				row.Border(1, p.accent.Alpha(0.5))
			}
			if in.Submitted() {
				if s := strings.TrimSpace(a.askDraft); s != "" {
					if !a.asked || ch.loading {
						a.freshChat()
					}
					a.asked = true
					a.askAgent(s)
				}
				a.askDraft = ""
			}
			ui.Text(c, "⌘J").FontSize(11).TextColor(p.faint)
		})
		if a.asked && len(ch.items) > 0 {
			a.askAnswer(c)
		}
	})
}

// freshChat starts a conversation afresh without leaving the page.
func (a *app) freshChat() {
	ch := &a.ag.chat
	if len(ch.items) == 0 && !ch.loading {
		return
	}
	a.leaveRun()
	a.ag.chat, a.ag.ask = chat{}, nil
	a.agentCall(map[string]any{"type": "new_session"}, nil)
}

// askAnswer is the assistant's answer under the overview's field: its
// last words, what it is doing, what it changed.
func (a *app) askAnswer(c *ui.Context) {
	p := &a.p
	ch := &a.ag.chat
	var reply, asked *chatItem
	var tool *chatItem
	var run *agentRun
	var errItem *chatItem
	for i := len(ch.items) - 1; i >= 0; i-- {
		it := ch.items[i]
		switch {
		case it.kind == itemUser && asked == nil:
			asked = it
		case it.kind == itemAssistant && reply == nil && asked == nil && it.text != "":
			reply = it
		case it.kind == itemTool && tool == nil && asked == nil:
			tool = it
		case it.kind == itemChanges && run == nil && asked == nil:
			run = it.run
		case it.kind == itemError && errItem == nil && asked == nil:
			errItem = it
		}
	}
	if run == nil && ch.run != nil && len(ch.run.changes) > 0 {
		run = ch.run
	}
	a.card(c).Padding(12, 16, 10).Gap(10).Children(func() {
		if asked != nil {
			ui.Text(c, asked.text).FontSize(12).TextColor(p.muted).MaxLines(2)
		}
		if reply != nil {
			ui.Scroll(c).MaxHeight(240).Children(func() {
				if reply.streaming {
					markdown.Stream(c.Key(reply.id), reply.text, &a.ag.md)
				} else {
					markdown.View(c.Key(reply.id), reply.text, &a.ag.md)
				}
			})
		}
		if ch.running && (reply == nil || !reply.streaming) {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(12, 12)
				label := a.tr.AgentWorking()
				if tool != nil && !tool.done {
					_, label = a.toolLabel(tool)
				} else if a.ag.client == nil {
					label = a.tr.AgentStarting()
				}
				ui.Text(c, label).FontSize(12).TextColor(p.muted).SingleLine()
			})
		}
		if errItem != nil {
			ui.Text(c, errItem.text).FontSize(12).TextColor(p.red).MaxLines(3)
		}
		if run != nil {
			ui.Column(c).Gap(2).Children(func() {
				for _, chg := range run.changes {
					title, sub, ic, open := a.changeLook(chg)
					b := ui.ButtonBase(c.Key(chg.kind+chg.id)).Padding(4, 6).Margin(0, -6).Gap(8).Radius(6).AlignItems(ui.Center).AlignSelf(ui.Start).
						Disabled(title == "" || run.undone)
					if b.Hovered() {
						b.Background(p.pill)
					}
					b.Children(func() {
						ui.Icon(c, ic).Size(13, 13).TextColor(p.accent)
						t := ui.Text(c, orUntitled(title, a.tr.AgentGone())).FontSize(12.5).TextColor(p.fg).SingleLine().MinWidth(0)
						if title == "" || run.undone {
							t.TextColor(p.muted).Strikethrough()
						}
						if sub != "" {
							ui.Text(c, sub).FontSize(11.5).TextColor(p.muted).SingleLine()
						}
					})
					if b.Clicked() && open != nil {
						open()
					}
				}
			})
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Margin(0, -6, 0, -6).Children(func() {
			if run != nil && !run.undone && !ch.running {
				if a.iconLabel(c, a.btn(c, "", btnPlain), iconUndo, a.tr.Undo()).Clicked() {
					a.undoRun(run)
				}
			}
			ui.Spacer(c)
			if a.btn(c, a.tr.AskContinue(), btnLink).Clicked() {
				a.asked = false
				a.navigate(PageAssistant)
				a.ag.list.ScrollToEnd()
			}
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.asked = false
			}
		})
	})
}

package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"nexttodo/internal/mcp"
	"nexttodo/internal/pi"
)

// The assistant is pi (github.com/earendil-works/pi), run in its RPC mode
// as NextTodo's agent: pi keeps the conversations, calls the model the
// user set up for it, and reaches the MCP servers they gave it, as web
// and paper search; NextTodo gives it its own tools (agent_tools.go) over
// an MCP server of its own, on the loopback interface, and shows the
// conversation. pi runs in a folder of NextTodo's, Assistant beside the
// data, which holds its sessions and the MCP server's address, and with
// neither shell nor file editing: it is an assistant, not a coding agent.
//
// What pi says comes on a goroutine of its own and is handed to the main
// thread, where the conversation is kept and drawn.

//go:embed prompts/assistant.md
var assistantPrompt string

// agent is the assistant's state.
type agent struct {
	// pi on this Mac, looked for once, and again as the settings ask.
	find    sync.Mutex
	inst    *pi.Install
	missing bool   // looked for and not found
	version string // as pi says, once found
	looked  bool   // the settings asked for it
	bin     string // where it was found, for the settings

	client   *pi.Client
	gen      int // which pi started, so that a pi ended says nothing more
	starting bool
	err      string // why pi could not start, or ended
	waiting  []func(*pi.Client)

	server *mcp.Server

	state  pi.State
	models []pi.Model
	levels []string

	chat     chat
	sessions []pi.Session
	cache    pi.SessionCache
	listing  bool

	draft   string
	field   ui.Handle
	list    ui.ListState
	md      markdown.Options
	ask     *pi.UIRequest // an extension's dialog, waiting for its answer
	askText string
	nextID  int
	// The menus of the composer and the sessions.
	modelOpen, levelOpen bool
	renaming             string // the session whose name is being edited
	rename               string
	renameFocus          bool
	// discard drops what pi says of a run stopped as another conversation
	// showed.
	discard bool
}

// chat is the conversation shown.
type chat struct {
	path    string // its session's file, "" until pi saves it
	name    string
	items   []*chatItem
	running bool      // from sending until pi has nothing more to do
	since   time.Time // when it started running
	run     *agentRun // what the run changed in NextTodo
	queued  []string  // what was sent while it ran, not yet taken
	loading bool      // its messages are being read
}

type itemKind int

const (
	itemUser itemKind = iota
	itemAssistant
	itemTool
	itemChanges
	itemError
	itemNote
)

// chatItem is a row of the conversation: a message of the user or of the
// assistant, a tool the assistant called, what it changed in NextTodo,
// or a note, as of an error.
type chatItem struct {
	id   int
	kind itemKind
	text string

	// The assistant's.
	thinking  string
	streaming bool
	thinkFrom time.Time     // when its thinking started
	thought   time.Duration // how long it thought
	block     int           // the content block being written
	showThink bool

	// A tool's.
	call    string
	tool    string
	args    json.RawMessage
	result  string
	failed  bool
	done    bool
	started time.Time
	took    time.Duration
	open    bool

	run *agentRun // itemChanges's
}

// agentRun is what a run of the assistant changed in NextTodo, and the
// steps that take it back.
type agentRun struct {
	changes []change
	steps   []*undoStep
	undone  bool
}

// change is a thing the assistant added or changed.
type change struct {
	kind string // task, event, project, direction
	id   string
	verb string // created, updated, completed, noted
}

func (a *app) newItem(kind itemKind, text string) *chatItem {
	a.ag.nextID++
	return &chatItem{id: a.ag.nextID, kind: kind, text: text}
}

func (a *app) addItem(kind itemKind, text string) *chatItem {
	it := a.newItem(kind, text)
	a.ag.chat.items = append(a.ag.chat.items, it)
	return it
}

// Where pi runs.

// agentDir is the assistant's folder, beside the data.
func (a *app) agentDir() string {
	if a.path == "" {
		return filepath.Join(os.TempDir(), "NextTodo", "Assistant")
	}
	return filepath.Join(filepath.Dir(a.path), "Assistant")
}

func (a *app) sessionsDir() string { return filepath.Join(a.agentDir(), "sessions") }

// findPi looks for pi, once: at the path the settings give, or where the
// login shell and the installers put it. It may take seconds, as the
// shell starts: call it off the main thread.
func (a *app) findPi(path string) (pi.Install, error) {
	a.ag.find.Lock()
	defer a.ag.find.Unlock()
	if a.ag.inst != nil {
		return *a.ag.inst, nil
	}
	if p := os.Getenv("NEXTTODO_PI"); p != "" && path == "" {
		path = p
	}
	in, err := pi.Find(context.Background(), path)
	if err != nil {
		return in, err
	}
	a.ag.inst = &in
	return in, nil
}

// lookForPi looks for pi again, as the settings ask, and says what it
// found.
func (a *app) lookForPi() {
	a.ag.find.Lock()
	a.ag.inst = nil
	a.ag.find.Unlock()
	a.ag.missing, a.ag.version = false, ""
	path := a.data.Prefs.PiPath
	go func() {
		in, err := a.findPi(path)
		v := ""
		if err == nil {
			v, _ = in.Version(context.Background())
		}
		mygo.RunOnMain(func() {
			a.ag.missing, a.ag.version, a.ag.bin = err != nil, v, in.Bin
			if err == nil && a.ag.err != "" && a.ag.client == nil {
				a.ag.err = ""
			}
			a.redraw()
		})
	}()
}

// toolsOff are pi's tools the assistant goes without: it runs no
// commands and changes no files, and reads them only as the settings
// allow.
func (a *app) toolsOff() string {
	off := "bash,edit,write,powershell"
	if !a.data.Prefs.AgentFiles {
		off += ",read,grep,find,ls"
	}
	return off
}

// startAgent starts pi, if it is not running, and runs what waits for it
// once it is.
func (a *app) startAgent() {
	if a.ag.client != nil || a.ag.starting || !a.live {
		return
	}
	a.ag.starting, a.ag.err = true, ""
	a.ag.gen++
	gen := a.ag.gen
	if a.ag.server == nil {
		a.ag.server = mcp.NewServer("nexttodo", appVersion(), a.agentTools())
		a.ag.server.Instructions = "The user's own tasks, events, projects, project journals and research directions in NextTodo. Read before writing; there is no deleting."
	}
	srv := a.ag.server
	dir, sessions, path := a.agentDir(), a.sessionsDir(), a.data.Prefs.PiPath
	args := []string{
		"--approve", // the folder's .pi/mcp.json is NextTodo's own
		"--exclude-tools", a.toolsOff(),
		"--session-dir", sessions,
		"--system-prompt", assistantPrompt,
		"--append-system-prompt", "The app shows itself in " + languageName(a.lang) + "; write in it unless the user writes in another language.",
	}
	model := a.data.Prefs.AgentModel
	level := a.data.Prefs.AgentThinking
	resume := a.ag.chat.path // a pi started again goes on with it
	go func() {
		fail := func(msg string, missing bool) {
			mygo.RunOnMain(func() {
				if gen != a.ag.gen {
					return
				}
				a.ag.starting, a.ag.err, a.ag.missing = false, msg, missing
				a.ag.waiting = nil
				a.agentFailed(msg)
			})
		}
		in, err := a.findPi(path)
		if err != nil {
			fail(err.Error(), true)
			return
		}
		url, err := srv.Start()
		if err == nil {
			err = writeWorkspace(dir, url)
		}
		if err != nil {
			fail(err.Error(), false)
			return
		}
		var c *pi.Client
		c, err = pi.Start(pi.Options{
			Bin:  in.Bin,
			Args: args,
			Env:  pi.Setenv(in.Env, "NEXTTODO_MCP_TOKEN", srv.Token()),
			Dir:  dir,
			OnEvent: func(e pi.Event) {
				mygo.RunOnMain(func() {
					if gen == a.ag.gen {
						a.piEvent(e)
						a.redraw()
					}
				})
			},
			OnExit: func(err error) {
				mygo.RunOnMain(func() {
					if gen == a.ag.gen && a.ag.client != nil {
						a.ag.client = nil
						a.agentFailed(a.tr.AgentEnded(err.Error()))
					}
				})
			},
		})
		if err != nil {
			fail(err.Error(), false)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The model and thinking chosen in NextTodo, if pi still has them.
		if i := strings.Index(model, "/"); i > 0 {
			c.Call(ctx, map[string]any{"type": "set_model", "provider": model[:i], "modelId": model[i+1:]})
		}
		if level != "" {
			c.Call(ctx, map[string]any{"type": "set_thinking_level", "level": level})
		}
		if resume != "" {
			c.Call(ctx, map[string]any{"type": "switch_session", "sessionPath": resume})
		}
		var st pi.State
		data, err := c.Call(ctx, map[string]any{"type": "get_state"})
		if err != nil {
			c.Close()
			fail(err.Error(), false)
			return
		}
		json.Unmarshal(data, &st)
		levels := thinkingLevels(ctx, c)
		v, _ := in.Version(ctx)
		mygo.RunOnMain(func() {
			if gen != a.ag.gen {
				go c.Close()
				return
			}
			a.ag.client, a.ag.starting, a.ag.state, a.ag.levels = c, false, st, levels
			a.ag.version, a.ag.bin, a.ag.missing = v, in.Bin, false
			waiting := a.ag.waiting
			a.ag.waiting = nil
			for _, fn := range waiting {
				fn(c)
			}
			a.redraw()
		})
	}()
}

func thinkingLevels(ctx context.Context, c *pi.Client) []string {
	var lv struct {
		Levels []string `json:"levels"`
	}
	if data, err := c.Call(ctx, map[string]any{"type": "get_available_thinking_levels"}); err == nil {
		json.Unmarshal(data, &lv)
	}
	return lv.Levels
}

// writeWorkspace writes pi's folder: the MCP server of NextTodo, at its
// address of this launch, its token in the environment.
func writeWorkspace(dir, url string) error {
	if err := os.MkdirAll(filepath.Join(dir, ".pi"), 0o755); err != nil {
		return err
	}
	cfg := map[string]any{"mcpServers": map[string]any{"nexttodo": map[string]any{
		"url":         url,
		"headers":     map[string]string{"Authorization": "Bearer ${NEXTTODO_MCP_TOKEN}"},
		"exposure":    "direct",
		"description": "The user's tasks, events, projects, journals and research directions in NextTodo",
	}}}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return writeFile(filepath.Join(dir, ".pi", "mcp.json"), b)
}

// withAgent runs fn with pi once it runs, starting it if need be.
func (a *app) withAgent(fn func(c *pi.Client)) {
	if a.ag.client != nil {
		fn(a.ag.client)
		return
	}
	a.ag.waiting = append(a.ag.waiting, fn)
	a.startAgent()
}

// agentCall sends a command to pi and gives done its answer, on the main
// thread.
func (a *app) agentCall(cmd map[string]any, done func(json.RawMessage, error)) {
	a.withAgent(func(c *pi.Client) {
		p := c.Queue(cmd) // in the order sent
		go func() {
			data, err := p.Wait(context.Background())
			if done != nil {
				mygo.RunOnMain(func() {
					done(data, err)
					a.redraw()
				})
			}
		}()
	})
}

// agentFailed ends the run shown, saying why.
func (a *app) agentFailed(msg string) {
	ch := &a.ag.chat
	if ch.running {
		a.finishRun()
		a.addItem(itemError, msg)
	}
	for _, it := range ch.items {
		it.streaming = false
		if it.kind == itemTool && !it.done {
			it.done, it.failed = true, true
		}
	}
	a.ag.ask = nil
	a.redraw()
}

// stopAgent ends pi, as the app quits.
func (a *app) stopAgent() {
	if c := a.ag.client; c != nil {
		a.ag.client = nil
		a.ag.gen++
		c.Close()
	}
	if a.ag.server != nil {
		a.ag.server.Close()
	}
}

// restartAgent ends pi so that the next message starts it again, with
// the settings as they are now.
func (a *app) restartAgent() {
	if c := a.ag.client; c != nil {
		a.ag.client = nil
		a.ag.gen++
		go c.Close()
	}
	a.ag.starting = false
	if a.ag.chat.running {
		a.agentFailed(a.tr.AgentRestarted())
	}
}

// Sending.

// nowTag tells the model the user's local time, ahead of each message.
func (a *app) nowTag() string {
	now := a.now()
	_, off := now.Zone()
	return "<now>" + now.Format("2006-01-02 Mon 15:04") + " " + utcOffset(off) + "</now>\n"
}

var nowTagRE = regexp.MustCompile(`^\s*<now>[^<]*</now>\s*`)

// stripNow takes the time NextTodo told the model from a message.
func stripNow(s string) string { return nowTagRE.ReplaceAllString(s, "") }

// askAgent sends what the user wrote: it starts a run, or steers the one
// going on.
func (a *app) askAgent(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	ch := &a.ag.chat
	it := a.addItem(itemUser, text)
	cmd := map[string]any{"type": "prompt", "message": a.nowTag() + text}
	if ch.running {
		cmd["streamingBehavior"] = "steer"
		ch.queued = append(ch.queued, text)
	} else {
		ch.running, ch.since, ch.run = true, a.now(), nil
	}
	a.ag.list.ScrollToEnd()
	a.agentCall(cmd, func(_ json.RawMessage, err error) {
		if err == nil {
			return
		}
		// Not taken: the message goes back to the composer.
		if i := slices.Index(ch.items, it); i >= 0 {
			ch.items = slices.Delete(ch.items, i, i+1)
		}
		ch.queued = slices.DeleteFunc(ch.queued, func(s string) bool { return s == text })
		if a.ag.draft == "" {
			a.ag.draft = text
		}
		if ch.running && cmd["streamingBehavior"] == nil {
			a.finishRun()
		}
		a.addItem(itemError, err.Error())
	})
}

// stopRun stops the run going on.
func (a *app) stopRun() {
	if !a.ag.chat.running {
		return
	}
	if a.ag.client == nil {
		a.ag.waiting = nil
		a.finishRun()
		return
	}
	a.agentCall(map[string]any{"type": "abort"}, nil)
}

// finishRun ends the run shown: what it changed shows, with Undo.
func (a *app) finishRun() {
	ch := &a.ag.chat
	ch.running, ch.queued = false, nil
	if r := ch.run; r != nil && len(r.changes) > 0 {
		it := a.addItem(itemChanges, "")
		it.run = r
	}
	ch.run = nil
	for _, it := range ch.items {
		if it.kind == itemAssistant && it.streaming {
			it.streaming = false
		}
	}
	a.listSessions()
}

// agentChange makes a change of the assistant's, as one step to undo,
// and notes it in the run.
func (a *app) agentChange(fn func(), ch change) {
	step := a.remember()
	fn()
	a.settle(step)
	a.changed()
	r := a.ag.chat.run
	if r == nil {
		r = &agentRun{}
		a.ag.chat.run = r
	}
	r.steps = append(r.steps, step)
	if i := slices.IndexFunc(r.changes, func(c change) bool { return c.kind == ch.kind && c.id == ch.id }); i >= 0 {
		if r.changes[i].verb != "created" {
			r.changes[i].verb = ch.verb
		}
		return
	}
	r.changes = append(r.changes, ch)
}

// undoRun takes back what a run changed, but what was changed since.
func (a *app) undoRun(r *agentRun) {
	for i := len(r.steps) - 1; i >= 0; i-- {
		if !r.steps[i].undone {
			a.restore(r.steps[i])
		}
	}
	r.undone = true
}

// What pi says.

// piEvent takes in what pi says of the session, on the main thread.
func (a *app) piEvent(e pi.Event) {
	ch := &a.ag.chat
	if a.ag.discard {
		// The end of a run left: dropped until it settles, or another starts.
		switch e.Type {
		case "agent_start":
			a.ag.discard = false
		case "agent_settled":
			a.ag.discard = false
			return
		default:
			return
		}
	}
	if e.Type == "extension_ui_request" {
		var req pi.UIRequest
		e.Decode(&req)
		a.extensionUI(req)
		return
	}
	var se pi.SessionEvent
	e.Decode(&se) // a field of a type unforeseen is left out, the rest read
	last := func(kind itemKind) *chatItem {
		for i := len(ch.items) - 1; i >= 0; i-- {
			if it := ch.items[i]; it.kind == kind {
				return it
			}
		}
		return nil
	}
	switch e.Type {
	case "agent_start":
		if !ch.running {
			ch.running, ch.since = true, a.now()
		}
	case "message_start":
		if se.Message == nil {
			return
		}
		switch se.Message.Role {
		case "user":
			// A message sent while the run went on is taken now.
			text := stripNow(se.Message.Text())
			if i := slices.Index(ch.queued, text); i >= 0 {
				ch.queued = slices.Delete(ch.queued, i, i+1)
			}
		case "assistant":
			it := a.addItem(itemAssistant, "")
			it.streaming, it.block = true, -1
		}
	case "message_update":
		it := last(itemAssistant)
		ev := se.AssistantMessageEvent
		if it == nil || !it.streaming || ev == nil {
			return
		}
		switch ev.Type {
		case "text_delta":
			if it.block != ev.ContentIndex && it.text != "" {
				it.text += "\n\n"
			}
			it.block = ev.ContentIndex
			it.text += ev.Delta
		case "thinking_start":
			it.thinkFrom = a.now()
		case "thinking_delta":
			if it.thinkFrom.IsZero() {
				it.thinkFrom = a.now()
			}
			it.thinking += ev.Delta
		case "thinking_end":
			if !it.thinkFrom.IsZero() {
				it.thought += a.now().Sub(it.thinkFrom)
				it.thinkFrom = time.Time{}
			}
		}
	case "message_end":
		m := se.Message
		if m == nil || m.Role != "assistant" {
			return
		}
		it := last(itemAssistant)
		if it == nil || !it.streaming {
			it = a.addItem(itemAssistant, "")
		}
		it.streaming = false
		if !it.thinkFrom.IsZero() {
			it.thought += a.now().Sub(it.thinkFrom)
			it.thinkFrom = time.Time{}
		}
		it.text, it.thinking = m.Text(), thinkingOf(*m)
		if it.text == "" && it.thinking == "" {
			ch.items = slices.DeleteFunc(ch.items, func(x *chatItem) bool { return x == it })
		}
		switch m.StopReason {
		case "error":
			a.addItem(itemError, orUntitled(m.ErrorMessage, a.tr.AgentError()))
		case "length":
			a.addItem(itemNote, a.tr.AgentTooLong())
		}
	case "tool_execution_start":
		it := a.addItem(itemTool, "")
		it.call, it.tool, it.args, it.started = se.ToolCallID, se.ToolName, se.Args, a.now()
	case "tool_execution_end":
		for i := len(ch.items) - 1; i >= 0; i-- {
			if it := ch.items[i]; it.kind == itemTool && it.call == se.ToolCallID {
				it.done, it.failed, it.result = true, se.IsError, se.Result.Text()
				it.took = a.now().Sub(it.started)
				break
			}
		}
	case "queue_update":
		ch.queued = nil
		for _, s := range append(se.Steering, se.FollowUp...) {
			ch.queued = append(ch.queued, stripNow(s))
		}
	case "auto_retry_start":
		a.addItem(itemNote, a.tr.AgentRetrying(se.Attempt, se.MaxAttempts))
	case "auto_retry_end":
		if !se.Success && se.FinalError != "" {
			a.addItem(itemError, se.FinalError)
		}
	case "compaction_start":
		a.addItem(itemNote, a.tr.AgentCompacting())
	case "session_info_changed":
		ch.name = se.Name
		a.listSessions()
	case "agent_settled":
		if se.Aborted {
			a.addItem(itemNote, a.tr.AgentStopped())
		}
		a.finishRun()
		if ch.path == "" {
			a.agentCall(map[string]any{"type": "get_state"}, func(data json.RawMessage, err error) {
				var st pi.State
				if err == nil && json.Unmarshal(data, &st) == nil {
					a.ag.state = st
					ch.path = st.SessionFile
				}
			})
		}
	case "extension_error":
		log.Printf("pi extension: %s", se.ErrorText())
	}
}

func thinkingOf(m pi.Message) string {
	var parts []string
	for _, b := range m.Blocks() {
		if b.Type == "thinking" && b.Thinking != "" {
			parts = append(parts, b.Thinking)
		}
	}
	return strings.Join(parts, "\n\n")
}

// extensionUI shows what an extension of the user's pi asks for: a
// dialog in the conversation, or a note.
func (a *app) extensionUI(req pi.UIRequest) {
	switch {
	case req.Dialog():
		a.ag.ask, a.ag.askText = &req, req.Prefill
	case req.Method == "notify":
		a.toast = req.Message
	case req.Method == "set_editor_text":
		a.ag.draft = req.Text
	}
}

// answerAsk answers the dialog shown: with a value, confirmed or not, or
// cancelled when both are nil.
func (a *app) answerAsk(value *string, confirmed *bool) {
	req := a.ag.ask
	a.ag.ask = nil
	if req == nil || a.ag.client == nil {
		return
	}
	a.ag.client.Send(pi.UIResponse(req.ID, value, confirmed))
}

// Sessions.

// listSessions reads the saved conversations, off the main thread.
func (a *app) listSessions() {
	if a.ag.listing || !a.live {
		return
	}
	a.ag.listing = true
	dir := a.sessionsDir()
	go func() {
		list := pi.Sessions(dir, &a.ag.cache)
		mygo.RunOnMain(func() {
			a.ag.sessions, a.ag.listing = list, false
			a.redraw()
		})
	}()
}

// newChat starts a conversation afresh.
func (a *app) newChat() {
	ch := &a.ag.chat
	if len(ch.items) == 0 && !ch.loading {
		a.ag.field.Focus()
		return
	}
	a.leaveRun()
	a.ag.chat = chat{}
	a.ag.ask = nil
	a.ag.field.Focus()
	a.agentCall(map[string]any{"type": "new_session"}, nil)
}

// leaveRun stops the run going on, as another conversation shows: what pi
// still says of it is dropped.
func (a *app) leaveRun() {
	if !a.ag.chat.running {
		return
	}
	if a.ag.client != nil {
		a.ag.discard = true
	}
	a.stopRun()
}

// openSession shows a saved conversation, which goes on from there.
func (a *app) openSession(path string) {
	if a.ag.chat.path == path && !a.ag.chat.loading {
		return
	}
	a.leaveRun()
	var name string
	for _, s := range a.ag.sessions {
		if s.Path == path {
			name = s.Name
		}
	}
	a.ag.chat = chat{path: path, name: name, loading: true}
	a.ag.ask = nil
	a.withAgent(func(c *pi.Client) {
		go func() {
			ctx := context.Background()
			_, err := c.Call(ctx, map[string]any{"type": "switch_session", "sessionPath": path})
			var msgs struct {
				Messages []pi.Message `json:"messages"`
			}
			var st pi.State
			if err == nil {
				var data json.RawMessage
				if data, err = c.Call(ctx, map[string]any{"type": "get_messages"}); err == nil {
					err = json.Unmarshal(data, &msgs)
				}
				if data, e := c.Call(ctx, map[string]any{"type": "get_state"}); e == nil {
					json.Unmarshal(data, &st)
				}
			}
			mygo.RunOnMain(func() {
				ch := &a.ag.chat
				if ch.path != path {
					return // another was opened since
				}
				ch.loading = false
				if err != nil {
					a.addItem(itemError, err.Error())
				} else {
					ch.items = a.itemsOf(msgs.Messages)
					a.ag.state = st
				}
				a.ag.list.ScrollToEnd()
				a.redraw()
			})
		}()
	})
}

// itemsOf makes the rows of a conversation read back.
func (a *app) itemsOf(msgs []pi.Message) []*chatItem {
	var items []*chatItem
	calls := map[string]*chatItem{}
	for _, m := range msgs {
		switch m.Role {
		case "user":
			if t := stripNow(m.Text()); t != "" {
				items = append(items, a.newItem(itemUser, t))
			}
		case "assistant":
			if t, th := m.Text(), thinkingOf(m); t != "" || th != "" {
				it := a.newItem(itemAssistant, t)
				it.thinking = th
				items = append(items, it)
			}
			for _, b := range m.Blocks() {
				if b.Type == "toolCall" {
					it := a.newItem(itemTool, "")
					it.call, it.tool, it.args, it.done = b.ID, b.Name, b.Arguments, true
					calls[b.ID] = it
					items = append(items, it)
				}
			}
			if m.StopReason == "error" && m.ErrorMessage != "" {
				items = append(items, a.newItem(itemError, m.ErrorMessage))
			}
		case "toolResult":
			if it := calls[m.ToolCallID]; it != nil {
				it.result, it.failed = m.Text(), m.IsError
			}
		}
	}
	return items
}

// renameSession names a conversation, as pi names it in its lists too.
func (a *app) renameSession(path, name string) {
	name = strings.TrimSpace(name)
	if path == a.ag.chat.path || path == "" {
		a.ag.chat.name = name
		a.agentCall(map[string]any{"type": "set_session_name", "name": name}, func(json.RawMessage, error) { a.listSessions() })
		return
	}
	// Another: pi names the session it has open, so it opens it first.
	a.openSession(path)
	a.withAgent(func(c *pi.Client) {
		go func() {
			ctx := context.Background()
			c.Call(ctx, map[string]any{"type": "switch_session", "sessionPath": path})
			c.Call(ctx, map[string]any{"type": "set_session_name", "name": name})
			mygo.RunOnMain(func() {
				if a.ag.chat.path == path {
					a.ag.chat.name = name
				}
				a.listSessions()
			})
		}()
	})
}

// deleteSession puts a conversation in the Trash.
func (a *app) deleteSession(path string) error {
	if path == "" {
		return errors.New("not saved")
	}
	if err := mygo.Shell.TrashItem(path); err != nil {
		return err
	}
	if a.ag.chat.path == path {
		a.ag.chat = chat{}
		a.agentCall(map[string]any{"type": "new_session"}, nil)
	}
	a.listSessions()
	return nil
}

// sessionTitle is what a conversation is called: its name, or its first
// message.
func (a *app) sessionTitle(s pi.Session) string {
	if s.Name != "" {
		return s.Name
	}
	t := strings.Join(strings.Fields(stripNow(s.First)), " ")
	return orUntitled(clip(t, 60), a.tr.AgentNewChat())
}

// Models.

// loadModels asks pi which models there are, for the menu.
func (a *app) loadModels() {
	a.agentCall(map[string]any{"type": "get_available_models"}, func(data json.RawMessage, err error) {
		var m struct {
			Models []pi.Model `json:"models"`
		}
		if err == nil && json.Unmarshal(data, &m) == nil {
			a.ag.models = m.Models
		}
	})
}

// setModel chooses the model of the conversations, kept in NextTodo's
// settings: pi's own default stays as the user set it.
func (a *app) setModel(m pi.Model) {
	a.agentCall(map[string]any{"type": "set_model", "provider": m.Provider, "modelId": m.ID}, func(data json.RawMessage, err error) {
		if err != nil {
			a.toast = err.Error()
			return
		}
		var got pi.Model
		json.Unmarshal(data, &got)
		a.ag.state.Model = &got
		a.data.Prefs.AgentModel = m.Provider + "/" + m.ID
		a.changed()
		a.ag.levels = nil
		a.agentCall(map[string]any{"type": "get_state"}, func(data json.RawMessage, err error) {
			if err == nil {
				json.Unmarshal(data, &a.ag.state)
			}
		})
		a.withAgent(func(c *pi.Client) {
			go func() {
				lv := thinkingLevels(context.Background(), c)
				mygo.RunOnMain(func() { a.ag.levels = lv; a.redraw() })
			}()
		})
	})
}

func (a *app) setThinking(level string) {
	a.agentCall(map[string]any{"type": "set_thinking_level", "level": level}, func(_ json.RawMessage, err error) {
		if err != nil {
			a.toast = err.Error()
			return
		}
		a.ag.state.ThinkingLevel = level
		a.data.Prefs.AgentThinking = level
		a.changed()
	})
}

// languageName names a language of the app in English, for the model.
func languageName(lang string) string {
	if strings.HasPrefix(lang, "zh") {
		return "Simplified Chinese"
	}
	return "English"
}

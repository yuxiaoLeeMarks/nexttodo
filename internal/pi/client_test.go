package pi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The test binary plays pi when FAKE_PI is set: it answers commands as
// pi's RPC mode does, enough to test the client.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FAKE_PI"); mode != "" {
		fakePi(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakePi(mode string) {
	out := bufio.NewWriter(os.Stdout)
	var mu sync.Mutex
	emit := func(v any) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := json.Marshal(v)
		out.Write(append(b, '\n'))
		out.Flush()
	}
	if mode == "stubborn" {
		// Ignores its input ending, as a pi stuck in a tool.
		for {
			time.Sleep(time.Hour)
		}
	}
	in := bufio.NewReader(os.Stdin)
	answers := make(chan map[string]any, 1)
	var switching atomic.Int32 // new sessions being made, as pi does it: not at once
	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			return
		}
		var cmd map[string]any
		json.Unmarshal(line, &cmd)
		id, _ := cmd["id"].(string)
		ok := func(data any) {
			emit(map[string]any{"id": id, "type": "response", "command": cmd["type"], "success": true, "data": data})
		}
		switch cmd["type"] {
		case "get_state":
			ok(map[string]any{"model": map[string]any{"id": "m1", "name": "Model One", "provider": "p"}, "thinkingLevel": "medium", "sessionId": "s1"})
		case "crash":
			fmt.Fprintln(os.Stderr, "Error: no API key for provider p")
			os.Exit(3)
		case "extension_ui_response":
			answers <- cmd
		case "new_session":
			switching.Add(1)
			go func() {
				time.Sleep(150 * time.Millisecond)
				switching.Add(-1)
				ok(map[string]any{"cancelled": false})
			}()
		case "prompt":
			if switching.Load() > 0 {
				emit(map[string]any{"id": id, "type": "response", "command": "prompt", "success": false, "error": "sent while the session was switching"})
				continue
			}
			msg, _ := cmd["message"].(string)
			if msg == "" {
				emit(map[string]any{"id": id, "type": "response", "command": "prompt", "success": false, "error": "empty prompt"})
				continue
			}
			ok(map[string]any{"disposition": "started"})
			go func() {
				emit(map[string]any{"type": "agent_start"})
				emit(map[string]any{"type": "message_start", "message": map[string]any{"role": "user", "content": msg}})
				emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "user", "content": msg}})
				reply := "Hello there"
				if strings.Contains(msg, "ask") {
					emit(map[string]any{"type": "extension_ui_request", "id": "ui-1", "method": "confirm", "title": "Allow?", "message": "It will write."})
					a := <-answers
					reply = fmt.Sprintf("confirmed=%v", a["confirmed"])
				}
				if strings.Contains(msg, "tool") {
					emit(map[string]any{"type": "tool_execution_start", "toolCallId": "c1", "toolName": "mcp__nexttodo__create_task", "args": map[string]any{"title": "Read"}})
					emit(map[string]any{"type": "tool_execution_end", "toolCallId": "c1", "toolName": "mcp__nexttodo__create_task", "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"id":"t1"}`}}}, "isError": false})
				}
				emit(map[string]any{"type": "message_start", "message": map[string]any{"role": "assistant", "content": []any{}, "stopReason": "pending"}})
				for _, d := range []string{reply[:2], reply[2:]} {
					emit(map[string]any{"type": "message_update", "assistantMessageEvent": map[string]any{"type": "text_delta", "contentIndex": 0, "delta": d}})
				}
				emit(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": reply}}, "stopReason": "stop"}})
				emit(map[string]any{"type": "agent_end"})
				emit(map[string]any{"type": "agent_settled", "aborted": false})
			}()
		default:
			emit(map[string]any{"id": id, "type": "response", "command": cmd["type"], "success": false, "error": "Unknown command"})
		}
	}
}

type recorder struct {
	mu     sync.Mutex
	events []Event
	exit   chan error
}

func (r *recorder) add(e Event) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *recorder) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

// waitFor waits until an event of type typ has come.
func (r *recorder) waitFor(t *testing.T, typ string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range r.types() {
			if s == typ {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no %s in %v", typ, r.types())
}

func startFake(t *testing.T, mode string) (*Client, *recorder) {
	t.Helper()
	r := &recorder{exit: make(chan error, 1)}
	c, err := Start(Options{
		Bin:     os.Args[0],
		Env:     append(os.Environ(), "FAKE_PI="+mode),
		OnEvent: r.add,
		OnExit:  func(err error) { r.exit <- err },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c, r
}

func TestCallAndStream(t *testing.T) {
	c, r := startFake(t, "pi")
	ctx := context.Background()
	data, err := c.Call(ctx, map[string]any{"type": "get_state"})
	if err != nil {
		t.Fatal(err)
	}
	var st State
	json.Unmarshal(data, &st)
	if st.Model == nil || st.Model.Name != "Model One" || st.ThinkingLevel != "medium" {
		t.Fatalf("state: %s", data)
	}
	if _, err := c.Call(ctx, map[string]any{"type": "nope"}); err == nil || !strings.Contains(err.Error(), "Unknown command") {
		t.Fatalf("a failed command says why: %v", err)
	}
	if _, err := c.Call(ctx, map[string]any{"type": "prompt", "message": "use a tool"}); err != nil {
		t.Fatal(err)
	}
	r.waitFor(t, "agent_settled")
	var text strings.Builder
	var final Message
	var tool SessionEvent
	for _, e := range r.events {
		var se SessionEvent
		if err := e.Decode(&se); err != nil {
			t.Fatalf("%s: %v", e.Type, err)
		}
		switch e.Type {
		case "message_update":
			text.WriteString(se.AssistantMessageEvent.Delta)
		case "message_end":
			final = *se.Message
		case "tool_execution_end":
			tool = se
		}
	}
	// U+2028 is in a string, not between records.
	if text.String() != "Hello there" || final.Text() != "Hello there" || final.Role != "assistant" {
		t.Fatalf("streamed %q, final %q", text.String(), final.Text())
	}
	if tool.ToolName != "mcp__nexttodo__create_task" || tool.Result.Text() != `{"id":"t1"}` {
		t.Fatalf("tool: %+v", tool)
	}
}

// A prompt queued right after new_session waits for it: pi would start it
// in the session it leaves.
func TestQueueWaitsForBarriers(t *testing.T) {
	c, r := startFake(t, "pi")
	ns := c.Queue(map[string]any{"type": "new_session"})
	p := c.Queue(map[string]any{"type": "prompt", "message": "after the switch"})
	if _, err := p.Wait(context.Background()); err != nil {
		t.Fatalf("the prompt was sent too soon: %v", err)
	}
	if _, err := ns.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	r.waitFor(t, "agent_settled")
}

func TestDialog(t *testing.T) {
	c, r := startFake(t, "pi")
	if _, err := c.Call(context.Background(), map[string]any{"type": "prompt", "message": "ask first"}); err != nil {
		t.Fatal(err)
	}
	r.waitFor(t, "extension_ui_request")
	var req UIRequest
	r.mu.Lock()
	for _, e := range r.events {
		if e.Type == "extension_ui_request" {
			e.Decode(&req)
		}
	}
	r.mu.Unlock()
	if !req.Dialog() || req.Message != "It will write." || req.Title != "Allow?" {
		t.Fatalf("request: %+v", req)
	}
	yes := true
	if err := c.Send(UIResponse(req.ID, nil, &yes)); err != nil {
		t.Fatal(err)
	}
	r.waitFor(t, "agent_settled")
	r.mu.Lock()
	defer r.mu.Unlock()
	var se SessionEvent
	for _, e := range r.events {
		if e.Type == "message_end" {
			e.Decode(&se)
		}
	}
	if se.Message.Text() != "confirmed=true" {
		t.Fatalf("the answer reached pi: %q", se.Message.Text())
	}
}

func TestExit(t *testing.T) {
	c, r := startFake(t, "pi")
	_, err := c.Call(context.Background(), map[string]any{"type": "crash"})
	if err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Fatalf("a crash says why, from stderr: %v", err)
	}
	select {
	case err := <-r.exit:
		if !strings.Contains(err.Error(), "no API key") {
			t.Fatalf("exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no exit")
	}
	if _, err := c.Call(context.Background(), map[string]any{"type": "get_state"}); err == nil {
		t.Fatal("calls fail once it ended")
	}
	if err := c.Send(map[string]any{"type": "abort"}); err == nil {
		t.Fatal("sends fail once it ended")
	}
}

func TestCloseStubborn(t *testing.T) {
	c, _ := startFake(t, "stubborn")
	start := time.Now()
	c.Close()
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("closing took %v", d)
	}
	select {
	case <-c.Done():
	default:
		t.Fatal("not ended")
	}
}

func TestCallCancelled(t *testing.T) {
	c, _ := startFake(t, "pi")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Call(ctx, map[string]any{"type": "get_state"}); err == nil {
		t.Fatal("a cancelled call returns")
	}
}

func TestSessions(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, lines ...string) string {
		p := filepath.Join(dir, "--ws--", name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		return p
	}
	a := write("a.jsonl",
		`{"type":"session","version":3,"id":"a","timestamp":"2026-11-20T10:00:00Z","cwd":"/ws"}`,
		`{"type":"message","id":"1","parentId":null,"message":{"role":"system","content":"","sections":{"preamble":"\"role\":\"user\""}}}`,
		`{"type":"message","id":"2","parentId":"1","message":{"role":"user","content":[{"type":"text","text":"What is RAG?"}]}}`,
		`{"type":"message","id":"3","parentId":"2","message":{"role":"user","content":"second"}}`,
	)
	b := write("b.jsonl",
		`{"type":"session","version":3,"id":"b","timestamp":"2026-11-21T10:00:00Z","cwd":"/ws"}`,
		`{"type":"message","id":"1","parentId":null,"message":{"role":"user","content":"Plan the review"}}`,
		`{"type":"session_info","id":"2","parentId":"1","name":"Review plan"}`,
	)
	write("junk.jsonl", `not json`)
	os.Chtimes(a, time.Now(), time.Now().Add(-time.Hour))
	os.Chtimes(b, time.Now(), time.Now())
	var cache SessionCache
	got := Sessions(dir, &cache)
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "a" {
		t.Fatalf("sessions: %+v", got)
	}
	if got[0].Name != "Review plan" || got[0].First != "Plan the review" || got[1].First != "What is RAG?" || got[1].Name != "" {
		t.Fatalf("read: %+v", got)
	}
	if !got[1].Created.Equal(time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("created: %v", got[1].Created)
	}
	// A file changed is read again; one removed goes.
	os.WriteFile(b, []byte(`{"type":"session","version":3,"id":"b","timestamp":"2026-11-21T10:00:00Z"}`+"\n"+`{"type":"session_info","name":"Renamed"}`+"\n"), 0o644)
	os.Remove(a)
	got = Sessions(dir, &cache)
	if len(got) != 1 || got[0].Name != "Renamed" {
		t.Fatalf("again: %+v", got)
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "pi")
	os.WriteFile(bin, []byte("#!/bin/sh\necho 1.6.0\n"), 0o755)
	in, err := Find(context.Background(), bin)
	if err != nil || in.Bin != bin {
		t.Fatalf("found %+v, %v", in, err)
	}
	if !strings.HasPrefix(lookup(in.Env, "PATH"), dir+":") {
		t.Fatalf("pi's directory comes first in PATH: %s", lookup(in.Env, "PATH"))
	}
	if v, err := in.Version(context.Background()); err != nil || v != "1.6.0" {
		t.Fatalf("version %q, %v", v, err)
	}
	if _, err := Find(context.Background(), filepath.Join(dir, "missing")); err != ErrNotFound {
		t.Fatalf("a path without pi: %v", err)
	}
	if compareVersions("/v22.10.1/bin/pi", "/v22.9.0/bin/pi") <= 0 || compareVersions("/v20.1.0/pi", "/v22.0.0/pi") >= 0 {
		t.Fatal("versions compare as numbers")
	}
	env := Setenv([]string{"A=1", "PATH=/x", "B=2"}, "PATH", "/y")
	if lookup(env, "PATH") != "/y" || len(env) != 3 {
		t.Fatalf("setenv: %v", env)
	}
}

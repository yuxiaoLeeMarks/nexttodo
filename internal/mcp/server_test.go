package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func start(t *testing.T) *Server {
	t.Helper()
	s := NewServer("test", "1.0", []Tool{
		{
			Name: "echo", Description: "Says it back", ReadOnly: true,
			Input: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			Call: func(ctx context.Context, args json.RawMessage) (string, error) {
				var a struct{ Text string }
				json.Unmarshal(args, &a)
				return a.Text, nil
			},
		},
		{Name: "fail", Call: func(context.Context, json.RawMessage) (string, error) { return "", errors.New("no such task") }},
		{Name: "panic", Call: func(context.Context, json.RawMessage) (string, error) { panic("boom") }},
	})
	s.Instructions = "Test tools."
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func post(t *testing.T, s *Server, token, body string, header ...string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", s.URL(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestHandshakeAndTools(t *testing.T) {
	s := start(t)
	code, body := post(t, s, s.Token(), `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"pi","version":"1"}}}`)
	if code != 200 || !strings.Contains(body, `"protocolVersion":"2025-03-26"`) || !strings.Contains(body, `"instructions":"Test tools."`) {
		t.Fatalf("initialize: %d %s", code, body)
	}
	if code, _ := post(t, s, s.Token(), `{"jsonrpc":"2.0","method":"notifications/initialized"}`); code != http.StatusAccepted {
		t.Fatalf("a notification is accepted, got %d", code)
	}
	_, body = post(t, s, s.Token(), `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var list struct {
		Result struct {
			Tools []struct {
				Name        string
				InputSchema json.RawMessage
				Annotations struct{ ReadOnlyHint bool }
			}
		}
	}
	json.Unmarshal([]byte(body), &list)
	if len(list.Result.Tools) != 3 || list.Result.Tools[0].Name != "echo" || !list.Result.Tools[0].Annotations.ReadOnlyHint {
		t.Fatalf("tools/list: %s", body)
	}
	if !strings.Contains(string(list.Result.Tools[1].InputSchema), `"object"`) {
		t.Fatalf("a tool without a schema takes an empty object: %s", list.Result.Tools[1].InputSchema)
	}
	_, body = post(t, s, s.Token(), `{"jsonrpc":"2.0","id":"a","method":"tools/call","params":{"name":"echo","arguments":{"text":"你好"}}}`)
	if !strings.Contains(body, `"text":"你好"`) || strings.Contains(body, "isError") {
		t.Fatalf("tools/call: %s", body)
	}
	_, body = post(t, s, s.Token(), `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"fail"}}`)
	if !strings.Contains(body, `"isError":true`) || !strings.Contains(body, "no such task") {
		t.Fatalf("a failing tool reports why: %s", body)
	}
	_, body = post(t, s, s.Token(), `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"panic"}}`)
	if !strings.Contains(body, `"isError":true`) || !strings.Contains(body, "boom") {
		t.Fatalf("a panicking tool fails alone: %s", body)
	}
	_, body = post(t, s, s.Token(), `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope"}}`)
	if !strings.Contains(body, `"code":-32602`) {
		t.Fatalf("an unknown tool is an error: %s", body)
	}
	_, body = post(t, s, s.Token(), `{"jsonrpc":"2.0","id":6,"method":"sampling/whatever"}`)
	if !strings.Contains(body, `"code":-32601`) {
		t.Fatalf("an unknown method: %s", body)
	}
	_, body = post(t, s, s.Token(), `[{"jsonrpc":"2.0","id":7,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"}]`)
	if !strings.HasPrefix(body, "[") || !strings.Contains(body, `"id":7`) {
		t.Fatalf("a batch: %s", body)
	}
}

func TestRefusals(t *testing.T) {
	s := start(t)
	ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	if code, _ := post(t, s, "", ping); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
	if code, _ := post(t, s, "wrong", ping); code != http.StatusUnauthorized {
		t.Errorf("a wrong token: %d", code)
	}
	if code, _ := post(t, s, s.Token(), ping, "Origin", "https://evil.example"); code != http.StatusForbidden {
		t.Errorf("a page elsewhere: %d", code)
	}
	if code, _ := post(t, s, s.Token(), ping, "Origin", "http://localhost:3000"); code != 200 {
		t.Errorf("a page of this Mac: %d", code)
	}
	req, _ := http.NewRequest("GET", s.URL(), nil)
	req.Header.Set("Authorization", "Bearer "+s.Token())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", res.StatusCode)
	}
	if code, body := post(t, s, s.Token(), `{not json`); !strings.Contains(body, "-32700") {
		t.Errorf("not JSON: %d %s", code, body)
	}
	if !strings.HasPrefix(s.URL(), "http://127.0.0.1:") {
		t.Errorf("listens on the loopback interface: %s", s.URL())
	}
}

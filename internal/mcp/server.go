// Package mcp serves tools over the Model Context Protocol's streamable
// HTTP transport: the least of it an agent on the same Mac needs to list
// the tools and call them. It listens on the loopback interface only and
// answers only requests that bear its token.
//
// It speaks JSON-RPC over POST, one message or a batch, and answers with
// JSON, never a stream: the tools answer quickly, and progress is not
// reported. GET, for a stream of the server's own messages, is refused,
// as the transport allows.
package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Tool is a tool the server offers.
type Tool struct {
	Name        string
	Title       string // for people, shown in place of the name
	Description string
	// Input is the JSON Schema of the arguments, an object.
	Input json.RawMessage
	// ReadOnly says the tool changes nothing, as clients may ask.
	ReadOnly bool
	// Call runs the tool. An error is reported to the agent as the tool's
	// failure, its text the reason, for the agent to correct its call.
	Call func(ctx context.Context, args json.RawMessage) (string, error)
}

// Server is an MCP server of tools.
type Server struct {
	Name, Version string
	// Instructions tell the agent what the tools are for, once.
	Instructions string

	token string
	tools []Tool

	mu  sync.Mutex
	srv *http.Server
	url string
}

// NewServer makes a server of tools with a token of its own, which
// requests must bear.
func NewServer(name, version string, tools []Tool) *Server {
	var b [24]byte
	rand.Read(b[:])
	return &Server{Name: name, Version: version, tools: tools, token: hex.EncodeToString(b[:])}
}

// Token is what requests bear, as "Authorization: Bearer <token>".
func (s *Server) Token() string { return s.token }

// URL is where the server listens, once started.
func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.url
}

// Start listens on a port of the loopback interface the system picks.
func (s *Server) Start() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return s.url, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	s.url = "http://" + ln.Addr().String() + "/mcp"
	s.srv = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Println("mcp:", err)
		}
	}()
	return s.url, nil
}

// Close stops listening.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	err := s.srv.Close()
	s.srv = nil
	return err
}

// The protocol's versions the server knows, the latest first.
var versions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeNoMethod       = -32601
	codeInvalidParams  = -32602
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A page in a browser could reach a port of the loopback interface:
	// requests from one, which say where they come from, are refused, as
	// are those without the token.
	if o := r.Header.Get("Origin"); o != "" && !loopback(o) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	auth, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(auth), []byte(s.token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
	case http.MethodDelete:
		w.WriteHeader(http.StatusOK) // sessions are not kept: nothing to end
		return
	default:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	body = bytes.TrimSpace(body)
	var out []response
	batch := len(body) > 0 && body[0] == '['
	var reqs []request
	if batch {
		err = json.Unmarshal(body, &reqs)
	} else {
		var one request
		err = json.Unmarshal(body, &one)
		reqs = []request{one}
	}
	if err != nil {
		writeJSON(w, response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "parse error"}})
		return
	}
	for _, q := range reqs {
		if res, ok := s.handle(r.Context(), q); ok {
			out = append(out, res)
		}
	}
	switch {
	case len(out) == 0:
		// Notifications and responses only: accepted, nothing to say.
		w.WriteHeader(http.StatusAccepted)
	case batch:
		writeJSON(w, out)
	default:
		writeJSON(w, out[0])
	}
}

// handle answers a request; notifications get no answer.
func (s *Server) handle(ctx context.Context, q request) (response, bool) {
	if q.Method == "" {
		// A response from the client, which the server never asks for.
		return response{}, false
	}
	notification := len(q.ID) == 0 || string(q.ID) == "null"
	res := response{JSONRPC: "2.0", ID: q.ID}
	result, err := s.dispatch(ctx, q)
	if notification {
		return response{}, false
	}
	if err != nil {
		var re *rpcError
		if !errors.As(err, &re) {
			re = &rpcError{codeInvalidParams, err.Error()}
		}
		res.Error = re
	} else {
		res.Result = result
	}
	return res, true
}

func (e *rpcError) Error() string { return e.Message }

func (s *Server) dispatch(ctx context.Context, q request) (any, error) {
	switch q.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(q.Params, &p)
		v := versions[0]
		if slices.Contains(versions, p.ProtocolVersion) {
			v = p.ProtocolVersion
		}
		res := map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		}
		if s.Instructions != "" {
			res["instructions"] = s.Instructions
		}
		return res, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			in := t.Input
			if len(in) == 0 {
				in = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			m := map[string]any{"name": t.Name, "description": t.Description, "inputSchema": in}
			ann := map[string]any{"readOnlyHint": t.ReadOnly, "destructiveHint": false, "openWorldHint": false}
			if t.Title != "" {
				m["title"] = t.Title
				ann["title"] = t.Title
			}
			m["annotations"] = ann
			list = append(list, m)
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(q.Params, &p); err != nil {
			return nil, &rpcError{codeInvalidParams, "invalid params"}
		}
		i := slices.IndexFunc(s.tools, func(t Tool) bool { return t.Name == p.Name })
		if i < 0 {
			return nil, &rpcError{codeInvalidParams, fmt.Sprintf("unknown tool %q", p.Name)}
		}
		args := p.Arguments
		if len(args) == 0 || string(args) == "null" {
			args = json.RawMessage("{}")
		}
		text, err := s.call(ctx, s.tools[i], args)
		if err != nil {
			return map[string]any{"content": []any{map[string]any{"type": "text", "text": err.Error()}}, "isError": true}, nil
		}
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}, nil
	case "notifications/initialized", "notifications/cancelled", "notifications/progress", "notifications/roots/list_changed":
		return nil, nil
	case "resources/list":
		return map[string]any{"resources": []any{}}, nil
	case "resources/templates/list":
		return map[string]any{"resourceTemplates": []any{}}, nil
	case "prompts/list":
		return map[string]any{"prompts": []any{}}, nil
	}
	return nil, &rpcError{codeNoMethod, "method not found: " + q.Method}
}

// call runs a tool, its panic its failure rather than the server's.
func (s *Server) call(ctx context.Context, t Tool, args json.RawMessage) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("the tool failed: %v", r)
		}
	}()
	return t.Call(ctx, args)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// loopback reports whether an origin is of this Mac.
func loopback(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

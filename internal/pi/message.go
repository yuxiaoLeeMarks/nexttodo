package pi

import (
	"encoding/json"
	"strings"
)

// Message is a message of a conversation, as pi's AgentMessage: of the
// user, of the assistant, a tool's result, or of pi itself.
type Message struct {
	Role string `json:"role"` // user, assistant, toolResult, system, custom…
	// Content is a string or a list of blocks; Blocks reads either.
	Content json.RawMessage `json:"content"`

	// The assistant's.
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	StopReason   string `json:"stopReason,omitempty"` // stop, length, toolUse, error, aborted
	ErrorMessage string `json:"errorMessage,omitempty"`

	// A tool's result.
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	IsError    bool   `json:"isError,omitempty"`

	// A custom message, as an extension's.
	CustomType string `json:"customType,omitempty"`
	Display    bool   `json:"display,omitempty"`

	Timestamp int64 `json:"timestamp"` // in milliseconds since 1970
}

// Block is a block of a message's content.
type Block struct {
	Type      string          `json:"type"` // text, thinking, toolCall, image
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Redacted  bool            `json:"redacted,omitempty"`
	ID        string          `json:"id,omitempty"`   // a tool call's
	Name      string          `json:"name,omitempty"` // the tool called
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Data      string          `json:"data,omitempty"` // an image's, in base64
	MimeType  string          `json:"mimeType,omitempty"`
}

// Blocks reads the content, a string being one block of text.
func (m Message) Blocks() []Block {
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return []Block{{Type: "text", Text: s}}
	}
	var bs []Block
	json.Unmarshal(m.Content, &bs)
	return bs
}

// Text is the text of the message's blocks of text, joined.
func (m Message) Text() string {
	var parts []string
	for _, b := range m.Blocks() {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// AssistantEvent is what a "message_update" says changed of the message
// the assistant is writing.
type AssistantEvent struct {
	Type         string          `json:"type"` // text_delta, thinking_delta, toolcall_start, …
	ContentIndex int             `json:"contentIndex"`
	Delta        string          `json:"delta,omitempty"`
	Content      string          `json:"content,omitempty"` // a block's whole text, as it ends
	ID           string          `json:"id,omitempty"`      // toolcall_start's
	ToolName     string          `json:"toolName,omitempty"`
	ToolCall     *Block          `json:"toolCall,omitempty"` // toolcall_end's
	Reason       string          `json:"reason,omitempty"`
	Error        json.RawMessage `json:"error,omitempty"`
}

// SessionEvent holds the fields of the session's events that NextTodo
// reads; Type says which apply.
type SessionEvent struct {
	Type string `json:"type"`

	// message_start, message_end, turn_end.
	Message *Message `json:"message,omitempty"`
	// message_update.
	AssistantMessageEvent *AssistantEvent `json:"assistantMessageEvent,omitempty"`

	// tool_execution_start, _update, _end.
	ToolCallID    string          `json:"toolCallId,omitempty"`
	ToolName      string          `json:"toolName,omitempty"`
	Args          json.RawMessage `json:"args,omitempty"`
	Result        *ToolResult     `json:"result,omitempty"`
	PartialResult *ToolResult     `json:"partialResult,omitempty"`
	IsError       bool            `json:"isError,omitempty"`
	DurationMs    float64         `json:"durationMs,omitempty"`

	// agent_settled, compaction_end.
	Aborted bool `json:"aborted,omitempty"`
	// agent_end, compaction_end.
	WillRetry bool `json:"willRetry,omitempty"`

	// auto_retry_start, auto_retry_end, compaction_end.
	Attempt      int    `json:"attempt,omitempty"`
	MaxAttempts  int    `json:"maxAttempts,omitempty"`
	DelayMs      int    `json:"delayMs,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	Success      bool   `json:"success,omitempty"`
	FinalError   string `json:"finalError,omitempty"`

	// queue_update.
	Steering []string `json:"steering,omitempty"`
	FollowUp []string `json:"followUp,omitempty"`

	// session_info_changed.
	Name string `json:"name,omitempty"`

	// extension_error; an object or a string.
	Error json.RawMessage `json:"error,omitempty"`
}

// ErrorText is the event's error, as text.
func (e SessionEvent) ErrorText() string {
	var s string
	if json.Unmarshal(e.Error, &s) == nil {
		return s
	}
	var o struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(e.Error, &o) == nil && o.Message != "" {
		return o.Message
	}
	return string(e.Error)
}

// UIRequest is an extension's request of the interface: a dialog, which
// waits for its answer (select, confirm, input, editor), or something to
// show (notify, setStatus, setWidget, setTitle, set_editor_text).
type UIRequest struct {
	ID          string   `json:"id"`
	Method      string   `json:"method"`
	Title       string   `json:"title,omitempty"`
	Message     string   `json:"message,omitempty"`
	Options     []string `json:"options,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Prefill     string   `json:"prefill,omitempty"`
	NotifyType  string   `json:"notifyType,omitempty"` // info, warning, error
	Text        string   `json:"text,omitempty"`
	Timeout     int      `json:"timeout,omitempty"` // in milliseconds
}

// Dialog reports whether the request waits for an answer.
func (r UIRequest) Dialog() bool {
	switch r.Method {
	case "select", "confirm", "input", "editor":
		return true
	}
	return false
}

// UIResponse answers a dialog: with a value, chosen or typed, as
// confirmed, or cancelled.
func UIResponse(id string, value *string, confirmed *bool) map[string]any {
	m := map[string]any{"type": "extension_ui_response", "id": id}
	switch {
	case value != nil:
		m["value"] = *value
	case confirmed != nil:
		m["confirmed"] = *confirmed
	default:
		m["cancelled"] = true
	}
	return m
}

// ToolResult is the result of a tool, as it reports it.
type ToolResult struct {
	Content []Block `json:"content"`
}

// Text is the text of the result's blocks of text, joined.
func (r *ToolResult) Text() string {
	if r == nil {
		return ""
	}
	var parts []string
	for _, b := range r.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// Model is a model pi can use.
type Model struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Provider  string   `json:"provider"`
	Reasoning bool     `json:"reasoning"`
	Input     []string `json:"input"`
	// ContextWindow is how many tokens it reads at most.
	ContextWindow int `json:"contextWindow"`
}

// State is the state of pi's session, as get_state answers.
type State struct {
	Model         *Model `json:"model"`
	ThinkingLevel string `json:"thinkingLevel"`
	IsStreaming   bool   `json:"isStreaming"`
	SessionFile   string `json:"sessionFile"`
	SessionID     string `json:"sessionId"`
	SessionName   string `json:"sessionName"`
	MessageCount  int    `json:"messageCount"`
}

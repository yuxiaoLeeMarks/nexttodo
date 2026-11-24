// Package pi runs the pi agent (github.com/earendil-works/pi) in its RPC
// mode, as NextTodo's agent: pi keeps the conversation, calls the model
// and the tools, and reads its own settings, models and MCP servers, so
// that the agent of the app is the one its user has set up for the
// terminal. This package starts the process, speaks its protocol of JSON
// lines, and finds the executable from an app opened in the Finder,
// which has none of the shell's PATH.
package pi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Event is a record pi writes that is not the answer to a command: an
// event of the session, as "message_update", or a request of an
// extension's interface, "extension_ui_request".
type Event struct {
	Type string
	Raw  json.RawMessage
}

// Decode reads the event into v.
func (e Event) Decode(v any) error { return json.Unmarshal(e.Raw, v) }

// Options say how to run pi.
type Options struct {
	Bin  string   // the executable
	Args []string // after "--mode rpc"
	Env  []string // the whole environment, or nil for this process's
	Dir  string   // the working directory, where pi looks for .pi/
	// OnEvent receives the events in order, on the goroutine reading them:
	// it must not block for long, nor call the client.
	OnEvent func(Event)
	// OnExit is called once the process has ended, with why, as the end
	// of what it wrote to stderr says.
	OnExit func(err error)
}

// Client is a running pi.
type Client struct {
	o     Options
	cmd   *exec.Cmd
	stdin io.WriteCloser

	wmu sync.Mutex // writes

	mu      sync.Mutex
	next    int
	waiting map[string]*Pending
	done    chan struct{}
	err     error // why the process ended
	stderr  tail

	queue chan *Pending // commands to send, in order
}

type response struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// ErrExited is the error of commands to a pi that has ended.
var ErrExited = errors.New("pi is not running")

// Start runs pi in RPC mode.
func Start(o Options) (*Client, error) {
	args := append([]string{"--mode", "rpc"}, o.Args...)
	cmd := exec.Command(o.Bin, args...)
	cmd.Dir = o.Dir
	cmd.Env = o.Env
	// Its own process group, so that quitting ends what it started too.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	c := &Client{o: o, cmd: cmd, stdin: stdin, waiting: map[string]*Pending{}, done: make(chan struct{}), queue: make(chan *Pending, 256)}
	cmd.Stderr = &c.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go c.read(stdout)
	go c.send()
	return c, nil
}

// read reads the records pi writes, a JSON object a line, until it ends.
func (c *Client) read(r io.Reader) {
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		// Lines end at LF alone: U+2028 and U+2029 are valid in JSON strings.
		line, err := br.ReadBytes('\n')
		line = bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
		if len(bytes.TrimSpace(line)) > 0 {
			c.record(line)
		}
		if err != nil {
			break
		}
	}
	err := c.cmd.Wait()
	if err == nil {
		err = ErrExited
	} else if s := c.stderr.String(); s != "" {
		err = fmt.Errorf("%w: %s", err, lastLines(s, 6))
	}
	c.mu.Lock()
	c.err = err
	waiting := c.waiting
	c.waiting = map[string]*Pending{}
	c.mu.Unlock()
	close(c.done)
	for _, p := range waiting {
		p.answer(response{}, false)
	}
	if c.o.OnExit != nil {
		c.o.OnExit(err)
	}
}

func (c *Client) record(line []byte) {
	var head struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if json.Unmarshal(line, &head) != nil {
		return // not JSON: pi's stdout is JSON alone, but be lenient
	}
	if head.Type == "response" {
		var res response
		json.Unmarshal(line, &res)
		c.mu.Lock()
		p := c.waiting[res.ID]
		delete(c.waiting, res.ID)
		c.mu.Unlock()
		if p != nil {
			p.answer(res, true)
		}
		return
	}
	if c.o.OnEvent != nil {
		c.o.OnEvent(Event{Type: head.Type, Raw: append(json.RawMessage(nil), line...)})
	}
}

// Call sends a command, as {"type": "prompt", "message": "…"}, and waits
// for its answer: its data, or why it failed.
func (c *Client) Call(ctx context.Context, cmd map[string]any) (json.RawMessage, error) {
	return c.Queue(cmd).Wait(ctx)
}

// Pending is a command queued, its answer to come.
type Pending struct {
	c        *Client
	id       string
	cmd      map[string]any
	ch       chan response
	answered chan struct{} // closed with the answer, or as pi ends
	once     sync.Once
}

// barriers are the commands pi does not finish before it reads the next:
// a prompt sent right after new_session starts in the session it leaves,
// and is lost. The commands queued after them wait for their answers.
var barriers = map[string]bool{
	"new_session": true, "switch_session": true, "fork": true, "clone": true,
	"abort": true, "set_model": true, "set_thinking_level": true,
}

// Queue sends a command after those queued before it, and after the
// answers of those that change the session, as new_session; it returns
// at once, with the answer to wait for.
func (c *Client) Queue(cmd map[string]any) *Pending {
	p := &Pending{c: c, cmd: cmd, ch: make(chan response, 1), answered: make(chan struct{})}
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		p.answer(response{}, false)
		return p
	}
	c.next++
	p.id = "nt-" + strconv.Itoa(c.next)
	c.waiting[p.id] = p
	c.mu.Unlock()
	select {
	case c.queue <- p:
	case <-c.done:
	}
	return p
}

// send writes the commands queued, one after another, and waits for the
// answer of each barrier before the next.
func (c *Client) send() {
	for {
		var p *Pending
		select {
		case p = <-c.queue:
		case <-c.done:
			return
		}
		msg := make(map[string]any, len(p.cmd)+1)
		for k, v := range p.cmd {
			msg[k] = v
		}
		msg["id"] = p.id
		if err := c.Send(msg); err != nil {
			c.forget(p.id)
			p.answer(response{Error: err.Error()}, true)
			continue
		}
		if t, _ := p.cmd["type"].(string); barriers[t] {
			select {
			case <-p.answered:
			case <-c.done:
				return
			}
		}
	}
}

// answer gives the command its answer, once; ok false says pi ended.
func (p *Pending) answer(res response, ok bool) {
	p.once.Do(func() {
		if ok {
			p.ch <- res
		}
		close(p.ch)
		close(p.answered)
	})
}

// Wait waits for the answer: its data, or why the command failed.
func (p *Pending) Wait(ctx context.Context) (json.RawMessage, error) {
	select {
	case res, ok := <-p.ch:
		if !ok {
			return nil, p.c.Err()
		}
		if !res.Success {
			if res.Error == "" {
				res.Error = res.Command + " failed"
			}
			return nil, errors.New(res.Error)
		}
		return res.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) forget(id string) {
	c.mu.Lock()
	delete(c.waiting, id)
	c.mu.Unlock()
}

// Send writes a record without waiting for an answer, as the answer to an
// extension's dialog.
func (c *Client) Send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	select {
	case <-c.done:
		return c.Err()
	default:
	}
	_, err = c.stdin.Write(append(b, '\n'))
	return err
}

// Done is closed once the process has ended.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err is why the process ended, or nil while it runs.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close ends pi: it closes its input, which it takes to quit, and ends
// it, and what it started, if it has not within a while.
func (c *Client) Close() error {
	c.wmu.Lock()
	c.stdin.Close()
	c.wmu.Unlock()
	select {
	case <-c.done:
		return nil
	case <-time.After(1500 * time.Millisecond):
	}
	if p := c.cmd.Process; p != nil {
		syscall.Kill(-p.Pid, syscall.SIGTERM)
	}
	select {
	case <-c.done:
	case <-time.After(1500 * time.Millisecond):
		if p := c.cmd.Process; p != nil {
			syscall.Kill(-p.Pid, syscall.SIGKILL)
		}
		<-c.done
	}
	return nil
}

// tail keeps the last 8 KB written to it.
type tail struct {
	mu sync.Mutex
	b  []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > 8<<10 {
		t.b = append([]byte(nil), t.b[len(t.b)-8<<10:]...)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.b))
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

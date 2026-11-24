package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"nexttodo/internal/mcp"
	"nexttodo/internal/pi"
)

// TestRealPi runs the pi of this Mac as the assistant and as the
// translator, and checks what tools each can call: codemode's ALL_TOOLS,
// which lists what scripts reach, not only what is declared. It asks the
// model to run a script, so it is run only when NT_REAL_PI is set.
func TestRealPi(t *testing.T) {
	if os.Getenv("NT_REAL_PI") == "" {
		t.Skip("NT_REAL_PI is not set")
	}
	in, err := pi.Find(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	a := testApp("en")
	a.path = filepath.Join(t.TempDir(), "data.json")
	srv := mcp.NewServer("nexttodo", "test", a.agentTools())
	url, err := srv.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	dir := a.agentDir()
	if err := writeWorkspace(dir, url); err != nil {
		t.Fatal(err)
	}
	list := func(args []string, dir string) []string {
		events := make(chan pi.Event, 1024)
		c, err := pi.Start(pi.Options{Bin: in.Bin, Args: args, Dir: dir,
			Env:     pi.Setenv(in.Env, "NEXTTODO_MCP_TOKEN", srv.Token()),
			OnEvent: func(e pi.Event) { events <- e }})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, err := c.Call(ctx, map[string]any{"type": "prompt", "message": "Call the codemode tool with exactly this script, then reply only OK:\nreturn ALL_TOOLS.map(t => t.name).join(\"\\n\")"}); err != nil {
			t.Fatal(err)
		}
		var names []string
		for {
			select {
			case e := <-events:
				var se pi.SessionEvent
				e.Decode(&se)
				if e.Type == "tool_execution_end" && se.ToolName == "codemode" {
					for _, l := range strings.Split(se.Result.Text(), "\n") {
						if l = strings.TrimSpace(l); strings.HasPrefix(l, "mcp__") || !strings.ContainsAny(l, " :") && l != "" {
							names = append(names, l)
						}
					}
				}
				if e.Type == "agent_settled" {
					return names
				}
			case <-ctx.Done():
				t.Fatal("no answer")
			}
		}
	}
	// Each tool reached must be one the patterns let in, and some of those
	// known to write, send or run code must not be.
	check := func(who string, names []string, allow []string, nexttodo bool) {
		t.Helper()
		if len(names) == 0 {
			t.Fatalf("%s: codemode listed no tools", who)
		}
		for _, n := range names {
			if strings.HasPrefix(n, "mcp__nexttodo__") {
				if !nexttodo {
					t.Errorf("%s can call %s", who, n)
				}
				continue
			}
			if !toolMatches(allow, n) || toolMatches(sendingTools, n) {
				t.Errorf("%s can call %s", who, n)
			}
		}
		for _, bad := range []string{"bash", "edit", "write", "mcp__magpie__run_code",
			"mcp__magpie__firecrawl__firecrawl_search_feedback", "mcp__magpie__github__create_issue",
			"mcp__magpie__github__push_files", "mcp__magpie__notion__notion_update_page",
			"mcp__magpie__notion__notion_delete_pages"} {
			if slices.Contains(names, bad) {
				t.Errorf("%s can call %s", who, bad)
			}
		}
	}

	names := list(a.assistantArgs(), dir)
	t.Logf("the assistant can call %d tools:\n%s", len(names), strings.Join(names, "\n"))
	if !slices.Contains(names, "mcp__nexttodo__create_task") {
		t.Error("the assistant has NextTodo's tools")
	}
	check("the assistant", names, lookupTools, true)

	names = list(translatorArgs(), t.TempDir())
	t.Logf("the translator can call %d tools:\n%s", len(names), strings.Join(names, "\n"))
	check("the translator", names, searchTools, false)
}

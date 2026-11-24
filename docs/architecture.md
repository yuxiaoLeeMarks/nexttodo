# How NextTodo fits together

NextTodo is one Go program, `package main` at the root, with two packages under `internal/` that know nothing of the app. The interface is [MyGo](https://mygo.egoist.dev/)'s immediate-mode UI: every frame, the page functions describe what is on screen from the app's state, and MyGo diffs, lays out and draws it with Metal. There is no separate model layer to keep in step with views: the state is `*app` and its `*Data`.

```
             ┌──────────────── NextTodo (main thread) ───────────────┐
  you ──────▶│ pages: overview, tasks, schedule, projects, …         │
             │ assistant.go, translate.go                            │
             │      │ draws             ▲ inbox: pi's events, posted  │
             │      ▼                   │ in a batch, one redraw     │
             │ app state + Data ◀── agent_tools.go (on main) ◀─┐     │
             └──────────│───────────────────────│──────────────│─────┘
                        │ data.json              │ JSONL stdin/out  │ MCP over HTTP,
                        ▼                        ▼                  │ 127.0.0.1 + token
                  ~/Library/…/NextTodo       pi --mode rpc ─────────┘
                                             (models, keys, MCP servers: the user's)
```

## The data

`store.go` holds `Data`: tasks, events, projects, directions, categories, kinds, subscriptions and the settings (`Prefs`), saved as one JSON file, `~/Library/Application Support/NextTodo/data.json`, a moment after each change (`app.changed`), with a copy a day in `Backups/`. Data that cannot be read is never saved over: the app asks again.

Everything that changes the data runs on the main thread, so there are no locks around it. Work that may take time (reading calendars on the web, finding pi, reading sessions) runs on goroutines and hands its result back with `mygo.RunOnMain`, or, for pi's stream of events, through the inbox (below).

**Undo** (`undo.go`): `remember()` copies the data before a change, `settle()` after; `restore()` undoes it by a three-way merge, so that what changed since is kept. Deletions by hand are one step each, offered in a toast and on ⌘Z. Each change of the assistant's is one step too, gathered in the run of the question that made it (`agentRun`), and the run's Undo restores its steps from the last.

## The assistant

The assistant is not an agent of NextTodo's: it is the user's **pi**, run as a child process in RPC mode, so that it has the models, keys, settings and MCP servers they set up for the terminal. NextTodo adds what pi lacks: its tools for the user's data, the conversation's interface, and limits on what pi may do from inside the app.

| | |
|---|---|
| `internal/pi` | Runs `pi --mode rpc` and speaks its protocol: commands with ids and their answers, events, extension dialogs. `Find` looks for pi from an app opened in the Finder, which has none of the shell's PATH, by asking the login shell and the usual install places. `Sessions` lists pi's session files, with a cache by modification time. |
| `internal/mcp` | A small MCP server over streamable HTTP, on `127.0.0.1` at a random port, that takes requests only with its bearer token, made new at each launch, and no browser's (`Origin`). |
| `agent_tools.go` | NextTodo's 14 tools: read the overview, search, list and get; create and update tasks, events, projects, journal entries and directions. None deletes. Each runs on the main thread, and each write is an undo step in the current run. |
| `agent.go` | Starts and stops pi and the server, sends the user's messages, takes in pi's events, keeps the conversation, lists and opens sessions, chooses models. |
| `assistant.go` | The Assistant page, the ask bar of the overview (⌘J), and how a conversation is drawn: messages in Markdown as they stream, thinking folded, tool calls as one line each, and the card of what a question changed. |
| `prompts/assistant.md` | What pi is told it is, appended to nothing: `--system-prompt` replaces pi's coding preamble. A line after it names the app's language. |

**How pi is run.** In `Application Support/NextTodo/Assistant`, where `.pi/mcp.json` names NextTodo's server (its token read from the environment, never written down) and `sessions/` keeps the conversations. With `--approve` (no prompts for tools), `--session-dir`, and a list of tools, `--tools`: `codemode`, NextTodo's, and MCP tools that look things up, by the patterns in `searchTools` and `lookupTools`. pi applies the list to MCP tools too, those reached from codemode scripts included, so nothing that writes elsewhere (GitHub, Notion) nor `bash`, `edit`, `write` is callable. Settings can add pi's file reading tools, or, with "Use all of pi's MCP tools", trade the list for `--exclude-tools` of the shell and the editing tools. `TestRealPi` (`NT_REAL_PI=1`) asks the real pi what its codemode can reach, under both lists.

**The order of commands.** pi reads its next command before it has finished some: a prompt sent right after `new_session` starts in the session being left, and is lost. `Client.Queue` sends commands in order, and after a *barrier* (`new_session`, `switch_session`, `abort`, `set_model`, …) waits for its answer before the next. If a session switch fails or an extension cancels it, what was queued after it fails too, rather than go to the session left. The app can thus fire `new_session` then `prompt` without waiting.

**Events to the interface.** pi's events arrive on the goroutine reading its output. `mygo.RunOnMain` blocks until the main thread has run the function, so posting each event with it would slow a stream to the frame rate and could deadlock as the app quits. The `inbox` gathers them instead: the first event schedules one flush on the main thread, which runs everything gathered since, then redraws once. Text deltas append to the message streaming, drawn with `markdown.Stream`.

**A conversation.** `askAgent` adds the user's message, prefixed by a `<now>` tag of the local date, time and UTC offset that pi's session keeps and the app strips when showing it. While a run goes on, a new message steers it. `agent_settled` ends the run: the card of its changes is added, and the session list refreshes. Switching conversations mid-run aborts it and drops what pi still says of it (`discard`) until the next run starts. A pi that ends is restarted on the next message and resumes the session shown.

**Extension dialogs.** pi's extensions may ask (`select`, `confirm`, `input`, `editor`); the dialog shows in the conversation and its answer goes back by `Client.Send`. A `notify` shows as a toast, `set_editor_text` fills the composer, and the other requests that need no answer are ignored.

## Translation

`translate.go` runs a second pi, apart from the assistant's, so that translating never waits on a conversation nor leaves one: `--no-session --no-context-files --no-skills`, only search tools, its own prompt (`prompts/translator.md`), thinking low. Each translation is a fresh `new_session` then `prompt`, queued without waiting. The answer is the translation, then, in a `<notes>` block, the terms it checked, shown apart. The target "Automatic" asks for Simplified Chinese, or English if the text is Chinese. Nothing is sent until Translate or ⌘↩; changing the target or the style translates again what was translated.

## Quitting

`OnBeforeQuit` kills both pis (their process groups, so what they started ends too) and closes the server, without waiting: quitting is never held up by a model.

## Checking it

- `go test -race ./...`: the views are driven without a window (`ui.NewTester`), and a fake pi (`internal/pi/client_test.go`, the test binary itself) plays the protocol, its races included.
- `NT_SNAPSHOTS=dir go test -run TestSnapshots .` pictures every page; `NT_PAGE` picks some, `NT_SIZE=940x600` the smallest window, `NT_NOW` the time.
- `NT_REAL_PI=1 go test -run TestRealPi .` checks the tools the real pi can reach.
- The app itself, on data of its own: `NEXTTODO_DATA=/tmp/x/data.json`, with `NEXTTODO_CAPTURE=dir` and `NEXTTODO_ASK="…"` (or `NEXTTODO_TRANSLATE`, and `NEXTTODO_OPEN=<session file>` to go on with one), asks the real pi, writes `transcript.txt` and `main.png`, and quits. A window behind others captures its last frame: trust the transcript and the snapshots for the layout.

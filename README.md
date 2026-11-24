<p align="center">
  <img src="docs/img/icon-256.png" width="128" height="128" alt="NextTodo">
</p>

<h1 align="center">NextTodo</h1>

<h3 align="center">Your research, planned. On your Mac.</h3>

<p align="center">
  <b>Schedules, tasks and projects in one quiet app,<br>with an assistant that keeps them for you.</b>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/macOS-12%2B-111111?logo=apple&logoColor=white" alt="macOS 12+">
  <img src="https://img.shields.io/badge/Go-1.27-111111?logo=go&logoColor=white" alt="Go 1.27">
  <img src="https://img.shields.io/badge/UI-native%2C%20no%20webview-4f46e5" alt="Native UI, no webview">
  <img src="https://img.shields.io/badge/size-10%20MB-111111" alt="10 MB">
  <img src="https://img.shields.io/badge/license-MIT-111111" alt="MIT">
</p>

<p align="center">
  <b><a href="../../releases/latest">Download</a></b> &nbsp;·&nbsp;
  <b><a href="#build-from-source">Build from source</a></b> &nbsp;·&nbsp;
  <b>English</b> · <a href="README.zh-CN.md">简体中文</a>
</p>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/overview-dark.png">
  <img src="docs/img/overview-light.png" alt="NextTodo's overview: a field to ask the assistant, the tasks left today, the week's load, the event coming next, today's agenda and upcoming deadlines">
</picture>

## Is NextTodo for you?

A research life lives in too many places: grants in a spreadsheet, meetings in a calendar, to-dos in another app, and the notes on how an experiment went nowhere at all. NextTodo keeps them together, on your Mac:

- **Projects, sorted your way.** Make your own categories (by funding, by lab, by anything) with a color and an icon, or group by research direction or status; see them as a list or on a timeline. Each project keeps its tasks, events, dated journal and particulars (grant number, lead, funder, dates) in one place, and goes to the archive when it ends.
- **A calendar that moves with you.** Week and month views, tags of your own, events that repeat, calendars on the web followed (your university's, Google's, the holidays of your region), drag to reschedule, to sketch a new one or to block time for a task, a notification before each event starts, and .ics import and export to get along with Calendar, Google and Outlook.
- **An assistant that does the filing.** Tell it "meeting with my advisor Monday, rebuttal due next Wednesday" and both are on your calendar and list. Think an idea through with it, and when it is worth keeping, it becomes a project with its first tasks. It looks things up with the search tools you gave it, and every change it makes has Undo.
- **Translation for research.** Any language into any other, naturally, academically or literally, with the terms it was unsure of checked and explained.
- **Today, from the menu bar.** What is left of the day, and a field to add a task or an event, one click or ⌃⌥Space away.
- **Yours, on your Mac.** No account, no cloud. One JSON file in your Library, saved as you type, copied every day, with Undo for every deletion. The assistant and translation use the model you set up in [pi](https://github.com/earendil-works/pi), and send it only what they need (below).

It's for you if you juggle more than one project and more than one kind of commitment. If one list of to-dos is enough, Reminders will do.

English and 简体中文. Light and dark.

## Features

<table>
  <tr>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/schedule-dark.png">
        <img src="docs/img/schedule-light.png" alt="The week, with events by kind">
      </picture>
      <p><b>Schedule.</b> Drag an event to move it, in whole days and quarter hours; drag its bottom edge to resize it; drag down empty hours to add one, or a task into the hours to block time for it. Events repeat daily, on weekdays, weekly or monthly. In the month, drag events and due tasks between days.</p>
    </td>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/tasks-dark.png">
        <img src="docs/img/tasks-light.png" alt="My Day, with a task's details beside it">
      </picture>
      <p><b>Tasks.</b> My Day gathers what you planned for today and what is due or late. Important, Planned (by day, as in Reminders), All and Completed, with priorities, due dates, notes and projects. ↑↓ to choose, ⌘K to finish.</p>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/projects-dark.png">
        <img src="docs/img/projects-light.png" alt="Projects grouped by category">
      </picture>
      <p><b>Projects.</b> Grouped by your categories, direction or status, each with its progress and when it ends. Or see them on a timeline, over the months they run. Open one for its summary, tasks, research journal and upcoming events; archive it when it's done.</p>
    </td>
    <td width="50%" align="center">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/panel-dark.png">
        <img src="docs/img/panel-light.png" width="300" alt="The menu bar panel">
      </picture>
      <p><b>Menu bar.</b> Today's events and tasks, checked off where they are, and a field that adds a task to My Day or an event today.</p>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/assistant-dark.png">
        <img src="docs/img/assistant-light.png" alt="The assistant, having made a project with two tasks, with Undo">
      </picture>
      <p><b>Assistant.</b> Ask it anything, or tell it what to remember. It reads your tasks, schedule and projects before it writes, says what it changed with a link to each, and takes it all back with one Undo. Conversations are kept on your Mac, to pick up later.</p>
    </td>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/translate-dark.png">
        <img src="docs/img/translate-light.png" alt="A paragraph translated into Chinese, with notes on its terms">
      </picture>
      <p><b>Translate.</b> From any language into any other: automatic picks Chinese or English for you. Natural, academic or literal. Terms of art are looked up, not guessed, and explained under the translation. ⌘↩ to translate.</p>
    </td>
  </tr>
  <tr>
    <td colspan="2">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/timeline-dark.png">
        <img src="docs/img/timeline-light.png" alt="The projects on a timeline, over the months they run">
      </picture>
      <p><b>Timeline.</b> Every project over the months it runs, in its category's color, filled as far as its tasks are done, with a line at today: which grant ends when, and what overlaps.</p>
    </td>
  </tr>
</table>

And: tasks that repeat and checklists of steps, type the day and the time with what you add ("Seminar fri 3-4pm", "明天下午3点 组会"), reminders for tasks, search everything with ⌘F, research directions with their projects, reminders before events (a default and one per event), a Dock badge for what is due, open at login, ⌘1–⌘6 for the pages, daily backups you can restore, and a clean start: the first launch welcomes you with where to begin, and sample data is a click away if you would rather look around first.

## Quick start

**1 · Install.** Download the disk image from [Releases](../../releases/latest), open it and drag NextTodo to Applications. NextTodo is not notarized by Apple yet, so the first time you open it macOS says it cannot: click Done, double-click **Privacy & Security** in the disk image's window, scroll to Security and click **Open Anyway** beside NextTodo. Only once. (Or, in Terminal: `xattr -dr com.apple.quarantine /Applications/NextTodo.app`.)

**2 · Plan today.** Add tasks to My Day, events to the week, and let the reminders come. The menu bar icon keeps today one click away.

**3 · Sort your projects.** Create a project for each grant, contract or line of inquiry, give it a direction, and its tasks and events follow it everywhere.

**4 · Meet the assistant (optional).** It runs on [pi](https://github.com/earendil-works/pi), the agent you may already use in the terminal. Install it with `npm install -g @earendil-works/pi-coding-agent`, run `pi` once to sign in or choose a model, and the Assistant page and ⌘J are ready. NextTodo finds pi on its own; if it can't, give its path in Settings › Assistant.

Your data is in `~/Library/Application Support/NextTodo/data.json`. Export a backup from Settings › Data.

## The assistant and translation

NextTodo does not ship a model or an agent of its own. The assistant is the **pi** of your Mac, run in its RPC mode: it uses the models, keys, settings and MCP servers you set up for pi, and NextTodo gives it one more MCP server, its own, on `127.0.0.1` with a token that changes at every launch. NextTodo never reads pi's keys.

**What it can do.** Read your tasks, events, projects, journals and directions; add tasks, events, projects, journal entries and directions; change them, mark them done, archive projects. It deletes nothing. It acts when you tell it something to do or remember, and when you are only thinking aloud it talks, and offers once to keep an idea, rather than filling your lists.

**What it can reach besides.** By default, only the MCP tools of pi that look things up: search, web pages, papers, and those named `get`, `list`, `read` or `query`. Not a shell, not your files, nothing that writes elsewhere. Settings › Assistant can let it read files you point it to, as a paper's PDF (never change them), or use all of pi's MCP tools (still no shell, nor editing files); what it then writes elsewhere, as in GitHub or Notion, NextTodo cannot undo.

**What is sent, and where.** Your message, with today's date and time, goes to the model you chose in pi, and so do the answers of the tools it calls: when it reads your tasks, the tasks it read. A search sends the search's words to that search tool. Translation sends the text you asked to translate, nothing more, and is told to search for terms alone, never the text. Nothing is sent before you press Return or Translate.

**What stays on your Mac.** Conversations are files of pi's, in `~/Library/Application Support/NextTodo/Assistant/sessions`, listed in the Assistant's sidebar to reopen, rename or delete. An `AGENTS.md` in the Assistant folder gives it standing instructions ("my advisor is Prof. Li", "I write in English"). Translations are not kept.

**Undo.** What a question changed is shown under its answer, each item a click from where it is, and Undo takes it all back, as long as that conversation stays open; what you changed by hand since is kept.

## How it's built

NextTodo is written in Go with [MyGo](https://mygo.egoist.dev/)'s native UI: no HTML, no JavaScript, no webview. MyGo draws every frame on the GPU with Metal and lays out text with Core Text, so the app opens at once, idles at 0% CPU, and ships as a 10 MB app in a 3 MB disk image. Its look follows [magpie](https://github.com/yetone/magpie): a soft gray window, white cards with hairline edges, SF Pro and one indigo accent.

| | |
|---|---|
| `store.go` | the data, its file and backups, queries and dates |
| `style.go`, `icons.go`, `art.go` | the palette, the components, the icons |
| `app.go` | the app's state, saving, the title bar |
| `overview.go`, `tasks.go`, `schedule.go`, `projects.go`, `directions.go`, `settings.go` | the pages |
| `repeat.go`, `ics.go` | events that repeat, and calendar files |
| `parse.go` | the day and the time in what is typed |
| `timeline.go` | the projects over the months they run |
| `categories.go`, `search.go`, `undo.go`, `backups.go` | categories, search, Undo, daily copies |
| `panel.go`, `alerts.go` | the menu bar panel, and when events alert |
| `assistant.go`, `agent.go`, `agent_tools.go` | the Assistant's page, running pi, and NextTodo's tools for it |
| `translate.go` | the Translate page, on a pi of its own |
| `prompts/` | what the assistant and the translator are told |
| `internal/pi` | pi's RPC: the process, its commands and events, its sessions |
| `internal/mcp` | a small MCP server, over HTTP on this Mac |
| `locales/` | every string, in English and Chinese |

## Build from source

Go 1.27 or later. In mainland China, `go env -w GOPROXY=https://goproxy.cn,direct` first.

```sh
git clone https://github.com/yuxiaoLeeMarks/nexttodo.git && cd nexttodo
go tool mygo dev      # run, reloading as you edit
go test ./...         # the tests drive every view without a window
go tool mygo build    # build/darwin-arm64/NextTodo.app and its .dmg
go tool mygo build -platform darwin/universal   # for Apple silicon and Intel
```

`NT_SNAPSHOTS=/tmp/shots go test -run TestSnapshots .` renders every page, light and dark, in both languages. Strings live in `locales/en.json` and `locales/zh-Hans.json`. `NT_REAL_PI=1 go test -run TestRealPi .` asks the pi of your Mac which tools the assistant and the translator can reach. How it fits together: [docs/architecture.md](docs/architecture.md).

## License

[MIT](LICENSE)

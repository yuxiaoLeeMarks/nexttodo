<p align="center">
  <img src="docs/img/icon-256.png" width="128" height="128" alt="NextTodo">
</p>

<h1 align="center">NextTodo</h1>

<h3 align="center">Your research, planned. On your Mac.</h3>

<p align="center">
  <b>Schedules, tasks and projects in one quiet app,<br>sorted by how they're funded and the questions they serve.</b>
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
  <img src="docs/img/overview-light.png" alt="NextTodo's overview: the tasks left today, the week's load, the event coming next, today's agenda and upcoming deadlines">
</picture>

## Is NextTodo for you?

A research life lives in too many places: grants in a spreadsheet, meetings in a calendar, to-dos in another app, and the notes on how an experiment went nowhere at all. NextTodo keeps them together, on your Mac:

- **Projects, sorted your way.** Group them by funding (government grants, industry contracts, your own inquiry, academic service), by research direction or by status, and keep each one's tasks, events, dated journal and particulars (grant number, lead, funder, dates) in one place.
- **A calendar that moves with you.** Week and month views, drag to reschedule, drag an edge to change how long, and a notification before each event starts.
- **Today, from the menu bar.** What is left of the day, and a field to add a task or an event, one click away.
- **Yours, offline.** No account, no cloud. One JSON file in your Library, saved as you type, with backups you can export.

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
      <p><b>Schedule.</b> Drag an event to move it, in whole days and quarter hours; drag its bottom edge to resize it; double-click to add one. In the month, drag events and due tasks between days.</p>
    </td>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/tasks-dark.png">
        <img src="docs/img/tasks-light.png" alt="My Day, with a task's details beside it">
      </picture>
      <p><b>Tasks.</b> My Day gathers what you planned for today and what is due or late. Important, Planned, All and Completed, with priorities, due dates, notes and projects.</p>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/projects-dark.png">
        <img src="docs/img/projects-light.png" alt="Projects grouped by category">
      </picture>
      <p><b>Projects.</b> Grouped by category, direction or status, each with its progress and when it ends. Open one for its summary, tasks, research journal and upcoming events.</p>
    </td>
    <td width="50%" align="center">
      <picture>
        <source media="(prefers-color-scheme: dark)" srcset="docs/img/panel-dark.png">
        <img src="docs/img/panel-light.png" width="300" alt="The menu bar panel">
      </picture>
      <p><b>Menu bar.</b> Today's events and tasks, checked off where they are, and a field that adds a task to My Day or an event today.</p>
    </td>
  </tr>
</table>

And: research directions with their projects, reminders before events (a default and one per event), a pop-up of new things in the title bar, ⌘1–⌘5 for the pages, JSON backups, and sample data on first launch to show the way.

## Quick start

**1 · Install.** Download the disk image from [Releases](../../releases/latest), open it and drag NextTodo to Applications. The build is signed ad hoc: the first time, right-click the app and choose Open.

**2 · Plan today.** Add tasks to My Day, events to the week, and let the reminders come. The menu bar icon keeps today one click away.

**3 · Sort your projects.** Create a project for each grant, contract or line of inquiry, give it a direction, and its tasks and events follow it everywhere.

Your data is in `~/Library/Application Support/NextTodo/data.json`. Export a backup from Settings › Data.

## How it's built

NextTodo is written in Go with [MyGo](https://mygo.egoist.dev/)'s native UI: no HTML, no JavaScript, no webview. MyGo draws every frame on the GPU with Metal and lays out text with Core Text, so the app opens at once, idles at 0% CPU, and ships as a 10 MB app in a 3 MB disk image. Its look follows [magpie](https://github.com/yetone/magpie): a soft gray window, white cards with hairline edges, SF Pro and one indigo accent.

| | |
|---|---|
| `store.go` | the data, its file and backups, queries and dates |
| `style.go`, `icons.go`, `art.go` | the palette, the components, the icons |
| `app.go` | the app's state, saving, the title bar |
| `overview.go`, `tasks.go`, `schedule.go`, `projects.go`, `directions.go`, `settings.go` | the pages |
| `panel.go`, `alerts.go` | the menu bar panel, and when events alert |
| `locales/` | every string, in English and Chinese |

## Build from source

Go 1.27 or later. In mainland China, `go env -w GOPROXY=https://goproxy.cn,direct` first.

```sh
git clone https://github.com/yuxiaoLeeMarks/nexttodo.git && cd nexttodo
go tool mygo dev      # run, reloading as you edit
go test ./...         # the tests drive every view without a window
go tool mygo build    # build/darwin-arm64/NextTodo.app and its .dmg
```

`NT_SNAPSHOTS=/tmp/shots go test -run TestSnapshots .` renders every page, light and dark, in both languages. Strings live in `locales/en.json` and `locales/zh-Hans.json`.

## License

[MIT](LICENSE)

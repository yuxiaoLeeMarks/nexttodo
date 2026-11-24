# Changelog

## 0.8.0

**Assistant**

- NextTodo has an assistant: the [pi](https://github.com/earendil-works/pi) agent of your Mac, with the models, keys and MCP servers you set up for it. Tell it "meeting with my advisor Monday 10, rebuttal due next Wednesday" and both are on your calendar and list; ask it to make an idea a project and it does, with its first tasks; ask it anything else and it answers, looking things up with your search tools when that helps. It reads before it writes, keeps to the times you gave (a meeting on Monday with no time is a task for Monday, and it asks when), and deletes nothing.
- What a question changed is listed under the answer, each item a click from where it is, with Undo for all of it; what you changed by hand since is kept.
- Ask from the top of the overview, even before there is anything in it, or from anywhere with ⌘J; the Assistant page (⌘2) keeps the conversations, to reopen, rename or delete, and starts a new one with ⇧⌘J. Answers stream in as they are written; thinking folds away; a message sent while it works steers it; Esc stops it; ↑ brings back what you last wrote. Choose its model and how hard it thinks under the field.
- It reaches only NextTodo and tools that look things up: no shell, none of pi's file tools, nothing that writes elsewhere. Settings › Assistant can let it read files you point it to, or use all of pi's MCP tools. An AGENTS.md in its folder gives it standing instructions.
- NextTodo finds pi when opened from the Finder too, or says how to install it.

**Translate**

- A Translate page (⇧⌘T, or the button in the title bar): any language into any other, natural, academic or literal. Automatic translates into Chinese, or into English what is Chinese. Terms it is unsure of are looked up with pi's search tools, not guessed, and explained under the translation; Translate back checks it. Nothing is sent until you press Translate or ⌘↩, and changing the language or the style translates again.

**Changed**

- The pages moved down one shortcut for the Assistant: ⌘3 Tasks, ⌘4 Schedule, ⌘5 Projects, ⌘6 Directions.

**助理**

- NextTodo 有了助理：就是你这台 Mac 上的 [pi](https://github.com/earendil-works/pi) agent，沿用你为它配置的模型、密钥和 MCP 服务器。对它说「周一十点和导师过实验结果，rebuttal 下周三前写完」，日程和待办就都排好了；让它把一个想法建成项目，它会建好并列出第一批待办；问它别的问题，它会回答，需要时用你的搜索工具查资料。它先查看再修改，只用你给出的时间（「周一和导师见面」没说几点，就先记为周一的待办，再问你时间），不会删除任何东西。
- 每个问题改动了什么都列在答复下方，每一项点一下即可打开，一次撤销即可全部还原；你之后手动做的修改会保留。
- 在总览顶部即可提问（还没有任何内容时也可以），在任何地方按 ⌘J 也可以；「助理」页（⌘2）保存对话，可重新打开、重命名或删除，⇧⌘J 开始新对话。答复边写边显示，思考过程可折叠；它工作时发的消息会引导它调整方向；Esc 停止；↑ 找回上一条消息。输入框下方可以选择模型和思考深度。
- 它只能访问 NextTodo 和查找类工具：没有 shell，没有 pi 的文件工具，也不往别处写入。在「设置 › 助理」中可以允许它读取你指给它的文件，或使用 pi 的全部 MCP 工具。在助理文件夹中放一个 AGENTS.md，可以给它长期指示。
- 从访达打开 NextTodo 也能找到 pi；找不到时会告诉你如何安装。

**翻译**

- 新增「翻译」页（⇧⌘T，或标题栏中的按钮）：任意语言互译，可选自然、学术或直译。「自动」会译成中文，原文是中文时译成英文。拿不准的术语会用 pi 的搜索工具查证而非猜测，并在译文下方说明；「反向翻译」可用来核对。在你按下「翻译」或 ⌘↩ 之前不会发送任何内容；更换语言或风格会重新翻译。

**变化**

- 为「助理」让出 ⌘2，其余页面的快捷键顺延：⌘3 待办、⌘4 日程、⌘5 项目、⌘6 方向。

## 0.7.2

**Updates**

- NextTodo updates itself: it checks for a new version a little after launch and once a day, shows what is new, and installs it and relaunches on a click, downloading only what changed when it can. Settings › Updates turns the daily check off, and "Check for Updates…" in the NextTodo menu and in Settings checks at once. Updates are signed, and an update that is not is refused.
- Settings… moved to the NextTodo menu, where it is in Mac apps.

**软件更新**

- NextTodo 现在可以自动更新：启动后稍等片刻、之后每天检查一次新版本，告诉你更新了什么，点一下即可安装并重新打开，能只下载有变化的部分时只下载这部分。在「设置 › 软件更新」可关闭自动检查；NextTodo 菜单和设置里的「检查更新…」可立即检查。更新均经过签名，未签名的更新会被拒绝。
- 「设置…」移到了 NextTodo 菜单中，与其他 Mac 应用一致。

## 0.7.1

**Projects**

- The timeline's months make room for themselves: in a narrow window they are marked every other month, each quarter or each year, never one over another, and today is a dot under them. A bar too short for its dates has them beside it.

**Schedule**

- A day of the month shows as many events as fit it, and how many more, rather than three, the third cut in half in a short window.
- The days the clocks change, the week draws and takes times by the clock, not an hour off; a calendar's "one day" lasts until the next.
- A series set to end before it begins comes once rather than never, and an end date set starts on the event's day. A series made one-off from a later occurrence stays on that occurrence's day.
- Dragging an all-day event of days by a later day moves it as far as that day went. The part of an overnight event before midnight no longer resizes its end the next day. A press that hardly moved no longer leaves a sketch for the next drag to start from.
- Alerts: an occurrence changed after its alert does not alert again, nor does a UID-less calendar's event at every refresh; alerts days ahead come then; the alert of an occurrence since deleted opens its week, not the series. Task reminders that came due while NextTodo was quit come as it launches.
- A calendar file imported no longer changes the events of a calendar subscribed to, and a deleted tag no longer comes back with one.

**Tasks**

- A repeating task of the 31st keeps coming on the 31st, or the last of a shorter month; one of February 29 on it in leap years. One repeating in a way of a later version no longer hangs the app when done. Unchecked after its next was done or changed, it no longer duplicates or deletes it.
- Quick add reads "tonight 10:30", 今晚10:30, 明早6点 and "tomorrow morning at 6" in their part of the day, 上午11点到1点 as two hours, "every other friday" and 隔周五, and February 29 as the next one. The menu bar's panel takes a time typed before Event is chosen.
- A task added to Planned at a time gone today is due when it reminds. Filtering by a project later archived no longer hides every task.

**Data**

- Clearing the sample keeps what you wrote into it: a project with your entry, task or event, and a direction with your project. Subscribing to holidays first no longer hides the welcome.
- Data that cannot be read at once is asked for again, and is never saved over.

## 0.7.0

**Tasks**

- Repeating tasks, as in Reminders: every day, weekday, week, two weeks, month or year. Done, a task comes again, due the next of its days still to come, its reminder as many days later; unchecked at once, it takes that back. Typed too: "every friday", 每周一, 每个月.
- A checklist of steps in a task's details, counted in its row (1/3) and found by search; a repeating task's next has them fresh.

**Schedule**

- Subscribe to calendars on the web: a university's or Google Calendar's iCal address, a conference's deadlines, or the public holidays of eight regions, a click away in Settings › Schedule. They refresh every hour; their events are read only, tagged as you choose, and alert only if you ask.
- Events repeat every year too, and a calendar's rules NextTodo cannot repeat by, as the fourth Thursday of November, come as their times from a year ago to two on.

**Fixed**

- Clear sample data took one's own things whose titles matched the sample's: the sample is now marked as such.
- Right-clicks and Control-clicks on the tabs and switches no longer switch them.
- Undo of a deletion made with an event's dialog open keeps the dialog and what was typed.
- A calendar's categories are read whole and in turn, and importing again keeps the tags you gave.

## 0.6.0

**Installing**

- The disk image shows how to install: drag NextTodo to Applications, and if macOS will not open it the first time (NextTodo is not yet notarized by Apple), double-click Privacy & Security in the image's window and click Open Anyway. In English and Chinese.

**A clean start**

- The first launch starts empty, but for the default categories of projects and tags of events, and the overview welcomes you with where to begin: a first task, an event, a project, a calendar to import. Sample data is a click away, to look around first.
- Data that still holds the sample data says so, with Clear sample data, which keeps what you made, and can be undone.

## 0.5.0

**Schedule**

- Event tags are yours: name them, color them, put them in order (the first tags new events), add and delete them, from Settings › Schedule or the tag button of an event. Calendar files carry them as categories.

**Feel**

- The settings open at once: they asked macOS on every frame whether the app opens at login, 16 ms each time.
- Pages, lists and views switch as the pointer goes down, as Safari's tabs do, and show at once, without the rise that stuttered as a page laid out its text.
- The switches and popovers move on short curves that end when they say, not springs that drew a second of frames that moved nothing.
- The window's traffic lights sit in the middle of the title bar, level with the name and the pages.

## 0.4.0

**Projects**

- A timeline: each project a bar over the months it runs, in its category's color, filled as far as its tasks are done, with a line at today. Switch between List and Timeline on the projects page.

**Schedule**

- Import calendars from Calendar, Google Calendar, Outlook or a conference's deadlines (.ics), and export yours. Times keep their zones, repeats their days, ends and exceptions; importing again updates what came before. Settings › Data, or the File menu.
- Drag a task from above the week's hours to a time to block an hour for it.
- Events start and end on a day each, so they can go over midnight or over days.
- ⌘←, ⌘→ and ⌘T move through the weeks and months, as in Calendar.

**Tasks**

- Reminders: a task can remind you on a day at a time, set beside its due date or typed with it ("Call Lin tomorrow 3pm"). A click on the notification opens the task; a bell in its row, and in the menu bar, says when.

**Typing the time**

- Fewer words are read as times or dates: 一点, 第2点, a task's 1:50, "1/3 of", "2 may". Chinese hours in words need their part of the day or more than 点 (下午三点, 三点半).
- The night: 晚上12点, "tonight at 12" and "at midnight" are the midnight coming; 05:30 is as written; ranges over midnight (10pm-2am, 晚上11点到凌晨1点) keep their length.
- A time already gone today is tomorrow's, and the chip says so.
- Years (2026年10月30日, 10/30/2026, Nov 3rd 2027), "Friday next week", the weekend and 下周末.
- In the menu bar, an event's day no longer carries over to the next line.

**Fixed**

- Undo puts back each field the deletion changed and nothing changed since, and the days a series skips one by one; undoing a large erase is quick.
- Calendars: repeat rules follow their own zone's days, every-other-week rules start in the right week, cancelled and changed occurrences are skipped even when they come in a file of their own, and odd files (a byte-order mark, an unreadable start, Windows zone names) import.

## 0.3.0

**Projects**

- Categories are yours: name them, describe them, give them a color and an icon, drag them into order, add and delete them. Data of 0.2 gets the four it had, in your language.
- Archive a project when it ends: it and its tasks leave your lists and pickers, its events stay on the calendar. Archived projects wait under a collapsed header; Undo is one click away.

**Schedule**

- Events repeat every day, weekday, week, two weeks or month, until a day or for good. Change one occurrence or all of them; drag one and it goes alone.
- Drag down empty hours to sketch a new event, as in Calendar.
- The week opens at the time now; a month's "+2 more" opens that day.

**Adding**

- Type the day and the time with the title, as in Fantastical: "Write the rebuttal fri" adds a task due Friday; in the menu bar, "Coffee with Ada tomorrow 3-4:30pm" or "明天下午3点到5点 组会" adds the event then. A chip shows what was understood; its × adds the line as typed.

**Tasks**

- Planned is grouped as Reminders' Scheduled list: overdue, today, tomorrow, this week, later. Completed by day.
- ↑ and ↓ choose a task, ⌘K marks it done, Delete deletes it.

**Everywhere**

- Search everything with ⌘F: tasks, events, projects (their journals too) and directions.
- Deleting no longer asks first: Undo in a toast, or ⌘Z, which brings back what went and keeps what you did since. Restoring a backup, importing and erasing can be undone too.
- The data is copied as each day begins, the last fourteen days kept; restore any of them from Settings.
- Open at login, a Dock badge for what is due, and ⌃⌥Space for the menu bar panel from any app.
- The menu bar panel opens at once: made once, frosted, as magpie's.
- Segmented controls slide, pages rise into place, sheets and popovers grow in.

**Fixed**

- Events over midnight or over several days keep their end when edited.
- Moving a whole series moves its skipped days and its end with it.
- Repeating all-day events no longer spill into the next day over a change of clocks.
- An event made within its alert's lead alerts at once.
- An event added from the menu bar after 23:30 goes to tomorrow.

## 0.2.0

The first release: overview, tasks with My Day, week and month schedules with drag to move and resize, projects with journals, research directions, reminders before events, a menu bar panel, JSON backups, English and 简体中文, light and dark.

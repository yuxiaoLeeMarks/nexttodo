# Changelog

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

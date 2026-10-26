package main

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The lists of the tasks page.
const (
	listMyDay = iota
	listImportant
	listPlanned
	listAll
	listDone
)

// inList reports whether t belongs in list: My Day gathers what was added
// to it today and what is due today or late, with what it finished today.
func (a *app) inList(t *Task, list int) bool {
	if list != listDone && a.data.shelved(t) {
		return false
	}
	today := day(a.now())
	switch list {
	case listMyDay:
		due := !t.Due.IsZero() && !day(t.Due).After(today)
		if t.Done {
			return sameDay(t.DoneAt, today) && (t.inMyDay(today) || due)
		}
		return t.inMyDay(today) || due
	case listImportant:
		return !t.Done && t.Starred
	case listPlanned:
		return !t.Done && !t.Due.IsZero()
	case listAll:
		return !t.Done
	}
	return t.Done
}

func (a *app) tasksIn(list int) []*Task {
	var out []*Task
	for _, t := range a.data.Tasks {
		if a.taskFilter != "" && t.Project != a.taskFilter {
			continue
		}
		if a.inList(t, list) {
			out = append(out, t)
		}
	}
	sortTasks(out)
	return out
}

// taskGroup is a part of a list of tasks under a title: the tasks due on
// a day, or done on one.
type taskGroup struct {
	title string
	late  bool
	tasks []*Task
}

// taskGroups sorts the tasks of the list shown by day, as Reminders does
// its scheduled ones: Planned by when they are due, Completed by when they
// were done. Other lists are one group without a title.
func (a *app) taskGroups(tasks []*Task, first time.Weekday) []taskGroup {
	today := day(a.now())
	var groups []taskGroup
	put := func(title string, late bool, t *Task) {
		if len(groups) == 0 || groups[len(groups)-1].title != title {
			groups = append(groups, taskGroup{title: title, late: late})
		}
		groups[len(groups)-1].tasks = append(groups[len(groups)-1].tasks, t)
	}
	switch a.taskList {
	case listPlanned:
		weekEnd := addDays(weekStart(today, first), 7)
		for _, t := range tasks {
			switch n := daysBetween(today, t.Due); {
			case n < 0:
				put(a.tr.StatOverdue(), true, t)
			case n == 0:
				put(a.tr.Today(), false, t)
			case n == 1:
				put(a.tr.DueIn(1), false, t)
			case day(t.Due).Before(weekEnd):
				put(a.tr.ThisWeek(), false, t)
			default:
				put(a.tr.Later(), false, t)
			}
		}
	case listDone:
		for _, t := range tasks {
			switch n := daysBetween(t.DoneAt, today); {
			case n <= 0:
				put(a.tr.Today(), false, t)
			case n == 1:
				put(a.tr.Yesterday(), false, t)
			default:
				put(a.tr.Earlier(), false, t)
			}
		}
	default:
		groups = []taskGroup{{tasks: tasks}}
	}
	return groups
}

func (a *app) openCount(ts []*Task) int {
	n := 0
	for _, t := range ts {
		if !t.Done {
			n++
		}
	}
	return n
}

// myDayOpen counts the open tasks of My Day, as the menu bar shows them.
func (a *app) myDayOpen() int {
	n := 0
	for _, t := range a.data.Tasks {
		if !t.Done && a.inList(t, listMyDay) {
			n++
		}
	}
	return n
}

func (a *app) listName(list int) string {
	switch list {
	case listImportant:
		return a.tr.ListImportant()
	case listPlanned:
		return a.tr.ListPlanned()
	case listAll:
		return a.tr.ListAll()
	case listDone:
		return a.tr.ListDone()
	}
	return a.tr.ListMyDay()
}

// addTask adds a task titled title, placed by where it was added.
func (a *app) addTask(title, project string, list int) *Task {
	now := a.now()
	t := &Task{ID: newID(), Title: strings.TrimSpace(title), Project: project, Created: now}
	switch list {
	case listMyDay:
		t.MyDay = day(now)
	case listImportant:
		t.Starred = true
	case listPlanned:
		t.Due = day(now)
	}
	a.data.Tasks = append(a.data.Tasks, t)
	a.changed()
	return t
}

func (a *app) toggleMyDay(t *Task) {
	if t.inMyDay(a.now()) {
		t.MyDay = time.Time{}
	} else {
		t.MyDay = day(a.now())
	}
	a.changed()
}

func (a *app) tasksPage(c *ui.Context) {
	if a.data.project(a.taskFilter) == nil {
		a.taskFilter = ""
	}
	sel := a.data.task(a.taskSel)
	if sel == nil {
		a.taskSel = ""
	}
	tasks := a.tasksIn(a.taskList)

	ui.Row(c).Padding(2, 18, 10).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		labels := make([]string, 5)
		for i := range labels {
			labels[i] = a.listName(i)
			if n := a.openCount(a.tasksIn(i)); n > 0 && i != listDone {
				labels[i] += "  " + itoa(n)
			}
		}
		if a.segTrack(c, "lists", &a.taskList, labels...).Changed() {
			a.taskSel = ""
		}
		ui.Spacer(c)
		if selectOf(a, c, "taskFilter", &a.taskFilter, a.projectChoices(a.taskFilter), func(id string) string {
			if id == "" {
				return a.tr.FilterAllProjects()
			}
			return a.projectName(id)
		}).MaxWidth(240).Label(a.tr.FieldProject()).Changed() {
			a.taskSel = ""
		}
	})
	ui.Row(c).Grow(1).MinHeight(0).Padding(0, 18, 18).Gap(14).AlignItems(ui.Stretch).Children(func() {
		ui.Scroll(c).Grow(1).MinWidth(0).Children(func() {
			groups := a.taskGroups(tasks, a.firstWeekday(c))
			a.list(c, func() {
				if a.taskList != listDone {
					a.quickAddTask(c, &a.taskDraft, a.tr.TaskAddPlaceholder(), &a.taskAdd, func(w when) {
						t := a.addTask(w.title, a.taskFilter, a.taskList)
						if !w.day.IsZero() {
							t.Due = w.day
						}
						if a.taskSel != "" {
							a.taskSel = t.ID
						}
					})
				}
				if len(tasks) == 0 {
					if a.taskList == listMyDay {
						a.empty(c, a.tr.TaskEmptyMyDay(), a.tr.TaskEmptyMyDaySub())
					} else {
						a.empty(c, a.tr.TaskEmpty(), a.tr.TaskEmptySub())
					}
					return
				}
				if len(groups) == 1 && groups[0].title == "" {
					for _, t := range tasks {
						a.taskRow(c, t, rowInTasks)
					}
				}
			})
			// Planned and Completed go by day, a list for each.
			if len(groups) > 1 || len(groups) == 1 && groups[0].title != "" {
				for _, g := range groups {
					ui.Column(c.Key("group:" + g.title)).Children(func() {
						col := a.p.muted
						if g.late {
							col = a.p.red
						}
						ui.Row(c).Padding(16, 4, 7).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, g.title).FontSize(13).FontWeight(600).TextColor(col)
							ui.Text(c, itoa(len(g.tasks))).FontSize(12).TextColor(a.p.faint)
						})
						a.list(c, func() {
							for _, t := range g.tasks {
								a.taskRow(c, t, rowInTasks)
							}
						})
					})
				}
			}
		})
		if sel != nil {
			a.taskDetail(c, sel)
		}
	})
	a.taskKeys(c, tasks, sel)
}

// taskKeys moves the choice of a task with the arrows, and does to it what
// its menu does: ⌘K marks it done or not, Delete deletes it. A text field
// with the focus takes the keys it edits with first.
func (a *app) taskKeys(c *ui.Context, tasks []*Task, sel *Task) {
	if c.Shortcut(0, ui.KeyEscape) {
		a.taskSel = ""
	}
	i := slices.Index(tasks, sel)
	switch {
	case c.Shortcut(0, ui.KeyDown) && len(tasks) > 0:
		a.taskSel = tasks[min(i+1, len(tasks)-1)].ID
	case c.Shortcut(0, ui.KeyUp) && len(tasks) > 0:
		a.taskSel = tasks[max(i-1, 0)].ID
	case sel == nil:
	case c.Shortcut(ui.Cmd, ui.KeyK):
		sel.setDone(!sel.Done, a.now())
		a.changed()
	case c.Shortcut(0, ui.KeyBackspace) || c.Shortcut(0, ui.KeyDelete):
		next := ""
		if i+1 < len(tasks) {
			next = tasks[i+1].ID
		} else if i > 0 {
			next = tasks[i-1].ID
		}
		a.deleteTask(c, sel)
		a.taskSel = next
	}
}

// rowMotion grows a row in as it is added and folds it away as it goes,
// as a task done leaving its list.
var rowMotion = ui.ElementTransition{
	Duration: 200 * time.Millisecond,
	Enter:    &ui.Motion{Collapse: true},
	Exit:     &ui.Motion{Collapse: true},
}

// Where a task row shows.
const (
	rowInTasks   = iota // the tasks page, where a click chooses the task
	rowElsewhere        // the overview, where a click opens it
	rowInProject        // its project's page, which needs no name of it
)

// taskRow is a task in a list: a round check box, its priority and title,
// what it belongs to and when it is due, and its star.
func (a *app) taskRow(c *ui.Context, t *Task, where int) {
	p := &a.p
	today := day(a.now())
	inTasks := where == rowInTasks
	on := inTasks && a.taskSel == t.ID
	row := ui.ButtonBase(c.Key(t.ID)).MinHeight(46).Padding(6, 10, 6, 14).Gap(11).AlignItems(ui.Center).Radius(0).
		FocusRing(false).Label(orUntitled(t.Title, a.tr.Untitled())).Transition(rowMotion)
	hover := row.Hovered()
	switch {
	case on:
		row.Background(p.sel)
	case hover:
		row.Background(p.card2)
	}
	row.Children(func() {
		done := t.Done
		if a.check(c, &done, a.tr.MarkDone()+": "+orUntitled(t.Title, a.tr.Untitled())).Changed() {
			t.setDone(done, a.now())
			a.changed()
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				if t.Priority > 0 && !t.Done {
					a.priorityMarks(c, t.Priority)
				}
				title := ui.Text(c, orUntitled(t.Title, a.tr.Untitled())).FontSize(13).TextColor(p.fg).MinWidth(0)
				if t.Done {
					title.TextColor(p.muted).Strikethrough().DecorationColor(p.faint)
				}
			})
			var meta []func()
			item := func(icon *ui.SVG, s string, col ui.Color) {
				meta = append(meta, func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icon).Size(11, 11).TextColor(col)
						ui.Text(c, s).FontSize(11.5).TextColor(col).SingleLine()
					})
				})
			}
			if t.inMyDay(today) && !(inTasks && a.taskList == listMyDay) {
				item(iconSun, a.tr.ListMyDay(), p.muted)
			}
			if t.Project != "" && where != rowInProject && (!inTasks || a.taskFilter == "") {
				item(iconFolder, a.projectName(t.Project), p.muted)
			}
			if !t.Due.IsZero() {
				col, when := p.muted, a.relDay(t.Due)
				if t.Done {
					when = a.fmt(t.Due, "MMMd")
				} else if !day(t.Due).After(today) {
					col = p.red
				}
				item(iconCalendar, when, col)
			}
			if strings.TrimSpace(t.Notes) != "" {
				item(iconNote, "", p.faint)
			}
			if len(meta) > 0 {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
					for _, m := range meta {
						m()
					}
				})
			}
		})
		starred := t.Starred
		if a.star(c, &starred, a.tr.FieldStarred(), hover || on).Changed() {
			t.Starred = starred
			a.changed()
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		label := a.tr.MarkDone()
		if t.Done {
			label = a.tr.MarkOpen()
		}
		if m.Item(label).Shortcut(ui.Cmd, ui.KeyK).Chosen() {
			t.setDone(!t.Done, a.now())
			a.changed()
		}
		myDay := a.tr.AddToMyDay()
		if t.inMyDay(today) {
			myDay = a.tr.RemoveFromMyDay()
		}
		if m.Item(myDay).Chosen() {
			a.toggleMyDay(t)
		}
		if m.Item(a.tr.FieldStarred()).Checked(t.Starred).Chosen() {
			t.Starred = !t.Starred
			a.changed()
		}
		m.Separator()
		if m.Item(a.tr.Delete()).Shortcut(0, ui.KeyBackspace).Chosen() {
			a.deleteTask(c, t)
		}
	})
	if row.Clicked() {
		if inTasks {
			a.taskSel = t.ID
		} else {
			a.openTask(t.ID)
		}
	}
}

// taskDetail edits the chosen task beside the list, its fields in rows as
// System Settings shows them.
func (a *app) taskDetail(c *ui.Context, t *Task) {
	p := &a.p
	today := a.now()
	a.card(c.Key("detail:" + t.ID)).Width(340).Shrink(0).Children(func() {
		ui.Row(c).Height(44).Padding(0, 8, 0, 14).Shrink(0).AlignItems(ui.Center).Gap(8).Children(func() {
			done := t.Done
			label := a.tr.MarkDone()
			if t.Done {
				label = a.tr.MarkOpen()
			}
			if a.labeledCheck(c, &done, label).Changed() {
				t.setDone(done, a.now())
				a.changed()
			}
			ui.Spacer(c)
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.taskSel = ""
			}
		})
		ui.Box(c).Height(1).Background(p.line2).Shrink(0)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(12, 14, 4).Children(func() {
				a.titleInput(c, &t.Title, a.tr.Untitled(), 17)
			})
			ui.Column(c).Dividers(1, p.line2).Gap(1).Margin(4, 0, 0, 0).Children(func() {
				a.formRow(c, a.tr.FieldDue(), func() { a.dateField(c, "due", &t.Due) })
				a.formRow(c, a.tr.FieldPriority(), func() {
					if selectOf(a, c, "priority", &t.Priority, []int{0, 1, 2, 3}, a.priorityName).Label(a.tr.FieldPriority()).Changed() {
						a.changed()
					}
				})
				a.formRow(c, a.tr.FieldProject(), func() {
					if selectOf(a, c, "project", &t.Project, a.projectChoices(t.Project), a.projectName).MaxWidth(180).Label(a.tr.FieldProject()).Changed() {
						a.changed()
					}
				})
				a.formRow(c, a.tr.FieldStarred(), func() {
					if ui.Switch(c, &t.Starred).Label(a.tr.FieldStarred()).Changed() {
						a.changed()
					}
				})
				a.formRow(c, a.tr.FieldMyDay(), func() {
					on := t.inMyDay(today)
					if ui.Switch(c, &on).Label(a.tr.FieldMyDay()).Changed() {
						a.toggleMyDay(t)
					}
				})
			})
			ui.Column(c).Padding(10, 14, 14).Gap(6).Children(func() {
				a.label(c, a.tr.FieldNotes())
				in := ui.TextAreaBase(c, &t.Notes).Placeholder(a.tr.FieldNotes()).Label(a.tr.FieldNotes()).FontSize(13).LineHeight(1.45).
					Padding(8, 10).MinHeight(120).Radius(8).Background(p.card2).Border(1, p.line2).TextColor(p.fg)
				if in.Changed() {
					a.changed()
				}
			})
		})
		ui.Box(c).Height(1).Background(p.line2).Shrink(0)
		ui.Row(c).Height(40).Padding(0, 8, 0, 14).Shrink(0).AlignItems(ui.Center).Children(func() {
			s := a.tr.TaskCreated(a.fmt(t.Created, "yMMMd"))
			if t.Done && !t.DoneAt.IsZero() {
				s = a.tr.TaskCompletedOn(a.fmt(t.DoneAt, "yMMMd"))
			}
			a.muted(c, s).FontSize(11.5).Grow(1).SingleLine()
			if a.iconBtn(c, iconTrash, a.tr.Delete()).Clicked() {
				a.deleteTask(c, t)
			}
		})
	})
}

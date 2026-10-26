package main

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The groupings of the projects page.
const (
	groupByCategory = iota
	groupByDirection
	groupByStatus
)

type projectGroup struct {
	title, hint string
	projects    []*Project
}

// projectGroups sorts the projects shown into the groups of the grouping
// chosen, leaving out empty groups.
func (a *app) projectGroups() []projectGroup {
	var shown []*Project
	for _, p := range a.data.Projects {
		if p.Archived || p.Status == StatusDone && !a.projShowDone {
			continue
		}
		if !matches(a.projQuery, p.Title, p.Code, p.Summary, p.Lead, p.Funder) {
			continue
		}
		shown = append(shown, p)
	}
	// Active first, then by end date, then as created.
	slices.SortStableFunc(shown, func(x, y *Project) int {
		rank := func(s Status) int { return []int{1, 0, 2, 3}[s] }
		if d := rank(x.Status) - rank(y.Status); d != 0 {
			return d
		}
		switch {
		case x.End.IsZero() && !y.End.IsZero():
			return 1
		case !x.End.IsZero() && y.End.IsZero():
			return -1
		}
		if c := x.End.Compare(y.End); c != 0 {
			return c
		}
		return x.Created.Compare(y.Created)
	})
	var groups []projectGroup
	add := func(title, hint string, keep func(*Project) bool) {
		g := projectGroup{title: title, hint: hint}
		for _, p := range shown {
			if keep(p) {
				g.projects = append(g.projects, p)
			}
		}
		if len(g.projects) > 0 {
			groups = append(groups, g)
		}
	}
	switch a.projGroup {
	case groupByDirection:
		for _, d := range a.sortedDirections() {
			hint := a.tr.DirectionSecondary()
			if d.Primary {
				hint = a.tr.DirectionPrimary()
			}
			add(orUntitled(d.Title, a.tr.Untitled()), hint, func(p *Project) bool { return p.Direction == d.ID })
		}
		add(a.tr.NoDirection(), "", func(p *Project) bool { return a.data.direction(p.Direction) == nil })
	case groupByStatus:
		for _, s := range []Status{StatusActive, StatusPlanning, StatusPaused, StatusDone} {
			add(a.statusName(s), "", func(p *Project) bool { return p.Status == s })
		}
	default:
		for _, k := range a.data.Categories {
			add(orUntitled(k.Name, a.tr.Untitled()), k.Note, func(p *Project) bool { return p.Category == k.ID })
		}
		add(a.tr.Uncategorized(), "", func(p *Project) bool { return a.data.category(p.Category) == nil })
	}
	return groups
}

// archived returns the archived projects the search matches, the last
// archived first.
func (a *app) archived() []*Project {
	var out []*Project
	for _, p := range a.data.Projects {
		if p.Archived && matches(a.projQuery, p.Title, p.Code, p.Summary, p.Lead, p.Funder) {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(x, y *Project) int { return y.ArchivedAt.Compare(x.ArchivedAt) })
	return out
}

// archive puts a project away, or takes it out again, saying so with a
// way back.
func (a *app) archive(c *ui.Context, pr *Project, on bool) {
	pr.Archived = on
	pr.ArchivedAt = time.Time{}
	if on {
		pr.ArchivedAt = a.now()
		if a.projOpen == pr.ID {
			a.projOpen = ""
		}
		c.ToastAction(a.tr.ArchivedToast(orUntitled(pr.Title, a.tr.Untitled())), a.tr.Undo(), func() {
			pr.Archived, pr.ArchivedAt = false, time.Time{}
			a.changed()
		})
	}
	a.changed()
}

// newProject adds a project and opens it.
func (a *app) newProject() {
	pr := &Project{ID: newID(), Status: StatusPlanning, Created: a.now()}
	if len(a.data.Categories) > 0 {
		pr.Category = a.data.Categories[0].ID
	}
	a.data.Projects = append(a.data.Projects, pr)
	a.changed()
	a.openProject(pr.ID)
}

func (a *app) projectsPage(c *ui.Context) {
	p := &a.p
	groups := a.projectGroups()
	archived := a.archived()
	a.pageBody(c, func() {
		ui.Row(c).Height(26).Width(200).Padding(0, 8).Gap(6).AlignItems(ui.Center).Radius(7).Border(1, p.line).Background(p.card).Children(func() {
			ui.Icon(c, iconSearch).Size(13, 13).TextColor(p.muted)
			ui.TextInputBase(c, &a.projQuery).Placeholder(a.tr.SearchProjects()).Label(a.tr.SearchProjects()).Grow(1).FontSize(12.5)
		})
		ui.Box(c).Width(4)
		a.segTrack(c, "group", &a.projGroup, a.tr.GroupCategory(), a.tr.GroupDirection(), a.tr.GroupStatus())
		if a.iconBtn(c, iconTag, a.tr.EditCategories()).Clicked() {
			a.catOpen = true
		}
		ui.Spacer(c)
		a.labeledCheck(c, &a.projShowDone, a.tr.ShowCompleted())
		ui.Box(c).Width(6)
		if a.iconLabel(c, a.btn(c, "", btnPrimary), iconPlus, a.tr.NewProject()).Clicked() {
			a.newProject()
		}
	}, func() {
		if len(a.data.Projects) == 0 {
			a.list(c, func() {
				ui.Column(c).AlignItems(ui.Center).PaddingY(8).Children(func() {
					a.empty(c, a.tr.ProjectsEmpty(), a.tr.ProjectsEmptySub())
					if a.btn(c, a.tr.NewProject(), btnSecondary).Clicked() {
						a.newProject()
					}
				})
			}).Margin(8, 0, 0, 0)
			return
		}
		if len(groups) == 0 && len(archived) == 0 {
			a.list(c, func() { a.empty(c, a.tr.NoMatches(), "") }).Margin(8, 0, 0, 0)
			return
		}
		for _, g := range groups {
			ui.Column(c.Key("group:" + g.title)).Children(func() {
				a.section(c, g.title, g.hint, func() {
					ui.Text(c, itoa(len(g.projects))).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
				})
				a.list(c, func() {
					for _, pr := range g.projects {
						a.projectRow(c, pr)
					}
				})
			})
		}
		if len(archived) > 0 {
			a.archivedSection(c, archived)
		}
	})
}

// archivedSection is the archived projects under the rest, behind a header
// that shows and hides them.
func (a *app) archivedSection(c *ui.Context, archived []*Project) {
	p := &a.p
	h := ui.ButtonBase(c.Key("archived")).Margin(10, 0, 0, 0).Padding(6, 4, 7).Gap(6).AlignItems(ui.Center).AlignSelf(ui.Start).Radius(6).FocusRing(true).
		Label(a.tr.Archived()).Expanded(a.projShowArchived)
	turn := h.Animate("turn", map[bool]float32{true: 90, false: 0}[a.projShowArchived], 160*time.Millisecond)
	h.Children(func() {
		ui.Icon(c, iconChevronRight).Size(12, 12).TextColor(p.muted).Rotate(turn)
		ui.Icon(c, iconArchive).Size(13, 13).TextColor(p.muted)
		ui.Text(c, a.tr.Archived()).FontSize(13).FontWeight(600).TextColor(p.fg2)
		ui.Text(c, itoa(len(archived))).FontSize(12).TextColor(p.faint)
	})
	if h.Clicked() {
		a.projShowArchived = !a.projShowArchived
	}
	if a.projShowArchived {
		a.list(c, func() {
			for _, pr := range archived {
				a.projectRow(c, pr)
			}
		})
	}
}

// projectRow is a project in a list, as magpie lists providers: a disc of
// its category, its name over its particulars, its progress and end, its
// status, and a chevron to open it.
func (a *app) projectRow(c *ui.Context, pr *Project) {
	p := &a.p
	b := ui.ButtonBase(c.Key(pr.ID)).MinHeight(54).Padding(8, 12, 8, 14).Gap(12).AlignItems(ui.Center).Radius(0).
		FocusRing(false).Label(orUntitled(pr.Title, a.tr.Untitled()))
	if b.Hovered() {
		b.Background(p.card2)
	}
	done, total := a.data.progress(pr.ID)
	b.Children(func() {
		icon, col := a.categoryLook(pr.Category)
		a.disc(c, icon, col, 30)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, orUntitled(pr.Title, a.tr.Untitled())).FontSize(13).FontWeight(500).TextColor(p.fg).SingleLine()
			var sub []string
			if pr.Code != "" {
				sub = append(sub, pr.Code)
			}
			if d := a.data.direction(pr.Direction); d != nil {
				sub = append(sub, orUntitled(d.Title, a.tr.Untitled()))
			} else {
				sub = append(sub, a.categoryName(pr.Category))
			}
			a.sub(c, strings.Join(sub, " · "))
		})
		ui.Column(c).Width(110).Shrink(0).Gap(5).Children(func() {
			frac := float32(0)
			if total > 0 {
				frac = float32(done) / float32(total)
			}
			a.progressBar(c, frac)
			ui.Text(c, a.tr.TasksProgress(done, total)).FontSize(11).FontFeatures("tnum").TextColor(p.muted)
		})
		end, endColor := "", p.muted
		if !pr.End.IsZero() {
			end = a.tr.ProjectEnds(a.fmt(pr.End, "yMMM"))
			if left := daysBetween(a.now(), pr.End); left >= 0 && left <= 30 && pr.Status != StatusDone {
				end, endColor = a.tr.DaysLeft(left), p.red
			}
		}
		ui.Text(c, end).FontSize(11.5).TextColor(endColor).Width(110).TextAlign(ui.End).SingleLine().Shrink(0)
		ui.Row(c).Width(86).Shrink(0).Justify(ui.End).Children(func() {
			if pr.Archived {
				a.chip(c, a.tr.Archived(), p.faint)
			} else {
				a.statusChip(c, pr.Status)
			}
		})
		ui.Icon(c, iconChevronRight).Size(13, 13).TextColor(p.faint)
	})
	if pr.Archived {
		b.Opacity(0.7)
	}
	b.ContextMenu(func(m *ui.Menu) {
		m.Submenu(a.tr.FieldStatus(), func(m *ui.Menu) {
			for _, s := range statuses {
				if m.Item(a.statusName(s)).Checked(pr.Status == s).Chosen() {
					pr.Status = s
					a.changed()
				}
			}
		})
		m.Submenu(a.tr.FieldCategory(), func(m *ui.Menu) {
			for _, id := range a.categoryIDs(true) {
				if m.Item(a.categoryName(id)).Checked(pr.Category == id).Chosen() {
					pr.Category = id
					a.changed()
				}
			}
		})
		m.Separator()
		label := a.tr.Archive()
		if pr.Archived {
			label = a.tr.Unarchive()
		}
		if m.Item(label).Chosen() {
			a.archive(c, pr, !pr.Archived)
		}
		if m.Item(a.tr.ProjectDelete()).Chosen() {
			a.openProject(pr.ID)
			a.askProject = true
		}
	})
	if b.Clicked() {
		a.openProject(pr.ID)
	}
}

// projectPage shows and edits the project open.
func (a *app) projectPage(c *ui.Context) {
	p := &a.p
	pr := a.data.project(a.projOpen)
	if ui.AlertDialog(c, &a.askProject, a.tr.ProjectDeleteConfirm(orUntitled(pr.Title, a.tr.Untitled())), a.tr.ProjectDeleteDetail(),
		a.tr.Cancel(), a.tr.Delete()) == 1 {
		step := a.remember()
		a.data.deleteProject(pr.ID)
		a.projOpen = ""
		a.changed()
		a.deleted(c, step, pr.Title)
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		a.projOpen = ""
	}
	a.pageBody(c, func() {
		if a.iconLabel(c, a.btn(c, "", btnPlain), iconChevronLeft, a.tr.NavProjects()).Clicked() {
			a.projOpen = ""
		}
		ui.Spacer(c)
		if !pr.Archived && a.iconLabel(c, a.btn(c, "", btnSecondary), iconArchive, a.tr.Archive()).Clicked() {
			a.archive(c, pr, true)
		}
		if a.iconBtn(c, iconTrash, a.tr.ProjectDelete()).Clicked() {
			a.askProject = true
		}
	}, func() {
		switch {
		case pr.Archived:
			a.notice(c, iconArchive, a.tr.ArchivedOn(a.fmt(pr.ArchivedAt, "yMMMd")), a.tr.Unarchive(), func() { a.archive(c, pr, false) })
		case pr.Status == StatusDone:
			a.notice(c, iconArchive, a.tr.ArchiveSuggest(), a.tr.Archive(), func() { a.archive(c, pr, true) })
		}
		a.card(c).Padding(16, 18).Gap(10).Margin(4, 0, 0, 0).Children(func() {
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				icon, col := a.categoryLook(pr.Category)
				a.disc(c, icon, col, 40)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					t := a.titleInput(c, &pr.Title, a.tr.ProjectTitlePlaceholder(), 20)
					if pr.Title == "" {
						t.AutoFocus()
					}
					ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
						a.statusChip(c, pr.Status)
						a.chip(c, a.categoryName(pr.Category), a.categoryColor(pr.Category))
						if d := a.data.direction(pr.Direction); d != nil {
							a.chip(c, orUntitled(d.Title, a.tr.Untitled()), ui.Color{})
						}
						if !pr.Start.IsZero() || !pr.End.IsZero() {
							span := ""
							if !pr.Start.IsZero() {
								span = a.fmt(pr.Start, "yMMM")
							}
							span += " – "
							if !pr.End.IsZero() {
								span += a.fmt(pr.End, "yMMM")
							}
							ui.Text(c, span).FontSize(12).TextColor(p.muted).Margin(0, 0, 0, 4)
						}
					})
				})
			})
		})
		ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				a.section(c, a.tr.FieldSummary(), "", nil)
				a.card(c).Children(func() { a.area(c, &pr.Summary, a.tr.ProjectSummaryPlaceholder()) })
				a.projectTasks(c, pr)
				a.projectJournal(c, pr)
			})
			ui.Column(c).Width(320).Shrink(0).Children(func() { a.projectDetails(c, pr) })
		})
	})
}

func (a *app) projectTasks(c *ui.Context, pr *Project) {
	tasks := a.data.projectTasks(pr.ID)
	done, total := a.data.progress(pr.ID)
	a.section(c, a.tr.ProjectTasks(), a.tr.TasksProgress(done, total), nil)
	a.list(c, func() {
		a.quickAddTask(c, &a.projTaskDraft, a.tr.ProjectAddTask(), nil, func(w when) {
			a.addTask(w.title, pr.ID, listAll).Due = w.day
		})
		for _, t := range tasks {
			a.taskRow(c, t, rowInProject)
		}
	})
}

func (a *app) projectJournal(c *ui.Context, pr *Project) {
	p := &a.p
	a.section(c, a.tr.ProjectJournal(), itoa(len(pr.Journal)), nil)
	var removed string
	a.list(c, func() {
		ui.Column(c).Children(func() {
			ui.TextAreaBase(c, &a.journalDraft).Placeholder(a.tr.JournalPlaceholder()).Label(a.tr.ProjectJournal()).
				FontSize(13).LineHeight(1.45).TextColor(p.fg).Padding(10, 14, 4).Lines(2, 8).FillWidth()
			ui.Row(c).Padding(0, 10, 8).Justify(ui.End).Children(func() {
				empty := strings.TrimSpace(a.journalDraft) == ""
				add := a.btn(c, a.tr.JournalAdd(), btnSecondary).Disabled(empty)
				if empty {
					add.Opacity(0.5)
				}
				if !empty && (add.Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter)) {
					pr.Journal = append([]Entry{{ID: newID(), Date: a.now(), Text: strings.TrimSpace(a.journalDraft)}}, pr.Journal...)
					a.journalDraft = ""
					a.changed()
				}
			})
		})
		if len(pr.Journal) == 0 {
			ui.Text(c, a.tr.JournalEmpty()).FontSize(12).TextColor(p.muted).Padding(12, 14)
		}
		for _, e := range pr.Journal {
			row := ui.Row(c.Key(e.ID)).Gap(14).Padding(10, 10, 10, 14).AlignItems(ui.Start)
			hover := row.Hovered()
			row.Children(func() {
				ui.Column(c).Width(84).Shrink(0).Gap(1).Children(func() {
					ui.Text(c, a.fmt(e.Date, "MMMd")).FontSize(12).FontWeight(500).TextColor(p.fg2)
					ui.Text(c, a.clock(e.Date)).FontSize(11).FontFeatures("tnum").TextColor(p.muted)
				})
				ui.Text(c, e.Text).FontSize(13).LineHeight(1.5).TextColor(p.fg).Grow(1).MinWidth(0).Selectable()
				del := a.iconBtn(c, iconTrash, a.tr.Delete())
				if !hover {
					del.Opacity(0)
				}
				if del.Clicked() {
					removed = e.ID
				}
			})
		}
	})
	if removed != "" {
		pr.Journal = slices.DeleteFunc(pr.Journal, func(e Entry) bool { return e.ID == removed })
		a.changed()
	}
}

// projectDetails is the column of the project's particulars, its progress
// and what is coming up.
func (a *app) projectDetails(c *ui.Context, pr *Project) {
	p := &a.p
	a.section(c, a.tr.ProjectDetails(), "", nil)
	a.list(c, func() {
		a.formRow(c, a.tr.FieldCategory(), func() {
			if selectOf(a, c, "category", &pr.Category, a.categoryIDs(true), a.categoryName).MaxWidth(170).Label(a.tr.FieldCategory()).Changed() {
				a.changed()
			}
		})
		a.formRow(c, a.tr.FieldStatus(), func() {
			if selectOf(a, c, "status", &pr.Status, statuses, a.statusName).Label(a.tr.FieldStatus()).Changed() {
				a.changed()
			}
		})
		a.formRow(c, a.tr.FieldDirection(), func() {
			ids := []string{""}
			for _, d := range a.sortedDirections() {
				ids = append(ids, d.ID)
			}
			if selectOf(a, c, "direction", &pr.Direction, ids, a.directionName).MaxWidth(170).Label(a.tr.FieldDirection()).Changed() {
				a.changed()
			}
		})
		a.formRow(c, a.tr.FieldStartDate(), func() { a.dateField(c, "start", &pr.Start) })
		a.formRow(c, a.tr.FieldEndDate(), func() { a.dateField(c, "end", &pr.End) })
		for _, f := range []struct {
			label string
			value *string
		}{{a.tr.FieldCode(), &pr.Code}, {a.tr.FieldLead(), &pr.Lead}, {a.tr.FieldFunder(), &pr.Funder}} {
			ui.Column(c.Key(f.label)).Padding(8, 14, 10).Gap(5).Children(func() {
				ui.Text(c, f.label).FontSize(12).TextColor(p.muted)
				in := ui.TextAreaBase(c, f.value).Lines(1, 3).Label(f.label).FontSize(13).TextColor(p.fg).FillWidth().
					OnTextKey(func(k ui.TextKey) bool { return k.Key == ui.KeyEnter })
				if in.Changed() {
					a.changed()
				}
			})
		}
	})
	done, total := a.data.progress(pr.ID)
	a.section(c, a.tr.Progress(), a.tr.TasksProgress(done, total), nil)
	a.card(c).Padding(12, 14).Gap(8).Children(func() {
		frac := float32(0)
		if total > 0 {
			frac = float32(done) / float32(total)
		}
		a.progressBar(c, frac)
		if !pr.End.IsZero() && pr.Status != StatusDone {
			if left := daysBetween(a.now(), pr.End); left >= 0 {
				ui.Text(c, a.tr.DaysLeft(left)).FontSize(12).TextColor(p.muted)
			}
		}
	})
	now := a.now()
	var upcoming []*Event
	for _, e := range a.data.eventsBetween(now, addDays(now, 365)) {
		if e.Project == pr.ID && len(upcoming) < 5 {
			upcoming = append(upcoming, e)
		}
	}
	a.section(c, a.tr.ProjectUpcoming(), "", func() {
		if a.iconBtn(c, iconPlus, a.tr.NewEvent()).Clicked() {
			a.newEvent(a.nextSlot(), pr.ID)
		}
	})
	a.list(c, func() {
		if len(upcoming) == 0 {
			ui.Text(c, a.tr.ProjectNoEvents()).FontSize(12).TextColor(p.muted).Padding(12, 14)
			return
		}
		for _, e := range upcoming {
			b := ui.ButtonBase(c.Key(e.key())).MinHeight(44).Gap(10).Padding(6, 14).AlignItems(ui.Center).Radius(0).FocusRing(false)
			if b.Hovered() {
				b.Background(p.card2)
			}
			b.Children(func() {
				ui.Box(c).Width(3).Height(26).Radius(1.5).Background(a.kindColor(e.Kind)).Shrink(0)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(12.5).FontWeight(500).TextColor(p.fg).SingleLine()
					when := a.fmt(e.Start, "MMMEd")
					if !e.AllDay {
						when += " · " + a.clock(e.Start)
					}
					a.sub(c, when)
				})
			})
			if b.Clicked() {
				a.editEvent(e)
			}
		}
	})
}

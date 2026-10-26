package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// hit is a thing the search found, and how to open it.
type hit struct {
	key   string
	group string
	title string
	sub   string
	icon  *ui.SVG
	color ui.Color
	rank  int // 0 the title starts with the query, 1 holds it, 2 elsewhere
	open  func()
}

// searchHits returns what matches the query in the tasks, events,
// projects and directions, those whose titles start with it first, at
// most a few of each.
func (a *app) searchHits(query string) []hit {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	rank := func(title string, more ...string) int {
		t := strings.ToLower(title)
		switch {
		case strings.HasPrefix(t, q):
			return 0
		case strings.Contains(t, q):
			return 1
		case matches(q, more...):
			return 2
		}
		return -1
	}
	var hits []hit
	add := func(group string, limit int, found []hit) {
		slices.SortStableFunc(found, func(x, y hit) int { return x.rank - y.rank })
		if len(found) > limit {
			found = found[:limit]
		}
		for i := range found {
			found[i].group = group
		}
		hits = append(hits, found...)
	}
	var found []hit
	for _, t := range a.data.Tasks {
		r := rank(t.Title, t.Notes)
		if r < 0 {
			continue
		}
		sub := a.tr.NoProject()
		if t.Project != "" {
			sub = a.projectName(t.Project)
		}
		switch {
		case t.Done && !t.DoneAt.IsZero():
			sub += " · " + a.tr.TaskCompletedOn(a.fmt(t.DoneAt, "MMMd"))
		case !t.Due.IsZero():
			sub += " · " + a.relDay(t.Due)
		}
		icon := iconCheck
		if !t.Done {
			icon = iconCircle
		}
		found = append(found, hit{key: "t" + t.ID, title: orUntitled(t.Title, a.tr.Untitled()), sub: sub, icon: icon,
			color: a.p.accent, rank: r, open: func() { a.openTask(t.ID) }})
	}
	add(a.tr.NavTasks(), 6, found)
	found = nil
	now := a.now()
	for _, e := range a.data.Events {
		r := rank(e.Title, e.Location, e.Notes)
		if r < 0 {
			continue
		}
		// A repeating event shows when it comes next.
		when := e
		if e.repeating() {
			if next := e.occurrences(day(now), addDays(day(now), 400)); len(next) > 0 {
				when = next[0]
			}
		}
		sub := a.fmt(when.Start, "yMMMEd")
		if !e.AllDay {
			sub += " · " + a.clock(when.Start)
		}
		if e.repeating() {
			sub += " · " + a.repeatName(e.Repeat)
		}
		found = append(found, hit{key: "e" + e.ID, title: orUntitled(e.Title, a.tr.Untitled()), sub: sub, icon: iconCalendar,
			color: a.kindColor(e.Kind), rank: r, open: func() { a.openEvent(when.key()) }})
	}
	add(a.tr.NavSchedule(), 6, found)
	found = nil
	for _, p := range a.data.Projects {
		var journal []string
		for _, j := range p.Journal {
			journal = append(journal, j.Text)
		}
		r := rank(p.Title, append(journal, p.Code, p.Summary, p.Lead, p.Funder)...)
		if r < 0 {
			continue
		}
		sub := a.categoryName(p.Category) + " · " + a.statusName(p.Status)
		if p.Archived {
			sub += " · " + a.tr.Archived()
		}
		icon, col := a.categoryLook(p.Category)
		found = append(found, hit{key: "p" + p.ID, title: orUntitled(p.Title, a.tr.Untitled()), sub: sub, icon: icon,
			color: col, rank: r, open: func() { a.openProject(p.ID) }})
	}
	add(a.tr.NavProjects(), 5, found)
	found = nil
	for _, d := range a.data.Directions {
		r := rank(d.Title, d.Summary)
		if r < 0 {
			continue
		}
		found = append(found, hit{key: "d" + d.ID, title: orUntitled(d.Title, a.tr.Untitled()), sub: d.Summary, icon: iconCompass,
			color: a.p.series[5], rank: r, open: func() { a.navigate(PageDirections) }})
	}
	add(a.tr.NavDirections(), 3, found)
	return hits
}

// openSearch shows the search over the window.
func (a *app) openSearch() {
	a.searchOpen = true
	a.searchSel = 0
	a.searchField.Focus()
}

// searchSheet is the search: a field and what it finds, grouped, the one
// chosen by the arrows opening on Return.
func (a *app) searchSheet(c *ui.Context) {
	p := &a.p
	hits := a.searchHits(a.searchQuery)
	a.searchSel = max(0, min(a.searchSel, len(hits)-1))
	ui.DialogBase(c, &a.searchOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.12)).Justify(ui.Start).PaddingY(90)
		popover(a, panel).Width(600).MaxWidth(600).Padding(0).Gap(0).Radius(14)
		ui.Row(c).Height(52).Padding(0, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconSearch).Size(18, 18).TextColor(p.muted)
			in := ui.TextInputBase(c, &a.searchQuery).Placeholder(a.tr.SearchPlaceholder()).Label(a.tr.SearchPlaceholder()).
				FontSize(17).TextColor(p.fg).Grow(1).AutoFocus().Bind(&a.searchField)
			if in.Changed() {
				a.searchSel = 0
			}
			if in.Submitted() && len(hits) > 0 {
				a.searchOpen = false
				hits[a.searchSel].open()
			}
		})
		if strings.TrimSpace(a.searchQuery) == "" {
			return
		}
		ui.Box(c).Height(1).Background(p.line2)
		if len(hits) == 0 {
			ui.Text(c, a.tr.SearchNone()).FontSize(13).TextColor(p.muted).Padding(18, 16)
			return
		}
		ui.Scroll(c).MaxHeight(420).Children(func() {
			ui.Column(c).Padding(6).Gap(1).Children(func() {
				group := ""
				for i, h := range hits {
					if h.group != group {
						group = h.group
						a.label(c, group).Padding(10, 10, 4)
					}
					row := ui.ButtonBase(c.Key(h.key)).MinHeight(44).Padding(5, 10).Gap(10).Radius(8).AlignItems(ui.Center).
						FocusRing(false).Label(h.title)
					switch {
					case i == a.searchSel:
						row.Background(p.sel)
						if a.searchMoved {
							row.ScrollIntoView()
						}
					case row.Hovered():
						row.Background(p.pill)
					}
					row.Children(func() {
						a.disc(c, h.icon, h.color, 28)
						ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
							ui.Text(c, h.title).FontSize(13).FontWeight(500).TextColor(p.fg).SingleLine()
							if h.sub != "" {
								a.sub(c, h.sub)
							}
						})
						if i == a.searchSel {
							ui.Text(c, "↩").FontSize(12).TextColor(p.muted)
						}
					})
					if row.Clicked() {
						a.searchOpen = false
						h.open()
					}
				}
			})
		})
	})
	a.searchMoved = false
	if a.searchOpen && len(hits) > 0 {
		switch {
		case c.Shortcut(0, ui.KeyDown):
			a.searchSel, a.searchMoved = min(a.searchSel+1, len(hits)-1), true
		case c.Shortcut(0, ui.KeyUp):
			a.searchSel, a.searchMoved = max(a.searchSel-1, 0), true
		}
	}
}

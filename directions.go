package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

// dirDraft is a direction being edited in the dialog.
type dirDraft struct {
	id      string // "" for a new direction
	title   string
	summary string
	primary bool
}

func (a *app) newDirection() {
	a.dir = dirDraft{primary: len(a.data.Directions) == 0}
	a.dirOpen = true
}

func (a *app) editDirection(d *Direction) {
	a.dir = dirDraft{id: d.ID, title: d.Title, summary: d.Summary, primary: d.Primary}
	a.dirOpen = true
}

func (a *app) saveDirection() {
	d := a.data.direction(a.dir.id)
	if d == nil {
		d = &Direction{ID: newID(), Created: a.now()}
		a.data.Directions = append(a.data.Directions, d)
	}
	d.Title, d.Summary, d.Primary = strings.TrimSpace(a.dir.title), strings.TrimSpace(a.dir.summary), a.dir.primary
	a.dirOpen = false
	a.changed()
}

// sortedDirections puts the primary directions first.
func (a *app) sortedDirections() []*Direction {
	dirs := slices.Clone(a.data.Directions)
	slices.SortStableFunc(dirs, func(x, y *Direction) int {
		if x.Primary != y.Primary {
			if x.Primary {
				return -1
			}
			return 1
		}
		return x.Created.Compare(y.Created)
	})
	return dirs
}

func (a *app) directionsPage(c *ui.Context) {
	p := &a.p
	dirs := a.sortedDirections()
	a.pageBody(c, func() {
		ui.Text(c, a.tr.DirectionsIntro()).FontSize(13).TextColor(p.muted).SingleLine().MinWidth(0)
		ui.Spacer(c)
		if a.iconLabel(c, a.btn(c, "", btnPrimary), iconPlus, a.tr.NewDirection()).Clicked() {
			a.newDirection()
		}
	}, func() {
		a.section(c, a.tr.NavDirections(), a.tr.DirectionsCount(len(dirs)), nil)
		a.list(c, func() {
			if len(dirs) == 0 {
				ui.Column(c).AlignItems(ui.Center).PaddingY(8).Children(func() {
					a.empty(c, a.tr.DirectionsEmpty(), a.tr.DirectionsEmptySub())
					if a.btn(c, a.tr.NewDirection(), btnSecondary).Clicked() {
						a.newDirection()
					}
				})
				return
			}
			for _, d := range dirs {
				a.directionRow(c, d)
			}
		})
	})
}

// directionRow is a direction: its name and kind, what it asks, and its
// projects, which open on a click.
func (a *app) directionRow(c *ui.Context, d *Direction) {
	p := &a.p
	row := ui.Row(c.Key(d.ID)).Gap(14).Padding(14, 10, 14, 14).AlignItems(ui.Start)
	hover := row.Hovered()
	if hover {
		row.Background(p.card2)
	}
	row.Children(func() {
		col := p.series[5]
		if d.Primary {
			col = p.accent
		}
		a.disc(c, iconCompass, col, 30)
		ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, orUntitled(d.Title, a.tr.Untitled())).FontSize(14).FontWeight(600).TextColor(p.fg)
				if d.Primary {
					a.softChip(c, a.tr.DirectionPrimary(), p.accent)
				} else {
					a.softChip(c, a.tr.DirectionSecondary(), p.series[6])
				}
			})
			if d.Summary != "" {
				ui.Text(c, d.Summary).FontSize(12.5).LineHeight(1.45).TextColor(p.fg2).MaxWidth(720)
			}
			ui.Row(c).Gap(6).Wrap().Margin(4, 0, 0, 0).Children(func() {
				n := 0
				for _, pr := range a.data.Projects {
					if pr.Direction != d.ID {
						continue
					}
					n++
					chip := ui.ButtonBase(c.Key(pr.ID)).Height(24).Gap(6).Padding(0, 10, 0, 8).AlignItems(ui.Center).Radius(999).
						Border(1, p.line).Background(p.card)
					if chip.Hovered() {
						chip.Background(p.pill)
					}
					chip.Children(func() {
						a.dot(c, a.statusColor(pr.Status), 6)
						ui.Text(c, orUntitled(pr.Title, a.tr.Untitled())).FontSize(12).TextColor(p.fg2).SingleLine()
					})
					if chip.Clicked() {
						a.openProject(pr.ID)
					}
				}
				if n == 0 {
					ui.Text(c, a.tr.ProjectsCount(0)).FontSize(12).TextColor(p.muted)
				}
			})
		})
		edit := a.iconBtn(c, iconEdit, a.tr.EditDirection())
		if !hover {
			edit.Opacity(0.5)
		}
		if edit.Clicked() {
			a.editDirection(d)
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(a.tr.Edit()).Chosen() {
			a.editDirection(d)
		}
	})
}

// directionDialog edits a.dir while a.dirOpen.
func (a *app) directionDialog(c *ui.Context) {
	p := &a.p
	d := &a.dir
	a.modal(c, &a.dirOpen, 460, func() {
		label := a.tr.NewDirection()
		if d.id != "" {
			label = a.tr.EditDirection()
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(p.fg).Grow(1)
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.dirOpen = false
			}
		})
		ui.TextInput(c, &d.title).Placeholder(a.tr.DirectionTitlePlaceholder()).Label(a.tr.FieldTitle()).AutoFocus().
			FontSize(15).Radius(8).Padding(7, 10)
		ui.TextArea(c, &d.summary).Placeholder(a.tr.DirectionSummaryPlaceholder()).Label(a.tr.FieldSummary()).MinHeight(90).Radius(8)
		ui.Column(c).Background(p.card2).Border(1, p.line2).Radius(10).Children(func() {
			a.formRow(c, a.tr.DirectionPrimary(), func() { ui.Switch(c, &d.primary).Label(a.tr.DirectionPrimary()) })
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if d.id != "" && a.btn(c, a.tr.Delete(), btnDanger).Clicked() {
				a.askDir = true
			}
			ui.Spacer(c)
			if a.btn(c, a.tr.Cancel(), btnSecondary).Clicked() {
				a.dirOpen = false
			}
			label := a.tr.Create()
			if d.id != "" {
				label = a.tr.Save()
			}
			if a.btn(c, label, btnPrimary).Clicked() {
				a.saveDirection()
			}
		})
		if c.Shortcut(ui.Cmd, ui.KeyEnter) {
			a.saveDirection()
		}
	})
	if ui.AlertDialog(c, &a.askDir, a.tr.DirectionDeleteConfirm(orUntitled(d.title, a.tr.Untitled())), a.tr.DirectionDeleteDetail(),
		a.tr.Cancel(), a.tr.Delete()) == 1 {
		a.data.deleteDirection(d.id)
		a.dirOpen = false
		a.changed()
	}
}

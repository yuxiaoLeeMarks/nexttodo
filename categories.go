package main

import "github.com/egoist/mygo/ui"

// categoriesSheet edits the categories of projects while a.catOpen: their
// names and descriptions in place, their colors and icons in a popover,
// their order by dragging, and new ones at the end.
func (a *app) categoriesSheet(c *ui.Context) {
	p := &a.p
	a.modal(c, &a.catOpen, 560, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr.CategoriesTitle()).FontSize(13).FontWeight(600).TextColor(p.fg).Grow(1)
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.catOpen = false
			}
		})
		ui.Text(c, a.tr.CategoriesIntro()).FontSize(12).TextColor(p.muted)
		ui.Scroll(c).MaxHeight(420).Children(func() {
			a.list(c, func() {
				// Moves and deletions wait for the rows to be built.
				var later []func()
				for _, k := range a.data.Categories {
					a.categoryRow(c, k, &later)
				}
				for _, fn := range later {
					fn()
				}
				a.quickAdd(c, &a.catDraft, a.tr.CategoryNew(), nil, func(name string) {
					a.data.Categories = append(a.data.Categories, &Category{
						ID: CatID(newID()), Name: name, Color: len(a.data.Categories) % 7, Icon: "folder",
					})
					a.changed()
				})
			})
		})
		ui.Row(c).Children(func() {
			ui.Spacer(c)
			if a.btn(c, a.tr.Done(), btnPrimary).Clicked() {
				a.catOpen = false
			}
		})
	})
	k := a.data.category(a.askCat)
	open := k != nil
	if open {
		n := a.projectsIn(k.ID)
		if ui.AlertDialog(c, &open, a.tr.CategoryDeleteConfirm(orUntitled(k.Name, a.tr.Untitled())), a.tr.CategoryDeleteDetail(n),
			a.tr.Cancel(), a.tr.Delete()) == 1 {
			step := a.remember()
			a.data.deleteCategory(k.ID)
			a.changed()
			a.deleted(c, step, k.Name)
		}
		if !open {
			a.askCat = ""
		}
	}
}

// projectsIn counts the projects of a category.
func (a *app) projectsIn(id CatID) int {
	n := 0
	for _, pr := range a.data.Projects {
		if pr.Category == id {
			n++
		}
	}
	return n
}

// categoryRow edits a category: a grip to drag it by, its look, its name
// and description, how many projects it has, and a button deleting it.
func (a *app) categoryRow(c *ui.Context, k *Category, later *[]func()) {
	p := &a.p
	row := ui.Row(c.Key(k.ID)).MinHeight(54).Padding(6, 10, 6, 6).Gap(10).AlignItems(ui.Center).Drag(k)
	if _, over := ui.DragOver[*Category](row); over {
		row.InsetShadow(0, 2, 0, 0, p.accent)
	}
	if moved, ok := ui.Drop[*Category](row); ok {
		*later = append(*later, func() {
			a.data.moveCategory(moved, k)
			a.changed()
		})
	}
	if row.Dragging() {
		row.Opacity(0.4)
	}
	n := a.projectsIn(k.ID)
	row.Children(func() {
		ui.Icon(c, iconGrip).Size(12, 12).TextColor(p.faint).Cursor(ui.CursorGrab)
		icon, col := a.categoryLook(k.ID)
		look := ui.ButtonBase(c.Key("look")).Radius(999).Label(a.tr.CategoryLook()).Tooltip(a.tr.CategoryLook())
		look.Children(func() { a.disc(c, icon, col, 32) })
		if look.Clicked() {
			a.catPop = k.ID
		}
		open := a.catPop == k.ID
		ui.PopoverBase(c, look, &open, func(panel ui.Element) {
			popover(a, panel).Margin(6, 0, 0, 0).Padding(10).Gap(10).Width(232)
			ui.Row(c).Gap(6).Children(func() {
				for i, sc := range p.series {
					sw := ui.ButtonBase(c.Key(i)).Size(24, 24).Radius(12).Center().Label(itoa(i + 1))
					sw.Children(func() { ui.Box(c).Size(18, 18).Radius(9).Background(sc) })
					if k.Color == i {
						sw.Border(2, sc.Alpha(0.45))
					}
					if sw.Clicked() {
						k.Color = i
						a.changed()
					}
				}
			})
			ui.Grid(c).Columns(6).Gap(4).Children(func() {
				for _, ic := range categoryIcons {
					b := ui.ButtonBase(c.Key(ic.name)).Size(32, 32).Radius(7).Center().Label(ic.name)
					switch {
					case k.Icon == ic.name:
						b.Background(a.tint(col)).TextColor(col)
					case b.Hovered():
						b.Background(p.pill).TextColor(p.fg)
					default:
						b.TextColor(p.muted)
					}
					b.Children(func() { ui.Icon(c, ic.icon).Size(16, 16) })
					if b.Clicked() {
						k.Icon = ic.name
						a.changed()
					}
				}
			})
		})
		if !open && a.catPop == k.ID {
			a.catPop = ""
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			name := ui.TextInputBase(c, &k.Name).Placeholder(a.tr.CategoryNamePlaceholder()).Label(a.tr.CategoryNamePlaceholder()).
				FontSize(13).FontWeight(500).TextColor(p.fg)
			if name.Changed() {
				a.changed()
			}
			note := ui.TextInputBase(c, &k.Note).Placeholder(a.tr.CategoryNotePlaceholder()).Label(a.tr.CategoryNotePlaceholder()).
				FontSize(11.5).TextColor(p.muted)
			if note.Changed() {
				a.changed()
			}
		})
		ui.Text(c, a.tr.ProjectsCount(n)).FontSize(11.5).TextColor(p.muted).SingleLine().Shrink(0)
		if a.iconBtn(c, iconTrash, a.tr.Delete()).Clicked() {
			if n > 0 {
				a.askCat = k.ID
			} else {
				*later = append(*later, func() {
					a.data.deleteCategory(k.ID)
					a.changed()
				})
			}
		}
	})
}

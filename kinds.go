package main

import "github.com/egoist/mygo/ui"

// kindsSheet edits the tags of events while a.kindsOpen: their names in
// place, their colors in a popover, their order by dragging, the first
// being the tag of new events, and new ones at the end.
func (a *app) kindsSheet(c *ui.Context) {
	p := &a.p
	a.modal(c, &a.kindsOpen, 480, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.tr.KindsTitle()).FontSize(13).FontWeight(600).TextColor(p.fg).Grow(1)
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.kindsOpen = false
			}
		})
		ui.Text(c, a.tr.KindsIntro()).FontSize(12).TextColor(p.muted)
		ui.Scroll(c).MaxHeight(420).Children(func() {
			a.list(c, func() {
				// Moves and deletions wait for the rows to be built.
				var later []func()
				for _, k := range a.data.Kinds {
					a.kindRow(c, k, &later)
				}
				for _, fn := range later {
					fn()
				}
				a.quickAdd(c, &a.kindDraft, a.tr.KindNew(), nil, func(name string) {
					a.data.Kinds = append(a.data.Kinds, &Kind{ID: KindID(newID()), Name: name, Color: len(a.data.Kinds) % len(p.series)})
					a.changed()
				})
			})
		})
		ui.Row(c).Children(func() {
			ui.Spacer(c)
			if a.btn(c, a.tr.Done(), btnPrimary).Clicked() {
				a.kindsOpen = false
			}
		})
	})
	k := a.data.kind(a.askKind)
	open := k != nil
	if open {
		if ui.AlertDialog(c, &open, a.tr.KindDeleteConfirm(orUntitled(k.Name, a.tr.Untitled())), a.tr.KindDeleteDetail(a.eventsTagged(k.ID)),
			a.tr.Cancel(), a.tr.Delete()) == 1 {
			a.deleteKind(c, k)
		}
		if !open {
			a.askKind = ""
		}
	}
}

// eventsTagged counts the events of a tag, a repeating one once.
func (a *app) eventsTagged(id KindID) int {
	n := 0
	for _, e := range a.data.Events {
		if e.Kind == id {
			n++
		}
	}
	return n
}

// deleteKind deletes a tag, which its events lose, with Undo.
func (a *app) deleteKind(c *ui.Context, k *Kind) {
	step := a.remember()
	a.data.deleteKind(k.ID)
	if a.evt.kind == k.ID {
		a.evt.kind = ""
	}
	a.changed()
	a.deleted(c, step, k.Name)
}

// kindRow edits a tag: a grip to drag it by, its color, its name, how
// many events have it, and a button deleting it.
func (a *app) kindRow(c *ui.Context, k *Kind, later *[]func()) {
	p := &a.p
	row := ui.Row(c.Key(k.ID)).MinHeight(44).Padding(4, 10, 4, 6).Gap(10).AlignItems(ui.Center).Drag(k)
	if _, over := ui.DragOver[*Kind](row); over {
		row.InsetShadow(0, 2, 0, 0, p.accent)
	}
	if moved, ok := ui.Drop[*Kind](row); ok {
		*later = append(*later, func() {
			a.data.moveKind(moved, k)
			a.changed()
		})
	}
	if row.Dragging() {
		row.Opacity(0.4)
	}
	n := a.eventsTagged(k.ID)
	row.Children(func() {
		ui.Icon(c, iconGrip).Size(12, 12).TextColor(p.faint).Cursor(ui.CursorGrab)
		col := a.kindColor(k.ID)
		look := ui.ButtonBase(c.Key("look")).Size(26, 26).Radius(13).Center().Label(a.tr.KindColor()).Tooltip(a.tr.KindColor())
		look.Children(func() { ui.Box(c).Size(16, 16).Radius(8).Background(col) })
		if look.Hovered() {
			look.Background(p.pill)
		}
		if look.Clicked() {
			a.kindPop = k.ID
		}
		open := a.kindPop == k.ID
		ui.PopoverBase(c, look, &open, func(panel ui.Element) {
			popover(a, panel).Margin(6, 0, 0, 0).Padding(10)
			ui.Row(c).Gap(6).Children(func() {
				for i, sc := range p.series {
					sw := ui.ButtonBase(c.Key(i)).Size(24, 24).Radius(12).Center().Label(itoa(i + 1))
					sw.Children(func() { ui.Box(c).Size(18, 18).Radius(9).Background(sc) })
					if k.Color == i {
						sw.Border(2, sc.Alpha(0.45))
					}
					if sw.Clicked() {
						k.Color = i
						a.kindPop = ""
						a.changed()
					}
				}
			})
		})
		if !open && a.kindPop == k.ID {
			a.kindPop = ""
		}
		name := ui.TextInputBase(c, &k.Name).Placeholder(a.tr.KindNamePlaceholder()).Label(a.tr.KindNamePlaceholder()).
			FontSize(13).FontWeight(500).TextColor(p.fg).Grow(1)
		if name.Changed() {
			a.changed()
		}
		if a.data.newKind() == k.ID {
			a.softChip(c, a.tr.KindDefault(), p.accent)
		}
		ui.Text(c, a.tr.EventsCount(n)).FontSize(11.5).TextColor(p.muted).SingleLine().Shrink(0)
		if a.iconBtn(c, iconTrash, a.tr.Delete()).Clicked() {
			if n > 0 {
				a.askKind = k.ID
			} else {
				*later = append(*later, func() { a.deleteKind(c, k) })
			}
		}
	})
}

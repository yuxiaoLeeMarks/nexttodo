package main

import "github.com/egoist/mygo/ui"

// undoStep is the data as it was before a change that can be undone.
type undoStep struct {
	data []byte
}

const undoDepth = 20

// remember keeps the data as it is, before a deletion, for undoing it.
func (a *app) remember() *undoStep {
	b, err := a.data.encode()
	if err != nil {
		return nil
	}
	step := &undoStep{data: b}
	a.undo = append(a.undo, step)
	if len(a.undo) > undoDepth {
		a.undo = a.undo[len(a.undo)-undoDepth:]
	}
	return step
}

// restore puts the data back as step kept it, with the settings of now,
// and forgets the steps since.
func (a *app) restore(step *undoStep) {
	for i, s := range a.undo {
		if s == step {
			a.undo = a.undo[:i]
			break
		}
	}
	d, err := decodeData(step.data)
	if err != nil {
		return
	}
	d.Prefs = a.data.Prefs
	a.data = d
	a.drag = eventDrag{}
	a.evtOpen, a.dirOpen = false, false
	a.changed()
}

// deleted says that something went, with Undo to bring it back.
func (a *app) deleted(c *ui.Context, step *undoStep, title string) {
	if step == nil {
		return
	}
	c.ToastAction(a.tr.DeletedToast(orUntitled(title, a.tr.Untitled())), a.tr.Undo(), func() { a.restore(step) })
}

// undoShortcut undoes the last deletion on ⌘Z, unless a text field takes
// it for its own text.
func (a *app) undoShortcut(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyZ) && len(a.undo) > 0 {
		a.restore(a.undo[len(a.undo)-1])
	}
}

// deleteTask deletes a task at once, with Undo.
func (a *app) deleteTask(c *ui.Context, t *Task) {
	step := a.remember()
	a.data.deleteTask(t.ID)
	if a.taskSel == t.ID {
		a.taskSel = ""
	}
	a.changed()
	a.deleted(c, step, t.Title)
}

// deleteEvent deletes an event, or an occurrence of a repeating one, at
// once, with Undo.
func (a *app) deleteEvent(c *ui.Context, e *Event, all bool) {
	step := a.remember()
	if e.series != nil && !all {
		a.data.skip(e)
	} else {
		a.data.deleteEvent(e.ID)
	}
	a.changed()
	a.deleted(c, step, e.Title)
}

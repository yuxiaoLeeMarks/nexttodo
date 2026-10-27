package main

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/egoist/mygo/ui"
)

// undoStep is a change that can be undone, as the data was before it and
// after. Undoing it brings back what it removed and puts back what it
// changed, but leaves alone whatever changed since, so that the work done
// after a deletion survives undoing it.
type undoStep struct {
	before, after []byte
}

const undoDepth = 20

// remember keeps the data as it is, before a change, for undoing it.
func (a *app) remember() *undoStep {
	b, err := a.data.encode()
	if err != nil {
		return nil
	}
	step := &undoStep{before: b}
	a.undo = append(a.undo, step)
	if len(a.undo) > undoDepth {
		a.undo = a.undo[len(a.undo)-undoDepth:]
	}
	return step
}

// settle keeps the data as the change of step left it.
func (a *app) settle(step *undoStep) {
	if step != nil && step.after == nil {
		step.after, _ = a.data.encode()
	}
}

// replaceData puts d in the place of all the data, keeping the settings,
// and says msg with Undo.
func (a *app) replaceData(d *Data, msg string) {
	step := a.remember()
	d.Prefs = a.data.Prefs
	a.data = d
	a.resetSelection()
	a.settle(step)
	a.changed()
	a.replaced, a.replacedMsg = step, msg
}

// restore undoes the change of step, with the settings of now.
func (a *app) restore(step *undoStep) {
	i := slices.Index(a.undo, step)
	if i >= 0 {
		a.undo = slices.Delete(a.undo, i, i+1)
	}
	a.settle(step)
	before, err := decodeData(step.before)
	if err != nil {
		return
	}
	after, err := decodeData(step.after)
	if err != nil {
		return
	}
	d := a.data
	d.Categories = unchange(d.Categories, before.Categories, after.Categories, func(k *Category) string { return string(k.ID) })
	d.Kinds = unchange(d.Kinds, before.Kinds, after.Kinds, func(k *Kind) string { return string(k.ID) })
	d.Subscriptions = unchange(d.Subscriptions, before.Subscriptions, after.Subscriptions, func(s *Subscription) string { return s.ID })
	d.Directions = unchange(d.Directions, before.Directions, after.Directions, func(x *Direction) string { return x.ID })
	d.Projects = unchange(d.Projects, before.Projects, after.Projects, func(p *Project) string { return p.ID })
	d.Tasks = unchange(d.Tasks, before.Tasks, after.Tasks, func(t *Task) string { return t.ID })
	d.Events = unchange(d.Events, before.Events, after.Events, func(e *Event) string { return e.ID })
	a.drag = eventDrag{}
	// A dialog stays, with what is typed, unless what it edits went.
	if a.evtOpen && a.evt.id != "" && a.data.event(a.evt.id) == nil {
		a.evtOpen = false
	}
	if a.dirOpen && a.dir.id != "" && a.data.direction(a.dir.id) == nil {
		a.dirOpen = false
	}
	a.changed()
}

// unchange undoes, in the list cur, what a change did to the list before
// to make after: what it removed comes back where it was, what it added
// goes unless changed since, and what it changed goes back field by field,
// but for the fields changed since.
func unchange[T any](cur, before, after []*T, id func(*T) string) []*T {
	enc := func(x *T) []byte {
		b, _ := json.Marshal(x)
		return b
	}
	was, is := map[string]*T{}, map[string]*T{}
	for _, x := range before {
		was[id(x)] = x
	}
	for _, x := range after {
		is[id(x)] = x
	}
	cur = slices.DeleteFunc(cur, func(x *T) bool {
		f, added := is[id(x)]
		_, old := was[id(x)]
		return added && !old && bytes.Equal(enc(x), enc(f))
	})
	here := map[string]bool{}
	for i, x := range cur {
		here[id(x)] = true
		if b, f := was[id(x)], is[id(x)]; b != nil && f != nil {
			if eb, ef := enc(b), enc(f); !bytes.Equal(eb, ef) {
				cur[i] = unchangeFields(x, enc(x), eb, ef)
			}
		}
	}
	// What it removed comes back after what it followed, which may come
	// back too.
	follow := map[string][]*T{} // by what each follows, "" the start
	last := ""
	for _, b := range before {
		k := id(b)
		if _, kept := is[k]; !kept && !here[k] {
			follow[last] = append(follow[last], b)
			last = k
		} else if here[k] {
			last = k
		}
	}
	out := make([]*T, 0, len(cur)+len(follow))
	var put func(k string)
	put = func(k string) {
		for _, b := range follow[k] {
			out = append(out, b)
			put(id(b))
		}
	}
	put("")
	for _, x := range cur {
		out = append(out, x)
		put(id(x))
	}
	return out
}

// unchangeFields undoes in x, as JSON c, what a change made of b to make
// f: a field it changed goes back unless changed since, and a list it
// changed, as the days a series skips, loses what it added and gets back
// what it removed.
func unchangeFields[T any](x *T, c, b, f []byte) *T {
	var cm, bm, fm map[string]json.RawMessage
	if json.Unmarshal(c, &cm) != nil || json.Unmarshal(b, &bm) != nil || json.Unmarshal(f, &fm) != nil {
		return x
	}
	keys := map[string]bool{}
	for k := range bm {
		keys[k] = true
	}
	for k := range fm {
		keys[k] = true
	}
	for k := range keys {
		bv, fv, cv := bm[k], fm[k], cm[k]
		var bl, fl, cl []json.RawMessage
		switch {
		case bytes.Equal(bv, fv):
		case bytes.Equal(cv, fv):
			if bv == nil {
				delete(cm, k)
			} else {
				cm[k] = bv
			}
		case list(bv, &bl) && list(fv, &fl) && list(cv, &cl):
			in := func(l []json.RawMessage, v json.RawMessage) bool {
				return slices.ContainsFunc(l, func(w json.RawMessage) bool { return bytes.Equal(v, w) })
			}
			cl = slices.DeleteFunc(cl, func(v json.RawMessage) bool { return in(fl, v) && !in(bl, v) })
			for _, v := range bl {
				if !in(fl, v) && !in(cl, v) {
					cl = append(cl, v)
				}
			}
			if len(cl) == 0 {
				delete(cm, k)
			} else {
				cm[k], _ = json.Marshal(cl)
			}
		}
	}
	m, err := json.Marshal(cm)
	y := new(T)
	if err != nil || json.Unmarshal(m, y) != nil {
		return x
	}
	*x = *y // in place, for what holds it
	return x
}

// list reads v, a JSON list or nothing, into l.
func list(v json.RawMessage, l *[]json.RawMessage) bool {
	return v == nil || json.Unmarshal(v, l) == nil
}

// deleted says that something went, with Undo to bring it back.
func (a *app) deleted(c *ui.Context, step *undoStep, title string) {
	if step == nil {
		return
	}
	a.settle(step)
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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Subscription is a calendar on the web the app follows, as a university's
// or Google Calendar's iCal address: its events come, read only, and come
// again as it changes.
type Subscription struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	URL     string    `json:"url"`
	Kind    KindID    `json:"kind,omitempty"` // the tag of its events
	Alerts  bool      `json:"alerts,omitempty"`
	Updated time.Time `json:"updated,omitzero"`
}

// subscriptionEvery is how often subscriptions are refreshed.
const subscriptionEvery = time.Hour

// calendarURL reads the address of a calendar as typed: webcal:// is
// https://, and an address without a scheme is https too.
func calendarURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(strings.ToLower(s), "webcal://"):
		s = "https://" + s[len("webcal://"):]
	case !strings.Contains(s, "://"):
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("not the address of a calendar")
	}
	return u.String(), nil
}

// fetchCalendar gets the calendar file at u, up to 20 MB.
func fetchCalendar(ctx context.Context, u string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NextTodo/"+appVersion())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 20<<20))
}

// subscription returns the subscription of an ID.
func (d *Data) subscription(id string) *Subscription {
	for _, s := range d.Subscriptions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// applySubscription puts the events of a calendar file of s in the place
// of those it had, which keep their IDs and alerts.
func (d *Data) applySubscription(s *Subscription, got icsImport, now time.Time) {
	old := map[string]*Event{}
	var keep []*Event
	for _, e := range d.Events {
		switch {
		case e.Sub != s.ID:
			keep = append(keep, e)
		case e.UID != "":
			old[e.UID] = e
		}
	}
	for _, e := range got.events {
		e.Sub, e.Kind = s.ID, s.Kind
		if o := old[e.UID]; o != nil && e.UID != "" {
			e.ID = o.ID
			if e.Alert == nil {
				e.Alert = o.Alert
			}
		}
		keep = append(keep, e)
	}
	d.Events = keep
	s.Updated = now
	if s.Name == "" {
		s.Name = got.name
	}
	if s.Name == "" {
		if u, err := url.Parse(s.URL); err == nil {
			s.Name = u.Host
		}
	}
}

// unsubscribe forgets a subscription and its events.
func (d *Data) unsubscribe(id string) {
	d.Subscriptions = slices.DeleteFunc(d.Subscriptions, func(s *Subscription) bool { return s.ID == id })
	d.Events = slices.DeleteFunc(d.Events, func(e *Event) bool { return e.Sub == id })
}

// background runs work off the main thread, and then done on it, in the
// app; in tests, at once.
func (a *app) background(work, done func()) {
	if !a.live {
		work()
		done()
		return
	}
	go func() {
		work()
		mygo.RunOnMain(func() {
			done()
			a.redraw()
		})
	}()
}

// subscribe follows the calendar at the address typed, at once.
func (a *app) subscribe(address string) error {
	u, err := calendarURL(address)
	if err != nil {
		return err
	}
	for _, s := range a.data.Subscriptions {
		if s.URL == u {
			return nil // followed already
		}
	}
	s := &Subscription{ID: newID(), URL: u, Kind: a.data.newKind()}
	a.data.Subscriptions = append(a.data.Subscriptions, s)
	a.changed()
	a.refreshSubscription(s)
	return nil
}

// refreshSubscription gets a subscription's calendar again, keeping its
// events as they were if it cannot, and why.
func (a *app) refreshSubscription(s *Subscription) {
	if a.subBusy[s.ID] {
		return
	}
	if a.subBusy == nil {
		a.subBusy, a.subErr = map[string]bool{}, map[string]string{}
	}
	a.subBusy[s.ID] = true
	var got icsImport
	var err error
	fetch, u := a.fetch, s.URL
	if fetch == nil {
		fetch = fetchCalendar
	}
	a.background(func() {
		var b []byte
		if b, err = fetch(context.Background(), u); err == nil {
			got, err = readICS(b, time.Local)
		}
	}, func() {
		delete(a.subBusy, s.ID)
		if a.data.subscription(s.ID) != s {
			return // unsubscribed meanwhile
		}
		if err != nil {
			a.subErr[s.ID] = err.Error()
			return
		}
		delete(a.subErr, s.ID)
		a.data.applySubscription(s, got, a.now())
		a.changed()
	})
}

// refreshSubscriptions gets every subscription's calendar again.
func (a *app) refreshSubscriptions() {
	for _, s := range a.data.Subscriptions {
		a.refreshSubscription(s)
	}
}

// subscriptionsRows lists the subscriptions in the settings, each with
// the tag of its events, its alerts, a refresh and an unsubscribe, and a
// field to follow another.
func (a *app) subscriptionsRows(c *ui.Context) {
	p := &a.p
	var gone *Subscription
	for _, s := range a.data.Subscriptions {
		ui.Row(c.Key(s.ID)).MinHeight(54).Padding(7, 10, 7, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			a.dot(c, a.kindColor(s.Kind), 10)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				name := ui.TextInputBase(c, &s.Name).Label(a.tr.SubName()).FontSize(13).FontWeight(500).TextColor(p.fg)
				if name.Changed() {
					a.changed()
				}
				status := a.tr.SubNever()
				col := p.muted
				switch {
				case a.subBusy[s.ID]:
					status = a.tr.SubUpdating()
				case a.subErr[s.ID] != "":
					status, col = a.tr.SubFailed(a.subErr[s.ID]), p.red
				case !s.Updated.IsZero():
					status = a.tr.SubUpdated(a.fmt(s.Updated, "MMMd") + " " + a.clock(s.Updated))
				}
				ui.Text(c, status+" · "+s.URL).FontSize(11).TextColor(col).SingleLine().MinWidth(0)
			})
			kinds := []KindID{""}
			for _, k := range a.data.Kinds {
				kinds = append(kinds, k.ID)
			}
			if selectOf(a, c, "kind", &s.Kind, kinds, func(id KindID) string {
				if id == "" {
					return a.tr.NoKind()
				}
				return a.kindName(id)
			}).MaxWidth(120).Label(a.tr.SubTag()).Changed() {
				for _, e := range a.data.Events {
					if e.Sub == s.ID {
						e.Kind = s.Kind
					}
				}
				a.changed()
			}
			bell := ui.ToggleBase(c.Key("alerts"), &s.Alerts).Size(26, 26).Center().Radius(7).Label(a.tr.SubAlerts()).Tooltip(a.tr.SubAlerts())
			if s.Alerts {
				bell.TextColor(p.accent).Background(p.accentSoft)
			} else {
				bell.TextColor(p.faint)
			}
			bell.Children(func() { ui.Icon(c, iconBell).Size(14, 14) })
			if bell.Changed() {
				a.changed()
			}
			if a.iconBtn(c, iconRepeat, a.tr.SubRefresh()).Clicked() {
				a.refreshSubscription(s)
			}
			if a.iconBtn(c, iconTrash, a.tr.Unsubscribe()).Clicked() {
				gone = s
			}
		})
	}
	ui.Row(c.Key("subscribe")).MinHeight(46).Padding(6, 10, 6, 14).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconCalendar).Size(14, 14).TextColor(p.muted)
		in := ui.TextInputBase(c, &a.subDraft).Placeholder(a.tr.SubURLPlaceholder()).Label(a.tr.SubURLPlaceholder()).
			FontSize(12.5).Grow(1).TextColor(p.fg)
		add := a.btn(c, a.tr.Subscribe(), btnSecondary)
		if strings.TrimSpace(a.subDraft) != "" && (in.Submitted() || add.Clicked()) {
			if err := a.subscribe(a.subDraft); err != nil {
				a.toast = a.tr.SubFailed(err.Error())
			} else {
				a.subDraft = ""
			}
		}
	})
	if gone != nil {
		step := a.remember()
		a.data.unsubscribe(gone.ID)
		a.changed()
		a.deleted(c, step, gone.Name)
	}
}

// eventInfo shows an event of a subscription while a.infoOpen: what and
// when, where it comes from, and where to change it.
func (a *app) eventInfo(c *ui.Context) {
	p := &a.p
	e := a.info
	if e == nil {
		return
	}
	a.modal(c, &a.infoOpen, 440, func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			a.dot(c, a.kindColor(e.Kind), 10)
			ui.Text(c, orUntitled(e.Title, a.tr.Untitled())).FontSize(15).FontWeight(600).TextColor(p.fg).Grow(1).MinWidth(0)
			if a.iconBtn(c, iconClose, a.tr.Close()).Clicked() {
				a.infoOpen = false
			}
		})
		when := a.fmt(e.Start, "yMMMEd")
		switch {
		case e.AllDay && daysBetween(e.Start, e.End) > 1:
			when += " – " + a.fmt(addDays(e.End, -1), "yMMMEd")
		case !e.AllDay:
			when += " · " + a.clock(e.Start) + " – " + a.clock(e.End)
		}
		row := func(icon *ui.SVG, s string) {
			ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
				ui.Icon(c, icon).Size(14, 14).TextColor(p.muted).Margin(2, 0, 0, 0)
				ui.Text(c, s).FontSize(13).LineHeight(1.45).TextColor(p.fg2).Grow(1).MinWidth(0).Selectable()
			})
		}
		row(iconClock, when)
		if e.Location != "" {
			row(iconPin, e.Location)
		}
		if e.Notes != "" {
			row(iconNote, e.Notes)
		}
		name := ""
		if s := a.data.subscription(e.Sub); s != nil {
			name = s.Name
		}
		a.notice(c, iconCalendar, a.tr.SubFrom(name)+" "+a.tr.SubReadOnly(), a.tr.Close(), func() { a.infoOpen = false })
	})
}

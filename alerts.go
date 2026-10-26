package main

import (
	"strconv"
	"time"
)

// alert is a notification due for an event.
type alert struct {
	id       string // "event:" and the event's ID, which a click on it opens
	title    string
	subtitle string
	body     string
}

// alertLead returns how long before e starts its alert goes, and whether
// it has one: its own, or the default of the settings. All-day events
// have none.
func (a *app) alertLead(e *Event) (time.Duration, bool) {
	n := a.data.Prefs.AlertMinutes
	if e.Alert != nil {
		n = *e.Alert
	}
	if n < 0 || e.AllDay {
		return 0, false
	}
	return time.Duration(n) * time.Minute, true
}

// dueAlerts returns the alerts that fell due since it was last asked, up
// to now, once each: an event moved to another time alerts again. An
// event that has ended alerts no more.
func (a *app) dueAlerts(now time.Time) []alert {
	from := a.alertedTo
	if now.After(a.alertedTo) {
		a.alertedTo = now
	}
	var out []alert
	// The events starting within the longest lead, a day, and those going
	// on, with the occurrences of repeating ones.
	for _, e := range a.data.eventsBetween(now.Add(-48*time.Hour), now.Add(48*time.Hour)) {
		lead, ok := a.alertLead(e)
		if !ok || !e.End.After(now) {
			continue
		}
		at := e.Start.Add(-lead)
		if !at.After(from) || at.After(now) {
			continue
		}
		key := e.ID + "@" + strconv.FormatInt(e.Start.Unix(), 10) // an occurrence, or a time moved to, alerts anew
		if a.alerted[key] {
			continue
		}
		a.alerted[key] = true
		body := a.clock(e.Start) + " – " + a.clock(e.End)
		if e.Location != "" {
			body += " · " + e.Location
		}
		if e.Project != "" {
			body += " · " + a.projectName(e.Project)
		}
		out = append(out, alert{
			id:       "event:" + e.key(),
			title:    orUntitled(e.Title, a.tr.Untitled()),
			subtitle: a.tr.AlertStartsIn(max(int(e.Start.Sub(now).Minutes()+0.5), 0)),
			body:     body,
		})
	}
	return out
}

package main

import (
	"strconv"
	"strings"
	"time"
)

// alert is a notification due for an event.
type alert struct {
	id       string // "event:" and the event's key, or "task:" and the task's ID, which a click on it opens
	group    string
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
	if e.Sub != "" {
		// A subscription's events alert only if it says so: its calendar
		// alerts too, where it comes from.
		if s := a.data.subscription(e.Sub); s == nil || !s.Alerts {
			return 0, false
		}
	}
	return time.Duration(n) * time.Minute, true
}

// dueAlerts returns the alerts that fell due since it was last asked, up
// to now, and those due before of events yet to start, as one made or
// moved within its lead: once each, though an event moved to another time
// alerts again. An event that has ended alerts no more.
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
		if at.After(now) || !at.After(from) && !e.Start.After(now) {
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
			group:    "events",
			title:    orUntitled(e.Title, a.tr.Untitled()),
			subtitle: a.tr.AlertStartsIn(max(int(e.Start.Sub(now).Minutes()+0.5), 0)),
			body:     body,
		})
	}
	// The tasks' reminders, once each, which a task done or put away with
	// its project does without.
	for _, t := range a.data.Tasks {
		if t.Done || t.Remind.IsZero() || !t.Remind.After(from) || t.Remind.After(now) || a.data.shelved(t) {
			continue
		}
		key := "task:" + t.ID + "@" + strconv.FormatInt(t.Remind.Unix(), 10)
		if a.alerted[key] {
			continue
		}
		a.alerted[key] = true
		var body []string
		if t.Project != "" {
			body = append(body, a.projectName(t.Project))
		}
		if !t.Due.IsZero() {
			body = append(body, a.relDay(t.Due))
		}
		out = append(out, alert{
			id:       "task:" + t.ID,
			group:    "tasks",
			title:    orUntitled(t.Title, a.tr.Untitled()),
			subtitle: a.tr.Reminder(),
			body:     strings.Join(body, " · "),
		})
	}
	return out
}

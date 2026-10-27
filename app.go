package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"nexttodo/locales"
)

// Page is a page of the main window, as the title bar's switch lists them.
type Page int

const (
	PageOverview Page = iota
	PageTasks
	PageSchedule
	PageProjects
	PageDirections
	PageSettings
)

var pages = []Page{PageOverview, PageTasks, PageSchedule, PageProjects, PageDirections}

// app is the state the windows show: the main window and the menu bar's
// panel. Everything here is read and changed on the main thread.
type app struct {
	data  *Data
	path  string // where data is saved; "" keeps it in memory
	win   *mygo.Window
	panel *mygo.Window // the menu bar's panel while it shows
	tray  *mygo.Tray
	live  bool // running as the app, not under tests
	// quitting lets the windows close for good.
	quitting bool
	badge    int // the count on the Dock icon
	now      func() time.Time

	// Set at the start of every build.
	p      palette
	tr     locales.Messages
	locale string // the locale dates are written in
	lang   string // the language the app shows

	page    Page
	addOpen bool // the title bar's menu of new things

	// Tasks.
	taskList   int    // My Day, Important, Planned, All, Completed
	taskFilter string // a project's ID, "" for all
	taskSel    string
	taskDraft  string
	taskAdd    ui.Handle // the field adding tasks, which New Task focuses

	// Schedule.
	schedAnchor   time.Time // a day of the week or month shown
	schedMode     int       // 0 week, 1 month
	schedScroll   ui.ScrollState
	schedScrolled bool
	evt           eventDraft
	evtOpen       bool
	askEvent      bool
	drag          eventDrag
	sketch        sketch
	colW          float32 // the width of a day in the week, as last painted

	// Projects.
	projGroup        int // by category, direction or status
	projQuery        string
	projShowDone     bool
	projShowArchived bool
	projOpen         string // the project shown, "" for the list
	projTaskDraft    string
	journalDraft     string
	askProject       bool

	// Categories.
	catOpen  bool
	catDraft string
	catPop   CatID // the category whose look is being chosen
	askCat   CatID // asking whether to delete it

	picturing bool // drawing the README's pictures, of the sample data as one's own

	// repeated holds, by task, the next a repeating task done brought, to
	// take back as it is undone again: its ID and how it repeated.
	repeated map[string]repeated

	stepDraft string // the step being typed into a task's checklist

	// The tags of events.
	kindsOpen bool
	kindDraft string
	kindPop   KindID // the tag whose color is being chosen
	askKind   KindID // asking whether to delete it

	// Directions.
	dir     dirDraft
	dirOpen bool
	askDir  bool

	undo []*undoStep // deletions to undo, the last last
	// replaced is the step of the data replaced, by a backup or erased, to
	// say so with Undo.
	replaced    *undoStep
	replacedMsg string

	// Search.
	searchOpen  bool
	searchQuery string
	searchSel   int
	searchMoved bool // the arrows moved the choice in this frame
	searchField ui.Handle

	// Settings.
	openAtLogin bool     // as macOS says, asked once at launch
	backupList  []backup // the automatic backups, as the settings show them
	backupsRead bool
	askErase    bool
	notifyNote  string // what the test notification found

	// The menu bar's panel.
	quick quickDraft

	// literal holds the fields whose line is added as typed, its time unread.
	literal map[*string]bool

	// Alerts.
	alerted   map[string]bool // events alerted, by ID and start
	alertedTo time.Time       // alerts up to this time have been looked at

	toast string // a message to show in the next frame
	// later runs in the next build of the main window, for what a menu
	// chooses and needs the window's context for, as a toast.
	later     []func(c *ui.Context)
	dirty     bool
	saveTimer *time.Timer
}

func newApp(d *Data, path string) *app {
	a := &app{data: d, path: path, now: time.Now, alerted: map[string]bool{}, literal: map[*string]bool{}}
	a.schedAnchor = day(a.now())
	a.alertedTo = a.now().Add(-time.Minute)
	a.quick = quickDraft{minutes: 60}
	return a
}

// changed notes that the data changed: the windows draw it, and it is
// saved shortly after.
func (a *app) changed() {
	a.dirty = true
	a.redraw()
	a.updateBadge()
	if a.path == "" {
		return
	}
	if !a.live {
		a.flush()
		return
	}
	if a.saveTimer == nil {
		a.saveTimer = time.AfterFunc(400*time.Millisecond, func() { mygo.RunOnMain(a.flush) })
		return
	}
	a.saveTimer.Reset(400 * time.Millisecond)
}

// redraw draws the windows anew, for what one of them changed.
func (a *app) redraw() {
	for _, w := range []*mygo.Window{a.win, a.panel} {
		if w != nil && !w.IsDestroyed() {
			w.Invalidate()
		}
	}
}

// flush writes the data if it changed.
func (a *app) flush() {
	if !a.dirty || a.path == "" {
		return
	}
	if err := backupDaily(a.path, a.now()); err != nil {
		log.Printf("backing up: %v", err)
	}
	b, err := a.data.encode()
	if err == nil {
		err = writeFile(a.path, b)
	}
	if err != nil {
		log.Printf("saving %s: %v", a.path, err)
		return
	}
	a.dirty = false
	a.backupsRead = false
}

// loadData reads the data at path; a first launch starts with nothing but
// the default categories and tags, for the user's own things.
func loadData(path string, now time.Time, lang string) *Data {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		d := freshData(lang)
		if b, err := d.encode(); err == nil {
			writeFile(path, b)
		}
		return d
	}
	if err == nil {
		var d *Data
		if d, err = decodeData(b); err == nil {
			d.migrate(lang)
			return d
		}
	}
	// Keep what could not be read, and start afresh.
	log.Printf("reading %s: %v", path, err)
	os.Rename(path, strings.TrimSuffix(path, ".json")+".unreadable-"+now.Format("20060102-150405")+".json")
	return freshData(lang)
}

// Names.

func (a *app) categoryName(id CatID) string {
	if k := a.data.category(id); k != nil {
		return orUntitled(k.Name, a.tr.Untitled())
	}
	return a.tr.Uncategorized()
}

// categoryLook returns the icon and the color a project's category shows
// it with: a gray folder without one.
func (a *app) categoryLook(id CatID) (*ui.SVG, ui.Color) {
	k := a.data.category(id)
	if k == nil {
		return iconFolder, a.p.series[6]
	}
	return categoryIcon(k.Icon), a.p.series[(k.Color%7+7)%7]
}

// categoryIDs returns the categories in their order, with none ("") first
// when withNone.
func (a *app) categoryIDs(withNone bool) []CatID {
	var ids []CatID
	if withNone {
		ids = append(ids, "")
	}
	for _, k := range a.data.Categories {
		ids = append(ids, k.ID)
	}
	return ids
}

func (a *app) statusName(s Status) string {
	switch s {
	case StatusActive:
		return a.tr.StatusActive()
	case StatusPaused:
		return a.tr.StatusPaused()
	case StatusDone:
		return a.tr.StatusDone()
	}
	return a.tr.StatusPlanning()
}

// kindName is the name of an event's tag, "" for none.
func (a *app) kindName(id KindID) string {
	if k := a.data.kind(id); k != nil {
		return orUntitled(k.Name, a.tr.Untitled())
	}
	return ""
}

func (a *app) priorityName(p int) string {
	switch p {
	case 1:
		return a.tr.PriorityLow()
	case 2:
		return a.tr.PriorityMedium()
	case 3:
		return a.tr.PriorityHigh()
	}
	return a.tr.PriorityNone()
}

func (a *app) projectName(id string) string {
	if id == "" {
		return a.tr.NoProject()
	}
	if p := a.data.project(id); p != nil {
		return orUntitled(p.Title, a.tr.Untitled())
	}
	return a.tr.NoProject()
}

// projectChoices are the projects a picker offers: none, those not
// archived, and current even if it is.
func (a *app) projectChoices(current string) []string {
	ids := []string{""}
	for _, pr := range a.data.Projects {
		if !pr.Archived || pr.ID == current {
			ids = append(ids, pr.ID)
		}
	}
	return ids
}

func (a *app) directionName(id string) string {
	if d := a.data.direction(id); d != nil {
		return orUntitled(d.Title, a.tr.Untitled())
	}
	return a.tr.NoDirection()
}

func orUntitled(s, untitled string) string {
	if strings.TrimSpace(s) == "" {
		return untitled
	}
	return s
}

func (a *app) navLabel(p Page) string {
	switch p {
	case PageTasks:
		return a.tr.NavTasks()
	case PageSchedule:
		return a.tr.NavSchedule()
	case PageProjects:
		return a.tr.NavProjects()
	case PageDirections:
		return a.tr.NavDirections()
	case PageSettings:
		return a.tr.NavSettings()
	}
	return a.tr.NavOverview()
}

// Dates, in the app's language and the user's formats.

func (a *app) fmt(t time.Time, skeleton string) string {
	return i18n.FormatDate(a.locale, t, i18n.DateOptions{Skeleton: skeleton})
}

// clock writes a time of day in the user's hour cycle.
func (a *app) clock(t time.Time) string { return a.fmt(t, "jm") }

// relDay writes a due date relative to today: Today, Tomorrow, In 3 days,
// 2 days late; beyond a week, the date itself.
func (a *app) relDay(t time.Time) string {
	n := daysBetween(a.now(), t)
	switch {
	case n < 0:
		return a.tr.LateBy(-n)
	case n <= 6:
		return a.tr.DueIn(n)
	}
	return a.fmt(t, "MMMd")
}

// until writes how long until t, from now: In 25 minutes, In 2 h 15 min.
func (a *app) until(t time.Time) string {
	m := int(t.Sub(a.now()).Minutes() + 0.5)
	if m < 60 {
		return a.tr.InMinutes(max(m, 1))
	}
	return a.tr.InHours(m/60, m%60)
}

func (a *app) firstWeekday(c *ui.Context) time.Weekday {
	switch a.data.Prefs.WeekStart {
	case 0:
		return time.Sunday
	case 1:
		return time.Monday
	}
	if r := c.Region(); r.HourCycle != 0 || r.Locale != "" {
		return r.FirstWeekday
	}
	return time.Monday
}

// The view.

// begin sets what every build of a window reads: the palette, the theme,
// the language and the formats.
func (a *app) begin(c *ui.Context) {
	if c.Theme().Dark {
		a.p = darkPalette()
	} else {
		a.p = lightPalette()
	}
	c.SetTheme(a.p.theme(c.Theme()))
	a.lang = c.Locale()
	a.locale = c.Region().FormatLocale(a.lang)
	a.tr = locales.In(a.lang)
	// Midnight passes while the app is open: draw the new day.
	now := a.now()
	c.After(at(addDays(now, 1), 0, 0).Sub(now) + time.Second)
}

// view is the main window: the title bar, with the pages' switch in its
// middle, over the page shown.
func (a *app) view(c *ui.Context) {
	a.begin(c)
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}
	if step := a.replaced; step != nil {
		a.replaced = nil
		c.ToastAction(a.replacedMsg, a.tr.Undo(), func() { a.restore(step) })
	}
	a.undoShortcut(c)
	if c.Shortcut(ui.Cmd, ui.KeyF) {
		a.openSearch()
	}
	ui.Column(c).Fill().Children(func() {
		a.header(c)
		key := any(a.page)
		if a.page == PageProjects && a.projOpen != "" {
			key = a.projOpen
		}
		ui.Column(c.Key(key)).Grow(1).MinHeight(0).Children(func() {
			switch a.page {
			case PageOverview:
				a.overview(c)
			case PageTasks:
				a.tasksPage(c)
			case PageSchedule:
				a.schedulePage(c)
			case PageProjects:
				if a.data.project(a.projOpen) != nil {
					a.projectPage(c)
				} else {
					a.projOpen = ""
					a.projectsPage(c)
				}
			case PageDirections:
				a.directionsPage(c)
			case PageSettings:
				a.settingsPage(c)
			}
		})
	})
	a.eventDialog(c)
	a.directionDialog(c)
	a.categoriesSheet(c)
	a.kindsSheet(c) // over the event's dialog, which opens it
	a.searchSheet(c)
}

// header is the title bar: the app's mark after the window's controls,
// the switch of pages in the middle, and new things and the settings on
// the right, as magpie's.
func (a *app) header(c *ui.Context) {
	p := &a.p
	bar := c.TitleBar()
	h := ui.Row(c).Height(max(bar.Height, 50)).Shrink(0).Padding(0, 12+bar.Right, 0, max(bar.Left+8, 16)).
		AlignItems(ui.Center).Gap(4).DragWindow()
	h.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconLogo).Size(17, 17).TextColor(p.fg)
			ui.Text(c, a.tr.AppName()).FontSize(13).FontWeight(600).TextColor(p.fg).SingleLine()
		})
		ui.Spacer(c)
		if a.iconBtn(c, iconSearch, a.tr.Search()+"  ⌘F").Clicked() {
			a.openSearch()
		}
		add := a.iconBtn(c, iconPlus, a.tr.MenuNewTask()+" / "+a.tr.NewEvent())
		if add.Clicked() {
			a.addOpen = !a.addOpen
		}
		ui.PopoverBase(c, add, &a.addOpen, func(panel ui.Element) {
			popover(a, panel).Margin(4, 0, 0, 0).Padding(4).MinWidth(190).Gap(1)
			item := func(icon *ui.SVG, label, keys string, run func()) {
				b := ui.ButtonBase(c.Key(label)).Height(28).Padding(0, 8).Gap(8).Radius(7).AlignItems(ui.Center)
				if b.Hovered() {
					b.Background(p.pill)
				}
				b.Children(func() {
					ui.Icon(c, icon).Size(14, 14).TextColor(p.muted)
					ui.Text(c, label).FontSize(12.5).TextColor(p.fg).Grow(1)
					ui.Text(c, keys).FontSize(11).TextColor(p.faint)
				})
				if b.Clicked() {
					a.addOpen = false
					run()
				}
			}
			item(iconCheck, a.tr.MenuNewTask(), "⌘N", a.startTask)
			item(iconCalendar, a.tr.MenuNewEvent(), "⇧⌘N", func() { a.newEvent(a.nextSlot(), "") })
			item(iconFolder, a.tr.MenuNewProject(), "⌥⌘N", a.newProject)
		})
		gear := a.iconBtn(c, iconGear, a.tr.NavSettings())
		if a.page == PageSettings {
			gear.Background(p.pill).TextColor(p.fg)
		}
		if gear.Clicked() || gear.Pressed() && a.page != PageSettings {
			a.navigate(PageSettings) // as the pointer goes down, as the pages do
		}
		a.pageSwitch(c).Attach(ui.AnchorCenter, ui.AnchorCenter)
	})
}

// pageSwitch is the switch of pages in the middle of the title bar: no
// track, a white thumb under the page shown, none on the settings.
func (a *app) pageSwitch(c *ui.Context) ui.Element {
	labels := make([]string, len(pages))
	chosen := -1
	for i, pg := range pages {
		labels[i] = a.navLabel(pg)
		if pg == a.page {
			chosen = i
		}
	}
	track := ui.Box(c).FocusGroup(ui.Horizontal).Role(ui.RoleTabList)
	a.segments(c, track, labels, chosen, func(i int) ui.Element {
		b := ui.ButtonBase(c.Key(pages[i])).Role(ui.RoleTab).Selected(i == chosen)
		// As the pointer goes down, as Safari's tabs do: a click's
		// hundred milliseconds sooner.
		if b.Clicked() || b.Pressed() && i != chosen {
			a.navigate(pages[i])
			c.Invalidate() // the thumb, built before, moves in the next frame
		}
		return b
	})
	return track
}

// navigate shows a page; the pages keep their own state. The next launch
// opens on it too.
func (a *app) navigate(p Page) {
	a.page = p
	if a.data.Prefs.Page != int(p) {
		a.data.Prefs.Page = int(p)
		a.dirty = true
	}
	if p == PageProjects {
		a.projOpen = ""
	}
	a.backupsRead = false // read again as the settings show
}

// startTask shows the tasks page with its field ready for a new task.
func (a *app) startTask() {
	a.navigate(PageTasks)
	if a.taskList == listDone {
		a.taskList = listMyDay
	}
	a.taskSel = ""
	a.taskAdd.Focus()
}

func (a *app) openProject(id string) {
	a.page = PageProjects
	a.projOpen = id
	a.projTaskDraft, a.journalDraft = "", ""
}

func (a *app) openTask(id string) {
	a.page = PageTasks
	a.taskSel = id
	if t := a.data.task(id); t != nil {
		a.taskFilter = ""
		switch {
		case a.inList(t, listMyDay):
			a.taskList = listMyDay
		case t.Done:
			a.taskList = listDone
		case !t.Due.IsZero():
			a.taskList = listPlanned
		default:
			a.taskList = listAll
		}
	}
}

// openEvent shows an event's week and opens it: ref is its ID, or for an
// occurrence its series' with its start, as Event.key writes it.
func (a *app) openEvent(ref string) {
	id, start := occurrenceRef(ref)
	e := a.data.event(id)
	if e == nil {
		return
	}
	if e.repeating() && !start.IsZero() {
		for _, occ := range e.occurrences(day(start), addDays(day(start), 1)) {
			if occ.Start.Equal(start) {
				e = occ
			}
		}
	}
	a.page, a.schedMode = PageSchedule, 0
	a.schedAnchor = day(e.Start)
	a.schedScrolled = false
	a.editEvent(e)
}

// page lays out a page's content: its toolbar, then what scrolls under it,
// no wider than reads well.
func (a *app) pageBody(c *ui.Context, toolbar func(), body func()) {
	if toolbar != nil {
		ui.Row(c).Padding(2, 18, 6).Gap(8).AlignItems(ui.Center).Shrink(0).MaxWidth(1180).Margin(0, ui.Auto).FillWidth().
			Children(toolbar)
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(0, 18, 22).MaxWidth(1180).Margin(0, ui.Auto).FillWidth().Children(body)
	})
}

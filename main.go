// NextTodo is a quiet planner for research on the Mac: schedules, tasks and
// projects sorted by how they are funded and the directions they serve,
// with reminders before events and a panel in the menu bar. It is drawn
// with MyGo's native UI, in the manner of magpie.
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"nexttodo/locales"
)

func main() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	a := newApp(newData(), filepath.Join(dir, "data.json"))
	a.live = true

	// A click on an event's alert opens the event, also one of an earlier
	// run that launched the app again.
	mygo.App.OnNotificationClick(func(id string) {
		if ev, ok := strings.CutPrefix(id, "event:"); ok {
			a.showMain()
			a.openEvent(ev)
			a.redraw()
		}
	})
	mygo.App.WhenReady(func() {
		// A first launch writes the sample data in the language the app
		// shows.
		a.data = loadData(a.path, time.Now(), mygo.App.Locale())
		mygo.Theme.SetSource(themeSource(a.data.Prefs.Appearance))
		mygo.App.SetMenu(a.menu())
		a.setMenuBar(!a.data.Prefs.HideMenuBar)
		// Opened at login, it waits in the menu bar for the day's alerts.
		if !mygo.App.WasOpenedAtLogin() || a.tray == nil {
			a.openMain()
		}
		a.setHotkey(!a.data.Prefs.NoHotkey)
		a.updateBadge()
		go a.watchAlerts()
		a.captureForCheck()
	})
	mygo.App.OnLocaleChanged(func(string) {
		mygo.App.SetMenu(a.menu())
		if a.win != nil {
			a.win.SetTitle(locales.AppName())
		}
	})
	mygo.App.OnWindowAllClosed(func() {
		a.flush()
		if runtime.GOOS != "darwin" && a.tray == nil {
			mygo.App.Quit()
		}
	})
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			a.showMain()
		}
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		a.flush()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// openMain opens the main window.
func (a *app) openMain() {
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:          locales.AppName(),
		Width:          1240,
		Height:         820,
		MinWidth:       940,
		MinHeight:      600,
		StateKey:       "main",
		TitleBarStyle:  mygo.TitleBarHiddenInset,
		TitleBarHeight: 50,
		Content:        ui.View(a.view),
	})
	a.win = win
	win.OnClose(func(*mygo.CloseEvent) { a.flush() })
	win.OnClosed(func() {
		if a.win == win {
			a.win = nil
		}
	})
}

// showMain brings the main window to the front, opening it again if it
// was closed, and hides the menu bar's panel.
func (a *app) showMain() {
	a.hidePanel()
	if a.win == nil || a.win.IsDestroyed() {
		a.openMain()
	} else {
		a.win.Show()
		a.win.Focus()
	}
	mygo.App.Focus()
}

// setMenuBar shows or hides the app's icon in the menu bar.
func (a *app) setMenuBar(show bool) {
	if !a.live {
		return
	}
	if !show {
		if a.panel != nil {
			a.panel.Destroy()
			a.panel = nil
		}
		if a.tray != nil {
			a.tray.Destroy()
			a.tray = nil
		}
		return
	}
	if a.tray != nil {
		return
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{ID: "today", Icon: trayIcon(), IconIsTemplate: true, ToolTip: a.tr.AppName()})
	if err != nil {
		log.Println("tray:", err)
		return
	}
	a.tray = tray
	a.ensurePanel()
	tray.OnClick(a.togglePanel)
	tray.OnRightClick(func() {
		a.hidePanel()
		// The menu for this popup only, so that clicks keep opening the
		// panel.
		tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Label: a.tr.PanelOpen(), Click: func(*mygo.MenuItem, *mygo.Window) { a.showMain() }},
			mygo.Separator(),
			{Label: a.tr.PanelQuit(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
		}))
		defer tray.SetMenu(nil)
		tray.PopUpMenu()
	})
}

const panelWidth, panelHeight = 340, 480

// ensurePanel makes the menu bar's panel once, hidden: as magpie's, a
// panel on a frosted material with rounded corners, which opening and
// closing only show and hide, so it opens at once, already laid out.
func (a *app) ensurePanel() *mygo.Window {
	if a.panel != nil && !a.panel.IsDestroyed() {
		return a.panel
	}
	panel := mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:         a.tray,
		Placement:    mygo.PlacementBottom,
		Gap:          6,
		Width:        panelWidth,
		Height:       panelHeight,
		Focusable:    true,
		Shadow:       true,
		Vibrancy:     mygo.VibrancyPopover,
		CornerRadius: 12,
		Hidden:       true,
		Content:      ui.View(a.panelView),
	})
	// A click elsewhere, Escape or the icon closes it: it hides instead,
	// unless the app is quitting.
	panel.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			panel.Hide()
		}
	})
	panel.OnClosed(func() {
		if a.panel == panel {
			a.panel = nil
		}
	})
	a.panel = panel
	return panel
}

// hotkey opens the menu bar's panel from anywhere.
const hotkey = "Ctrl+Alt+Space"

func (a *app) setHotkey(on bool) {
	if !a.live {
		return
	}
	if !on {
		mygo.GlobalShortcut.Unregister(hotkey)
		return
	}
	if mygo.GlobalShortcut.IsRegistered(hotkey) {
		return
	}
	err := mygo.GlobalShortcut.Register(hotkey, func() {
		if a.tray == nil {
			a.showMain()
			a.startTask()
			return
		}
		if a.panel == nil || !a.panel.IsVisible() {
			a.togglePanel()
		}
		mygo.App.Focus()
	})
	if err != nil {
		log.Println("shortcut:", err)
	}
}

// updateBadge counts on the Dock icon the tasks due today or late, unless
// the settings say not to.
func (a *app) updateBadge() {
	if !a.live {
		return
	}
	n := 0
	if !a.data.Prefs.NoBadge {
		today := day(a.now())
		for _, t := range a.data.Tasks {
			if !t.Done && !t.Due.IsZero() && !day(t.Due).After(today) && !a.data.shelved(t) {
				n++
			}
		}
	}
	if n != a.badge {
		a.badge = n
		mygo.App.SetBadgeCount(n)
	}
}

// togglePanel shows the menu bar's panel below the icon, or hides it.
func (a *app) togglePanel() {
	panel := a.ensurePanel()
	if panel.IsVisible() {
		panel.Hide()
		return
	}
	a.quick.start, a.quick.added = nextHalfHour(a.now()), ""
	panel.SetSize(panelWidth, panelHeight) // under the icon, wherever it is now
	panel.Show()
	panel.Focus()
	a.quick.field.Focus()
}

func (a *app) hidePanel() {
	if a.panel != nil && !a.panel.IsDestroyed() {
		a.panel.Hide()
	}
}

// watchAlerts sends the alerts of events as they fall due, for as long as
// the app runs, the main window open or not.
func (a *app) watchAlerts() {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for range tick.C {
		var due []alert
		mygo.RunOnMain(func() {
			a.useAppLanguage()
			due = a.dueAlerts(time.Now())
			a.updateBadge() // as days turn
		})
		for _, al := range due {
			n := mygo.NewNotification(mygo.NotificationOptions{ID: al.id, Title: al.title, Subtitle: al.subtitle, Body: al.body, Group: "events"})
			if err := n.Show(); err != nil {
				log.Println("alert:", err)
			}
		}
	}
}

// useAppLanguage sets the language and formats for work done outside a
// window's build, as alerts.
func (a *app) useAppLanguage() {
	a.lang = mygo.App.Locale()
	a.locale = mygo.App.Region().FormatLocale(a.lang)
	a.tr = locales.In(a.lang)
}

// menu is the menu bar's menus, in the language the app shows.
func (a *app) menu() *mygo.Menu {
	tr := locales.In(mygo.App.Locale())
	run := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			if a.win == nil || a.win.IsDestroyed() {
				a.openMain()
			}
			fn()
			a.redraw()
		}
	}
	page := func(p Page) func(*mygo.MenuItem, *mygo.Window) { return run(func() { a.navigate(p) }) }
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Role: mygo.RoleFileMenu, Submenu: []*mygo.MenuItem{
			{Label: tr.MenuNewTask(), Accelerator: "CmdOrCtrl+N", Click: run(a.startTask)},
			{Label: tr.MenuNewEvent(), Accelerator: "CmdOrCtrl+Shift+N", Click: run(func() { a.newEvent(a.nextSlot(), "") })},
			{Label: tr.MenuNewProject(), Accelerator: "CmdOrCtrl+Alt+N", Click: run(a.newProject)},
			mygo.Separator(),
			{Label: tr.MenuExport(), Accelerator: "CmdOrCtrl+Shift+E", Click: run(a.exportBackup)},
			{Label: tr.MenuImport(), Click: run(a.importBackup)},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: tr.MenuGo(), Submenu: []*mygo.MenuItem{
			{Label: tr.NavOverview(), Accelerator: "CmdOrCtrl+1", Click: page(PageOverview)},
			{Label: tr.NavTasks(), Accelerator: "CmdOrCtrl+2", Click: page(PageTasks)},
			{Label: tr.NavSchedule(), Accelerator: "CmdOrCtrl+3", Click: page(PageSchedule)},
			{Label: tr.NavProjects(), Accelerator: "CmdOrCtrl+4", Click: page(PageProjects)},
			{Label: tr.NavDirections(), Accelerator: "CmdOrCtrl+5", Click: page(PageDirections)},
			mygo.Separator(),
			{Label: tr.NavSettings(), Accelerator: "CmdOrCtrl+,", Click: page(PageSettings)},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

// captureForCheck saves what the windows show, to check the app as it
// really renders: NEXTTODO_CAPTURE=<dir> writes main.png once the window
// has drawn, NEXTTODO_PAGE picks its page (0 to 5), and NEXTTODO_PANEL
// opens the menu bar's panel and writes panel.png too.
func (a *app) captureForCheck() {
	dir := os.Getenv("NEXTTODO_CAPTURE")
	if dir == "" {
		return
	}
	if pg := os.Getenv("NEXTTODO_PAGE"); pg != "" {
		a.navigate(Page(pg[0] - '0'))
	}
	if os.Getenv("NEXTTODO_PANEL") != "" && a.tray != nil {
		a.togglePanel()
	}
	go func() {
		time.Sleep(2 * time.Second)
		var wins = map[string]*mygo.Window{}
		mygo.RunOnMain(func() {
			wins["main"] = a.win
			if a.panel != nil && a.panel.IsVisible() {
				wins["panel"] = a.panel
			}
		})
		for name, w := range wins {
			if w == nil {
				continue
			}
			if b, err := w.CapturePage(); err == nil {
				os.WriteFile(filepath.Join(dir, name+".png"), b, 0o644)
			} else {
				log.Println("capture:", err)
			}
		}
	}()
}

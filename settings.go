package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var languageTags = []string{"", "en", "zh-Hans"}

func themeSource(appearance string) mygo.ThemeSource {
	switch appearance {
	case "light":
		return mygo.ThemeLight
	case "dark":
		return mygo.ThemeDark
	}
	return mygo.ThemeSystem
}

// The defaults an event's alert can take.
var alertDefaults = []int{alertNone, 0, 5, 10, 15, 30, 60}

func (a *app) settingsPage(c *ui.Context) {
	p := &a.p
	prefs := &a.data.Prefs
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(0, 18, 24).MaxWidth(680).Margin(0, ui.Auto).FillWidth().Children(func() {
			a.section(c, a.tr.SettingsGeneral(), "", nil)
			a.list(c, func() {
				a.formRow(c, a.tr.SettingsLanguage(), func() {
					if selectOf(a, c, "language", &prefs.Language, languageTags, func(tag string) string {
						switch tag {
						case "en":
							return "English"
						case "zh-Hans":
							return "简体中文"
						}
						return a.tr.LanguageSystem()
					}).Label(a.tr.SettingsLanguage()).Changed() {
						a.changed()
						a.setLanguage(prefs.Language)
					}
				})
				a.formRow(c, a.tr.SettingsAppearance(), func() {
					i := max(0, indexOf([]string{"", "light", "dark"}, prefs.Appearance))
					if a.segTrack(c, "appearance", &i, a.tr.AppearanceSystem(), a.tr.AppearanceLight(), a.tr.AppearanceDark()) {
						prefs.Appearance = []string{"", "light", "dark"}[i]
						a.changed()
						if a.live {
							mygo.Theme.SetSource(themeSource(prefs.Appearance))
						}
					}
				})
				a.formRowHint(c, a.tr.SettingsOpenAtLogin(), a.tr.SettingsOpenAtLoginHint(), func() {
					// As read at launch: asking macOS takes some 16 ms, too long
					// for a frame.
					on := a.openAtLogin
					if ui.Switch(c, &on).Label(a.tr.SettingsOpenAtLogin()).Disabled(!a.live).Changed() {
						if err := mygo.App.SetOpenAtLogin(on); err != nil {
							a.toast = err.Error()
						} else {
							a.openAtLogin = on
						}
					}
				})
				a.formRow(c, a.tr.SettingsWeekStart(), func() {
					if selectOf(a, c, "weekStart", &prefs.WeekStart, []int{-1, 1, 0}, func(n int) string {
						switch n {
						case 0:
							return a.tr.WeekStartSunday()
						case 1:
							return a.tr.WeekStartMonday()
						}
						return a.tr.WeekStartRegion()
					}).Label(a.tr.SettingsWeekStart()).Changed() {
						a.changed()
					}
				})
			})

			a.section(c, a.tr.NavProjects(), "", nil)
			a.list(c, func() {
				a.formRowHint(c, a.tr.CategoriesTitle(), a.tr.CategoriesCount(len(a.data.Categories)), func() {
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconTag, a.tr.EditCategories()).Clicked() {
						a.catOpen = true
					}
				})
			})

			a.section(c, a.tr.NavSchedule(), "", nil)
			a.list(c, func() {
				a.formRowHint(c, a.tr.KindsTitle(), a.tr.KindsCount(len(a.data.Kinds)), func() {
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconTag, a.tr.EditKinds()).Clicked() {
						a.kindsOpen = true
					}
				})
			})

			a.section(c, a.tr.SettingsNotifications(), "", nil)
			a.list(c, func() {
				a.formRowHint(c, a.tr.SettingsEventAlerts(), a.tr.SettingsEventAlertsHint(), func() {
					if selectOf(a, c, "alerts", &prefs.AlertMinutes, alertDefaults, a.alertName).Label(a.tr.SettingsEventAlerts()).Changed() {
						a.changed()
					}
				})
				a.formRowHint(c, a.tr.SettingsBadge(), a.tr.SettingsBadgeHint(), func() {
					on := !prefs.NoBadge
					if ui.Switch(c, &on).Label(a.tr.SettingsBadge()).Changed() {
						prefs.NoBadge = !on
						a.changed()
					}
				})
				a.formRowHint(c, a.tr.TestNotification(), a.orNote(a.notifyNote), func() {
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconBell, a.tr.TestNotification()).Clicked() {
						a.testNotification()
					}
				})
			})

			a.section(c, a.tr.SettingsMenuBar(), "", nil)
			a.list(c, func() {
				a.formRowHint(c, a.tr.SettingsShowInMenuBar(), a.tr.SettingsShowInMenuBarHint(), func() {
					show := !prefs.HideMenuBar
					if ui.Switch(c, &show).Label(a.tr.SettingsShowInMenuBar()).Changed() {
						prefs.HideMenuBar = !show
						a.changed()
						a.setMenuBar(show)
					}
				})
				a.formRowHint(c, a.tr.SettingsHotkey(), a.tr.SettingsHotkeyHint("⌃⌥Space"), func() {
					on := !prefs.NoHotkey
					if ui.Switch(c, &on).Label(a.tr.SettingsHotkey()).Changed() {
						prefs.NoHotkey = !on
						a.changed()
						a.setHotkey(on)
					}
				})
			})

			a.section(c, a.tr.SettingsData(), "", nil)
			a.list(c, func() {
				if a.path != "" {
					ui.Row(c).MinHeight(50).Padding(7, 10, 7, 14).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, a.tr.DataLocation()).FontSize(13).TextColor(p.fg)
							ui.Text(c, a.path).Font("monospace").FontSize(11).TextColor(p.muted).Selectable()
						})
						if a.iconLabel(c, a.btn(c, "", btnSecondary), iconFinder, a.tr.RevealData()).Clicked() {
							a.flush()
							mygo.Shell.ShowItemInFolder(a.path)
						}
					})
				}
				if a.path != "" {
					a.formRowHint(c, a.tr.AutoBackups(), a.tr.AutoBackupsHint(keepBackups), func() {
						if !a.backupsRead {
							a.backupList, a.backupsRead = backups(a.path), true
						}
						list := a.backupList
						if len(list) == 0 {
							return
						}
						none := backup{}
						chosen := none
						if selectOf(a, c, "restore", &chosen, append([]backup{none}, list...), func(b backup) string {
							if b.path == "" {
								return a.tr.RestoreBackup()
							}
							return a.fmt(b.day, "yMMMEd")
						}).Label(a.tr.RestoreBackup()).Changed() && chosen.path != "" {
							if err := a.restoreBackup(chosen); err != nil {
								a.toast = err.Error()
							}
						}
					})
				}
				a.formRowHint(c, a.tr.SettingsCalendar(), a.tr.SettingsCalendarHint(), func() {
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconDownload, a.tr.ExportBackup()).Clicked() {
						a.exportCalendar()
					}
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconUpload, a.tr.ImportBackup()).Clicked() {
						a.importCalendar()
					}
				})
				a.formRow(c, a.tr.SettingsBackup(), func() {
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconDownload, a.tr.ExportBackup()).Clicked() {
						a.exportBackup()
					}
					if a.iconLabel(c, a.btn(c, "", btnSecondary), iconUpload, a.tr.ImportBackup()).Clicked() {
						a.importBackup()
					}
				})
				empty := len(a.data.Projects)+len(a.data.Tasks)+len(a.data.Events)+len(a.data.Directions) == 0
				if empty {
					a.formRow(c, a.tr.SettingsSampleData(), func() {
						if a.btn(c, a.tr.LoadSample(), btnSecondary).Clicked() {
							d := sampleData(a.now(), a.lang)
							if len(a.data.Categories) > 0 {
								d.Categories = a.data.Categories
							}
							if len(a.data.Kinds) > 0 {
								d.Kinds = a.data.Kinds
							}
							a.replaceData(d, a.tr.SampleLoaded())
						}
					})
				} else {
					a.formRow(c, a.tr.EraseAll(), func() {
						if a.btn(c, a.tr.EraseButton(), btnDanger).Clicked() {
							a.askErase = true
						}
					})
				}
			})

			a.section(c, a.tr.SettingsAbout(), "", nil)
			a.card(c).Padding(14).Children(func() {
				ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(52, 52).Shrink(0).Draw(drawAppIcon)
					ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
						ui.Row(c).AlignItems(ui.Baseline).Gap(8).Children(func() {
							ui.Text(c, a.tr.AppName()).FontSize(15).FontWeight(600).TextColor(p.fg)
							ui.Text(c, a.tr.Version(appVersion())).FontSize(12).TextColor(p.muted)
						})
						ui.Text(c, a.tr.AboutText()).FontSize(12.5).LineHeight(1.45).TextColor(p.fg2)
					})
				})
			})
		})
	})
	if ui.AlertDialog(c, &a.askErase, a.tr.EraseConfirm(), "", a.tr.Cancel(), a.tr.Erase()) == 1 {
		d := newData()
		d.Categories, d.Kinds = a.data.Categories, a.data.Kinds
		a.replaceData(d, a.tr.Erased())
	}
}

func (a *app) orNote(note string) string {
	if note != "" {
		return note
	}
	return a.tr.TestNotificationBody()
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// appVersion is the app's version, asked of the bundle once.
var appVersion = sync.OnceValue(func() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return "0.4.0"
})

func (a *app) setLanguage(tag string) {
	if a.live {
		mygo.App.SetLocale(tag)
	}
}

// resetSelection forgets what was chosen in data replaced.
func (a *app) resetSelection() {
	a.taskSel, a.projOpen, a.taskFilter = "", "", ""
	a.evtOpen, a.dirOpen = false, false
	a.drag = eventDrag{}
}

// testNotification sends a notification now, and notes why it could not.
func (a *app) testNotification() {
	if !a.live {
		return
	}
	if !mygo.NotificationsSupported() {
		a.notifyNote = a.tr.NotificationsUnavailable()
		return
	}
	title, body, denied := a.tr.TestNotificationTitle(), a.tr.TestNotificationBody(), a.tr.NotificationsDenied()
	go func() {
		err := mygo.NewNotification(mygo.NotificationOptions{ID: "test", Title: title, Body: body}).Show()
		mygo.RunOnMain(func() {
			switch {
			case errors.Is(err, mygo.ErrNotificationsDenied):
				a.notifyNote = denied
			case err != nil:
				a.notifyNote = err.Error()
			default:
				a.notifyNote = ""
			}
			a.redraw()
		})
	}()
}

// exportBackup asks where to save a backup of the data, and saves it.
func (a *app) exportBackup() {
	if a.win == nil {
		return
	}
	b, err := a.data.encode()
	if err != nil {
		a.toast = err.Error()
		return
	}
	win, done := a.win, a.tr.Exported()
	name := "NextTodo " + a.now().Format("2006-01-02") + ".json"
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			DefaultPath: name,
			Filters:     []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
		})
		if err != nil || path == "" {
			return
		}
		err = os.WriteFile(path, b, 0o644)
		win.Update(func() {
			if err != nil {
				a.toast = err.Error()
			} else {
				a.toast = done + " · " + filepath.Base(path)
			}
		})
	}()
}

// exportCalendar asks where to save the events as a calendar file, and
// saves them.
func (a *app) exportCalendar() {
	if a.win == nil {
		return
	}
	b := writeICS(a.data.Events, a.now(), a.kindName)
	win, done := a.win, a.tr.CalendarExported()
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			DefaultPath: "NextTodo.ics",
			Filters:     []mygo.FileFilter{{Name: "iCalendar", Extensions: []string{"ics"}}},
		})
		if err != nil || path == "" {
			return
		}
		err = os.WriteFile(path, b, 0o644)
		win.Update(func() {
			if err != nil {
				a.toast = err.Error()
			} else {
				a.toast = done + " · " + filepath.Base(path)
			}
		})
	}()
}

// importCalendar asks for a calendar file and adds its events, changing
// those it added before, with Undo.
func (a *app) importCalendar() {
	if a.win == nil {
		return
	}
	win, failed := a.win, a.tr.CalendarFailed()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Filters: []mygo.FileFilter{{Name: "iCalendar", Extensions: []string{"ics", "ical", "ifb", "icalendar"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		b, err := os.ReadFile(paths[0])
		var got icsImport
		if err == nil {
			got, err = readICS(b, time.Local)
		}
		if err != nil {
			mygo.Dialog.Error(failed, err.Error())
			return
		}
		win.Update(func() { a.addCalendar(got) })
	}()
}

// addCalendar adds the events of a calendar file, and says how many.
func (a *app) addCalendar(got icsImport) {
	step := a.remember()
	added, changed := a.data.importCalendar(got)
	a.settle(step)
	a.changed()
	msg := a.tr.CalendarImported(added + changed)
	if got.simple > 0 {
		msg += " · " + a.tr.CalendarSimplified(got.simple)
	}
	a.replaced, a.replacedMsg = step, msg
}

// importBackup asks for a backup and, once the user agrees, replaces the
// data with it, keeping the settings of this Mac.
func (a *app) importBackup() {
	if a.win == nil {
		return
	}
	win := a.win
	lang := a.lang
	failed, confirm, detail, replace, cancel, done :=
		a.tr.ImportFailed(), a.tr.ImportConfirm(), a.tr.ImportConfirmDetail(), a.tr.ImportReplace(), a.tr.Cancel(), a.tr.Imported()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		b, err := os.ReadFile(paths[0])
		var d *Data
		if err == nil {
			if d, err = decodeData(b); err == nil {
				d.migrate(lang)
			}
		}
		if err != nil {
			mygo.Dialog.Error(failed, err.Error())
			return
		}
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: win, Type: mygo.MessageWarning, Message: confirm, Detail: detail,
			Buttons: []string{replace, cancel}, CancelButton: 1,
		})
		if err != nil || res.Button != 0 {
			return
		}
		win.Update(func() {
			a.replaceData(d, done)
		})
	}()
}

package main

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"nexttodo/internal/pi"
)

// Translation is pi's too, in a process of its own: an agent rather than a
// bare model, so that it can look up the terms it is unsure of with the
// search tools the user gave pi, which a bare model cannot. It keeps no
// session, has none of NextTodo's tools, and is told to search for terms
// alone, never the text.

//go:embed prompts/translator.md
var translatorPrompt string

// translator is the state of the translation page.
type translator struct {
	client   *pi.Client
	gen      int
	starting bool
	err      string
	waiting  []func(*pi.Client)

	src    string
	field  ui.Handle
	raw    string // what the model wrote: the translation and its notes
	busy   bool
	status string // what it is doing, as looking a term up
	since  time.Time
	took   time.Duration
	run    int  // the translation shown, so that one given up says no more
	live   bool // pi started it: what it says before is of the one before
	// What raw translates, so that a change of the language or the style
	// translates again.
	doneSrc, doneTo, doneStyle string

	langOpen bool
	other    string // a language typed
	md       markdown.Options
}

// The languages offered, by the names the model reads; the menu shows
// each in itself.
var translateLangs = []struct{ name, native string }{
	{"Simplified Chinese", "简体中文"},
	{"English", "English"},
	{"Traditional Chinese", "繁體中文"},
	{"Japanese", "日本語"},
	{"Korean", "한국어"},
	{"French", "Français"},
	{"German", "Deutsch"},
	{"Spanish", "Español"},
	{"Russian", "Русский"},
	{"Portuguese", "Português"},
	{"Italian", "Italiano"},
	{"Arabic", "العربية"},
}

var translateStyles = []string{"natural", "academic", "literal"}

// translateTarget is the language to translate src into: the one chosen,
// or, automatic, the app's language, and English for text already in it.
func (a *app) translateTarget(src string) string {
	if to := a.data.Prefs.TranslateTo; to != "" {
		return to
	}
	mine := languageName(a.lang)
	if script := mainScript(src); script == unicode.Han && mine == "Simplified Chinese" || script == unicode.Latin && mine == "English" {
		if mine == "English" {
			return "Simplified Chinese"
		}
		return "English"
	}
	return mine
}

// mainScript is the script most of the letters of s are in: Han, Latin,
// or another.
func mainScript(s string) *unicode.RangeTable {
	counts := map[*unicode.RangeTable]int{}
	scripts := []*unicode.RangeTable{unicode.Han, unicode.Latin, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Cyrillic, unicode.Arabic}
	n := 0
	for _, r := range s {
		if n > 2000 {
			break
		}
		for _, t := range scripts {
			if unicode.Is(t, r) {
				counts[t]++
				n++
				break
			}
		}
	}
	// Japanese mixes kana with Han: any kana makes it Japanese.
	if counts[unicode.Hiragana]+counts[unicode.Katakana] > 0 {
		return unicode.Hiragana
	}
	var best *unicode.RangeTable
	for _, t := range scripts {
		if best == nil || counts[t] > counts[best] {
			best = t
		}
	}
	if best == nil || counts[best] == 0 {
		return nil
	}
	return best
}

// langNative names a language of the menu in itself.
func langNative(name string) string {
	for _, l := range translateLangs {
		if l.name == name {
			return l.native
		}
	}
	return name
}

// Running the translator.

func (a *app) startTranslator() {
	x := &a.xl
	if x.client != nil || x.starting || !a.live {
		return
	}
	x.starting, x.err = true, ""
	x.gen++
	gen := x.gen
	dir := filepath.Join(filepath.Dir(a.agentDir()), "Translate")
	path := a.data.Prefs.PiPath
	model := a.data.Prefs.AgentModel
	args := translatorArgs()
	go func() {
		fail := func(msg string) {
			mygo.RunOnMain(func() {
				if gen == a.xl.gen {
					a.xl.starting, a.xl.err, a.xl.busy, a.xl.waiting = false, msg, false, nil
					a.redraw()
				}
			})
		}
		in, err := a.findPi(path)
		if err != nil {
			fail(a.tr.TranslateNoPi())
			return
		}
		os.MkdirAll(dir, 0o755)
		var c *pi.Client
		c, err = pi.Start(pi.Options{
			Bin: in.Bin, Args: args, Env: in.Env, Dir: dir,
			OnEvent: func(e pi.Event) {
				a.inbox.post(func() {
					if gen == a.xl.gen {
						a.translatorEvent(e)
					}
				})
			},
			OnExit: func(err error) {
				a.inbox.post(func() {
					if gen == a.xl.gen && a.xl.client != nil {
						a.xl.client, a.xl.busy, a.xl.err = nil, false, err.Error()
					}
				})
			},
		})
		if err != nil {
			fail(err.Error())
			return
		}
		if i := strings.Index(model, "/"); i > 0 {
			c.Call(context.Background(), map[string]any{"type": "set_model", "provider": model[:i], "modelId": model[i+1:]})
		}
		mygo.RunOnMain(func() {
			if gen != a.xl.gen {
				go c.Close()
				return
			}
			a.xl.client, a.xl.starting = c, false
			waiting := a.xl.waiting
			a.xl.waiting = nil
			for _, fn := range waiting {
				fn(c)
			}
		})
	}()
}

// stopTranslator ends the translator, for the next translation to start
// it again as the settings are now; one starting is given up.
func (a *app) stopTranslator() {
	a.xl.gen++
	if c := a.xl.client; c != nil {
		a.xl.client = nil
		go c.Close()
	}
	a.xl.starting, a.xl.err, a.xl.busy, a.xl.waiting = false, "", false, nil
}

// killTranslator ends it at once, as the app quits.
func (a *app) killTranslator() {
	a.xl.gen++
	if c := a.xl.client; c != nil {
		a.xl.client = nil
		c.Kill()
	}
}

// translatorArgs run pi as the translator: no session, none of the
// user's context files or skills, and of the tools only those looking
// things up, which it is told to use for terms alone.
func translatorArgs() []string {
	return []string{
		"--no-session", "--no-context-files", "--no-skills",
		"--tools", strings.Join(append([]string{"codemode"}, searchTools...), ","),
		"--exclude-tools", strings.Join(sendingTools, ","),
		"--system-prompt", translatorPrompt,
		"--thinking", "low",
	}
}

// translate translates the text in the field, giving up one going on.
func (a *app) translate() {
	x := &a.xl
	src := strings.TrimSpace(x.src)
	if src == "" {
		return
	}
	to, style := a.translateTarget(src), a.data.Prefs.TranslateStyle
	if style == "" {
		style = "natural"
	}
	wasBusy := x.busy
	x.run++
	x.raw, x.err, x.status, x.busy, x.since, x.took = "", "", "", true, a.now(), 0
	x.doneSrc, x.doneTo, x.doneStyle = x.src, to, style
	msg := "Target: " + to + "\nStyle: " + style + "\n\n<text>\n" + src + "\n</text>"
	run := x.run
	x.live = false // until the translation starts, what pi says is of the last
	send := func(c *pi.Client) {
		if wasBusy {
			c.Queue(map[string]any{"type": "abort"})
		}
		c.Queue(map[string]any{"type": "new_session"})
		p := c.Queue(map[string]any{"type": "prompt", "message": msg})
		go func() {
			_, err := p.Wait(context.Background())
			mygo.RunOnMain(func() {
				if err != nil && run == a.xl.run {
					a.xl.busy, a.xl.err = false, err.Error()
					a.redraw()
				}
			})
		}()
	}
	if x.client != nil {
		send(x.client)
		return
	}
	x.waiting = []func(*pi.Client){send}
	a.startTranslator()
}

func (a *app) stopTranslating() {
	x := &a.xl
	if !x.busy {
		return
	}
	x.run++
	x.busy, x.status = false, ""
	if x.client != nil {
		x.client.Queue(map[string]any{"type": "abort"})
	}
}

// translatorEvent takes in what the translator says.
func (a *app) translatorEvent(e pi.Event) {
	x := &a.xl
	if !x.busy {
		return // given up
	}
	if e.Type == "agent_start" {
		x.live = true
	}
	if !x.live {
		return // the end of a translation given up
	}
	var se pi.SessionEvent
	e.Decode(&se)
	switch e.Type {
	case "message_start":
		if se.Message != nil && se.Message.Role == "assistant" {
			x.raw = "" // what it wrote before looking terms up is not the translation
		}
	case "message_update":
		if ev := se.AssistantMessageEvent; ev != nil && ev.Type == "text_delta" {
			x.raw += ev.Delta
			x.status = ""
		}
	case "message_end":
		if m := se.Message; m != nil && m.Role == "assistant" {
			if t := m.Text(); t != "" || m.StopReason != "toolUse" {
				x.raw = t
			}
			if m.StopReason == "error" {
				x.err = orUntitled(m.ErrorMessage, a.tr.AgentError())
			}
		}
	case "tool_execution_start":
		_, x.status = a.toolLabel(&chatItem{tool: se.ToolName, args: se.Args})
	case "tool_execution_end":
		x.status = ""
	case "agent_settled":
		x.busy, x.status, x.took = false, "", a.now().Sub(x.since)
	}
}

// splitNotes parts what the model wrote into the translation and its
// notes on terms.
func splitNotes(raw string) (text, notes string) {
	i := strings.Index(raw, "<notes>")
	if i < 0 {
		// A tag being written is not shown.
		if j := strings.LastIndex(raw, "<"); j >= 0 && strings.HasPrefix("<notes>", raw[j:]) {
			return strings.TrimSpace(raw[:j]), ""
		}
		return strings.TrimSpace(raw), ""
	}
	notes = raw[i+len("<notes>"):]
	notes, _, _ = strings.Cut(notes, "</notes>")
	return strings.TrimSpace(raw[:i]), strings.TrimSpace(notes)
}

// The page.

func (a *app) translatePage(c *ui.Context) {
	p := &a.p
	x := &a.xl
	if x.err == "" {
		a.startTranslator() // warm; not again once it failed, until asked to translate
	}
	if x.md.Theme == nil {
		x.md.Theme = &markdown.Theme{FontSize: 12.5}
	}
	if c.Shortcut(ui.Cmd, ui.KeyEnter) {
		a.translate()
	}
	// A change of the language or the style translates again what was
	// translated: the text was sent already. Text typed or pasted is not
	// sent until the user asks.
	if src := strings.TrimSpace(x.src); src != "" && !x.busy && x.doneSrc == x.src && x.raw != "" {
		style := orUntitled(a.data.Prefs.TranslateStyle, "natural")
		if a.translateTarget(src) != x.doneTo || style != x.doneStyle {
			a.translate()
		}
	}
	ui.Column(c).Grow(1).MinHeight(0).Padding(0, 18, 16).Gap(10).MaxWidth(1240).Margin(0, ui.Auto).FillWidth().Children(func() {
		a.translateBar(c)
		ui.Row(c).Grow(1).MinHeight(0).Gap(12).AlignItems(ui.Stretch).Children(func() {
			// The text.
			a.card(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				ui.Row(c).Padding(10, 10, 0, 16).Height(36).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, a.sourceName()).FontSize(12).FontWeight(500).TextColor(p.muted).Grow(1)
					if x.src != "" && a.iconBtn(c, iconClose, a.tr.TranslateClear()).Clicked() {
						x.src, x.raw, x.doneSrc = "", "", ""
						a.stopTranslating()
						x.field.Focus()
					}
				})
				in := ui.TextAreaBase(c, &x.src).Grow(1).MinHeight(0).Padding(6, 16, 16).FontSize(14).LineHeight(1.6).
					Placeholder(a.tr.TranslatePlaceholder()).Bind(&x.field)
				if a.page == PageTranslate && x.src == "" {
					in.AutoFocus()
				}
				ui.Row(c).Padding(0, 16, 10).Height(30).AlignItems(ui.Center).Children(func() {
					ui.Text(c, a.tr.TranslateCount(len([]rune(x.src)))).FontSize(11).TextColor(p.faint).Grow(1)
					ui.Text(c, "⌘↩").FontSize(11).TextColor(p.faint)
				})
			})
			// The translation.
			a.card(c).Grow(1).Basis(0).MinWidth(0).Background(p.card2).Children(func() {
				text, notes := splitNotes(x.raw)
				ui.Row(c).Padding(10, 10, 0, 16).Height(36).AlignItems(ui.Center).Gap(6).Children(func() {
					target := a.translateTarget(x.src)
					if x.doneTo != "" && x.raw != "" {
						target = x.doneTo
					}
					ui.Text(c, langNative(target)).FontSize(12).FontWeight(500).TextColor(p.muted).Grow(1)
					if text != "" && !x.busy {
						if a.iconBtn(c, iconSwap, a.tr.TranslateSwap()).Clicked() {
							a.swapTranslation(text)
						}
						if a.iconBtn(c, iconCopy, a.tr.Copy()).Clicked() {
							mygo.Clipboard.WriteText(text)
							c.Toast(a.tr.Copied())
						}
					}
				})
				ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).Padding(6, 16, 16).Gap(14).Children(func() {
						switch {
						case text != "":
							ui.Text(c, text).FontSize(14).LineHeight(1.6).TextColor(p.fg).Selectable()
						case x.busy:
						case x.err == "":
							ui.Text(c, a.tr.TranslateEmpty()).FontSize(13).TextColor(p.faint)
						}
						if x.err != "" {
							ui.Text(c, x.err).FontSize(12).TextColor(p.red).Selectable()
						}
						if notes != "" && !x.busy {
							ui.Column(c).Gap(6).Padding(10, 12).Radius(9).Background(p.card).Border(1, p.line2).Children(func() {
								a.label(c, a.tr.TranslateNotes())
								markdown.View(c.Key("notes"), notes, &x.md)
							})
						}
					})
				})
				ui.Row(c).Padding(0, 16, 10).Height(30).AlignItems(ui.Center).Gap(7).Children(func() {
					switch {
					case x.busy:
						ui.Spinner(c).Size(12, 12)
						label := a.tr.TranslateWorking()
						if x.status != "" {
							label = x.status
						} else if x.client == nil {
							label = a.tr.AgentStarting()
						}
						ui.Text(c, label).FontSize(11.5).TextColor(p.muted).SingleLine().MinWidth(0)
					case x.took > 0 && text != "":
						ui.Text(c, a.tr.AgentSeconds(max(int(x.took.Seconds()+0.5), 1))).FontSize(11).TextColor(p.faint)
					}
				})
			})
		})
	})
}

// sourceName says what language the text is in, as far as its letters
// tell, or that it will be detected.
func (a *app) sourceName() string {
	switch mainScript(a.xl.src) {
	case unicode.Han:
		return a.tr.TranslateDetected(langNative("Simplified Chinese"))
	case unicode.Hiragana:
		return a.tr.TranslateDetected(langNative("Japanese"))
	case unicode.Hangul:
		return a.tr.TranslateDetected(langNative("Korean"))
	case unicode.Cyrillic:
		return a.tr.TranslateDetected(langNative("Russian"))
	case unicode.Arabic:
		return a.tr.TranslateDetected(langNative("Arabic"))
	}
	return a.tr.TranslateAuto()
}

// swapTranslation makes the translation the text, to translate back.
func (a *app) swapTranslation(text string) {
	x := &a.xl
	from := x.src
	x.src, x.raw = text, ""
	// Into the language of the text it came from, as far as its letters
	// tell; else as automatic.
	switch mainScript(from) {
	case unicode.Han:
		a.data.Prefs.TranslateTo = "Simplified Chinese"
	case unicode.Latin:
		a.data.Prefs.TranslateTo = "English"
	default:
		a.data.Prefs.TranslateTo = ""
	}
	a.dirty = true
	a.translate()
}

// translateBar chooses the language and the style, and translates.
func (a *app) translateBar(c *ui.Context) {
	p := &a.p
	x := &a.xl
	ui.Row(c).Gap(10).AlignItems(ui.Center).Height(32).Children(func() {
		ui.Icon(c, iconTrans).Size(16, 16).TextColor(p.muted)
		ui.Text(c, a.tr.TranslateInto()).FontSize(12.5).TextColor(p.muted)
		// The language.
		label := a.tr.TranslateAutoTarget(langNative(a.translateTarget(x.src)))
		if to := a.data.Prefs.TranslateTo; to != "" {
			label = langNative(to)
		}
		b := ui.ButtonBase(c.Key("lang")).Height(26).Padding(0, 8, 0, 10).Gap(6).Radius(7).AlignItems(ui.Center).
			Background(p.card).Border(1, p.line).Label(a.tr.TranslateInto())
		if b.Hovered() || x.langOpen {
			b.Background(p.pill)
		}
		b.Children(func() {
			ui.Text(c, label).FontSize(12.5).FontWeight(500).TextColor(p.fg).SingleLine()
			ui.Icon(c, iconChevronUpDown).Size(10, 10).TextColor(p.muted)
		})
		if b.Clicked() {
			x.langOpen = !x.langOpen
		}
		ui.PopoverBase(c, b, &x.langOpen, func(panel ui.Element) {
			popover(a, panel).Margin(4, 0, 0, 0).Padding(4).MinWidth(210).Gap(1)
			item := func(key, text, value string) {
				it := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 8).Gap(8).Radius(7).AlignItems(ui.Center)
				if it.Hovered() {
					it.Background(p.pill)
				}
				it.Children(func() {
					if a.data.Prefs.TranslateTo == value {
						ui.Icon(c, iconCheck).Size(12, 12).TextColor(p.accent)
					} else {
						ui.Box(c).Size(12, 12)
					}
					ui.Text(c, text).FontSize(12.5).TextColor(p.fg).SingleLine()
				})
				if it.Clicked() {
					x.langOpen = false
					a.data.Prefs.TranslateTo = value
					a.dirty = true
				}
			}
			item("auto", a.tr.TranslateAutomatic(), "")
			ui.Box(c).Height(1).Background(p.line2).Margin(3, 6)
			for _, l := range translateLangs {
				item(l.name, l.native, l.name)
			}
			ui.Box(c).Height(1).Background(p.line2).Margin(3, 6)
			in := a.input(c, &x.other, a.tr.TranslateOther()).Margin(2, 4, 4)
			if in.Submitted() && strings.TrimSpace(x.other) != "" {
				x.langOpen = false
				a.data.Prefs.TranslateTo = strings.TrimSpace(x.other)
				a.dirty = true
			}
		})
		// The style.
		style := slicesIndex(translateStyles, orUntitled(a.data.Prefs.TranslateStyle, "natural"))
		if a.segTrack(c, "style", &style, a.tr.TranslateNatural(), a.tr.TranslateAcademic(), a.tr.TranslateLiteral()) {
			a.data.Prefs.TranslateStyle = translateStyles[style]
			a.dirty = true
		}
		ui.Spacer(c)
		if x.busy {
			if a.btn(c, a.tr.AgentStop(), btnSecondary).Clicked() {
				a.stopTranslating()
			}
		} else if a.btn(c, a.tr.TranslateGo(), btnPrimary).Disabled(strings.TrimSpace(x.src) == "").Clicked() {
			a.translate()
		}
	})
}

func slicesIndex(list []string, s string) int {
	for i, x := range list {
		if x == s {
			return i
		}
	}
	return 0
}

package main

import "github.com/egoist/mygo/ui"

// categoryIcons are the icons a category can take, by name.
var categoryIcons = []struct {
	name string
	icon *ui.SVG
}{
	{"flask", iconFlask}, {"building", iconBuilding}, {"bulb", iconBulb}, {"book", iconBook},
	{"folder", iconFolder}, {"people", iconPeople}, {"globe", iconGlobe}, {"award", iconAward},
	{"code", iconCode}, {"heart", iconHeart}, {"briefcase", iconBriefcase}, {"compass", iconCompass},
}

func categoryIcon(name string) *ui.SVG {
	for _, i := range categoryIcons {
		if i.name == name {
			return i.icon
		}
	}
	return iconFolder
}

// icon makes a line icon on a 16-unit grid, drawn in the color of text, in
// the manner of SF Symbols.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

func filled(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconPlus          = icon(`<path d="M8 3.5v9M3.5 8h9"/>`)
	iconClose         = icon(`<path d="M4.5 4.5l7 7M11.5 4.5l-7 7"/>`)
	iconCheck         = icon(`<path d="M3.5 8.5l3 3 6-7"/>`)
	iconChevronLeft   = icon(`<path d="M9.8 3.8L5.6 8l4.2 4.2"/>`)
	iconChevronRight  = icon(`<path d="M6.2 3.8L10.4 8l-4.2 4.2"/>`)
	iconChevronDown   = icon(`<path d="M4 6.2l4 4 4-4"/>`)
	iconChevronUpDown = icon(`<path d="M5 6l3-3 3 3M5 10l3 3 3-3"/>`)
	iconTrash         = icon(`<path d="M3 4.5h10M6.2 4.5V3h3.6v1.5M4.3 4.5l.6 8.6c.05.8.7 1.4 1.5 1.4h3.2c.8 0 1.45-.6 1.5-1.4l.6-8.6"/>`)
	iconStar          = icon(`<path d="M8 2.2l1.75 3.6 3.95.55-2.86 2.78.68 3.94L8 11.2l-3.52 1.87.68-3.94L2.3 6.35l3.95-.55z"/>`)
	iconStarFilled    = filled(`<path d="M8 2.2l1.75 3.6 3.95.55-2.86 2.78.68 3.94L8 11.2l-3.52 1.87.68-3.94L2.3 6.35l3.95-.55z"/>`)
	iconSun           = icon(`<circle cx="8" cy="8" r="2.7"/><path d="M8 1.6v1.3M8 13.1v1.3M1.6 8h1.3M13.1 8h1.3M3.5 3.5l.9.9M11.6 11.6l.9.9M3.5 12.5l.9-.9M11.6 4.4l.9-.9"/>`)
	iconCalendar      = icon(`<rect x="2.2" y="3" width="11.6" height="10.8" rx="2.2"/><path d="M2.2 6.6h11.6M5.4 1.8v2.4M10.6 1.8v2.4"/>`)
	iconClock         = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 4.6V8l2.3 1.4"/>`)
	iconPin           = icon(`<path d="M8 14s4.5-4.1 4.5-7.4a4.5 4.5 0 0 0-9 0C3.5 9.9 8 14 8 14z"/><circle cx="8" cy="6.6" r="1.5"/>`)
	iconSearch        = icon(`<circle cx="7.2" cy="7.2" r="4.4"/><path d="M10.5 10.5L13.6 13.6"/>`)
	iconFolder        = icon(`<path d="M2.2 4.6c0-.9.7-1.6 1.6-1.6h2.6l1.5 1.6h4.3c.9 0 1.6.7 1.6 1.6v5.6c0 .9-.7 1.6-1.6 1.6H3.8c-.9 0-1.6-.7-1.6-1.6z"/>`)
	iconFlask         = icon(`<path d="M6.2 2h3.6M6.7 2v4.2L3 12.4c-.5.9.1 1.8 1.1 1.8h7.8c1 0 1.6-.9 1.1-1.8L9.3 6.2V2M4.6 9.8h6.8"/>`)
	iconBuilding      = icon(`<path d="M3 14V3.5c0-.8.6-1.4 1.4-1.4h4.2c.8 0 1.4.6 1.4 1.4V14M10 6.5h2.6c.8 0 1.4.6 1.4 1.4V14M2 14h12M5.4 5h1.8M5.4 7.8h1.8M5.4 10.6h1.8"/>`)
	iconBulb          = icon(`<path d="M6 12.2h4M6.5 14h3M8 2a4 4 0 0 0-2.4 7.2c.5.4.8.9.8 1.5v.1h3.2v-.1c0-.6.3-1.1.8-1.5A4 4 0 0 0 8 2z"/>`)
	iconBook          = icon(`<path d="M2.5 3.4C4.2 2.6 6.4 2.7 8 3.8c1.6-1.1 3.8-1.2 5.5-.4v9.4c-1.7-.8-3.9-.7-5.5.4-1.6-1.1-3.8-1.2-5.5-.4zM8 3.8v9.4"/>`)
	iconCompass       = icon(`<circle cx="8" cy="8" r="6"/><path d="M10.6 5.4L9.1 9.1 5.4 10.6 6.9 6.9z"/>`)
	iconNote          = icon(`<path d="M4 2.5h6l2.5 2.5v8.5H4z"/><path d="M6 7h4.4M6 9.6h4.4"/>`)
	iconGear          = icon(`<circle cx="8" cy="8" r="2.1"/><path d="M8 1.8l1.1 1.6 1.9-.4.6 1.8 1.8.6-.4 1.9L14.2 8l-1.6 1.1.4 1.9-1.8.6-.6 1.8-1.9-.4L8 14.2l-1.1-1.6-1.9.4-.6-1.8-1.8-.6.4-1.9L1.8 8l1.6-1.1-.4-1.9 1.8-.6.6-1.8 1.9.4z"/>`)
	iconOpen          = icon(`<path d="M9 2.8h4.2V7M13.2 2.8L7.6 8.4M11.4 9.4v2.8c0 .9-.7 1.6-1.6 1.6H3.8c-.9 0-1.6-.7-1.6-1.6V6.2c0-.9.7-1.6 1.6-1.6h2.8"/>`)
	iconFinder        = icon(`<path d="M2.2 4.6c0-.9.7-1.6 1.6-1.6h2.6l1.5 1.6h4.3c.9 0 1.6.7 1.6 1.6v5.6c0 .9-.7 1.6-1.6 1.6H3.8c-.9 0-1.6-.7-1.6-1.6z"/>`)
	iconDownload      = icon(`<path d="M8 2.5v7.6M4.8 7.2L8 10.4l3.2-3.2M3 13.5h10"/>`)
	iconUpload        = icon(`<path d="M8 10.4V2.8M4.8 6L8 2.8 11.2 6M3 13.5h10"/>`)
	iconBell          = icon(`<path d="M4 11.3V7.2a4 4 0 0 1 8 0v4.1l1.2 1.3H2.8zM6.6 14h2.8"/>`)
	iconEdit          = icon(`<path d="M10.4 3l2.6 2.6-7.4 7.4H3v-2.6z"/>`)
	iconPeople        = icon(`<circle cx="6" cy="5.6" r="2.3"/><path d="M1.8 13.4c.4-2.3 2.1-3.6 4.2-3.6s3.8 1.3 4.2 3.6M10.6 3.4a2.3 2.3 0 0 1 0 4.4M12.2 9.9c1 .5 1.7 1.6 2 3.5"/>`)
	iconGlobe         = icon(`<circle cx="8" cy="8" r="6"/><path d="M2 8h12M8 2c1.7 1.7 2.5 3.7 2.5 6S9.7 12.3 8 14C6.3 12.3 5.5 10.3 5.5 8S6.3 3.7 8 2z"/>`)
	iconAward         = icon(`<circle cx="8" cy="6.2" r="4"/><path d="M5.6 9.4L4.6 14l3.4-1.8 3.4 1.8-1-4.6"/>`)
	iconCode          = icon(`<path d="M5.6 4.6L2.2 8l3.4 3.4M10.4 4.6L13.8 8l-3.4 3.4M9 3l-2 10"/>`)
	iconHeart         = icon(`<path d="M8 13.6S2 10.1 2 6.1A3.2 3.2 0 0 1 8 4.6a3.2 3.2 0 0 1 6 1.5c0 4-6 7.5-6 7.5z"/>`)
	iconBriefcase     = icon(`<rect x="2" y="4.8" width="12" height="8.6" rx="1.8"/><path d="M5.8 4.8V3.6c0-.6.5-1.1 1.1-1.1h2.2c.6 0 1.1.5 1.1 1.1v1.2M2 8.6h12"/>`)
	iconArchive       = icon(`<rect x="2" y="2.8" width="12" height="3.4" rx="1"/><path d="M3 6.2v6.4c0 .6.5 1.1 1.1 1.1h7.8c.6 0 1.1-.5 1.1-1.1V6.2M6.4 9h3.2"/>`)
	iconGrip          = filled(`<circle cx="6" cy="4" r="1"/><circle cx="10" cy="4" r="1"/><circle cx="6" cy="8" r="1"/><circle cx="10" cy="8" r="1"/><circle cx="6" cy="12" r="1"/><circle cx="10" cy="12" r="1"/>`)
	iconTag           = icon(`<path d="M2.4 8.6V3.2c0-.5.4-.8.8-.8h5.4l5 5c.4.4.4 1 0 1.4l-4.6 4.6c-.4.4-1 .4-1.4 0z"/><circle cx="5.4" cy="5.4" r="1"/>`)
	// iconLogo is the app's mark: a page of a calendar with a check, as the
	// title bar and the menu bar show it.
	iconLogo = icon(`<rect x="2" y="2.8" width="12" height="11.2" rx="2.6"/><path d="M2 6.2h12M5.4 9.6l1.8 1.8 3.4-3.6"/>`)
)

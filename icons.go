// The Lucide icons Tuck draws: a handful, inlined so the app carries no icon
// package. Each is a 24-unit stroke drawing, shown through ui.Icon. Their
// licenses are in THIRD_PARTY_NOTICES.md.
package main

import "github.com/egoist/mygo/ui"

// lucide wraps an icon's shapes in Lucide's common 24-unit SVG frame.
func lucide(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconCalendarDays = lucide(
		`<path d="M8 2v3"/>` +
			`<path d="M16 2v3"/>` +
			`<rect x="3" y="3" width="18" height="18" rx="2"/>` +
			`<path d="M3 9h18"/>` +
			`<path d="M8 13h.01"/>` +
			`<path d="M12 13h.01"/>` +
			`<path d="M16 13h.01"/>` +
			`<path d="M8 17h.01"/>` +
			`<path d="M12 17h.01"/>` +
			`<path d="M16 17h.01"/>`,
	)
	iconCheck        = lucide(`<path d="M20 6 9 17l-5-5"/>`)
	iconChevronLeft  = lucide(`<path d="m15 18-6-6 6-6"/>`)
	iconChevronRight = lucide(`<path d="m9 18 6-6-6-6"/>`)
	iconListTodo     = lucide(
		`<path d="M13 5h8"/>` +
			`<path d="M13 12h8"/>` +
			`<path d="M13 19h8"/>` +
			`<path d="m3 17 2 2 4-4"/>` +
			`<rect x="3" y="4" width="6" height="6" rx="1"/>`,
	)
	iconPencil = lucide(
		`<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/>` +
			`<path d="m15 5 4 4"/>`,
	)
	iconPin = lucide(
		`<path d="M12 17v5"/>` +
			`<path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/>`,
	)
	iconPlus = lucide(`<path d="M5 12h14"/><path d="M12 5v14"/>`)
	iconX    = lucide(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)
)

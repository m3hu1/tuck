// The notes view: one markdown note per local day, written in place and read
// as rendered markdown, with a calendar that marks the days that hold a note.
package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"tuck/internal/notes"
)

const (
	// noteSaveDelay is the quiet time before a keystroke reaches the store.
	noteSaveDelay = 600 * time.Millisecond
	// noteIndent is what Tab adds to a line.
	noteIndent = "  "
	// noteErrorFor is how long a failure stays on the panel.
	noteErrorFor = 4 * time.Second
	// quietOpacity is how a control with nothing behind it fades: the design
	// asks for 0.35, and MyGo paints a disabled element at half opacity, so
	// the element asks for twice that.
	quietOpacity = 0.7
	// calWidth is the width the picker falls back to on the one frame
	// before the note has a box to cover; the next frame takes the note's
	// own width, so that the picker is the page turned over.
	calWidth = 300
	// calPad is the picker's own padding, the space it leaves around what
	// it holds.
	calPad = 10
	// calDot is the size of the mark of a day that holds a note.
	calDot = 4
	// calGap is the space between the picker's days.
	calGap = 1
)

// noteState is the notes view's own state: the day on screen, the note, and
// the editor. It lives in the app, not the view, so the panel can be closed
// and opened again without losing the day, as the design keeps its state
// between the panel's openings.
type noteState struct {
	day          string     // the day on screen
	note         notes.Note // its stored note
	text         string     // what the editor holds, which may be ahead of the store
	caret        int        // where in it the writing was left, in runes
	sel          [2]int     // the editor's selection, in runes
	setSel       *[2]int    // a selection the editor takes as it is built, from Tab
	dirty        bool       // the editor holds text the store has not taken
	typedAt      time.Time  // when the last keystroke came
	reading      bool       // the note is rendered rather than typed in
	calOpen      bool       // the picker is over the editor
	calPressOpen bool       // the picker was open as its button was pressed
	calPressed   bool       // the button was under the pointer last frame
	calMonth     time.Time  // the month the calendar shows
	calDay       time.Time  // the day the calendar's keys are on
	focus        bool       // give the note the focus as it is built
	focusCal     bool       // give the calendar the focus as it is built
	wasFocus     bool       // the editor had the focus last frame
	err          string     // the last failure, shown under the head
	errAt        time.Time

	// The source as the preview reads it, and the text it was made from:
	// the preview is redrawn on every frame, and the line breaks it needs
	// are only worth making when the text has moved.
	rendered    string
	renderedFor string
}

// initNotes opens the panel on today, as the view opens on it.
func (a *app) initNotes() {
	a.ns.day = today()
	a.ns.note, _ = a.notes.Get(a.ns.day)
	a.ns.text = a.ns.note.Text
	a.ns.caret = len([]rune(a.ns.text))
	a.ns.sel = [2]int{a.ns.caret, a.ns.caret}
	a.ns.reading = true
	a.ns.calMonth = monthStart(time.Now())
	a.ns.calDay = time.Now()
}

// typedNote says the editor's text changed, arming the autosave. The moment
// is taken from the next frame's clock, so the tester's clock drives it too.
func (a *app) typedNote() {
	a.ns.dirty = true
	a.ns.typedAt = time.Time{}
}

// autosave hands a quietly typed note to the store: a keystroke reaches it
// after noteSaveDelay without one, wherever the panel is on — the notes tab
// may be left, and the panel closed, in the meantime.
func (a *app) autosave(c *ui.Context) {
	if !a.ns.dirty {
		return
	}
	if a.ns.typedAt.IsZero() {
		a.ns.typedAt = c.Now()
	}
	if left := noteSaveDelay - c.Now().Sub(a.ns.typedAt); left > 0 {
		c.After(left)
	} else {
		a.flushNote()
	}
}

// flushNote hands the editor's text to the store, unless it already has.
func (a *app) flushNote() {
	if !a.ns.dirty {
		return
	}
	a.ns.dirty = false
	note, err := a.notes.Save(a.ns.day, a.ns.text)
	if err != nil {
		a.ns.dirty = true // hold on to it for the next attempt
		// The attempt waits as long as a keystroke does: a store that
		// cannot be written to is not written to on every frame.
		a.ns.typedAt = time.Time{}
		a.noteError(err)
		return
	}
	a.ns.note = note
}

// noteError shows what went wrong with the last action, for a while.
func (a *app) noteError(err error) {
	if err == nil {
		return
	}
	if a.ns.err == err.Error() {
		return // the same failure keeps the place it has on the panel
	}
	a.ns.err = err.Error()
	a.ns.errAt = time.Now()
}

// showText puts a day's text in the state the note opens in: rendered when it
// holds something, and rendered and empty when it does not, so the note is
// only ever written by asking for it.
func (a *app) showText(text string) {
	a.ns.text = text
	a.ns.caret = len([]rune(text))
	a.ns.sel = [2]int{a.ns.caret, a.ns.caret}
	a.ns.reading = true
	a.ns.focus = true
}

// editNote goes from reading to writing and puts the caret back where the
// writing was left.
func (a *app) editNote() {
	if !a.ns.reading {
		return
	}
	a.ns.reading = false
	a.ns.focus = true
}

// saveNote writes the note and reads it back as markdown, as Save and Cmd+S
// do. A save that went through is the note; one that failed keeps the typing,
// so nothing written is lost. Whitespace alone saves as no note, so it reads
// back empty rather than as blank lines.
func (a *app) saveNote() {
	a.flushNote()
	if a.ns.dirty {
		a.showText(a.ns.text)
	} else {
		a.showText(a.ns.note.Text)
	}
}

// settleNote ends writing as the panel closes, so the note comes back
// rendered. The state belongs to the note, not to the view, so this does not
// care which tab is on, and coming back does not steal the keyboard.
func (a *app) settleNote() {
	a.saveNote()
	a.ns.focus = false
}

// openDay puts a day on screen, saving what the day before it holds. A day
// still to come opens nothing: there is no today to write about.
func (a *app) openDay(day string) bool {
	if day > today() {
		return false
	}
	a.flushNote()
	if a.ns.dirty {
		return false // the note is not on disk yet: stay on it
	}
	a.ns.day = day
	a.ns.note, _ = a.notes.Get(day)
	a.showText(a.ns.note.Text)
	if at, err := time.ParseInLocation(dayLayout, day, time.Local); err == nil {
		a.ns.calDay = at
		if at.Month() != a.ns.calMonth.Month() || at.Year() != a.ns.calMonth.Year() {
			a.ns.calMonth = monthStart(at)
		}
	}
	return true
}

// openNote is where the global shortcut lands: the notes tab comes on screen
// and the note opens in writing — the shortcut asks for the note, not to read
// it — so the day can be typed or pasted into without a click first.
func (a *app) openNote(day string) {
	if day == "" || day > today() {
		day = today()
	}
	// The day may already be on screen: the shortcut still asks for its editor.
	if a.ns.day == day || a.openDay(day) {
		a.tab = 1
		a.editNote()
	}
}

// notesToday follows the date: after midnight an untouched day becomes
// today's, while a note in use stays where it is.
func (a *app) notesToday() {
	if a.ns.day == today() || notes.HasText(a.ns.note.Text) {
		return
	}
	a.openDay(today())
}

// deleteNote empties the note in place: the editor first, so the rendered
// note goes too, rather than reading as still there.
func (a *app) deleteNote() {
	a.ns.text = ""
	a.ns.dirty = true
	a.flushNote()
	a.showText("")
}

// pinNote keeps the day's note forever, or hands it back to the window.
func (a *app) pinNote() {
	note, err := a.notes.Pin(a.ns.day, !a.ns.note.Pinned)
	if err != nil {
		a.noteError(err)
		return
	}
	a.ns.note = note
	if !notes.HasText(note.Text) {
		// Releasing a note the rolling window has already left behind
		// removes it, so the page has to read empty rather than keep
		// showing text the store no longer holds.
		a.showText("")
	}
}

// toggleCalendar opens or closes the picker over the editor.
func (a *app) toggleCalendar() {
	a.ns.calOpen = !a.ns.calOpen
	if !a.ns.calOpen {
		// The picker is going: the note takes the keyboard back, as it
		// had before the picker took it.
		a.ns.focus = true
		return
	}
	a.ns.focusCal = true
	if at, err := time.ParseInLocation(dayLayout, a.ns.day, time.Local); err == nil {
		a.ns.calDay = at
	}
}

// moveCalDay walks the calendar's keys by days, never past today: there is no
// today to write about.
func (a *app) moveCalDay(delta int) {
	next := a.ns.calDay.AddDate(0, 0, delta)
	if next.Format(dayLayout) > today() {
		return
	}
	a.ns.calDay = next
	if next.Month() != a.ns.calMonth.Month() || next.Year() != a.ns.calMonth.Year() {
		a.ns.calMonth = monthStart(next)
	}
}

// moveCalMonth moves the calendar by months, as far back as the store can
// name one, never further.
func (a *app) moveCalMonth(delta int) {
	next := a.ns.calMonth.AddDate(0, delta, 0)
	if next.Year() < notes.OldestYear {
		return
	}
	a.ns.calMonth = next
}

// pickDay opens a day and closes the picker, so the note it chose is what the
// user sees next.
func (a *app) pickDay(day string) {
	a.ns.calOpen = false
	if a.openDay(day) {
		a.ns.focus = true
	}
}

// monthStart is the first day of a month, in its own location.
func monthStart(at time.Time) time.Time {
	return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, at.Location())
}

// dayName is the notes head's title: "Today", or the day's name.
func (a *app) dayName() string {
	if a.ns.day == today() {
		return "Today"
	}
	at, err := time.ParseInLocation(dayLayout, a.ns.day, time.Local)
	if err != nil {
		return a.ns.day
	}
	return at.Format("Monday, Jan 2")
}

// pinLabel is what the Keep button holds.
func (a *app) pinLabel() string {
	if a.ns.note.Pinned {
		return "Kept"
	}
	return "Keep"
}

// previewSource is the note's source as the preview reads it: the line
// breaks the design's renderer makes. It is kept until the text moves, so a
// frame that changes nothing does not rebuild the string.
func (a *app) previewSource() string {
	if a.ns.renderedFor != a.ns.text {
		a.ns.rendered = hardBreaks(a.ns.text)
		a.ns.renderedFor = a.ns.text
	}
	return a.ns.rendered
}

// ---- The view ----

// notesView is the day's note: the head with the calendar and Keep, the
// editor, Save over it, the picker over that, and the footer.
func (a *app) notesView(c *ui.Context, t *ui.Theme) {
	a.md.Theme = markdownTheme(t)
	anchor := a.noteHead(c, t)
	a.noteErrorLine(c, t)
	editor := a.noteEditor(c, t)
	a.calendar(c, t, anchor, editor)
	a.saveButton(c, t, editor)
	a.notesFoot(c, t)
}

// noteHead is the calendar button, the day, and Keep on the right. It
// returns the calendar button, which the picker hangs on.
func (a *app) noteHead(c *ui.Context, t *ui.Theme) ui.Element {
	var anchor ui.Element
	ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(0, 4, 10, 4).Children(func() {
		cal := ui.ButtonBase(c.Key("calToggle")).Size(30, 30).Radius(9).
			Label("Calendar").Cursor(ui.CursorPointer)
		bg, fg := fillColor(t), t.TextMuted
		if a.ns.calOpen {
			bg, fg = accent, t.AccentText
		} else if cal.Hovered() {
			fg = t.Text
		}
		cal.Background(bg).TextColor(fg)
		cal.Children(func() { ui.Icon(c, iconCalendarDays).Size(16, 16) })
		if pressed := cal.Pressed(); pressed && !a.ns.calPressed {
			// A press on the button while the picker is open closes it
			// through the popover; the click that ends it must not open
			// it again.
			a.ns.calPressOpen = a.ns.calOpen
		}
		a.ns.calPressed = cal.Pressed()
		if cal.Clicked() {
			if a.ns.calPressOpen {
				a.ns.calPressOpen = false
			} else {
				a.toggleCalendar()
			}
		}
		anchor = cal

		ui.Text(c, a.dayName()).FontSize(20).FontWeight(650).LetterSpacing(-0.4).
			Grow(1).SingleLine()

		// Keep is always on the right; it only goes quiet when the day has
		// nothing to keep yet.
		pin := ui.ButtonBase(c.Key("pinNote")).Label(a.pinLabel()).Gap(5).
			Padding(5, 9, 5, 7).Radius(8)
		hasText := notes.HasText(a.ns.note.Text)
		pfg, pborder, pbg := t.TextMuted, t.Border, ui.Transparent
		switch {
		case a.ns.note.Pinned:
			pfg, pborder, pbg = accent, accent.Alpha(0.45), accent.Alpha(0.12)
		case hasText && pin.Hovered():
			pfg, pbg = t.Text, fillColor(t)
		}
		pin.TextColor(pfg).Border(1, pborder).Background(pbg)
		if hasText {
			pin.Cursor(ui.CursorPointer)
			if pin.Clicked() {
				a.pinNote()
			}
		} else {
			// Nothing to keep: the button only fades; border and all, it
			// stays in place, as the design's disabled Keep does.
			pin.Opacity(quietOpacity).Disabled(true)
		}
		pin.Children(func() {
			ui.Icon(c, iconPin).Size(13, 13)
			ui.Text(c, a.pinLabel()).FontSize(12).SingleLine()
		})
	})
	return anchor
}

// noteErrorLine is what went wrong with the last action, for a while.
func (a *app) noteErrorLine(c *ui.Context, t *ui.Theme) {
	if a.ns.err == "" {
		return
	}
	if left := noteErrorFor - time.Since(a.ns.errAt); left <= 0 {
		a.ns.err = ""
		return
	} else {
		c.After(left)
	}
	ui.Text(c, a.ns.err).FontSize(12).TextColor(t.Danger).Padding(0, 4, 6, 4)
}

// noteEditor is the note's page: the rendered note while it is read, the
// source while it is written.
func (a *app) noteEditor(c *ui.Context, t *ui.Theme) ui.Element {
	box := ui.ButtonBase(c.Key("editor")).Grow(1)
	if a.ns.reading {
		a.ns.wasFocus = false
		a.readingNote(c, t, box)
	} else {
		a.writingNote(c, t, box)
	}
	return box
}

// readingNote is the note as rendered markdown. A click, or a keystroke, goes
// back to writing it.
func (a *app) readingNote(c *ui.Context, t *ui.Theme, box ui.Element) {
	if a.ns.focus {
		box.Focus()
		a.ns.focus = false
	}
	box.Focusable().FocusRing(false).Label("Note").Cursor(ui.CursorText)
	box.Radius(12)
	if a.ns.calOpen {
		// The picker floats over the note: the note's own sheet would
		// show as an empty box around it.
		box.Background(ui.Transparent)
	} else {
		// The page's preview: a fill over the panel, a line around it.
		box.Background(fillColor(t)).Border(1, t.Border)
	}
	box.HandleInput(a.noteTyped)
	if box.FocusVisible() {
		box.DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, ui.RGBA(10, 132, 255, 0.65), 11, 2)
		})
	}
	if box.Clicked() {
		a.editNote()
	}
	box.Children(func() {
		// The picker floats over the page, and its surface is mostly
		// transparent: what it covers would read through it, a word at a
		// time. So while it is open the page draws none of its own: the
		// frost shows the note's veil alone, as it does over an empty day,
		// and no word or hint shows through it. The elements stay, drawn at
		// no opacity, so the note keeps its scroll and the picker closing
		// brings it back as it was.
		if notes.HasText(a.ns.text) {
			preview := ui.Scroll(c.Key("preview")).Fill()
			if a.ns.calOpen {
				// The picker is over the note: its surface is the note's
				// own fill, which is thin, and the words would read
				// through it.
				preview.Opacity(0)
			}
			preview.Children(func() {
				ui.Column(c).FillWidth().Padding(10, 12).Children(func() {
					markdown.View(c, a.previewSource(), &a.md)
				})
			})
		} else {
			// The page's editor-empty is a layer of its own with the same
			// surface veil as the page under it, so an empty note reads as
			// the frosted sheet the page shows, not a clear pane. Its hint
			// goes the way of the note under the picker.
			empty := ui.Column(c).Fill().Center().Radius(12).
				Background(surfaceColor(t).Alpha(0.4))
			empty.Children(func() {
				hint := ui.Text(c, "No note on this day.").FontSize(13).TextColor(t.TextMuted)
				if a.ns.calOpen {
					hint.Opacity(0)
				}
			})
		}
	})
}

// writingNote is the note's source in a text area, which is only left by Save
// or by the panel closing.
func (a *app) writingNote(c *ui.Context, t *ui.Theme, box ui.Element) {
	box.Radius(12)
	box.Children(func() {
		input := ui.TextAreaBase(c.Key("noteBody"), &a.ns.text).
			Label("Note").FontSize(13).Cursor(ui.CursorText).
			Fill().Padding(10, 12).FocusRing(false).OnTextKey(a.noteKey)
		if a.ns.calOpen {
			// The picker covers the source, and its surface is mostly
			// transparent: the text goes with it, so no word reads
			// through the frost. The area keeps its text and caret.
			input.Opacity(0)
		}
		if a.ns.focus {
			input.Focus()
			input.SetTextSelection(a.ns.caret, a.ns.caret)
			a.ns.focus = false
		}
		if a.ns.setSel != nil {
			input.SetTextSelection(a.ns.setSel[0], a.ns.setSel[1])
			a.ns.setSel = nil
		}
		if input.Changed() {
			a.typedNote()
		}
		a.ns.sel[0], a.ns.sel[1] = input.TextSelection()
		_, caret := input.TextAnchorCaret()
		a.ns.caret = caret

		// Losing focus ends writing a note that was never written in: the
		// day goes back to being read as the empty note it is. A note with
		// words stays in the editor — the caret keeps it — until Save or
		// the panel closing reads it back.
		focused := input.Focused()
		if a.ns.wasFocus && !focused {
			if !notes.HasText(a.ns.text) {
				a.ns.reading = true
				// This frame drew the source, so the empty page it reads
				// back as is a frame away: ask for it now, rather than
				// leaving the hint to the next pointer move.
				c.After(0)
			}
			a.flushNote()
		}
		a.ns.wasFocus = focused

		border := t.Border
		if input.Focused() {
			border = accent.Alpha(0.55)
		}
		if a.ns.calOpen {
			box.Background(ui.Transparent)
		} else {
			// The page's editor: the same fill and line as its preview.
			box.Background(fillColor(t)).Border(1, border)
		}
	})
}

// noteTyped writes a keystroke that arrived while the note was being read,
// for the keyboard's sake.
func (a *app) noteTyped(ev ui.InputEvent) bool {
	if ev.Mods&(ui.Ctrl|ui.Alt|ui.Super) != 0 {
		return false
	}
	switch {
	case ev.Kind == ui.InputText:
		a.typeNote(ev.Text)
		return true
	case ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEnter:
		a.typeNote("\n")
		return true
	case ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyBackspace:
		a.typeNote("Backspace")
		return true
	case ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyDelete:
		a.typeNote("Delete")
		return true
	}
	return false
}

// typeNote writes text into the source where the caret is, as a keystroke
// would.
func (a *app) typeNote(key string) {
	if a.ns.reading {
		a.editNote()
	}
	r := []rune(a.ns.text)
	at := min(max(a.ns.caret, 0), len(r))
	switch key {
	case "Backspace":
		if at == 0 {
			return
		}
		r = append(r[:at-1], r[at:]...)
		at--
	case "Delete":
		if at >= len(r) {
			return
		}
		r = append(r[:at], r[at+1:]...)
	default:
		ins := []rune(key)
		next := make([]rune, 0, len(r)+len(ins))
		next = append(next, r[:at]...)
		next = append(next, ins...)
		next = append(next, r[at:]...)
		r, at = next, at+len(ins)
	}
	a.ns.text = string(r)
	a.ns.caret = at
	a.ns.sel = [2]int{at, at}
	a.typedNote()
}

// noteKey is the editor's keys beyond typing: Tab indents the lines the
// selection touches, which is what Tab does in a markdown editor, and
// Backspace at the start of the note takes a level back.
func (a *app) noteKey(k ui.TextKey) bool {
	switch {
	case k.Key == ui.KeyTab && k.Mods == 0:
		text, start, end := indentLines(k.Text(), a.ns.sel[0], a.ns.sel[1])
		k.SetText(text)
		a.ns.setSel = &[2]int{start, end}
		a.typedNote()
		return true
	case k.Key == ui.KeyBackspace && k.Mods == 0 && a.ns.sel == [2]int{0, 0} && strings.HasPrefix(k.Text(), " "):
		k.SetText(unindentLine(k.Text()))
		a.ns.setSel = &[2]int{0, 0}
		a.typedNote()
		return true
	}
	return false
}

// indentLines adds a level to every line the selection touches, which is what
// Tab does in a markdown editor: it returns the text and the selection it
// leaves.
func indentLines(text string, from, to int) (string, int, int) {
	r := []rune(text)
	from = min(max(from, 0), len(r))
	to = min(max(to, from), len(r))
	start := 0
	for i := from - 1; i >= 0; i-- {
		if r[i] == '\n' {
			start = i + 1
			break
		}
	}
	end := len(r)
	for i := to; i < len(r); i++ {
		if r[i] == '\n' {
			end = i
			break
		}
	}
	lines := strings.Split(string(r[start:end]), "\n")
	for i := range lines {
		lines[i] = noteIndent + lines[i]
	}
	block := strings.Join(lines, "\n")
	return string(r[:start]) + block + string(r[end:]), start, start + len([]rune(block))
}

// unindentLine removes one level from the first line, for Backspace at its
// start.
func unindentLine(text string) string {
	if strings.HasPrefix(text, noteIndent) {
		return text[len(noteIndent):]
	}
	if strings.HasPrefix(text, " ") {
		return text[1:]
	}
	return text
}

// saveButton is Save, over the note's corner: it ends writing and renders the
// note. It belongs to writing, and only when there is something to save; the
// picker covers it, so Save hides under it.
func (a *app) saveButton(c *ui.Context, t *ui.Theme, editor ui.Element) {
	if a.ns.reading || a.ns.calOpen || !notes.HasText(a.ns.text) {
		return
	}
	ui.Overlay(c, func() {
		b := ui.ButtonBase(c.Key("saveNote")).Label("Save").Cursor(ui.CursorPointer)
		// The page's bottom: 8px and right: 8px from the note's box: attached,
		// Right and Bottom are the room kept from that corner.
		b.AttachTo(editor, ui.AnchorBottomRight, ui.AnchorBottomRight).Right(8).Bottom(8)
		b.Gap(5).Padding(4, 10, 4, 8).Radius(8).TextColor(t.AccentText)
		bg := accent
		if b.Hovered() {
			bg = accentHover
		}
		b.Background(bg).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.2))
		b.Children(func() {
			ui.Icon(c, iconCheck).Size(12, 12)
			// The page's 1.4 line height, which its button's height is.
			ui.Text(c, "Save").FontSize(11.5).FontWeight(550).LineHeight(1.4)
		})
		if b.Clicked() {
			a.saveNote()
		}
	})
}

// calendar is the picker over the note: the page turned over, with the same
// box, corners and fill. mygo closes it on Escape or a press outside it and
// gives the keyboard back to the note when it goes. A dot marks a day with a
// note, a filled one a note kept forever, and greyed numbers are archived and
// future days.
func (a *app) calendar(c *ui.Context, t *ui.Theme, anchor, editor ui.Element) {
	if !anchor.Valid() || !editor.Valid() {
		return
	}
	was := a.ns.calOpen
	box := editor.LayoutBounds()
	width, height := box.W, box.H
	if width <= 0 || height <= 0 {
		// The first frame has no box to cover yet: fall back to the design's
		// width, and take the note's own box on the next frame.
		width, height = calWidth, 0
		c.After(0)
	}
	ui.PopoverBase(c, anchor, &a.ns.calOpen, func(panel ui.Element) {
		// The picker is laid exactly over the note's page: its box, its
		// corners, its fill and its line, so that it reads as the page
		// turned over rather than a window laid on top of it.
		panel.AttachTo(editor, ui.AnchorTopLeft, ui.AnchorTopLeft).Margin(0)
		panel.Width(width)
		if height > 0 {
			panel.Height(height)
		}
		panel.Column().Padding(calPad).Gap(0).Radius(t.Radius*1.2).
			Background(fillColor(t)).Border(1, t.Border).TextColor(t.Text).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
		a.calendarPanel(c, t, panel, width)
	})
	// The picker closed on this frame — on Escape, or on a press outside
	// it, which mygo hands to the popover: the note takes the keyboard
	// back, as it had before the picker took it.
	if was && !a.ns.calOpen {
		a.ns.focus = true
	}
}

// calendarPanel is what the picker holds: the month's name, the days, and
// what the legend says.
func (a *app) calendarPanel(c *ui.Context, t *ui.Theme, panel ui.Element, width float32) {
	panel.Padding(calPad).Gap(0)
	panel.Focusable().FocusRing(false).Label("Note calendar")
	if a.ns.focusCal {
		panel.Focus()
		a.ns.focusCal = false
	}

	// Arrows walk the month, Enter opens the day under the cursor.
	if panel.Shortcut(0, ui.KeyLeft) {
		a.moveCalDay(-1)
	}
	if panel.Shortcut(0, ui.KeyRight) {
		a.moveCalDay(1)
	}
	if panel.Shortcut(0, ui.KeyUp) {
		a.moveCalDay(-7)
	}
	if panel.Shortcut(0, ui.KeyDown) {
		a.moveCalDay(7)
	}
	if panel.Shortcut(0, ui.KeyEnter) || panel.Shortcut(0, ui.KeySpace) {
		a.pickDay(a.ns.calDay.Format(dayLayout))
	}

	panel.Children(func() {
		ui.Column(c).Fill().Children(func() {
			// The month's name, one month at a time.
			ui.Row(c).AlignItems(ui.Center).Gap(4).Padding(0, 0, 6, 0).Children(func() {
				prev := ui.ButtonBase(c.Key("calPrev")).Size(22, 22).Radius(6).
					Label("Previous month").TextColor(t.TextMuted)
				prev.Children(func() { ui.Icon(c, iconChevronLeft).Size(14, 14) })
				oldest := a.ns.calMonth.Year() <= notes.OldestYear && a.ns.calMonth.Month() == time.January
				switch {
				case oldest:
					// The store cannot name a month before this one:
					// the button is inert, pointer and all.
					prev.Opacity(0.3).Disabled(true)
				case prev.Hovered():
					prev.Cursor(ui.CursorPointer).Background(fillColor(t)).TextColor(t.Text)
				default:
					prev.Cursor(ui.CursorPointer)
				}
				if !oldest && prev.Clicked() {
					a.moveCalMonth(-1)
				}

				ui.Text(c, fmt.Sprintf("%s %d", a.ns.calMonth.Month(), a.ns.calMonth.Year())).
					FontSize(12.5).FontWeight(600).TextAlign(ui.Center).Grow(1)

				next := ui.ButtonBase(c.Key("calNext")).Size(22, 22).Radius(6).
					Label("Next month").Cursor(ui.CursorPointer).TextColor(t.TextMuted)
				next.Children(func() { ui.Icon(c, iconChevronRight).Size(14, 14) })
				if next.Hovered() {
					next.Background(fillColor(t)).TextColor(t.Text)
				}
				if next.Clicked() {
					a.moveCalMonth(1)
				}
			})

			ui.Row(c).Children(func() {
				for _, wd := range [7]string{"S", "M", "T", "W", "T", "F", "S"} {
					ui.Text(c, wd).FontSize(10).FontWeight(600).TextColor(t.TextMuted).
						TextAlign(ui.Center).Grow(1).Padding(0, 0, 3, 0)
				}
			})

			grid := ui.Grid(c.Key("calGrid")).Columns(7).Gap(calGap)
			// The cells are square, as the design's aspect-ratio: 1: the grid
			// is as wide as the picker's content box, which is the note's
			// own width less the padding and the line around it.
			cellH := max((width-2*(calPad+1)-6*calGap)/7, 1)
			marks := a.monthMarks(a.ns.calMonth)
			first := monthStart(a.ns.calMonth)
			grid.Children(func() {
				for range int(first.Weekday()) {
					ui.Box(c).Height(cellH)
				}
				todayKey := today()
				for _, mark := range marks {
					a.calendarDay(c, t, panel, mark, cellH, todayKey)
				}
			})

			// The legend, as the design's: what the marks of the days mean.
			ui.Row(c).Gap(10).Padding(8, 0, 0, 0).Children(func() {
				for _, item := range []struct {
					color ui.Color
					label string
				}{
					{goldColor(t), "Kept forever"},
					{accent, "Has a note"},
					{t.TextMuted.Alpha(0.45), "Archived"},
				} {
					ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
						ui.Box(c).Size(calDot, calDot).Radius(calDot / 2).Background(item.color)
						ui.Text(c, item.label).FontSize(10.5).TextColor(t.TextMuted)
					})
				}
			})
		})
	})
}

// calendarDay is one day of the picker: a pill for the day on screen, a dot
// for a note, a ring for today, and greyed numbers for the days the window
// has let go and the days still to come.
func (a *app) calendarDay(c *ui.Context, t *ui.Theme, panel ui.Element, mark notes.DayMark, cellH float32, todayKey string) {
	at, err := time.ParseInLocation(dayLayout, mark.Day, time.Local)
	if err != nil {
		return
	}
	future := mark.Day > todayKey
	cell := ui.ButtonBase(c.Key(mark.Day)).Height(cellH).Radius(8).Label(dayAria(at, mark))
	if !future {
		cell.Cursor(ui.CursorPointer)
	}
	selected := mark.Day == a.ns.day
	fg := t.Text
	if future || (mark.Archived && !mark.Has) {
		// Greyed numbers, as the design's archived and future days, which
		// hover only behind, never in their ink.
		fg = t.TextMuted
	}
	switch {
	case selected:
		cell.Background(accent)
		fg = t.AccentText
	case cell.Hovered():
		cell.Background(fillColor(t))
	}
	cell.TextColor(fg)
	if future || (mark.Archived && !mark.Has) {
		// The whole cell fades, dot and all, as the design's opacity does.
		cell.Opacity(0.4)
	}
	cell.Children(func() {
		ui.Text(c, strconv.Itoa(at.Day())).FontSize(11.5).FontFeatures("tnum").
			FontWeight(weightFor(mark, selected))
	})
	cell.Draw(func(p *ui.Painter, r ui.Rect) {
		if mark.Day == todayKey {
			p.Stroke(ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}, hairlineColor(t), 8, 1)
		}
	})
	cell.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if a.ns.calDay.Format(dayLayout) == mark.Day && panel.FocusVisible() {
			p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, accent.Alpha(0.65), 8, 2)
		}
		if !mark.Has {
			return
		}
		dot := accent
		if mark.Pinned {
			dot = goldColor(t)
		}
		if selected {
			dot = t.AccentText
			if mark.Pinned {
				dot = goldColor(t)
			}
		}
		p.Fill(ui.Rect{X: r.X + r.W/2 - calDot/2, Y: r.Y + r.H - 7, W: calDot, H: calDot}, dot, calDot/2)
	})
	if cell.Clicked() && !future {
		a.pickDay(mark.Day)
	}
}

// weightFor is the weight of a day's number: today's is bold, the day on
// screen's a little heavier than the rest.
func weightFor(mark notes.DayMark, selected bool) int {
	switch {
	case selected:
		return 600
	case mark.Day == today():
		return 700
	default:
		return 400
	}
}

// dayAria names a day for assistive technology, with the mark the calendar
// draws for it.
func dayAria(at time.Time, mark notes.DayMark) string {
	what := ""
	switch {
	case mark.Pinned:
		what = ", kept forever"
	case mark.Has:
		what = ", has a note"
	case mark.Archived:
		what = ", archived"
	}
	return at.Format("Monday, January 2") + what
}

// notesFoot counts the notes and ends theirs: the footer's info and Delete.
func (a *app) notesFoot(c *ui.Context, t *ui.Theme) {
	info := a.notesInfo()
	count := "No notes yet"
	if info.Days > 0 {
		plural := "notes"
		if info.Days == 1 {
			plural = "note"
		}
		count = fmt.Sprintf("%d %s", info.Days, plural)
	}
	kept := fmt.Sprintf("the last %d days", info.Window)
	if info.Pinned > 0 {
		kept = fmt.Sprintf("%d kept forever", info.Pinned)
	}
	ui.Row(c).AlignItems(ui.Center).Gap(12).Margin(8, 0, 0, 0).Padding(10, 4, 2, 4).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, count+" · "+kept).FontSize(12).TextColor(t.TextMuted)
		ui.Spacer(c)
		has := notes.HasText(a.ns.note.Text)
		// Delete stays on the right, like Keep, as the design's quiet link:
		// it only fades when the day has nothing to delete yet, and a note
		// cannot be brought back, so it is held rather than clicked.
		key := "deleteNote:" + a.ns.day
		holdLink(c, key, "Delete note", "Hold to delete the note", !has, a.deleteNote)
	})
}

// hardBreaks is the note's source as its preview reads it: every newline
// inside a paragraph breaks the line, the way the preview renders the note.
// The parser reads a newline between a paragraph's lines as a space, so a
// note typed on several lines would read on one; two trailing spaces read
// as a hard break, and a hard break at a line's end is nothing where a
// block ends there instead. Code fences hold, as the design's own do.
func hardBreaks(src string) string {
	lines := strings.Split(src, "\n")
	inFence := false
	for i := 0; i < len(lines); i++ {
		if isFence(lines[i]) {
			inFence = !inFence
			continue
		}
		if inFence || i == len(lines)-1 || strings.TrimRight(lines[i], " \t") == "" ||
			strings.TrimRight(lines[i+1], " \t") == "" {
			continue
		}
		rest := strings.TrimRight(lines[i], " ")
		if len(lines[i])-len(rest) >= 2 || // it breaks already
			strings.HasSuffix(rest, "\\") { // so does a backslash
			continue
		}
		lines[i] = lines[i] + "  "
	}
	return strings.Join(lines, "\n")
}

// isFence says whether a line opens or closes a ``` code fence.
func isFence(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), "```")
}

// markdownTheme is the design's rendered markdown, laid over the framework's
// default: the sizes and colors of the preview's stylesheet.
func markdownTheme(t *ui.Theme) *markdown.Theme {
	em := func(px float32) float32 { return px / 13 }
	heading := func(size float32, color ui.Color) markdown.HeadingStyle {
		return markdown.HeadingStyle{
			Size:   em(size),
			Weight: 650,
			Color:  color,
			Top:    em(14),
			Bottom: em(5),
		}
	}
	return &markdown.Theme{
		FontSize:     13,
		LineHeight:   1.4,
		BlockSpacing: em(9),
		ItemSpacing:  em(2),
		ListIndent:   em(20),
		StrongWeight: 700,
		CodeSize:     em(11.5),
		// The page's links are its accent blue, not the text's own color.
		Link: accent,
		Headings: [6]markdown.HeadingStyle{
			heading(17, t.Text),
			heading(15, t.Text),
			heading(13.5, t.Text),
			heading(12.5, t.TextMuted),
			heading(12.5, t.TextMuted),
			heading(12.5, t.TextMuted),
		},
		CodeBackground:      t.TextMuted.Alpha(0.18),
		CodeBlockBackground: t.TextMuted.Alpha(0.14),
		CodeBlockRadius:     9,
		CodeBlockPadding:    em(8),
		QuoteBar:            t.Border,
		QuoteBarWidth:       2,
		QuoteText:           t.TextMuted,
		Rule:                t.Border,
		Checkbox:            accent,
	}
}

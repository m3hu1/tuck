package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"tuck/internal/notes"
)

// TestNotesView walks the notes tab the way it is used: typed in, saved,
// kept, picked from the calendar, and deleted.
func TestNotesView(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)

	openNotesTab(t, tt)
	if !tt.HasText("No note on this day.") {
		t.Fatalf("no empty day: %q", tt.Texts())
	}
	if !tt.HasText("No notes yet · the last 30 days") {
		t.Fatalf("no footer count: %q", tt.Texts())
	}
	if tt.HasText("Save") {
		t.Fatalf("Save shows on an empty note: %q", tt.Texts())
	}

	// Typing goes straight into the note, which starts writing itself.
	tt.Type("First line")
	if !tt.HasText("Save") {
		t.Fatalf("Save does not show: %q", tt.Texts())
	}
	if note, _ := a.notes.Get(today()); notes.HasText(note.Text) {
		t.Fatal("the keystroke reached the store before the delay")
	}

	// The autosave carries it there once the typing is quiet.
	tt.Advance(noteSaveDelay + 50*time.Millisecond)
	if note, _ := a.notes.Get(today()); note.Text != "First line" {
		t.Fatalf("the autosave left %+v", note)
	}

	// Save reads it back as markdown and takes Save away.
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Save") {
		t.Fatalf("Save still shows after saving: %q", tt.Texts())
	}
	if !tt.HasText("First line") {
		t.Fatalf("the note is not on the panel: %q", tt.Texts())
	}
	if !tt.HasText("1 note · the last 30 days") {
		t.Fatalf("the footer does not count it: %q", tt.Texts())
	}

	// Keep holds the note forever, and the footer counts it.
	if err := tt.Click("Keep"); err != nil {
		t.Fatal(err)
	}
	if !a.ns.note.Pinned {
		t.Fatal("the note is not kept")
	}
	if !tt.HasText("1 note · 1 kept forever") {
		t.Fatalf("the footer does not count the kept note: %q", tt.Texts())
	}
	if err := tt.Click("Kept"); err != nil {
		t.Fatal(err)
	}
	if a.ns.note.Pinned {
		t.Fatal("the note is still kept")
	}

	// The calendar over the editor: the month on top, today's cell marked,
	// the arrows walking the month.
	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	now := time.Now()
	if !tt.HasText(fmt.Sprintf("%s %d", now.Month(), now.Year())) {
		t.Fatalf("no month on the calendar: %q", tt.Texts())
	}
	// The cell of today, as the calendar names it: the mark of the day
	// comes from the store, as the cell's own name does.
	marks, err := a.notes.Month(now.Year(), int(now.Month()))
	if err != nil {
		t.Fatal(err)
	}
	var todayMark notes.DayMark
	for _, m := range marks {
		if m.Day == today() {
			todayMark = m
		}
	}
	if !tt.HasText(dayAria(now, todayMark)) {
		t.Fatalf("today's cell is not on the calendar: %q", tt.Texts())
	}
	if err := tt.Click("Next month"); err != nil {
		t.Fatal(err)
	}
	next := now.AddDate(0, 1, 0)
	if !tt.HasText(fmt.Sprintf("%s %d", next.Month(), next.Year())) {
		t.Fatalf("the next month did not come: %q", tt.Texts())
	}
	if err := tt.Click("Previous month"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(fmt.Sprintf("%s %d", now.Month(), now.Year())) {
		t.Fatalf("the previous month did not come back: %q", tt.Texts())
	}

	// Picking a day closes the picker and leaves the note on the panel.
	if err := tt.Click(dayAria(now, todayMark)); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if tt.HasText(fmt.Sprintf("%s %d", now.Month(), now.Year())) {
		t.Fatalf("the calendar stayed open: %q", tt.Texts())
	}
	if a.ns.day != today() {
		t.Fatalf("the calendar opened %q", a.ns.day)
	}
	if !tt.HasText("First line") {
		t.Fatalf("the note went away with the calendar: %q", tt.Texts())
	}

	// Delete is held, not clicked: a click does nothing.
	if err := tt.Click("Delete note"); err != nil {
		t.Fatal(err)
	}
	if note, _ := a.notes.Get(today()); note.Text != "First line" {
		t.Fatalf("a click deleted the note: %+v", note)
	}

	// Holding it empties the day where it stands.
	hold(t, tt, "Delete note")
	if !tt.HasText("No note on this day.") {
		t.Fatalf("the deleted day does not read empty: %q", tt.Texts())
	}
	if note, _ := a.notes.Get(today()); note.Text != "" {
		t.Fatalf("the delete left %+v", note)
	}
	if !tt.HasText("No notes yet · the last 30 days") {
		t.Fatalf("the footer does not see the delete: %q", tt.Texts())
	}
}

// TestNotesViewTabs covers the note across the tabs: it keeps writing while
// the day's list is on, the autosave follows it there, and coming back
// settles the note as the panel closing does.
func TestNotesViewTabs(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)

	openNotesTab(t, tt)
	tt.Type("Half written")
	if !a.ns.dirty {
		t.Fatal("the keystroke did not arm the autosave")
	}

	// The note keeps writing as the other tab comes on, and the add field
	// takes the keyboard there.
	if err := tt.Click("Tasks"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("New task") {
		t.Fatal("the add field is not focused after the switch")
	}
	tt.Advance(noteSaveDelay + 50*time.Millisecond)
	if note, _ := a.notes.Get(today()); note.Text != "Half written" {
		t.Fatalf("the autosave did not reach the store: %+v", note)
	}

	// Coming back, the note is still being written, with what it holds.
	if err := tt.Click("Notes"); err != nil {
		t.Fatal(err)
	}
	if a.ns.reading {
		t.Fatal("the note came back as a page")
	}
	if !tt.HasText("Save") {
		t.Fatalf("the note did not come back: %q, text %q", tt.Texts(), a.ns.text)
	}

	// Settling the note reads it back, as the panel closing does.
	a.settleNote()
	tt.Frame()
	if !a.ns.reading {
		t.Fatal("the closing did not settle the note")
	}
	if !tt.HasText("Half written") {
		t.Fatalf("the settled note is not on the panel: %q", tt.Texts())
	}
}

// TestNotesEditorKeys walks the editor's keys: Tab indents the line the
// caret is on, Backspace at the start of the note takes the level back, and
// Cmd+S reads the note as markdown.
func TestNotesEditorKeys(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)
	tt.Type("first")
	tt.Key(0, ui.KeyEnter)
	tt.Type("second")
	tt.Key(0, ui.KeyTab)
	if a.ns.text != "first\n  second" {
		t.Fatalf("Tab left %q", a.ns.text)
	}
	if a.ns.caret != len([]rune("first\n  second")) {
		t.Fatalf("the caret is at %d, not after the indent", a.ns.caret)
	}

	// Backspace at the start of the note takes a level back.
	a.ns.reading = false
	a.ns.text = "  first line"
	a.ns.caret, a.ns.focus = 0, true
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	if a.ns.text != "first line" {
		t.Fatalf("Backspace at the start left %q", a.ns.text)
	}

	// Cmd+S ends writing and reads the note back as markdown.
	tt.Key(ui.Cmd, ui.KeyS)
	if !a.ns.reading {
		t.Fatal("Cmd+S did not end writing")
	}
	if note, _ := a.notes.Get(today()); note.Text != "first line" {
		t.Fatalf("Cmd+S left %+v", note)
	}
}

// TestNotesOpenNote covers what the shortcut and the turn of the day do to
// the day on screen.
func TestNotesOpenNote(t *testing.T) {
	a := testApp(t)

	// The shortcut asks for today's note, in writing.
	a.openNote("")
	if a.tab != 1 {
		t.Fatalf("the shortcut left tab %d", a.tab)
	}
	if a.ns.day != today() || a.ns.reading {
		t.Fatalf("the shortcut left day %q, reading %v", a.ns.day, a.ns.reading)
	}

	// A day the panel was left on comes back to today at the turn of the
	// day, while a note in use stays where it is.
	yesterday := time.Now().AddDate(0, 0, -1).Format(dayLayout)
	a.ns.day = yesterday
	a.notesToday()
	if a.ns.day != today() {
		t.Fatalf("an untouched day did not turn: %q", a.ns.day)
	}
	a.ns.note = notes.Note{Text: "in flight"}
	a.ns.day = yesterday
	a.notesToday()
	if a.ns.day != yesterday {
		t.Fatalf("a note in use turned: %q", a.ns.day)
	}
}

// TestNotesIndent covers Tab's indent on its own: the caret's line, a line
// the selection covers, and every line it touches.
func TestNotesIndent(t *testing.T) {
	text := "alice\nbob\ncarol"

	if got, start, end := indentLines(text, 0, 0); got != "  alice\nbob\ncarol" || start != 0 || end != 7 {
		t.Fatalf("indenting one line gives %q, %d, %d", got, start, end)
	}
	if got, start, end := indentLines(text, 0, 9); got != "  alice\n  bob\ncarol" || start != 0 || end != 13 {
		t.Fatalf("indenting a selected line gives %q, %d, %d", got, start, end)
	}
	if got, _, _ := indentLines(text, 0, len([]rune(text))); got != "  alice\n  bob\n  carol" {
		t.Fatalf("indenting across lines gives %q", got)
	}
}

// TestNotesSnapshot writes the notes tab to PNGs for a look, when SNAP names
// the directory to write them in: SNAP=/tmp go test -run Snapshot.
func TestNotesSnapshot(t *testing.T) {
	dir := os.Getenv("SNAP")
	if dir == "" {
		t.Skip("set SNAP to the directory for the snapshots")
	}
	a := testApp(t)
	// Yesterday's note, kept forever, and the day before it, empty: what
	// the calendar has to say.
	yesterday := time.Now().AddDate(0, 0, -1).Format(dayLayout)
	if _, err := a.notes.Save(yesterday, sampleNote); err != nil {
		t.Fatal(err)
	}
	if _, err := a.notes.Pin(yesterday, true); err != nil {
		t.Fatal(err)
	}

	tt := ui.NewTester(a.view, 340, 470)
	// shot writes the panel out under the name it is given.
	shot := func(name string) {
		t.Helper()
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Log(path)
	}

	// Today, in both appearances, reading the same note.
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		name := "light"
		if dark {
			name = "dark"
		}
		if _, err := a.notes.Save(today(), sampleNote); err != nil {
			t.Fatal(err)
		}
		openNotesTab(t, tt)
		a.openDay(today())
		tt.Advance(500 * time.Millisecond)
		shot("tuck-notes-" + name + ".png")

		// The calendar over it, with yesterday's pin and today's ring. It
		// closes again the way picking a day closes it.
		if err := tt.Click("Calendar"); err != nil {
			t.Fatal(err)
		}
		tt.Advance(300 * time.Millisecond)
		shot("tuck-notes-calendar-" + name + ".png")
		if err := tt.Click("Calendar"); err != nil {
			t.Fatal(err)
		}
		tt.Advance(200 * time.Millisecond)

		// The note being written, the caret at the end of what is there.
		a.openDay(today())
		a.ns.reading = false
		a.ns.text = "Form drags on.\n\n  so does the second line,\nand the list below\n"
		a.ns.caret = len([]rune(a.ns.text))
		// The state is the app's, not the view's: a frame has to be asked
		// for, or the panel keeps drawing what it last drew.
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		shot("tuck-notes-writing-" + name + ".png")

		// A day the window has let go, which reads empty.
		a.openDay(time.Now().AddDate(0, 0, -2).Format(dayLayout))
		tt.Frame()
		tt.Advance(150 * time.Millisecond)
		shot("tuck-notes-empty-" + name + ".png")
	}
}

// sampleNote is the note the snapshots render: a heading, a list, a task
// list, emphasis and a quote.
const sampleNote = `# Form
Dragons are the paperwork of the world: **every** task list is a nest of
them, and they are stamped in triplicate.

- one for the day
- one for the year
- one nobody ever reads

- [x] the thing that was done
- [ ] the thing that was not

> A note is a dragon's hoard in a very small cave.

    indented code, which is what code looks like
`

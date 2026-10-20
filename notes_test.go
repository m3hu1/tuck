package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"tuck/internal/notes"
	"tuck/internal/todos"
)

// TestNotesCalendarEscape covers the picker as a popover: it opens on the
// calendar button, takes the keyboard, and closes on Escape, which is what
// mygo gives a popover and what the design's own picker did not have.
func TestNotesCalendarEscape(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)

	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if !a.ns.calOpen {
		t.Fatal("the picker did not open")
	}
	if !tt.HasText("Previous month") {
		t.Fatalf("the picker is not on the panel: %q", tt.Texts())
	}

	// Escape closes it, and the note is left as it was.
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.ns.calOpen {
		t.Fatal("Escape did not close the picker")
	}
	if !tt.HasText("No note on this day.") {
		t.Fatalf("the note did not come back: %q", tt.Texts())
	}
	// The note takes the keyboard back: the picker had it, and the note
	// is what is typed into.
	if !tt.Focused("Note") {
		t.Fatal("the note did not take the keyboard back")
	}
	tt.Type("!")
	tt.Advance(100 * time.Millisecond)
	if a.ns.text != "!" {
		t.Fatalf("the note holds %q", a.ns.text)
	}
}

// TestNotesCalendarClickOutside covers the other way the picker closes: a
// press outside it, which mygo hands to the popover.
func TestNotesCalendarClickOutside(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)

	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if !a.ns.calOpen {
		t.Fatal("the picker did not open")
	}

	// A press on the note, which is under the picker, is outside it.
	tt.ClickAt(170, 430)
	tt.Advance(200 * time.Millisecond)
	if a.ns.calOpen {
		t.Fatal("the press outside did not close the picker")
	}
}

// TestNotesCalendarCoversNote covers the picker's box: it is the note's own,
// the page turned over, and the button under it closes and opens it in turn.
func TestNotesCalendarCoversNote(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)

	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	panel, ok := tt.Find("Note calendar")
	note, noteOK := tt.Find("Note")
	if !ok || !noteOK {
		t.Fatalf("the picker or the note is missing: %v, %v", ok, noteOK)
	}
	if panel != note {
		t.Fatalf("the picker is %+v, the note %+v", panel, note)
	}

	// The button closes the picker through the popover; the click that
	// ends the press must not open it again.
	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if a.ns.calOpen {
		t.Fatal("the button did not close the picker")
	}
	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if !a.ns.calOpen {
		t.Fatal("the button did not open the picker again")
	}
}

// TestNotesCalendarKeys covers the picker's own keys: the arrows walk the
// days, Enter opens the one under the cursor.
func TestNotesCalendarKeys(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)

	if err := tt.Click("Calendar"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)

	// The cursor starts on the day on screen; the left arrow walks it back,
	// and Enter opens the day it is on, and closes the picker with it.
	tt.Key(0, ui.KeyLeft)
	tt.Advance(50 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if a.ns.calOpen {
		t.Fatal("picking a day did not close the picker")
	}
	want := time.Now().AddDate(0, 0, -1).Format(dayLayout)
	if got := a.ns.day; got != want {
		t.Fatalf("the picker opened %q, not %q", got, want)
	}
}

// TestPreviewSource covers the note's source as the preview reads it: the
// line breaks the design's renderer makes, and the copy of it the view keeps
// between frames.
func TestPreviewSource(t *testing.T) {
	a := testApp(t)
	a.ns.text = "one\ntwo"
	if got, want := a.previewSource(), hardBreaks("one\ntwo"); got != want {
		t.Fatalf("the preview reads %q, want %q", got, want)
	}
	// The copy is kept while the text stands, and remade when it moves.
	kept := a.previewSource()
	if got := a.previewSource(); got != kept {
		t.Fatalf("the preview was remade: %q then %q", kept, got)
	}
	a.ns.text = "one\ntwo\nthree"
	if got, want := a.previewSource(), hardBreaks(a.ns.text); got != want {
		t.Fatalf("the preview reads %q, want %q", got, want)
	}
}

// TestStoreRevision covers the counters the view reads the stores by: they
// move when the store does, and stay where they are while it does not.
func TestStoreRevision(t *testing.T) {
	a := testApp(t)

	before := a.store.Revision()
	if got := a.items(); len(got) != 0 {
		t.Fatalf("the store holds %d todos", len(got))
	}
	if a.store.Revision() != before {
		t.Fatal("reading the list moved the revision")
	}

	if _, err := a.store.Add("Buy milk"); err != nil {
		t.Fatal(err)
	}
	if a.store.Revision() == before {
		t.Fatal("adding a todo did not move the revision")
	}
	if got := a.items(); len(got) != 1 || got[0].Title != "Buy milk" {
		t.Fatalf("the view reads %+v", got)
	}

	// The same for the notes.
	notesBefore := a.notes.Revision()
	if _, err := a.notes.Save(today(), "a note"); err != nil {
		t.Fatal(err)
	}
	if a.notes.Revision() == notesBefore {
		t.Fatal("saving a note did not move the revision")
	}
	if got := a.notesInfo(); got.Days != 1 {
		t.Fatalf("the view reads %+v", got)
	}
}

// TestMonthMarks covers the calendar's marks: they are read for the month on
// screen, and the copy the view keeps is remade when the month moves.
func TestMonthMarks(t *testing.T) {
	a := testApp(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format(dayLayout)
	if _, err := a.notes.Save(yesterday, "kept"); err != nil {
		t.Fatal(err)
	}

	at := time.Now()
	marks := a.monthMarks(at)
	var found bool
	for _, m := range marks {
		if m.Day == yesterday {
			found = true
			if !m.Has {
				t.Fatalf("the mark of %q says it has no note: %+v", yesterday, m)
			}
		}
	}
	if !found {
		t.Fatalf("the month has no mark for %q", yesterday)
	}

	// A month the notes do not touch has no marks of its own.
	other := a.monthMarks(at.AddDate(0, -3, 0))
	for _, m := range other {
		if m.Has {
			t.Fatalf("the month of %q has a note it does not hold: %+v", m.Day, m)
		}
	}
}

// TestNotesTabFocus covers what the keyboard does when the tabs change: the
// day's list takes it on its tab, and the note takes it on its own.
func TestNotesTabFocus(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)

	if !tt.Focused("New task") {
		t.Fatal("the add field does not have the focus at the start")
	}
	openNotesTab(t, tt)
	if a.tab != 1 {
		t.Fatalf("the tab is %d", a.tab)
	}
	if err := tt.Click("Tasks"); err != nil {
		t.Fatal(err)
	}
	if a.tab != 0 {
		t.Fatalf("the tab is %d", a.tab)
	}
	if !tt.Focused("New task") {
		t.Fatal("the add field did not take the focus back")
	}
}

// TestNotesDeleteKept covers deleting a note that is kept: the day goes, and
// the keep goes with it.
func TestNotesDeleteKept(t *testing.T) {
	a := testApp(t)
	if _, err := a.notes.Save(today(), "a note"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.notes.Pin(today(), true); err != nil {
		t.Fatal(err)
	}
	a.openDay(today())
	if !a.ns.note.Pinned {
		t.Fatal("the note is not kept")
	}

	a.deleteNote()
	if note, _ := a.notes.Get(today()); note.Text != "" {
		t.Fatalf("the delete left %+v", note)
	}
	if a.ns.note.Pinned {
		t.Fatal("the kept note is still kept")
	}
	if info := a.notesInfo(); info.Days != 0 || info.Pinned != 0 {
		t.Fatalf("the footer reads %+v", info)
	}
}

// TestNotesUnpinArchived covers releasing a note the rolling window has
// already left behind: the store removes it at once, and the page reads
// empty rather than keeping text that is no longer stored.
func TestNotesUnpinArchived(t *testing.T) {
	a := testApp(t)
	old := time.Now().AddDate(0, 0, -45).Format(dayLayout)
	if _, err := a.notes.Save(old, "from the archive"); err != nil {
		t.Fatal(err)
	}
	if !a.openDay(old) {
		t.Fatal("the archived day did not open")
	}
	if !a.ns.note.Pinned {
		t.Fatal("the archived note is not kept")
	}

	a.pinNote()
	if notes.HasText(a.ns.text) {
		t.Fatalf("the page still shows %q", a.ns.text)
	}
	if !a.ns.reading {
		t.Fatal("the page is still being written")
	}
	if note, _ := a.notes.Get(old); notes.HasText(note.Text) {
		t.Fatalf("the store still holds %+v", note)
	}
}

// TestNotesDeleteHoldsAgain covers the hold after it has done its work: it
// comes back with the next note, rather than staying done forever.
func TestNotesDeleteHoldsAgain(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)

	tt.Type("first")
	tt.Advance(noteSaveDelay + 50*time.Millisecond)
	hold(t, tt, "Delete note")
	if note, _ := a.notes.Get(today()); note.Text != "" {
		t.Fatalf("the first hold left %+v", note)
	}
	if !tt.HasText("No note on this day.") {
		t.Fatalf("the deleted day does not read empty: %q", tt.Texts())
	}

	// A new note can be deleted in turn: the hold starts from the top.
	tt.Type("second")
	tt.Advance(noteSaveDelay + 50*time.Millisecond)
	if note, _ := a.notes.Get(today()); note.Text != "second" {
		t.Fatalf("the new note is not in the store: %+v", note)
	}
	hold(t, tt, "Delete note")
	if note, _ := a.notes.Get(today()); note.Text != "" {
		t.Fatalf("the second hold left %+v", note)
	}
}

// TestStatsResetClock covers the time of the reset in the footer: the
// counters are cached, but the time of the reset moves with the clock and
// not with the list, so it is read afresh.
func TestStatsResetClock(t *testing.T) {
	a := testApp(t)
	if got := a.stats().ResetsAt; got == 0 {
		t.Fatal("the reset has no time")
	}
	// The counters the view holds are the store's, but the time of the
	// reset is not among them: a cached time would freeze the countdown.
	a.statCache.value.ResetsAt = 0
	if got := a.stats().ResetsAt; got == 0 {
		t.Fatal("the reset's time came from the cached counters")
	}
	if _, err := a.store.Add("Buy milk"); err != nil {
		t.Fatal(err)
	}
	if got := a.stats(); got.Total != 1 {
		t.Fatalf("the counters read %+v", got)
	}
}

// TestNoteSaveRetry covers a store that cannot be written to: the note is
// held, the failure is shown, and the app does not write to the store again
// on every frame.
func TestNoteSaveRetry(t *testing.T) {
	// A store whose file cannot be written: the directory is not there.
	a := &app{
		store: todos.NewStore(filepath.Join(t.TempDir(), "todos.json")),
		notes: notes.NewNotes(filepath.Join(t.TempDir(), "no-such-dir", "notes.json")),
	}
	a.initNotes()
	tt := ui.NewTester(a.view, 340, 470)
	openNotesTab(t, tt)
	tt.Type("a note")
	tt.Advance(noteSaveDelay + 50*time.Millisecond)
	if !a.ns.dirty {
		t.Fatal("the note reached the store")
	}
	if a.ns.err == "" {
		t.Fatal("the failure is not on the panel")
	}
	// The attempt waits as long as a keystroke does, rather than running
	// again on the next frame.
	first := a.ns.typedAt
	tt.Advance(50 * time.Millisecond)
	if !a.ns.typedAt.Equal(first) {
		t.Fatal("the attempt did not wait")
	}
}

// TestShortcuts covers what the two global shortcuts ask for: the day's
// list, and today's note in writing.
func TestShortcuts(t *testing.T) {
	a := testApp(t)

	// The note is on screen, and the shortcut for the list comes back to
	// it: the note keeps the keyboard, and the keystrokes would go into it.
	a.openNote("")
	if a.tab != 1 {
		t.Fatalf("the note shortcut left tab %d", a.tab)
	}
	a.openTasks()
	if a.tab != 0 {
		t.Fatalf("the list shortcut left tab %d", a.tab)
	}

	// The note shortcut asks for today's note, in writing.
	a.openTodayNote()
	if a.tab != 1 {
		t.Fatalf("the note shortcut left tab %d", a.tab)
	}
	if a.ns.day != today() {
		t.Fatalf("the note shortcut left day %q", a.ns.day)
	}
	if a.ns.reading {
		t.Fatal("the note shortcut left the note to be read")
	}
}

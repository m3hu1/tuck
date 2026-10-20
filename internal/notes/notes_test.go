package notes

import (
	"path/filepath"
	"testing"
	"time"
)

// testNotes is a store on a temporary path, without the app around it.
func testNotes(t *testing.T) *Notes {
	t.Helper()
	return NewNotes(filepath.Join(t.TempDir(), "notes.json"))
}

// today names the current local day, in the layout notes are stored in.
func today() string { return time.Now().Format(dayLayout) }

// TestNotesStore covers the day store: the rolling window, pinning, the
// marks the calendar draws, and what the footer counts.
func TestNotesStore(t *testing.T) {
	store := testNotes(t)
	day := today()

	// A note is written and read back.
	if _, err := store.Save(day, "Write it down"); err != nil {
		t.Fatal(err)
	}
	note, err := store.Get(day)
	if err != nil {
		t.Fatal(err)
	}
	if note.Text != "Write it down" || note.Pinned {
		t.Fatalf("the note reads back as %+v", note)
	}

	// Whitespace alone is no note: it removes the day.
	if _, err := store.Save(day, " \n "); err != nil {
		t.Fatal(err)
	}
	if note, _ := store.Get(day); HasText(note.Text) {
		t.Fatalf("a blank note remains: %+v", note)
	}

	// A day the window has left behind cannot be saved as a rolling note:
	// writing it keeps it permanently.
	old := time.Now().AddDate(0, 0, -45).Format(dayLayout)
	note, err = store.Save(old, "From the archive")
	if err != nil {
		t.Fatal(err)
	}
	if !note.Pinned {
		t.Fatal("an old note is not kept when it is written")
	}

	// Handing it back lets the window take it, at once: it is already
	// outside the window.
	if _, err := store.Pin(old, false); err != nil {
		t.Fatal(err)
	}
	if note, _ := store.Get(old); HasText(note.Text) {
		t.Fatalf("a released old note remains: %+v", note)
	}

	// A note inside the window is kept, and the footer counts it.
	if _, err := store.Save(day, "Buy milk"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pin(day, true); err != nil {
		t.Fatal(err)
	}
	info := store.Info()
	if info.Window != noteWindow || info.Days != 1 || info.Pinned != 1 {
		t.Fatalf("the footer counts %+v", info)
	}

	// The month's marks say which days hold a note.
	if _, err := store.Save(day, "Again"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	marks, err := store.Month(now.Year(), int(now.Month()))
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != now.AddDate(0, 1, -now.Day()).Day() {
		t.Fatalf("the month has %d marks", len(marks))
	}
	var todayMark DayMark
	for _, mark := range marks {
		if mark.Day == day {
			todayMark = mark
		}
	}
	if !todayMark.Has || todayMark.Archived {
		t.Fatalf("today's mark is %+v", todayMark)
	}

	// A month the window has entirely left behind reports its days as
	// archived.
	older := now.AddDate(0, -2, 0)
	marks, err = store.Month(older.Year(), int(older.Month()))
	if err != nil {
		t.Fatal(err)
	}
	for _, mark := range marks {
		if !mark.Archived {
			t.Fatalf("an old day is not archived: %+v", mark)
		}
	}

	// A year the store cannot name is refused.
	if _, err := store.Month(OldestYear-1, 1); err == nil {
		t.Fatal("a year before the store's oldest is accepted")
	}
}

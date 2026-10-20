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
	"tuck/internal/todos"
)

// testApp is the app with its stores on temporary paths, without the tray.
func testApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	a := &app{
		store: todos.NewStore(filepath.Join(dir, "todos.json")),
		notes: notes.NewNotes(filepath.Join(dir, "notes.json")),
	}
	a.initNotes()
	return a
}

// openNotesTab brings the notes tab on screen, so the tests do not repeat the
// click that does it.
func openNotesTab(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("Notes"); err != nil {
		t.Fatal(err)
	}
}

func TestTasksView(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)

	// The empty state, and the field takes the focus as the panel opens.
	if !tt.HasText("Nothing yet. Add your first task above.") {
		t.Fatalf("no empty state: %q", tt.Texts())
	}
	if !tt.Focused("New task") {
		t.Fatal("the add field is not focused")
	}

	// A task is typed and entered.
	tt.Type("Buy milk")
	tt.Key(0, ui.KeyEnter)
	if n := len(a.store.List()); n != 1 {
		t.Fatalf("the store holds %d todos", n)
	}
	if !tt.HasText("Buy milk") {
		t.Fatalf("the task is not on the panel: %q", tt.Texts())
	}
	if !tt.HasText("1 left") {
		t.Fatalf("the footer does not count it: %q", tt.Texts())
	}

	// Checking it strikes it through, and the footer notices.
	if err := tt.Click(`Mark "Buy milk" done`); err != nil {
		t.Fatal(err)
	}
	if !a.store.List()[0].Done {
		t.Fatal("the task is not done")
	}
	if !tt.HasText("All done") {
		t.Fatalf("the footer does not see it: %q", tt.Texts())
	}

	// Clear done is held, not clicked: a click does nothing.
	if err := tt.Click("Clear"); err != nil {
		t.Fatal(err)
	}
	if n := len(a.store.List()); n != 1 {
		t.Fatalf("a click cleared the list: %d todos", n)
	}

	// Holding it clears the list, back to the empty state.
	hold(t, tt, "Clear")
	if n := len(a.store.List()); n != 0 {
		t.Fatalf("the store holds %d todos", n)
	}
	if !tt.HasText("Nothing yet. Add your first task above.") {
		t.Fatalf("no empty state: %q", tt.Texts())
	}
}

// TestTasksViewClearsAgain covers the hold after it has done its work: the
// next day's done tasks can be cleared in turn.
func TestTasksViewClearsAgain(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 340, 470)

	for _, title := range []string{"Ship it", "Ship it again"} {
		todo, err := a.store.Add(title)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.store.Toggle(todo.ID); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		hold(t, tt, "Clear")
		if n := len(a.store.List()); n != 0 {
			t.Fatalf("the hold left %d todos", n)
		}
	}
}

// hold holds the button with the label down for long enough to complete the
// hold, and lets it go.
func hold(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("no %q on the panel", label)
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Advance(1500 * time.Millisecond)
	tt.Release(x, y)
	tt.Advance(50 * time.Millisecond)
}

func TestTasksViewRemoves(t *testing.T) {
	a := testApp(t)
	if _, err := a.store.Add("Ship it"); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 340, 470)
	if !tt.HasText("Ship it") {
		t.Fatalf("the task is not on the panel: %q", tt.Texts())
	}
	if err := tt.Click("Remove Ship it"); err != nil {
		t.Fatal(err)
	}
	if n := len(a.store.List()); n != 0 {
		t.Fatalf("the store holds %d todos", n)
	}
}

// TestRingSnapshot captures the ring mid-fill, when SNAP names the directory:
// it should be part way round, not stuck at a quarter. SNAP=/tmp go test -run Snapshot.
func TestRingSnapshot(t *testing.T) {
	dir := os.Getenv("SNAP")
	if dir == "" {
		t.Skip("set SNAP to the directory for the snapshots")
	}
	a := testApp(t)
	var last todos.Todo
	for i, title := range []string{"Buy milk", "Book the tickets", "Water the plants"} {
		todo, err := a.store.Add(title)
		if err != nil {
			t.Fatal(err)
		}
		last = todo
		if i < 2 {
			if err := a.store.Toggle(todo.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	tt := ui.NewTester(a.view, 340, 470)
	if err := tt.Click(fmt.Sprintf("Mark %q done", last.Title)); err != nil {
		t.Fatal(err)
	}
	for i, at := range []time.Duration{100 * time.Millisecond, 120 * time.Millisecond, 600 * time.Millisecond} {
		tt.Advance(at)
		path := filepath.Join(dir, fmt.Sprintf("tuck-ring-%d.png", i))
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
}

// TestTasksSnapshot writes the panel to PNGs for a look, when SNAP names the
// directory to write them in: SNAP=/tmp go test -run Snapshot.
func TestTasksSnapshot(t *testing.T) {
	dir := os.Getenv("SNAP")
	if dir == "" {
		t.Skip("set SNAP to the directory for the snapshots")
	}
	a := testApp(t)
	for _, title := range []string{"Buy milk", "Book the tickets", "Water the plants"} {
		todo, err := a.store.Add(title)
		if err != nil {
			t.Fatal(err)
		}
		if title != "Water the plants" {
			if err := a.store.Toggle(todo.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	tt := ui.NewTester(a.view, 340, 470)
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Advance(500 * time.Millisecond) // let the colors settle
		tt.Frame()
		name := "light"
		if dark {
			name = "dark"
		}
		path := filepath.Join(dir, "tuck-"+name+".png")
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
}

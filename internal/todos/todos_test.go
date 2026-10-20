package todos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testStore is a store on a temporary path, without the app around it.
func testStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "todos.json"))
}

func TestAddToggleClear(t *testing.T) {
	s := testStore(t)
	a, err := s.Add("Buy milk")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Add("Ship it")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID >= b.ID {
		t.Fatalf("ids go %d then %d", a.ID, b.ID)
	}
	if err := s.Toggle(a.ID); err != nil {
		t.Fatal(err)
	}
	if st := s.Stats(); st.Total != 2 || st.Done != 1 || st.Active != 1 {
		t.Fatalf("stats %+v", st)
	}
	n, err := s.ClearDone()
	if err != nil || n != 1 {
		t.Fatalf("clear removed %d, %v", n, err)
	}
	got := s.List()
	if len(got) != 1 || got[0].ID != b.ID || got[0].Done {
		t.Fatalf("list %+v", got)
	}
	if err := s.Remove(b.ID); err != nil {
		t.Fatal(err)
	}
	if l := s.List(); len(l) != 0 {
		t.Fatalf("list %+v", l)
	}
	if err := s.Toggle(b.ID); err == nil {
		t.Fatal("toggling a removed todo should fail")
	}
}

func TestAddNeedsATitle(t *testing.T) {
	s := testStore(t)
	for _, blank := range []string{"", " ", "  \t\n"} {
		if _, err := s.Add(blank); err == nil {
			t.Fatalf("%q should not be a title", blank)
		}
	}
	if _, err := s.Add("  keep  "); err != nil {
		t.Fatal(err)
	}
	if got := s.List()[0].Title; got != "keep" {
		t.Fatalf("title %q, want it trimmed", got)
	}
}

func TestSaveKeepsTheDay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.json")
	s := NewStore(path)
	if _, err := s.Add("one"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sv saved
	if err := json.Unmarshal(data, &sv); err != nil {
		t.Fatal(err)
	}
	if sv.Day != today() || len(sv.Items) != 1 || sv.Items[0].Title != "one" {
		t.Fatalf("saved %+v", sv)
	}
}

func TestResetIfNewDay(t *testing.T) {
	s := testStore(t)
	if _, err := s.Add("one"); err != nil {
		t.Fatal(err)
	}
	// The same day changes nothing.
	s.ResetIfNewDay(time.Now())
	if len(s.List()) != 1 {
		t.Fatal("the same day cleared the list")
	}
	// The next day starts over.
	tomorrow := time.Now().AddDate(0, 0, 1)
	s.ResetIfNewDay(tomorrow)
	if got := s.List(); len(got) != 0 {
		t.Fatalf("list %+v", got)
	}
	if got := s.Stats().Day; got != todayIn(tomorrow) {
		t.Fatalf("day %q, want %q", got, todayIn(tomorrow))
	}
	// And a new todo gets its ids from the start again.
	if todo, err := s.Add("two"); err != nil || todo.ID != 1 {
		t.Fatalf("todo %+v, %v", todo, err)
	}
}

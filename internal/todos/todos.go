// Package todos is the day's list: its items, persistence, and the midnight
// reset that keeps the list a daily one.
package todos

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// dayLayout is how a local day is written down.
const dayLayout = "2006-01-02"

// Todo is one item of the day's list.
type Todo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// Stats summarizes the day: the header's ring, the footer's counters and
// countdown, and the empty state.
type Stats struct {
	Day      string
	Total    int
	Done     int
	Active   int
	ResetsAt int64
}

// Store is the list for the current local day. It is stored as JSON in the
// app's user data directory and starts empty again after midnight.
type Store struct {
	mu    sync.Mutex
	path  string
	day   string
	items []Todo
	next  int64
	rev   int64

	// saveMu holds the write to the file, so that two saves — the panel's
	// and the midnight reset's — cannot interleave and leave the file
	// half written.
	saveMu sync.Mutex
}

// saved is the on-disk shape.
type saved struct {
	Day   string `json:"day"`
	Items []Todo `json:"items"`
}

// OpenStore loads the saved list. A list written on an earlier day is
// yesterday's and is discarded, so the app always opens on a fresh day.
func OpenStore() (*Store, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	s := NewStore(filepath.Join(dir, "todos.json"))
	data, err := os.ReadFile(s.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(data) > 0 {
		var sv saved
		if err := json.Unmarshal(data, &sv); err != nil {
			return nil, err
		}
		s.day = sv.Day
		s.items = sv.Items
	}
	for _, it := range s.items {
		s.next = max(s.next, it.ID)
	}
	if s.day != today() {
		s.day = today()
		s.items = nil
		s.next = 0
		if err := s.save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// NewStore is a store on a path, without the app around it: what the tests
// build, where OpenStore is the app's own on its user data path.
func NewStore(path string) *Store {
	return &Store{path: path, day: today()}
}

// List returns the day's todos in the order they were added.
func (s *Store) List() []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Todo, len(s.items))
	copy(out, s.items)
	return out
}

// Revision counts the changes to the list, so that a view reading it on every
// frame can tell when the copy it holds has gone stale: it moves whenever
// the list does, and stays where it is while the list does not.
func (s *Store) Revision() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rev
}

// Add appends a todo and returns it.
func (s *Store) Add(title string) (Todo, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Todo{}, errors.New("a todo needs a title")
	}
	s.mu.Lock()
	s.next++
	todo := Todo{ID: s.next, Title: title}
	s.items = append(s.items, todo)
	s.rev++
	s.mu.Unlock()
	return todo, s.save()
}

// Toggle flips a todo between done and not done.
func (s *Store) Toggle(id int64) error {
	return s.update(func() error {
		i := s.index(id)
		if i < 0 {
			return errors.New("todo not found")
		}
		s.items[i].Done = !s.items[i].Done
		return nil
	})
}

// Remove deletes a todo.
func (s *Store) Remove(id int64) error {
	return s.update(func() error {
		i := s.index(id)
		if i < 0 {
			return errors.New("todo not found")
		}
		s.items = slices.Delete(s.items, i, i+1)
		return nil
	})
}

// ClearDone removes completed todos and returns how many were removed.
func (s *Store) ClearDone() (int, error) {
	n := 0
	err := s.update(func() error {
		before := len(s.items)
		s.items = slices.DeleteFunc(s.items, func(it Todo) bool { return it.Done })
		n = before - len(s.items)
		return nil
	})
	return n, err
}

// Stats returns the counters for the header's ring and the footer.
func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats()
}

// ResetIfNewDay clears the list when the local date has moved on, which is
// what "resets daily at midnight" means for the list.
func (s *Store) ResetIfNewDay(now time.Time) {
	s.mu.Lock()
	if s.day == todayIn(now) {
		s.mu.Unlock()
		return
	}
	s.day = todayIn(now)
	s.items = nil
	s.next = 0
	s.rev++
	s.mu.Unlock()
	_ = s.save()
}

func (s *Store) index(id int64) int {
	return slices.IndexFunc(s.items, func(it Todo) bool { return it.ID == id })
}

func (s *Store) stats() Stats {
	st := Stats{Day: s.day, Total: len(s.items), ResetsAt: NextMidnight(time.Now()).UnixMilli()}
	for _, it := range s.items {
		if it.Done {
			st.Done++
		} else {
			st.Active++
		}
	}
	return st
}

func (s *Store) update(fn func() error) error {
	s.mu.Lock()
	err := fn()
	if err == nil {
		s.rev++
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.save()
}

// save writes the list to disk.
func (s *Store) save() error {
	// The write holds its own lock, so that two of them do not interleave:
	// the file would be left half written, and the app would not start.
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	data, err := json.MarshalIndent(saved{Day: s.day, Items: s.items}, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	// A crash mid-write must leave the old file, not half a new one.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func today() string { return todayIn(time.Now()) }

func todayIn(now time.Time) string { return now.Format(dayLayout) }

// NextMidnight is the next local midnight strictly after now, which is what
// the app sleeps until to reset the list and redraw the panel.
func NextMidnight(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

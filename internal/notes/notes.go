// Package notes is the day's notes: a rolling window of markdown notes, the
// pins that keep one forever, and the marks the calendar draws.
package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// noteWindow is how many days of notes are kept, today included. Notes older
// than that are dropped on launch and after every save, unless they were kept
// permanently.
const noteWindow = 30

// dayLayout is the format days are named in, everywhere.
const dayLayout = "2006-01-02"

// OldestYear is as far back as the store can name a month, and as far back
// as the calendar walks.
const OldestYear = 1970

// HasText tells an empty note from one holding words: spaces and newlines
// alone are no note, so they never enable Save, Keep, Delete, or a calendar
// dot.
func HasText(s string) bool { return strings.TrimSpace(s) != "" }

// Note is the text of one local day. Days without a note are not stored at
// all, so the file only ever holds what someone actually wrote.
type Note struct {
	Text   string `json:"text"`
	Pinned bool   `json:"pinned"`
}

// DayMark is everything the calendar draws for one day, so it can colour a
// month without fetching every note it holds.
type DayMark struct {
	Day string
	// Has is true when the day has note text.
	Has bool
	// Pinned is true when the note is kept permanently.
	Pinned bool
	// Archived is true for a past day the window no longer holds and that
	// was never kept permanently.
	Archived bool
}

// NotesInfo is what the notes footer reports.
type NotesInfo struct {
	Days   int
	Pinned int
	Window int
}

// Notes stores one note per local day, in the app's user data directory. It
// keeps the last noteWindow days and drops the rest; a pinned note is kept
// until it is released.
type Notes struct {
	mu   sync.Mutex
	path string
	days map[string]Note
	rev  int64

	// saveMu holds the write to the file, so that two saves — the panel's
	// and the midnight reset's — cannot interleave and leave the file
	// half written.
	saveMu sync.Mutex
}

// savedNotes is the on-disk shape.
type savedNotes struct {
	Days map[string]Note `json:"days"`
}

// OpenNotes loads the saved notes and drops everything the rolling window no
// longer holds.
func OpenNotes() (*Notes, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	n := NewNotes(filepath.Join(dir, "notes.json"))
	data, err := os.ReadFile(n.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(data) > 0 {
		var s savedNotes
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		for day, note := range s.Days {
			// An empty note is no note: drop it rather than keeping it
			// around as an empty day the calendar would mark. Whitespace
			// alone is empty too, so space or newline spam never becomes a
			// note.
			if _, err := time.ParseInLocation(dayLayout, day, time.Local); err == nil && HasText(note.Text) {
				n.days[day] = note
			}
		}
	}
	if n.prune(time.Now()) {
		if err := n.saveFile(); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// NewNotes is a store on a path, without the app around it: what the tests
// build, where OpenNotes is the app's own on its user data path.
func NewNotes(path string) *Notes {
	return &Notes{path: path, days: map[string]Note{}}
}

// Get returns the note of a day. A day without one comes back empty.
func (n *Notes) Get(day string) (Note, error) {
	if err := validDay(day); err != nil {
		return Note{}, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.days[day], nil
}

// Revision counts the changes to the notes, so that a view reading them on
// every frame can tell when the copy it holds has gone stale: it moves
// whenever the notes do, and stays where it is while they do not.
func (n *Notes) Revision() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.rev
}

// Save writes a day's note, which the panel does as the user types. Empty
// text removes the day, kept permanently or not, and comes back as an empty
// note. Text without a word in it — only spaces or newlines — counts as
// empty, so it removes the day rather than saving a blank note. A note on a
// day the window has already moved past is kept permanently: it cannot be
// held in the rolling window, and writing it is the point.
func (n *Notes) Save(day, text string) (Note, error) {
	if err := validDay(day); err != nil {
		return Note{}, err
	}
	now := time.Now()
	n.mu.Lock()
	note := n.days[day]
	if !HasText(text) {
		delete(n.days, day)
		note = Note{}
	} else {
		note.Text = text
		note.Pinned = note.Pinned || day < windowStart(now).Format(dayLayout)
		n.days[day] = note
	}
	n.pruneLocked(now)
	n.rev++
	n.mu.Unlock()
	if err := n.saveFile(); err != nil {
		return Note{}, err
	}
	return note, nil
}

// Pin keeps a day's note forever. Releasing it hands the note back to the
// rolling window, which removes it at once when it already sits outside it.
func (n *Notes) Pin(day string, pinned bool) (Note, error) {
	if err := validDay(day); err != nil {
		return Note{}, err
	}
	now := time.Now()
	n.mu.Lock()
	note, ok := n.days[day]
	if !ok {
		n.mu.Unlock()
		return Note{}, errors.New("there is no note on that day")
	}
	if !pinned && day < windowStart(now).Format(dayLayout) {
		delete(n.days, day)
		note = Note{}
	} else {
		note.Pinned = pinned
		n.days[day] = note
	}
	n.pruneLocked(now)
	n.rev++
	n.mu.Unlock()
	if err := n.saveFile(); err != nil {
		return Note{}, err
	}
	return note, nil
}

// Month returns a mark for every day of a month, which is all the calendar
// needs to colour them.
func (n *Notes) Month(year, month int) ([]DayMark, error) {
	if year < OldestYear || year > 9999 || month < 1 || month > 12 {
		return nil, fmt.Errorf("%04d-%02d is not a month", year, month)
	}
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	now := time.Now()
	start := windowStart(now).Format(dayLayout)
	n.mu.Lock()
	defer n.mu.Unlock()
	marks := make([]DayMark, first.AddDate(0, 1, -1).Day())
	for i := range marks {
		day := first.AddDate(0, 0, i).Format(dayLayout)
		note, ok := n.days[day]
		marks[i] = DayMark{
			Day:      day,
			Has:      ok,
			Pinned:   ok && note.Pinned,
			Archived: day < start && !(ok && note.Pinned),
		}
	}
	return marks, nil
}

// Info reports how many days are stored, how many of those are kept
// permanently, and how long the rolling window is.
func (n *Notes) Info() NotesInfo {
	n.mu.Lock()
	defer n.mu.Unlock()
	info := NotesInfo{Window: noteWindow}
	for _, note := range n.days {
		info.Days++
		if note.Pinned {
			info.Pinned++
		}
	}
	return info
}

// prune drops the notes the window no longer holds, pinned ones except, and
// reports whether anything went.
func (n *Notes) prune(now time.Time) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.pruneLocked(now)
}

func (n *Notes) pruneLocked(now time.Time) bool {
	start := windowStart(now).Format(dayLayout)
	removed := false
	for day, note := range n.days {
		if note.Pinned || day >= start {
			continue
		}
		delete(n.days, day)
		removed = true
	}
	return removed
}

// saveFile writes the notes as JSON, holding the lock only while reading
// them.
func (n *Notes) saveFile() error {
	// The write holds its own lock, so that two of them do not interleave:
	// the file would be left half written, and the app would not start.
	n.saveMu.Lock()
	defer n.saveMu.Unlock()
	n.mu.Lock()
	data, err := json.MarshalIndent(savedNotes{Days: n.days}, "", "  ")
	n.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(n.path, data, 0o644)
}

// windowStart is the oldest day the rolling window still keeps, today
// included.
func windowStart(now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return day.AddDate(0, 0, -(noteWindow - 1))
}

// validDay rejects anything that is not a local calendar day, so the map keys
// stay comparable as plain strings.
func validDay(day string) error {
	if _, err := time.ParseInLocation(dayLayout, day, time.Local); err != nil {
		return fmt.Errorf("%q is not a day, want YYYY-MM-DD", day)
	}
	return nil
}

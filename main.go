// Tuck is a menu bar app for the day: a list that resets at midnight and one
// markdown note per day. The panel is drawn by MyGo itself, with no webview.
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"tuck/internal/notes"
	"tuck/internal/todos"
)

// dayLayout is how a local day is written down, the layout both stores use.
const dayLayout = "2006-01-02"

// app holds the menu bar state. Everything here is touched on the main
// thread only, by tray callbacks and the view.
type app struct {
	store  *todos.Store
	tray   *mygo.Tray
	panel  *mygo.Window
	hidden time.Time // when the panel went away, to ignore the dismissing click

	// The panel is built once and shown and hidden after that, so its
	// state lives here, and not in the view.
	draft string // what the add field holds
	err   string // the last failure, shown above the list
	list  ui.ListState

	// The stores are read on every frame, and what they hold is kept here
	// instead: a frame that changes nothing copies nothing.
	itemCache cached[[]todos.Todo]
	statCache cached[todos.Stats]
	infoCache cached[notes.NotesInfo]
	markCache cached[[]notes.DayMark]

	// The notes tab: its store, the tab on screen, and the day's note with
	// its editor, which survives the panel closing.
	notes      *notes.Notes
	tab        int
	focusInput bool
	md         markdown.Options
	ns         noteState
}

// cached holds a value read from a store, and the revision it was read at.
// A view reads the stores on every frame; this keeps the copy it holds
// until the store moves on.
type cached[T any] struct {
	value T
	rev   int64
	key   string
	ok    bool
}

// get returns the value, reading it afresh when the store has moved on, or
// when the key it was read under has changed.
func (c *cached[T]) get(rev int64, key string, read func() T) T {
	if !c.ok || c.rev != rev || c.key != key {
		c.value = read()
		c.rev = rev
		c.key = key
		c.ok = true
	}
	return c.value
}

// items is the day's list, as the store holds it.
func (a *app) items() []todos.Todo {
	return a.itemCache.get(a.store.Revision(), "", a.store.List)
}

// stats is the day's counters, on the same terms as the list. The time of
// the reset is not among the counters the store holds: it moves with the
// clock, not with the list, so it is read afresh every time.
func (a *app) stats() todos.Stats {
	st := a.statCache.get(a.store.Revision(), "", a.store.Stats)
	st.ResetsAt = todos.NextMidnight(time.Now()).UnixMilli()
	return st
}

// notesInfo is what the notes footer reports.
func (a *app) notesInfo() notes.NotesInfo {
	return a.infoCache.get(a.notes.Revision(), "", a.notes.Info)
}

// monthMarks is the calendar's marks for a month, read only when the month,
// the notes or the day have moved. The day is part of the key because a day
// the window no longer holds is archived, and the window moves at midnight.
func (a *app) monthMarks(at time.Time) []notes.DayMark {
	key := at.Format("2006-01") + "|" + today()
	rev := a.notes.Revision()
	if a.markCache.ok && a.markCache.rev == rev && a.markCache.key == key {
		return a.markCache.value
	}
	marks, err := a.notes.Month(at.Year(), int(at.Month()))
	if err != nil {
		return nil
	}
	return a.markCache.get(rev, key, func() []notes.DayMark { return marks })
}

// visible reports whether the panel is on screen. The timers ask it before
// waking the app for a panel nobody is looking at.
func (a *app) visible() bool {
	return a.panel != nil && a.panel.IsVisible()
}

// toggle opens the panel next to the icon, or closes it. The panel is built
// once and shown and hidden after that, so opening it again is instant; the
// system takes it away altogether when the user clicks outside, and the next
// open builds a new one.
func (a *app) toggle() {
	if time.Since(a.hidden) < 150*time.Millisecond {
		return // the same click that dismissed the panel
	}
	if a.panel != nil && a.panel.IsVisible() {
		a.panel.Hide()
		return
	}
	go mygo.RunOnMain(a.show)
}

// show brings the panel up on the view it is left on. It runs on the main
// thread.
func (a *app) show() {
	if a.tray == nil {
		return // there is no icon to show the panel by
	}
	if a.panel == nil {
		a.panel = mygo.NewFlyout(mygo.FlyoutOptions{
			Tray:      a.tray,
			Placement: mygo.PlacementBottom,
			Gap:       6,
			Width:     340,
			Height:    470,
			Focusable: true,
			Hidden:    true,
			Shadow:    true,
			// A plain flyout appears at once, without the popover arrow.
			Popover: false,
			Content: ui.View(a.view),
		})
		a.panel.OnHide(a.panelGone)
		a.panel.OnClosed(a.panelClosed)
	}
	if a.panel.IsVisible() {
		return
	}
	// A day the panel was left on may have gone by while it was away;
	// opening it again comes back to today.
	a.notesToday()
	a.panel.Show()
	a.panel.Focus()
}

// panelGone is called when the panel is hidden, which the app does when the
// icon is clicked again: the writing is saved and read back, and the picker,
// which the panel was left on, is closed with it. The panel itself lives on,
// and comes back the next time the icon is clicked.
func (a *app) panelGone() {
	a.settleNote()
	a.ns.calOpen = false
}

// panelClosed is called when the panel is taken away for good, as the
// system takes it when the user clicks outside it. That is the one click
// that can be both a dismissal and a request — on the platforms whose
// flyouts close on a press outside them, the press on the tray icon closes
// the panel and asks for it at once — so it is the one the next click is
// held back from.
func (a *app) panelClosed() {
	a.panelGone()
	a.hidden = time.Now()
	a.panel = nil
}

// openTasks brings the panel up on the day's list.
func (a *app) openTasks() {
	a.tab = 0
	a.show()
}

// openTodayNote brings the panel up on today's note, in writing, ready to
// type or paste into. The note is put on screen before the panel comes up,
// so the panel opens on it.
func (a *app) openTodayNote() {
	a.openNote("")
	a.show()
}

// ready builds the tray icon once the application is ready.
func (a *app) ready() {
	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon(),
		IconIsTemplate: true,
		ToolTip:        "Tuck",
	})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Open Tuck", Click: func(*mygo.MenuItem, *mygo.Window) { a.toggle() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	if runtime.GOOS == "linux" {
		// AppIndicator reports neither clicks nor bounds: open from the menu.
		a.tray.SetMenu(menu)
	} else {
		a.tray.OnClick(a.toggle)
		a.tray.OnRightClick(func() {
			if a.panel != nil && a.panel.IsVisible() {
				a.panel.Hide()
			}
			a.tray.SetMenu(menu)
			defer a.tray.SetMenu(nil)
			a.tray.PopUpMenu()
		})
	}
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.tray.Destroy()
		mygo.GlobalShortcut.UnregisterAll()
	})

	// Hyper-key shortcuts: T for the day's list, N for today's note, ready
	// to type.
	if err := mygo.GlobalShortcut.Register("Cmd+Ctrl+Alt+Shift+T", func() {
		go mygo.RunOnMain(a.openTasks)
	}); err != nil {
		log.Println("shortcut: ", err)
	}
	if err := mygo.GlobalShortcut.Register("Cmd+Ctrl+Alt+Shift+N", func() {
		go mygo.RunOnMain(a.openTodayNote)
	}); err != nil {
		log.Println("shortcut: ", err)
	}
}

// watchMidnight resets the list and opens the new day for an untouched note
// as each local midnight passes, waking only once per day, and asks the panel
// to redraw.
func (a *app) watchMidnight() {
	for {
		time.Sleep(time.Until(todos.NextMidnight(time.Now())) + time.Second)
		a.store.ResetIfNewDay(time.Now())
		mygo.RunOnMain(func() {
			a.notesToday()
			if a.visible() {
				a.panel.Invalidate()
			}
		})
	}
}

// addDraft adds what the field holds and empties it.
func (a *app) addDraft() {
	if strings.TrimSpace(a.draft) == "" {
		return
	}
	if _, err := a.store.Add(a.draft); err != nil {
		a.err = err.Error()
		return
	}
	a.draft = ""
	a.err = ""
}

func (a *app) toggleTodo(id int64) {
	if err := a.store.Toggle(id); err != nil {
		a.err = err.Error()
	}
}

func (a *app) removeTodo(id int64) {
	if err := a.store.Remove(id); err != nil {
		a.err = err.Error()
	}
}

func (a *app) clearDone() {
	if _, err := a.store.ClearDone(); err != nil {
		a.err = err.Error()
	}
}

// trayIcon draws a check mark, black on transparent so macOS tints it to
// match the menu bar.
func trayIcon() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	ink := color.NRGBA{A: 255}
	if runtime.GOOS != "darwin" {
		ink = color.NRGBA{R: 10, G: 122, B: 255, A: 255}
	}
	// The two strokes of the check, with round caps and joins.
	a := [2]float64{8, 16.5}
	b := [2]float64{13.5, 22}
	c := [2]float64{24, 10}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := [2]float64{float64(x) + 0.5, float64(y) + 0.5}
			if distToSegment(p, a, b) < 2.4 || distToSegment(p, b, c) < 2.4 {
				img.Set(x, y, ink)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// distToSegment is the distance from p to the segment a-b.
func distToSegment(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	len2 := dx*dx + dy*dy
	t := 0.0
	if len2 > 0 {
		t = ((p[0]-a[0])*dx + (p[1]-a[1])*dy) / len2
		t = math.Max(0, math.Min(1, t))
	}
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dy))
}

// today names the current local day, in the layout the stores use.
func today() string { return time.Now().Format(dayLayout) }

func main() {
	mygo.App.SetName("Tuck")
	// No Dock icon: this is a menu bar app.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	// The panel is the app's only window: keep running as it hides.
	mygo.App.OnWindowAllClosed(func() {})

	// SetName first: it determines the user data directory.
	todosStore, err := todos.OpenStore()
	if err != nil {
		log.Fatal(err)
	}
	notesStore, err := notes.OpenNotes()
	if err != nil {
		log.Fatal(err)
	}

	a := &app{store: todosStore, notes: notesStore}
	a.initNotes()
	go a.watchMidnight()
	mygo.App.WhenReady(a.ready)

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

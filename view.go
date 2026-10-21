// The tasks view: today's list, the progress ring, the add row and the
// footer, all drawn by the ui package.
package main

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"tuck/internal/todos"
)

// tabsThumbDuration is how long the tab strip's thumb takes to slide to the
// chosen tab.
const tabsThumbDuration = 180 * time.Millisecond

// easeOutQuint is the ring's timing, cubic-bezier(0.22, 1, 0.36, 1).
func easeOutQuint(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u*u*u
}

// iconScale maps the design's icons, drawn in a 24-unit box, onto an element of
// size px: the scale and the centred margin the box takes in it.
func iconScale(size, px float32) (scale, off float32) {
	q := size / 24
	return q, (px - size) / 2
}

// view draws the panel. It reads the store every build, so a change shows on
// the next frame.
func (a *app) view(c *ui.Context) {
	// The midnight timer can wake late after a sleep or a clock change; the
	// first frame puts the list on the right day before anything reads it.
	a.store.ResetIfNewDay(time.Now())
	// The panel's colors are Tuck's tokens: the accent blue among them.
	c.SetTheme(panelTheme(c.Theme()))
	t := c.Theme()
	a.autosave(c)
	c.Root().Background(ui.Transparent)
	panel := ui.Column(c).Fill()
	switch {
	case c.Vibrancy(): // over the popover's material on macOS
	case runtime.GOOS == "darwin": // whose shadow outlines the flyout
		panel.Background(t.Background).Radius(10)
	case runtime.GOOS == "windows": // which rounds and outlines the flyout
		panel.Background(t.Background)
	default:
		panel.Background(t.Background).Radius(10).Border(1, t.Border)
	}

	items := a.items()
	stats := a.stats()

	panel.Padding(18, 14, 10, 14).Children(func() {
		a.tabs(c, t)
		if a.tab == 0 {
			a.tasksView(c, t, items, stats)
		} else {
			a.notesView(c, t)
		}
	})

	if a.tab == 1 && panel.Shortcut(ui.Cmd, ui.KeyS) {
		a.saveNote()
	}

	// The footer counts down to the reset: wake as its minute turns. A panel
	// that is not on screen has nothing to count down to, and a timer that
	// kept waking the app for a hidden panel would spend the app's idle
	// minutes on a panel nobody is looking at.
	if a.tab == 0 && a.visible() {
		c.After(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)) + 40*time.Millisecond)
	}
}

// tabs is the panel's row of tabs: Tasks and Notes, each with its icon, in a
// segmented track whose thumb slides to the chosen one. The two tabs share
// the track between them, as the design's do.
func (a *app) tabs(c *ui.Context, t *ui.Theme) {
	parts := ui.SegmentedBase(c, &a.tab, 2)
	// The track is the text's own color at a low share, so that it recesses
	// whatever it sits on, and the thumb is the raised face above it.
	track := t.Text.Alpha(0.06)
	face := t.Surface
	shadow := ui.RGBA(0, 0, 0, 0.14)
	if t.Dark {
		shadow = ui.RGBA(0, 0, 0, 0.4)
	}
	parts.Track.Padding(3).Gap(2).Radius(10).Background(track).Shrink(0).Margin(0, 0, 10, 0)

	var active ui.Element
	parts.Track.Children(func() {
		for i, tab := range []struct {
			label string
			icon  *ui.SVG
		}{{"Tasks", iconListTodo}, {"Notes", iconPencil}} {
			seg := parts.Segment(i).Grow(1).Gap(6).Padding(5, 0).Radius(8).
				Cursor(ui.CursorPointer)
			on := i == a.tab
			fg := t.Text
			if !on {
				fg = fg.Alpha(0.6)
			}
			seg.TextColor(fg)
			if on {
				active = seg
			}
			seg.Children(func() {
				ui.Icon(c, tab.icon).Size(14, 14)
				ui.Text(c, tab.label).FontSize(12.5).FontWeight(550)
			})
		}
	})
	if parts.Track.Changed() {
		// The picker belongs to the note's tab: it goes with it.
		a.ns.calOpen = false
		if a.tab == 0 {
			a.focusInput = true
		} else {
			a.ns.focus = true
		}
	}
	if !active.Valid() {
		return
	}
	// The thumb slides to the chosen tab: it reads the layout as it is
	// painted, and animates the geometry relative to the track, so moving
	// the whole control does not move the thumb.
	parts.Track.Draw(func(p *ui.Painter, r ui.Rect) {
		bounds := active.LayoutBounds()
		x := parts.Track.Animate("thumb-x", bounds.X-r.X, tabsThumbDuration)
		w := parts.Track.Animate("thumb-width", bounds.W, tabsThumbDuration)
		if c.Preferences().ReduceMotion {
			x, w = bounds.X-r.X, bounds.W
		}
		thumb := ui.Rect{X: r.X + x, Y: r.Y, W: w, H: r.H}
		radius := thumb.H / 2
		p.Shadow(thumb, radius, 0, 1, 2, 0, shadow)
		p.Fill(thumb, face, radius)
	})
}

// tasksView is the day's list: the header, the add row, the list and the
// footer.
func (a *app) tasksView(c *ui.Context, t *ui.Theme, items []todos.Todo, stats todos.Stats) {
	a.header(c, t, stats)
	a.addRow(c, t)
	a.lastError(c, t)
	a.rows(c, t, items)
	a.footer(c, t, stats)
}

// header is "Today", the date, and the ring of how much is done.
func (a *app) header(c *ui.Context, t *ui.Theme, stats todos.Stats) {
	ui.Row(c).AlignItems(ui.Center).Gap(12).Padding(0, 4, 12, 4).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "Today").FontSize(20).FontWeight(650).LetterSpacing(-0.4)
			ui.Text(c, dayLabel(stats.Day)).FontSize(12).TextColor(t.TextMuted)
		})
		a.ring(c, t, stats)
	})
}

// ring is the progress circle: the share of the day done, with the count in
// its middle. It fills as the design's does.
func (a *app) ring(c *ui.Context, t *ui.Theme, stats todos.Stats) {
	target := float32(0)
	if stats.Total > 0 {
		target = float32(stats.Done) / float32(stats.Total)
	}
	ring := ui.Column(c.Key("ring")).Size(40, 40).Center().Label("Progress").Role(ui.RoleProgress)
	pct := ring.AnimateTo("pct", target, ui.Animation{Duration: 500 * time.Millisecond, Ease: easeOutQuint})
	ring.Draw(func(p *ui.Painter, r ui.Rect) {
		// The page's ring: circle r 15.5 and stroke 3.4 in a 36-unit box.
		s := r.W / 36
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		rad, width := 15.5*s, 3.4*s
		p.StrokePath((&ui.Path{}).Circle(cx, cy, rad), width, t.Border)
		if pct > 0 {
			// A path starts at 3 o'clock and trim stops at its end, so the
			// fill runs from 12 o'clock in two strokes: the quarter to
			// 3 o'clock, then from 3 o'clock on.
			quarter := min(pct, 0.25)
			p.StrokePath((&ui.Path{}).Circle(cx, cy, rad).Trim(0.75, 0.75+quarter), width, accent)
			if pct > 0.25 {
				p.StrokePath((&ui.Path{}).Circle(cx, cy, rad).Trim(0, pct-0.25), width, accent)
			}
		}
	})
	// The count sits in the middle, as the design's grid places it: the
	// layout centers the digits' line box, so no guessed cap height nudges
	// them off the ring's center.
	ring.Children(func() {
		ui.Text(c, strconv.Itoa(stats.Done)).FontSize(10.5).FontWeight(600).TextColor(t.TextMuted)
	})
}

// addRow is the field for a new task and the button that adds it.
func (a *app) addRow(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Padding(0, 0, 8, 0).AlignItems(ui.Center).Children(func() {
		// The field is the design's: a fill over the panel, a line around it,
		// and a blue border and a soft blue band when it has the keyboard.
		input := ui.TextInputBase(c.Key("title"), &a.draft).
			Placeholder("Add a task…").Label("New task").
			Height(39).Radius(10).Grow(1).AutoFocus().
			Padding(9, 12).FontSize(14).FocusRing(false).
			Background(fillColor(t)).Border(1, t.Border)
		if input.Focused() {
			input.Border(1, accent.Alpha(0.55))
			input.DrawOver(func(p *ui.Painter, r ui.Rect) {
				// The page's focus ring: a soft band around the field.
				p.Stroke(ui.Rect{X: r.X - 1.5, Y: r.Y - 1.5, W: r.W + 3, H: r.H + 3},
					accent.Alpha(0.14), t.Radius+1.5, 3)
			})
		}
		if a.focusInput {
			input.Focus()
			a.focusInput = false
		}
		if input.Submitted() {
			a.addDraft()
		}
		add := ui.ButtonBase(c.Key("add")).Label("Add task").Tooltip("Add task").
			Size(37, 39).Radius(10).Cursor(ui.CursorPointer)
		bg := accent
		if add.Hovered() {
			bg = accentHover
		}
		add.Background(bg).TextColor(t.AccentText)
		add.Children(func() {
			ui.Icon(c, iconPlus).Size(18, 18)
		})
		if add.Clicked() {
			a.addDraft()
		}
	})
}

// lastError shows what went wrong with the last action.
func (a *app) lastError(c *ui.Context, t *ui.Theme) {
	if a.err == "" {
		return
	}
	ui.Text(c, a.err).FontSize(12).TextColor(t.Danger).Padding(0, 4, 6, 4)
}

// rows is the day's list: a row per task, or the empty state. With no
// tasks there is no list to scroll, so nothing takes its room: a column
// grown to the list's room centers the hint in it, along and across, as
// the design's empty state sits in the room the list would take.
func (a *app) rows(c *ui.Context, t *ui.Theme, items []todos.Todo) {
	if len(items) == 0 {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "Nothing yet. Add your first task above.").
				FontSize(13).TextColor(t.TextMuted).TextAlign(ui.Center).MaxLines(2)
		})
		return
	}
	a.list.Key = func(i int) any { return items[i].ID }
	a.list.Label = func(i int) string { return items[i].Title }
	ui.List(c, &a.list, len(items), func(i int) {
		a.itemRow(c, t, items[i])
	}).Grow(1).Label("Tasks").Role(ui.RoleList)
}

// itemRow is one task: the check, the title, and the remove button that
// shows as the row is hovered.
func (a *app) itemRow(c *ui.Context, t *ui.Theme, item todos.Todo) {
	row := ui.Row(c.Key(item.ID)).AlignItems(ui.Center).Gap(10).Padding(9, 8).Radius(9)
	if row.Hovered() {
		row.Background(fillColor(t))
	}
	row.Children(func() {
		a.check(c, t, item)
		a.title(c, t, item)
		a.removeButton(c, t, row, item)
	})
}

// title is a task's words, with the strikethrough that sweeps across them
// from the left as the task is marked done.
func (a *app) title(c *ui.Context, t *ui.Theme, item todos.Todo) {
	inkAt, strikeAt := t.Text, float32(0)
	if item.Done {
		inkAt, strikeAt = t.TextMuted, 1
	}
	title := ui.Text(c, item.Title).FontSize(14).Grow(1).SingleLine()
	ink := title.AnimateColor("ink", inkAt, ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseInOut})
	strike := title.AnimateTo("strike", strikeAt, ui.Animation{Duration: 320 * time.Millisecond, Ease: ui.EaseInOut})
	title.TextColor(ink)
	title.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if strike <= 0.002 {
			return
		}
		// The page's line: only as long as the words, 0.63em down, 1.4 px.
		span := ui.Span{Text: item.Title, Size: 14, Weight: 400}
		w, _ := p.MeasureText(0, span)
		w = min(w, r.W)
		y := r.Y + 14*0.63
		line := (&ui.Path{}).MoveTo(r.X, y).LineTo(r.X+w, y).Trim(0, strike)
		p.StrokePath(line, 1.4, ink.Alpha(0.7))
	})
}

// check is the circle that toggles a task: it fills with the accent, and a
// tick draws itself in. The tick is drawn by hand rather than taken from the
// icon set, because it draws itself: the stroke grows along its own path, as
// the design's does.
func (a *app) check(c *ui.Context, t *ui.Theme, item todos.Todo) {
	label := fmt.Sprintf("Mark %q done", item.Title)
	if item.Done {
		label = fmt.Sprintf("Mark %q not done", item.Title)
	}
	btn := ui.ButtonBase(c.Key("check")).Size(21, 21).Radius(21).
		Label(label).Role(ui.RoleCheckBox).Cursor(ui.CursorPointer)

	fillAt, borderAt := ui.Transparent, t.TextMuted.Alpha(0.7)
	tickAt := float32(0)
	if item.Done {
		fillAt, borderAt, tickAt = accent, accent, 1
	} else if btn.Hovered() {
		borderAt = accent
	}
	circle := ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseInOut}
	fill := btn.AnimateColor("fill", fillAt, circle)
	border := btn.AnimateColor("border", borderAt, circle)
	tick := btn.AnimateTo("tick", tickAt, ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.EaseInOut})

	btn.Background(fill).Border(1.6, border)
	btn.Draw(func(p *ui.Painter, r ui.Rect) {
		if tick <= 0.002 {
			return
		}
		// The page's tick: its 24-unit icon drawn at 13 px.
		q, o := iconScale(13, r.W)
		pt := func(x, y float32) (float32, float32) { return r.X + o + x*q, r.Y + o + y*q }
		x1, y1 := pt(5.5, 12.6)
		x2, y2 := pt(9.7, 16.8)
		x3, y3 := pt(18.6, 7.5)
		path := (&ui.Path{}).MoveTo(x1, y1).LineTo(x2, y2).LineTo(x3, y3)
		p.StrokePath(path.Trim(0, tick), 3*q, t.AccentText)
	})
	if btn.Clicked() {
		a.toggleTodo(item.ID)
	}
}

// removeButton deletes a task, and only shows itself on the row's hover.
func (a *app) removeButton(c *ui.Context, t *ui.Theme, row ui.Element, item todos.Todo) {
	btn := ui.ButtonBase(c.Key("remove")).Size(21, 21).Radius(6).
		Label("Remove " + item.Title).Cursor(ui.CursorPointer)
	showAt := float32(0)
	if row.Hovered() {
		showAt = 1
	}
	btn.Opacity(btn.AnimateWith("show", showAt, 150*time.Millisecond, ui.EaseInOut))

	hot := btn.Hovered()
	if hot {
		btn.Background(fillColor(t))
	}
	col := t.TextMuted
	if hot {
		col = t.Danger
	}
	btn.TextColor(col)
	btn.Children(func() {
		ui.Icon(c, iconX).Size(13, 13)
	})
	if btn.Clicked() {
		a.removeTodo(item.ID)
	}
}

// footer counts the day down: what is left, Clear done, and the time to the
// midnight reset.
func (a *app) footer(c *ui.Context, t *ui.Theme, stats todos.Stats) {
	ui.Row(c).AlignItems(ui.Center).Gap(12).Margin(8, 0, 0, 0).Padding(10, 4, 2, 4).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, footerLeft(stats)).FontSize(12).TextColor(t.TextMuted)
		ui.Spacer(c)
		if stats.Done > 0 {
			// Clearing what is done cannot be taken back, so the link is
			// held rather than clicked. It sits in the footer as the
			// page's own link does, and goes with what it asks about.
			holdLink(c, "clearDone", "Clear", "Hold to clear what is done", false, a.clearDone)
		}
		if left := resetLabel(stats); left != "" {
			ui.Text(c, left).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

// dayLabel names the stored day the way the header shows it.
func dayLabel(day string) string {
	at, err := time.ParseInLocation(dayLayout, day, time.Local)
	if err != nil {
		return day
	}
	return at.Format("Mon, Jan 2")
}

// footerLeft is the left end of the footer, as the page words it.
func footerLeft(stats todos.Stats) string {
	switch {
	case stats.Total == 0:
		return "No tasks yet"
	case stats.Active == 0:
		return "All done"
	default:
		return fmt.Sprintf("%d left", stats.Active)
	}
}

// resetLabel counts down to the midnight reset.
func resetLabel(stats todos.Stats) string {
	left := time.Until(time.UnixMilli(stats.ResetsAt))
	if left <= 0 {
		return ""
	}
	h := int(left / time.Hour)
	m := int(left/time.Minute) % 60
	if h > 0 {
		return fmt.Sprintf("Resets in %dh %dm", h, m)
	}
	return fmt.Sprintf("Resets in %dm", m)
}

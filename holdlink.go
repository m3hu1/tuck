// A footer link that acts only on a hold: Tuck's quiet Delete and Clear
// links, with the restraint that keeps a stray click from doing what cannot
// be taken back.
package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	// holdLinkDuration is how long the link takes to act.
	holdLinkDuration = 1200 * time.Millisecond
	// holdLinkSlack is how far the pointer may slide off the link before
	// the hold lets go.
	holdLinkSlack = 24
)

type holdLinkSource uint8

const (
	holdLinkIdle holdLinkSource = iota
	holdLinkPointer
	holdLinkKey
)

// holdLinkState is the link's hold between frames: where it started and the
// fill it started from, so a quick re-press resumes rather than starting over.
type holdLinkState struct {
	source   holdLinkSource
	start    time.Time
	from     float32
	shown    float32
	released bool
	w, h     float32
}

func (s *holdLinkState) begin(source holdLinkSource, at time.Time) {
	if s.source == holdLinkIdle {
		s.source, s.start, s.from = source, at, s.shown
	}
}

func (s *holdLinkState) release(source holdLinkSource) {
	if s.source == source {
		s.source, s.released = holdLinkIdle, true
	}
}

// holdLink draws a footer link that sets nothing off until it is held: a
// thin line fills under the words, and one that is let go early takes it
// back. It is a link at the footer's size, so it sits in the footer rather
// than beside it.
func holdLink(c *ui.Context, key, label, hint string, onConfirm func()) {
	b := ui.ButtonBase(c.Key(key)).Label(label).Description(hint).
		Cursor(ui.CursorPointer).Padding(0, 0, 2, 0).Radius(4).
		Background(ui.Transparent).Border(0, ui.Transparent).TextColor(accent)
	b.Children(func() {
		ui.Text(c, label).FontSize(12).SingleLine()
	})

	s := ui.Local(b, "hold", func() holdLinkState { return holdLinkState{} })
	disabled := b.IsDisabled()
	b.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			// Taking the press keeps the base from reporting a click.
			if !disabled {
				s.begin(holdLinkPointer, ev.Time)
			}
			return true
		case ui.InputPointerMove:
			if s.source == holdLinkPointer && (ev.X < -holdLinkSlack || ev.Y < -holdLinkSlack || ev.X > s.w+holdLinkSlack || ev.Y > s.h+holdLinkSlack) {
				s.release(holdLinkPointer)
			}
		case ui.InputPointerUp:
			s.release(holdLinkPointer)
			return ev.Button == 0
		case ui.InputKeyDown, ui.InputKeyUp:
			if ev.Key != ui.KeySpace && ev.Key != ui.KeyEnter {
				return false
			}
			if ev.Kind == ui.InputKeyUp {
				s.release(holdLinkKey)
			} else if !ev.Repeat && !disabled {
				s.begin(holdLinkKey, ev.Time)
			}
			return true
		}
		return false
	})
	if bounds := b.LayoutBounds(); bounds.W > 0 {
		s.w, s.h = bounds.W, bounds.H
	}
	if disabled || (s.source == holdLinkKey && !b.Focused()) {
		s.release(s.source)
	}

	var fill float32
	done := false
	if s.source != holdLinkIdle {
		p := s.from + float32(max(c.Now().Sub(s.start), 0))/float32(holdLinkDuration)
		if p >= 1 {
			s.source, done = holdLinkIdle, true
			p = 1
		} else {
			c.AnimationFrame()
		}
		fill = b.Jump("hold", p)
	} else {
		if s.released {
			// Jumping in place zeroes the velocity the linear fill left.
			b.Jump("hold", s.shown)
		}
		fill = b.AnimateTo("hold", 0, ui.Animation{Duration: 200 * time.Millisecond})
	}
	s.shown, s.released = fill, false

	b.DrawOver(func(p *ui.Painter, r ui.Rect) {
		y := r.Y + r.H - 1.5
		if fill <= 0.001 {
			if b.Hovered() {
				p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 1}, accent.Alpha(0.3), 0.5)
			}
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: y, W: r.W * min(fill, 1), H: 1.5}, accent, 0.75)
	})

	if done {
		if onConfirm != nil {
			onConfirm()
		}
		c.After(0)
	}
}

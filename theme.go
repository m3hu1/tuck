// The panel's theme: Tuck's colors and sizes laid over the framework's
// themes, so both appearances share the same accent, greys, and spacing.
package main

import "github.com/egoist/mygo/ui"

// accent is Tuck's blue: its buttons, links, the selected day and the ring.
var (
	accent      = ui.Hex("#0a84ff")
	accentHover = brighten(accent, 1.06)
)

// panelTheme returns Tuck's tokens for the appearance of base.
func panelTheme(base *ui.Theme) *ui.Theme {
	t := ui.LightTheme()
	if base.Dark {
		t = ui.DarkTheme()
		t.Background = ui.Hex("#18181b")
		t.Text = ui.Hex("#f5f5f7")
		t.TextMuted = ui.Hex("#98989d")
		t.Border = ui.RGBA(255, 255, 255, 0.12)
	} else {
		t.Background = ui.Hex("#ffffff")
		t.Text = ui.Hex("#1c1c1e")
		t.TextMuted = ui.Hex("#86868b")
		t.Border = ui.RGBA(60, 60, 67, 0.12)
	}
	// Surface is the raised face components sit above the panel: opaque white
	// in light, a veil of white in dark. The segmented tab's thumb uses it.
	t.Surface = surfaceColor(t)
	// The raised face's states mix towards the text rather than using Fill,
	// which is a veil over whatever it sits on.
	t.SurfaceHover = t.Surface.Mix(t.Text, 0.08)
	t.SurfacePressed = t.Surface.Mix(t.Text, 0.16)
	t.Accent = accent
	t.AccentHover = accentHover
	t.AccentPressed = brighten(accent, 0.93)
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#ff453a")
	t.Focus = ui.RGBA(10, 132, 255, 0.45)
	t.Selection = ui.RGBA(10, 132, 255, 0.3)
	t.Radius = 10
	t.FontSize = 14
	return t
}

// brighten scales a color's channels, as a CSS brightness filter does.
func brighten(c ui.Color, f float32) ui.Color {
	scale := func(v uint8) uint8 { return uint8(min(255, float32(v)*f+0.5)) }
	c.R, c.G, c.B = scale(c.R), scale(c.G), scale(c.B)
	c.A = 255
	return c
}

// surfaceColor is the panel's raised white, opaque in light and a veil of
// white in dark.
func surfaceColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.12)
	}
	return ui.Hex("#ffffff")
}

// fillColor is the veil a control takes under the pointer, a wash of the text
// over whatever it sits on. It is not Surface, which is the raised face the
// components draw above a surface.
func fillColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(120, 120, 128, 0.24)
	}
	return ui.RGBA(120, 120, 128, 0.12)
}

// hairlineColor outlines raised surfaces.
func hairlineColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.1)
	}
	return ui.RGBA(60, 60, 67, 0.08)
}

// goldColor marks the dot of a note kept forever.
func goldColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#f0bf5c")
	}
	return ui.Hex("#c98a12")
}

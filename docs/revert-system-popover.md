# Restoring the system popover

Tuck's menu bar panel uses a plain MyGo flyout so it appears at once, without
the macOS popover's arrow or open animation. The earlier version used the
system's `NSPopover`.

## Quick revert

In `main.go`, in `func (a *app) show()`, change:

```go
Popover: false,
```

to:

```go
Popover: true,
```

The `case c.Vibrancy():` branch in `view.go` then styles the content over the
popover's material, as it did before.

## Full block

If the flyout call has changed since this note was written, replace the active
`a.panel = mygo.NewFlyout(...)` block in `show()` with this whole block. Leave
the surrounding `if a.panel == nil { ... }` and the `OnHide`/`OnClosed`
listeners in place.

File: `main.go`

```go
a.panel = mygo.NewFlyout(mygo.FlyoutOptions{
	Tray:      a.tray,
	Placement: mygo.PlacementBottom,
	Gap:       6,
	Width:     340,
	Height:    470,
	Focusable: true,
	Hidden:    true,
	Shadow:    true,
	Popover:   true,
	Content:   ui.View(a.view),
})
```

No other changes are needed. With `Popover: true`, AppKit draws the system
material and the arrow pointing at the tray icon, and animates the panel as it
opens and closes.

# Tuck

Tuck is a native menu bar app for the day: a task list that starts fresh at
midnight and one markdown note per day. The tray icon opens a compact panel
with two tabs, Tasks and Notes. Everything is drawn by [MyGo](https://mygo.egoist.dev)'s
`ui` package; there is no webview and no frontend build.

## Features

- **A daily list** — add a task, check it off, or remove it from the row's
  hover control. The header's ring shows how much of the day is done, and the
  footer counts what is left and when the list resets. A background timer
  clears the list at local midnight; a list from an earlier day is discarded
  on launch.
- **One note per day** — the Notes tab opens on today, rendered as markdown.
  Click the page to write, save with the button or `Cmd+S`, or let the
  autosave take the note 600 ms after typing stops. Closing the panel saves
  and renders it too.
- **Thirty days, or forever** — the note store keeps a rolling 30-day window.
  *Keep* pins a day so it is never aged out, and *Kept* hands it back.
- **A calendar over the note** — the picker is laid exactly over the note's
  page. A blue dot marks a day with a note, a gold dot marks a kept day, and
  archived or future days are greyed. The arrows walk the calendar, Enter
  opens the day under the cursor, and Escape or a press outside closes it.
- **Held, not clicked** — *Clear* and *Delete note* are quiet footer links.
  Holding one fills a thin line under the words; letting go early rewinds it.
  A stray click cannot throw away the day's list or note.
- **Tabs that move** — Tasks and Notes slide between each other with an
  animated thumb, and the keyboard follows the chosen view.
- **Menu bar first** — no Dock icon. The panel snaps straight into place,
  without the system popover's arrow, and is built once so reopening it is
  instant. Right-click the tray icon for *Open Tuck* and *Quit*.
- **Light and dark** — the panel follows the system appearance.
- **Autosaved and local** — notes and tasks are JSON files under
  `~/Library/Application Support/Tuck/`. Nothing leaves the Mac.

## Requirements

- macOS 12 or later
- Go 1.27.1, which the module records
- Xcode Command Line Tools for packaging

No Node, Bun, or web toolchain is needed. The native template keeps the MyGo
CLI as a Go tool, so `go tool mygo` always uses the version pinned in
`go.mod`.

## Commands

```sh
go tool mygo dev     # build and run with live reload
go test ./...        # stores and panel, headless through ui.NewTester
go tool mygo vet     # Go vet plus MyGo's native lifetime checks
go tool mygo build   # package the .app and .dmg under build/
go run .             # run the app directly, without the tool
```

Snapshots of the panel can be written during tests:

```sh
SNAP=/tmp/tuck-shots go test . -run Snapshot
```

## Keyboard

| Where | Keys | Action |
| --- | --- | --- |
| Anywhere | `Cmd+Ctrl+Alt+Shift+T` | Open the panel on today's tasks |
| Anywhere | `Cmd+Ctrl+Alt+Shift+N` | Open today's note, ready to write |
| Note | `Cmd+S` | Save and render the note |
| Note | `Tab` | Indent the selected lines by two spaces |
| Note | `Backspace` | Unindent at the start of the note |
| Calendar | Arrow keys | Move by a day or a week |
| Calendar | `Enter` / `Space` | Open the selected day |
| Calendar | `Escape` | Close the calendar |

The two global shortcuts use the hyper-key pattern: Caps Lock is set up as
`Cmd+Ctrl+Alt+Shift`, then press `T` or `N`.

## Layout

```
main.go                 app lifecycle: tray, panel, shortcuts, midnight
view.go                 tasks tab: tabs, list, ring, add row, footer
notesview.go            notes tab: state, editor, preview, calendar, footer
holdlink.go             the footer links that act on a hold
icons.go                the nine Lucide SVGs, parsed once
theme.go                panel colors and sizes over MyGo's themes
internal/todos/         the day's list: items, JSON store, reset clock
internal/notes/         notes: rolling window, pins, calendar marks, JSON store
resources/icon.png      app icon
mygo.json               name, identifier, version, and packaging options
THIRD_PARTY_NOTICES.md  the Lucide and Feather licenses
docs/                   notes for maintainers
```

The tests sit beside the views (`*_test.go`) and beside each store. They drive
the panel through `ui.NewTester`: clicks, holds, typing, focus, calendar keys,
and popover dismissal all run without a window.

## Design and dependencies

Tuck uses one library, **[MyGo](https://mygo.egoist.dev)**, for the status bar
item, the flyout panel, input and layout, and for `ui/markdown`. Its nine
[Lucide](https://lucide.dev) icons are inlined as SVG in `icons.go`, so there is
no icon dependency; their licenses are in `THIRD_PARTY_NOTICES.md`. The check
tick is drawn by hand because it grows along its own path.

The tab strip, calendar, progress ring, and footer holds are Tuck's own. The
calendar opens as a MyGo popover so it inherits native focus order, Escape
handling, and dismissal on an outside press. To bring back the animated system
popover panel, follow `docs/revert-system-popover.md`.

Tuck stores:

- `todos.json` — today's day and its items.
- `notes.json` — a map of `YYYY-MM-DD` days to their note text and pin state.

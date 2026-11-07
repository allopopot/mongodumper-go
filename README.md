# mongodumper-go — TUI Backup & Restore

A terminal UI (TUI) utility for backing up and restoring MongoDB databases,
built in Go with Bubble Tea.

**Status:** fully implemented — see [Testing](#testing) below. This document
records the original plan and the decisions behind it.

## Decisions

| Topic | Decision |
|---|---|
| TUI framework | Bubble Tea + bubbles + lipgloss |
| Restore conflict behavior | Drop collection, then insert (restore = point in time) |
| Backup scope | Multi-database (checklists across all DBs) |
| Backup format | Plain JSON for now; isolated in `operations/` so it can be swapped (gzip, etc.) later |
| Backup location | `./backups/<database-name>/<collection-name>.json` |
| Restore default location | `./backups` |
| System DBs | `admin`, `config`, `local` filtered out of the backup list |

## Page flow

```
1. URI        → connect + ping (inline error on failure)
2. Mode       → [Backup] / [Restore]
   ├─ BACKUP
   │   3. Select   → databases (left pane) + collections checkboxes (right pane)
   │   4. Progress → per-collection bars + overall bar + result log
   └─ RESTORE
       3. Location → text input, default ./backups
       4. Select   → tree of dbs/collections found in backup (all pre-checked)
       5. Progress → same progress page as backup
```

**Keybindings:** `Esc` = back one page (quits on page 1; cancels a running
operation on the progress page), `Ctrl+C` = quit anywhere, `q` = quit on
non-input pages, `Tab` = switch pane, `Space` = toggle checkbox,
`Enter` = confirm / start, `A` = toggle all.

## File layout

```
main.go                    # entrypoint: tea.NewProgram(ui.New(), AltScreen)
dbcontext/
  main.go                  # Connect(ctx, uri) error (ping + 5s timeout),
                           # Disconnect, ListDatabases, ListCollections
                           # — no hardcoded URI, no panics, system DBs filtered
operations/
  backup.go                # ExportCollection(ctx, db, coll, outDir, report) (int64, error)
  restore.go               # ImportCollection(ctx, db, coll, filePath, report) (int64, error)
                           #   → drops collection first (drop-then-insert)
ui/
  app.go                   # root model: page enum, shared state, routing, key handling
  events.go                # all message types + tea.Cmd constructors
  styles.go                # shared lipgloss styles + number/byte formatting
  page_uri.go              # 1. textinput + connect
  page_mode.go             # 2. two-option chooser
  selector.go              # shared two-pane db list | collection checkboxes
                           #   (used by backup page 3 and restore page 4)
  page_restore_loc.go      # 3. path input (default ./backups)
  progress.go              # shared progress page (4/5)
  runner.go                # goroutine running the operation over a channel
```

## Design points

### Format isolation

The backup format is deferred ("figure out later"), so the `operations`
layer exposes thin signatures that hide it:

- `ExportCollection(ctx, db, coll, outDir, report func(done, total int64)) (int64, error)`
- `ImportCollection(ctx, filePath, db, coll, report func(done, total int64)) (int64, error)`

Both return the number of documents written/read. Today they write/read
JSON-array files; switching to gzip later means wrapping the file in
`gzip.Writer`/`gzip.Reader` — no TUI changes.

### Error handling

All current `panic(err)` calls become returned errors so the TUI can show
them inline instead of crashing.

### Progress reporting

Operations run in a goroutine and report via `program.Send(...)`:

- **Backup total** per collection: `EstimatedDocumentCount` upfront;
  `done` = docs written.
- **Restore total** per collection: file size; `done` = bytes read
  (avoids a costly pre-parse of the JSON).
- Overall bar = completed collections / total; current collection gets its
  own bar + spinner + live doc count.
- Result log below:
  - `✔ mydb.users — 12,345 docs`
  - `✘ mydb.logs — connection reset` (red)
- Cancel (`Esc`) cancels the shared `context.Context`.

### Backup selection (page 3, multi-DB)

- Left pane: database list (system DBs filtered).
- Right pane: collection checkboxes for the highlighted database
  (fetched lazily via `ListCollections` when the highlight moves).
- `Tab` switches panes, `Space` toggles the highlighted entry (a whole
  database's collections when the left pane is focused), `Enter` on a
  collection toggles it, `A` toggles every loaded collection,
  `Enter` on the databases pane = start (when ≥1 collection selected
  across all DBs).

### Restore selection (page 4)

- Walk the chosen root dir one level deep: `./backups/*/*.json`
  → `<db>/<coll>` checkbox entries, all checked by default.
- Missing/empty dir → inline error on page 3.

### Shared state (root model)

`uri`, `mode`, `backupDir`, and the per-page models (`uriPage`,
`modePage`, `backupSel`, `restoreLoc`, `restoreSel`, `progress`); the Mongo
client itself lives in `dbcontext.DB`. The root `Update` intercepts typed
navigation/lifecycle messages and forwards everything else to the active
page.

## Dependencies

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles` (textinput, progress, spinner)
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/x/ansi` (display-width-aware truncation)

## Implementation order

All steps below are **complete**; listed as originally planned.

1. **Refactor `dbcontext`** — `Connect(ctx, uri)` with ping + timeout, list
   helpers, no hardcoded URI, errors propagate (no panics).
2. **Refactor `operations`** — error returns, `context.Context` support,
   `report` callbacks, drop-before-insert in restore.
3. **`ui/app.go`** — root state machine, page routing, shared key handling;
   stub pages so the flow is navigable end-to-end.
4. **Pages 1–2** — URI input with real connect/ping, mode chooser.
5. **Backup: page 3 + progress page** — db/collection picker, async export
   with progress.
6. **Restore: pages 3–5** — location input, backup scanner/selection,
   async import.
7. **Polish** — lipgloss styling, empty states, error surfaces,
   `go vet` + manual test against a local MongoDB.

## Testing

A PTY harness in `/tmp/opencode/tui-test` drives the real binary through
three end-to-end scenarios (backup, restore, bad URI) plus a full
back-navigation pass, asserting on reconstructed terminal screens:

- `drive3.py` — backup, restore, and connection-error scenarios
- `test_nav.py` — every `Esc` path and input re-focus
- `verify_restore.py` — deletes documents, restores through the TUI,
  asserts exact counts return

`go build ./...`, `go vet ./...`, and `gofmt -l .` are all clean.

## Open items

- **Backup format** — plain JSON vs gzip vs something else; deferred by
  choice. The `operations` layer is designed so this changes in one file.

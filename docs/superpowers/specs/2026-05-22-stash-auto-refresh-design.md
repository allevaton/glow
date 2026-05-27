# Stash auto-refresh — design

## Goal

The TUI stash list discovers local markdown files at startup via a recursive
`gitcha` walk of the working directory, then never re-scans unless the user
presses `F`. Make the list update automatically as files are added, removed, or
renamed on disk.

## Approach

Periodic full rescan on a `tea.Tick`. Reuse the existing discovery path
(`gitcha`, gitignore-aware), diff the result set against the in-memory model
by file path, and apply adds and removals. No new dependency; no recursive
`fsnotify`. The manual `F` refresh keeps working unchanged.

Rejected alternatives:

- **Recursive `fsnotify`** — `fsnotify` is non-recursive on Linux/macOS, so we
  would have to walk the tree, `watcher.Add` every directory, dynamically add
  and remove watches as subdirectories appear and disappear, re-implement
  gitignore filtering on every event, and handle editor rename dances. Too
  much surface area for the win.
- **Hybrid (shallow `fsnotify` + slow rescan)** — uneven latency story
  ("instant at root, eventually-consistent deeper") for marginal CPU savings.

## Configuration

One new option:

- Flag: `--refresh-interval` (`time.Duration`).
- `glow.yml` key: `refresh-interval` (via the standard Viper flag binding).

Default: `5s`. Setting to `0` disables auto-refresh; the manual `F` shortcut
remains available regardless.

No environment-variable support. An earlier draft proposed `GLOW_REFRESH_INTERVAL`,
but mixing `caarlos0/env` struct tags with a Viper-bound flag in `runTUI`
would silently discard the env value (the flag's default overwrites the
env-parsed field). The existing `ui.Config` fields are either env-only or
flag-only; this option is flag-only, matching `ShowAllFiles` / `EnableMouse`
/ `PreserveNewLines`.

## Components

### `ui/config.go`

Add a `RefreshInterval time.Duration` field to `ui.Config` with an `env` tag
of `GLOW_REFRESH_INTERVAL`. The value is populated from Viper in
`main.runTUI` alongside the other config fields.

### `main.go`

- Register the `--refresh-interval` flag in `init()`.
- `viper.BindPFlag` it.
- Read it in `validateOptions` and assign to `Config.RefreshInterval`.

### `ui/ui.go`

- New message type:
  ```go
  type rescanFinishedMsg struct {
      paths []string
  }
  ```
- New command `rescanLocalFiles(m commonModel) tea.Cmd`. Same gitcha walk as
  `findLocalFiles`, but synchronously drains the channel into a `[]string` of
  absolute paths and returns a single `rescanFinishedMsg`. Atomicity matters:
  the diff must see the full new set in one go, so we do not stream results
  the way the initial load does.
- New command `scheduleRescan(d time.Duration) tea.Cmd` that wraps
  `tea.Tick(d, ...)` and returns a `rescanTickMsg`.
- New message `rescanTickMsg struct{}`.
- Dispatch:
  - On `localFileSearchFinished` (initial load done): if
    `RefreshInterval > 0`, schedule the first tick.
  - On `rescanTickMsg`: if conditions allow (see Guardrails), set
    `stash.rescanInFlight = true` and run `rescanLocalFiles`. Otherwise just
    re-arm the tick.
  - On `rescanFinishedMsg`: forward to `stash.update`, which applies the diff
    and re-arms the tick.

### `ui/stash.go`

- New field on `stashModel`:
  - `rescanInFlight bool`
- New handler in `update` for `rescanFinishedMsg` that performs the diff
  described below, clears `rescanInFlight`, and returns a `tea.Batch` of
  `scheduleRescan(cfg.RefreshInterval)` plus (if a filter is active)
  `filterMarkdowns(m)`.

## Diff algorithm

On `rescanFinishedMsg{paths}`:

1. Remember the currently selected markdown's `localPath` (empty string if
   nothing selected).
2. Build `existing := map[string]*markdown` from `m.markdowns` keyed by
   `localPath`.
3. Build `incoming := map[string]struct{}` from `paths`.
4. **Additions:** for each path in `incoming` not in `existing`, create a
   `markdown` via `localFileToMarkdown(m.common.cwd, ...)`, build its filter
   value if a filter is applied, and append to `m.markdowns`.
5. **Removals:** rebuild `m.markdowns` in place, keeping only entries whose
   `localPath` is in `incoming`. If filtering, also prune
   `m.filteredMarkdowns` the same way.
6. If anything changed and no filter is active, call
   `sortMarkdowns(m.markdowns, m.sortMode)`.
7. If a filter is active, batch `filterMarkdowns(m)` into the returned
   commands so the filtered view is recomputed.
8. `updatePagination()` (so `PerPage` / `TotalPages` reflect the new size).
9. Restore cursor: find the index of the remembered `localPath` in the
   current visible list. If found, set the paginator's page and the cursor
   to point at it. If not found (selection was removed) or there was no
   prior selection, clamp the cursor to `[0, itemsOnPage-1]` for the
   current page, matching the clamp that `handleDocumentBrowsing` already
   performs at the end of every update.

## Guardrails

A tick skips the rescan (and just re-arms) when any of the following hold:

- `RefreshInterval == 0` (disabled — in this case we never schedule in the
  first place, so the check is defensive).
- `m.rescanInFlight` (previous rescan still running).
- The top-level model state is not `stateShowStash` (e.g. a doc is open in
  the pager).
- `m.stash.filterState == filtering` (user is actively typing a filter
  query — don't churn the list).
- `m.stash.viewState != stashStateReady` (loading or error state).

The top-level model state needs to be readable from the tick handler;
since the tick handler lives in `ui.go`'s main `Update`, it already has
access to `m.state`.

## What we are explicitly not doing

- No recursive `fsnotify`. If profiling shows rescans are a real cost in
  practice, we revisit.
- No status message or spinner on auto-refresh — updates are silent. Manual
  `F` keeps its existing behavior.
- No content-change detection on existing files. The pager handles that for
  the currently open document via its own `fsnotify` watcher. The stash only
  cares about the set of files.
- No partial / incremental scans. A skipped tick is fine; the next tick will
  catch up.

## Testing

Unit-test the diff in isolation by factoring the add/remove/sort/cursor-
restore logic into a method on `stashModel` that takes a `[]string` and
mutates the model. Cases:

- Empty before, files added.
- Files removed, including the currently selected file (cursor clamps).
- Files added and removed in the same tick.
- Selected file still present (cursor stays on it even if its index shifted
  due to sorting).
- Filter applied: filtered view updates without resetting the filter.

End-to-end behavior (file appears on disk → shows up within `RefreshInterval`)
is left to manual verification with `task log` tailing the debug log.

## Open questions

None blocking. Worth revisiting later: whether to expose a "pause auto-
refresh" key binding for users on slow filesystems who want explicit control
without disabling via config.

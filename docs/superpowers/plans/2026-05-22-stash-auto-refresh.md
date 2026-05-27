# Stash Auto-Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Commit policy override:** The user has a standing instruction in `~/.claude/CLAUDE.md` forbidding `git commit`/`git push`/`gh pr create` without explicit per-action approval. Instead of "Commit" steps, this plan ends each task with a "Pause for review" step that hands control back. Do not commit; let the user run any commits themselves.

**Goal:** Periodically rescan the working directory and reconcile the stash list so newly created, deleted, or renamed markdown files appear/disappear without the user pressing `F`.

**Architecture:** A `tea.Tick`-driven rescan command runs the existing `gitcha` walk on an interval, collects results into a single message, and a new `stashModel.applyRescan` method diffs that set against `m.markdowns` by path — adding new entries, removing gone ones, and preserving cursor selection. The interval is configurable via flag/env/Viper; setting it to `0` disables the feature without affecting the manual `F` refresh.

**Tech Stack:** Go 1.25+, Bubble Tea, `muesli/gitcha`, `caarlos0/env/v11`, Cobra, Viper. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-05-22-stash-auto-refresh-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `ui/config.go` | Modify | Add `RefreshInterval` field |
| `main.go` | Modify | Add `--refresh-interval` flag, Viper binding, runTUI wiring |
| `ui/ui.go` | Modify | New `rescanLocalFiles` / `scheduleRescan` commands, new message types, dispatch in main `Update` |
| `ui/stash.go` | Modify | New `rescanInFlight` field, new `applyRescan` method, handler for `rescanFinishedMsg` |
| `ui/stash_rescan_test.go` | Create | Unit tests for `applyRescan` |

---

## Task 1: Add `RefreshInterval` config plumbing

**Files:**
- Modify: `ui/config.go`
- Modify: `main.go` (vars block ~line 38, `init()` ~line 410, `validateOptions` ~line 172, `runTUI` ~line 369)

- [ ] **Step 1: Add `RefreshInterval` to `ui.Config`**

Edit `ui/config.go`. Add `"time"` to imports and a new field:

```go
package ui

import "time"

// Config contains TUI-specific configuration.
type Config struct {
	ShowAllFiles     bool
	ShowLineNumbers  bool
	Gopath           string `env:"GOPATH"`
	HomeDir          string `env:"HOME"`
	GlamourMaxWidth  uint
	GlamourStyle     string `env:"GLAMOUR_STYLE"`
	EnableMouse      bool
	PreserveNewLines bool

	// Working directory or file path
	Path string

	// Interval between automatic rescans of the working directory in TUI
	// mode. Zero disables auto-refresh; the manual 'F' shortcut still works.
	RefreshInterval time.Duration `env:"GLOW_REFRESH_INTERVAL" envDefault:"5s"`

	// For debugging the UI
	HighPerformancePager bool `env:"GLOW_HIGH_PERFORMANCE_PAGER" envDefault:"true"`
	GlamourEnabled       bool `env:"GLOW_ENABLE_GLAMOUR"         envDefault:"true"`
}
```

- [ ] **Step 2: Add the package-level flag var in `main.go`**

In the var block around line 38, add `refreshInterval time.Duration` alongside the others:

```go
var (
	configFile       string
	pager            bool
	tui              bool
	style            string
	width            uint
	showAllFiles     bool
	showLineNumbers  bool
	preserveNewLines bool
	mouse            bool
	refreshInterval  time.Duration
	// ... existing rootCmd definition below
)
```

Add `"time"` to the imports if not already present (it is not currently imported in `main.go`; check with grep first).

- [ ] **Step 3: Register the flag and bind it to Viper**

In `init()` in `main.go`, after the other `rootCmd.Flags().*VarP` calls (around line 418), add:

```go
rootCmd.Flags().DurationVar(&refreshInterval, "refresh-interval", 5*time.Second, "interval for auto-refreshing the file list (TUI-mode only, 0 disables)")
```

After the existing `viper.BindPFlag` block (around line 430), add:

```go
_ = viper.BindPFlag("refreshInterval", rootCmd.Flags().Lookup("refresh-interval"))
```

- [ ] **Step 4: Read the value in `validateOptions`**

In `validateOptions` in `main.go`, after the other `viper.Get*` reads (around line 180), add:

```go
refreshInterval = viper.GetDuration("refreshInterval")
```

- [ ] **Step 5: Wire it into `cfg` inside `runTUI`**

In `runTUI` in `main.go`, after the other `cfg.X = x` assignments (around line 374), add:

```go
cfg.RefreshInterval = refreshInterval
```

- [ ] **Step 6: Verify the build and the flag**

Run:

```bash
cd /Users/nickallevato/workspace/glow && go build ./...
```

Expected: clean build.

```bash
./glow --help | grep refresh-interval
```

Expected: a line containing `--refresh-interval` with default `5s`.

```bash
./glow --refresh-interval 7s --help
```

Expected: exits 0 (flag is accepted; no crash).

- [ ] **Step 7: Pause for review**

Summarize the diff for the user. Suggested commit message (do not run it):

```
Add --refresh-interval config plumbing for stash auto-refresh
```

---

## Task 2: Add `applyRescan` skeleton — handle additions only (TDD)

**Files:**
- Create: `ui/stash_rescan_test.go`
- Modify: `ui/stash.go` (add field + method)

Goal of this task: introduce the diff method with the minimum behavior needed to make a single test pass — adding new paths that aren't in `m.markdowns`. Removals and cursor preservation come in later tasks.

The method takes a factory function so tests can avoid touching disk:

```go
type markdownFactory func(path string) *markdown
```

- [ ] **Step 1: Write the failing test**

Create `ui/stash_rescan_test.go`:

```go
package ui

import (
	"testing"
)

// stubFactory returns a minimal markdown for a path without touching disk.
func stubFactory(path string) *markdown {
	return &markdown{localPath: path, Note: path}
}

// newTestStash returns a stashModel wired up enough for applyRescan tests.
func newTestStash() stashModel {
	initSections()
	m := newStashModel(&commonModel{
		cfg: Config{},
	})
	m.common.width = 80
	m.common.height = 40
	m.loaded = true
	return m
}

func pathsOf(mds []*markdown) []string {
	out := make([]string, 0, len(mds))
	for _, md := range mds {
		out = append(out, md.localPath)
	}
	return out
}

func TestApplyRescan_AddsNewPaths(t *testing.T) {
	m := newTestStash()
	m.addMarkdowns(stubFactory("/a.md"))

	m.applyRescan([]string{"/a.md", "/b.md"}, stubFactory)

	got := pathsOf(m.markdowns)
	want := []string{"/a.md", "/b.md"}
	if len(got) != len(want) {
		t.Fatalf("len(markdowns) = %d, want %d (paths=%v)", len(got), len(want), got)
	}
	// Sort is by name ascending (sortByName is the default).
	for i, p := range want {
		if got[i] != p {
			t.Errorf("markdowns[%d].localPath = %q, want %q", i, got[i], p)
		}
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run:

```bash
cd /Users/nickallevato/workspace/glow && go test ./ui -run TestApplyRescan_AddsNewPaths -v
```

Expected: fails to compile with `m.applyRescan undefined`.

- [ ] **Step 3: Add `rescanInFlight` field and minimal `applyRescan` to `stash.go`**

In `ui/stash.go`, add to the `stashModel` struct (after `lastClickIndex int`):

```go
	// rescanInFlight is true when a periodic rescan command is running.
	// Prevents overlapping rescans.
	rescanInFlight bool
```

Define the factory type and method. Place after the existing methods on `stashModel`, before `// INIT` (around line 380):

```go
// markdownFactory builds a *markdown for a given path. Injected so tests
// can avoid touching disk.
type markdownFactory func(path string) *markdown

// applyRescan reconciles m.markdowns against a fresh set of paths from a
// directory rescan. Adds new entries via mkMarkdown, removes entries whose
// paths are no longer present, preserves cursor selection where possible,
// and re-sorts (unless a filter is active).
func (m *stashModel) applyRescan(paths []string, mkMarkdown markdownFactory) {
	incoming := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		incoming[p] = struct{}{}
	}

	existing := make(map[string]*markdown, len(m.markdowns))
	for _, md := range m.markdowns {
		existing[md.localPath] = md
	}

	// Additions.
	for _, p := range paths {
		if _, ok := existing[p]; ok {
			continue
		}
		md := mkMarkdown(p)
		if md == nil {
			continue
		}
		if m.filterApplied() {
			md.buildFilterValue()
		}
		m.markdowns = append(m.markdowns, md)
	}

	if !m.filterApplied() {
		sortMarkdowns(m.markdowns, m.sortMode)
	}

	m.updatePagination()
}
```

- [ ] **Step 4: Run the test and confirm it passes**

Run:

```bash
cd /Users/nickallevato/workspace/glow && go test ./ui -run TestApplyRescan_AddsNewPaths -v
```

Expected: PASS.

- [ ] **Step 5: Run the full suite**

Run:

```bash
cd /Users/nickallevato/workspace/glow && go test ./...
```

Expected: PASS for all packages. No regressions.

- [ ] **Step 6: Pause for review**

Summarize the diff for the user. Suggested commit message (do not run it):

```
Add applyRescan diff method (additions only) with unit test
```

---

## Task 3: Extend `applyRescan` to handle removals (TDD)

**Files:**
- Modify: `ui/stash_rescan_test.go`
- Modify: `ui/stash.go`

- [ ] **Step 1: Add the failing test**

Append to `ui/stash_rescan_test.go`:

```go
func TestApplyRescan_RemovesGonePaths(t *testing.T) {
	m := newTestStash()
	m.addMarkdowns(
		stubFactory("/a.md"),
		stubFactory("/b.md"),
		stubFactory("/c.md"),
	)

	// /b.md no longer exists on disk.
	m.applyRescan([]string{"/a.md", "/c.md"}, stubFactory)

	got := pathsOf(m.markdowns)
	want := []string{"/a.md", "/c.md"}
	if len(got) != len(want) {
		t.Fatalf("len(markdowns) = %d, want %d (paths=%v)", len(got), len(want), got)
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("markdowns[%d].localPath = %q, want %q", i, got[i], p)
		}
	}
}

func TestApplyRescan_RemovesFromFilteredView(t *testing.T) {
	m := newTestStash()
	m.addMarkdowns(
		stubFactory("/a.md"),
		stubFactory("/b.md"),
	)

	// Simulate an active filter that currently shows both entries.
	for _, md := range m.markdowns {
		md.buildFilterValue()
	}
	m.filterState = filterApplied
	m.filteredMarkdowns = append([]*markdown{}, m.markdowns...)

	m.applyRescan([]string{"/a.md"}, stubFactory)

	if got := pathsOf(m.markdowns); len(got) != 1 || got[0] != "/a.md" {
		t.Errorf("markdowns = %v, want [/a.md]", got)
	}
	if got := pathsOf(m.filteredMarkdowns); len(got) != 1 || got[0] != "/a.md" {
		t.Errorf("filteredMarkdowns = %v, want [/a.md]", got)
	}
}
```

- [ ] **Step 2: Run the tests and watch them fail**

Run:

```bash
cd /Users/nickallevato/workspace/glow && go test ./ui -run TestApplyRescan_Removes -v
```

Expected: both tests fail — the current `applyRescan` only appends, it does not prune.

- [ ] **Step 3: Add removal logic to `applyRescan`**

Replace the body of `applyRescan` in `ui/stash.go` with (changes are: the new "Removals" block, and pruning `filteredMarkdowns`):

```go
func (m *stashModel) applyRescan(paths []string, mkMarkdown markdownFactory) {
	incoming := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		incoming[p] = struct{}{}
	}

	existing := make(map[string]*markdown, len(m.markdowns))
	for _, md := range m.markdowns {
		existing[md.localPath] = md
	}

	// Additions.
	for _, p := range paths {
		if _, ok := existing[p]; ok {
			continue
		}
		md := mkMarkdown(p)
		if md == nil {
			continue
		}
		if m.filterApplied() {
			md.buildFilterValue()
		}
		m.markdowns = append(m.markdowns, md)
	}

	// Removals.
	m.markdowns = filterMarkdownsByPath(m.markdowns, incoming)
	if m.filterApplied() {
		m.filteredMarkdowns = filterMarkdownsByPath(m.filteredMarkdowns, incoming)
	}

	if !m.filterApplied() {
		sortMarkdowns(m.markdowns, m.sortMode)
	}

	m.updatePagination()
}

// filterMarkdownsByPath returns a new slice containing only entries whose
// localPath is present in keep.
func filterMarkdownsByPath(in []*markdown, keep map[string]struct{}) []*markdown {
	out := in[:0]
	for _, md := range in {
		if _, ok := keep[md.localPath]; ok {
			out = append(out, md)
		}
	}
	return out
}
```

Note: reusing the underlying array via `in[:0]` is safe because the caller assigns the result back to the same slice header and we never reference the dropped pointers afterward.

- [ ] **Step 4: Run the tests and confirm they pass**

Run:

```bash
cd /Users/nickallevato/workspace/glow && go test ./ui -run TestApplyRescan -v
```

Expected: all `TestApplyRescan_*` tests PASS.

- [ ] **Step 5: Run the full suite**

```bash
cd /Users/nickallevato/workspace/glow && go test ./...
```

Expected: PASS for all packages.

- [ ] **Step 6: Pause for review**

Summarize the diff. Suggested commit message:

```
Apply removals in applyRescan, prune filtered view too
```

---

## Task 4: Preserve cursor selection across rescans (TDD)

**Files:**
- Modify: `ui/stash_rescan_test.go`
- Modify: `ui/stash.go`

- [ ] **Step 1: Add the failing tests**

Append to `ui/stash_rescan_test.go`:

```go
func TestApplyRescan_PreservesCursorOnSelectedFile(t *testing.T) {
	m := newTestStash()
	// Add three files; with sortByName the order is /a.md, /b.md, /c.md.
	m.addMarkdowns(
		stubFactory("/a.md"),
		stubFactory("/b.md"),
		stubFactory("/c.md"),
	)
	// Select /b.md (index 1 on page 0).
	m.setCursor(1)

	// /a.md is deleted on disk. After diff, the visible order is /b.md, /c.md
	// — selection should stay on /b.md, which is now index 0.
	m.applyRescan([]string{"/b.md", "/c.md"}, stubFactory)

	if got := m.selectedMarkdown(); got == nil || got.localPath != "/b.md" {
		var p string
		if got != nil {
			p = got.localPath
		}
		t.Errorf("selectedMarkdown = %q, want %q", p, "/b.md")
	}
}

func TestApplyRescan_ClampsCursorWhenSelectionRemoved(t *testing.T) {
	m := newTestStash()
	m.addMarkdowns(
		stubFactory("/a.md"),
		stubFactory("/b.md"),
		stubFactory("/c.md"),
	)
	m.setCursor(2) // selecting /c.md

	// /c.md is deleted; selection target is gone.
	m.applyRescan([]string{"/a.md", "/b.md"}, stubFactory)

	if got := m.cursor(); got > 1 {
		t.Errorf("cursor = %d after removal of selection; want <= 1 (clamped to list size)", got)
	}
	if got := m.selectedMarkdown(); got == nil {
		t.Errorf("selectedMarkdown is nil; want a valid entry after clamp")
	}
}
```

- [ ] **Step 2: Run the tests and watch them fail**

```bash
cd /Users/nickallevato/workspace/glow && go test ./ui -run TestApplyRescan_(Preserves|Clamps) -v
```

Expected: at least the "Preserves" test fails because the cursor is not restored after sort. The "Clamps" test may or may not fail (with three→two markdowns it might happen to land on index 1), but it locks in the contract.

- [ ] **Step 3: Add cursor preservation to `applyRescan`**

Edit `applyRescan` in `ui/stash.go`. Capture the selected path before mutating, and after `updatePagination()` move the page/cursor to the new index of that path (or clamp the cursor in bounds if the path is gone). Replace the method body with:

```go
func (m *stashModel) applyRescan(paths []string, mkMarkdown markdownFactory) {
	// Remember which document was selected so we can restore the cursor.
	var selectedPath string
	if md := m.selectedMarkdown(); md != nil {
		selectedPath = md.localPath
	}

	incoming := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		incoming[p] = struct{}{}
	}

	existing := make(map[string]*markdown, len(m.markdowns))
	for _, md := range m.markdowns {
		existing[md.localPath] = md
	}

	// Additions.
	for _, p := range paths {
		if _, ok := existing[p]; ok {
			continue
		}
		md := mkMarkdown(p)
		if md == nil {
			continue
		}
		if m.filterApplied() {
			md.buildFilterValue()
		}
		m.markdowns = append(m.markdowns, md)
	}

	// Removals.
	m.markdowns = filterMarkdownsByPath(m.markdowns, incoming)
	if m.filterApplied() {
		m.filteredMarkdowns = filterMarkdownsByPath(m.filteredMarkdowns, incoming)
	}

	if !m.filterApplied() {
		sortMarkdowns(m.markdowns, m.sortMode)
	}

	m.updatePagination()
	m.restoreCursor(selectedPath)
}

// restoreCursor moves page/cursor to the entry with the given localPath in
// the current visible list. If no such entry exists, the cursor is clamped
// to the number of items currently on the page.
func (m *stashModel) restoreCursor(selectedPath string) {
	mds := m.getVisibleMarkdowns()

	if selectedPath != "" {
		for i, md := range mds {
			if md.localPath != selectedPath {
				continue
			}
			perPage := m.paginator().PerPage
			if perPage <= 0 {
				perPage = 1
			}
			m.paginator().Page = i / perPage
			m.setCursor(i % perPage)
			return
		}
	}

	// Selected path is gone (or there was no selection): clamp the cursor
	// to the items on the current page.
	itemsOnPage := m.paginator().ItemsOnPage(len(mds))
	if m.cursor() > itemsOnPage-1 {
		m.setCursor(max(0, itemsOnPage-1))
	}
}
```

- [ ] **Step 4: Run the cursor tests and confirm they pass**

```bash
cd /Users/nickallevato/workspace/glow && go test ./ui -run TestApplyRescan -v
```

Expected: all five `TestApplyRescan_*` tests PASS.

- [ ] **Step 5: Run the full suite**

```bash
cd /Users/nickallevato/workspace/glow && go test ./...
```

Expected: PASS for all packages.

- [ ] **Step 6: Pause for review**

Summarize the diff. Suggested commit message:

```
Preserve cursor selection across stash rescans
```

---

## Task 5: Add the rescan command and tick message types

**Files:**
- Modify: `ui/ui.go`

No new automated test here — this code talks to the filesystem and the channel API of gitcha. We rely on the existing `applyRescan` unit tests for the diff and on Task 8's manual smoke test for the wiring.

- [ ] **Step 1: Add the new message types and a `pathToMarkdown` helper near `localFileToMarkdown`**

In `ui/ui.go`, in the `MSG` block (around line 60 where `foundLocalFileMsg` is defined), add:

```go
type (
	// rescanTickMsg is emitted by scheduleRescan when it's time to consider
	// running another rescan.
	rescanTickMsg struct{}

	// rescanFinishedMsg carries the result of a periodic rescan: the full
	// set of file paths currently visible to gitcha.
	rescanFinishedMsg struct {
		paths []string
	}
)
```

In the `ETC` section near `localFileToMarkdown` (around line 427), add:

```go
// pathToMarkdown builds a *markdown for a path discovered by a rescan,
// statting the file for its modtime. Returns nil if the file can no longer
// be stat'd (it may have been deleted between the walk and now).
func pathToMarkdown(cwd, path string) *markdown {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return &markdown{
		localPath: path,
		Note:      stripAbsolutePath(path, cwd),
		Modtime:   info.ModTime(),
	}
}
```

- [ ] **Step 2: Add `rescanLocalFiles` and `scheduleRescan` commands**

In `ui/ui.go`, in the `COMMANDS` section after `findNextLocalFile` (around line 413), add:

```go
// rescanLocalFiles runs the same gitcha walk as findLocalFiles but drains
// the result channel synchronously into a single rescanFinishedMsg so the
// stash can apply the diff atomically.
func rescanLocalFiles(m commonModel) tea.Cmd {
	return func() tea.Msg {
		var (
			cwd = m.cfg.Path
			err error
		)

		if cwd == "" {
			cwd, err = os.Getwd()
		} else {
			var info os.FileInfo
			info, err = os.Stat(cwd)
			if err == nil && info.IsDir() {
				cwd, err = filepath.Abs(cwd)
			}
		}
		if err != nil {
			log.Debug("rescanLocalFiles: cwd resolve failed", "error", err)
			return rescanFinishedMsg{paths: nil}
		}

		var ch chan gitcha.SearchResult
		if m.cfg.ShowAllFiles {
			ch, err = gitcha.FindAllFilesExcept(cwd, markdownExtensions, nil)
		} else {
			ch, err = gitcha.FindFilesExcept(cwd, markdownExtensions, ignorePatterns(m))
		}
		if err != nil {
			log.Debug("rescanLocalFiles: gitcha failed", "error", err)
			return rescanFinishedMsg{paths: nil}
		}

		var paths []string
		for res := range ch {
			paths = append(paths, res.Path)
		}
		return rescanFinishedMsg{paths: paths}
	}
}

// scheduleRescan returns a command that fires a rescanTickMsg after d.
// Callers should not call this with d <= 0; check the interval first.
func scheduleRescan(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return rescanTickMsg{}
	})
}
```

Ensure `"time"` is in the imports of `ui/ui.go` (check first; it likely is).

- [ ] **Step 3: Verify the build**

```bash
cd /Users/nickallevato/workspace/glow && go build ./...
```

Expected: clean build. Nothing calls the new functions yet, so `go vet` may warn about unused — that's fine, Task 6 wires them up next.

If the build fails with "declared and not used" (Go is strict on unused vars, not unused funcs), proceed to Task 6.

- [ ] **Step 4: Pause for review**

Summarize the diff. Suggested commit message:

```
Add rescanLocalFiles command and tick message types
```

---

## Task 6: Wire up the tick + dispatch in the main `Update`

**Files:**
- Modify: `ui/ui.go`

- [ ] **Step 1: Schedule the first tick after initial load finishes**

In `ui/ui.go`, find the `case localFileSearchFinished:` branch in the main `Update` (around line 285). Change it from:

```go
		case localFileSearchFinished:
			// Always pass these messages to the stash so we can keep it updated
			// about network activity, even if the user isn't currently viewing
			// the stash.
			stashModel, cmd := m.stash.update(msg)
			m.stash = stashModel
			return m, cmd
```

to:

```go
		case localFileSearchFinished:
			// Always pass these messages to the stash so we can keep it updated
			// about network activity, even if the user isn't currently viewing
			// the stash.
			stashModel, cmd := m.stash.update(msg)
			m.stash = stashModel
			cmds = append(cmds, cmd)
			if m.common.cfg.RefreshInterval > 0 {
				cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
			}
			return m, tea.Batch(cmds...)
```

- [ ] **Step 2: Handle `rescanTickMsg` in the main `Update`**

Add a new case before the closing `}` of the message switch (after the `case filteredMarkdownMsg:` block around line 309):

```go
		case rescanTickMsg:
			if m.common.cfg.RefreshInterval <= 0 {
				break
			}
			// Skip this tick if we shouldn't rescan right now; re-arm.
			if m.state != stateShowStash ||
				m.stash.rescanInFlight ||
				m.stash.viewState != stashStateReady ||
				m.stash.filterState == filtering {
				cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
				break
			}
			m.stash.rescanInFlight = true
			cmds = append(cmds, rescanLocalFiles(*m.common))

		case rescanFinishedMsg:
			m.stash.rescanInFlight = false
			cwd := m.common.cwd
			m.stash.applyRescan(msg.paths, func(p string) *markdown {
				return pathToMarkdown(cwd, p)
			})
			if m.stash.filterApplied() {
				cmds = append(cmds, filterMarkdowns(m.stash))
			}
			if m.common.cfg.RefreshInterval > 0 {
				cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
			}
```

- [ ] **Step 3: Verify the build**

```bash
cd /Users/nickallevato/workspace/glow && go build ./...
```

Expected: clean build.

- [ ] **Step 4: Run the test suite**

```bash
cd /Users/nickallevato/workspace/glow && go test ./...
```

Expected: PASS.

- [ ] **Step 5: Run the linter**

```bash
cd /Users/nickallevato/workspace/glow && golangci-lint run
```

Expected: clean. If `wrapcheck` or `nestif` complains about the new code, follow the project's existing `//nolint:<linter> // reason` convention to silence it locally — match the style in the codebase, do not turn off the linter wholesale.

- [ ] **Step 6: Pause for review**

Summarize the diff. Suggested commit message:

```
Wire periodic stash rescans into the main Update loop
```

---

## Task 7: Manual smoke test

**Files:** none

- [ ] **Step 1: Build a fresh binary**

```bash
cd /Users/nickallevato/workspace/glow && go build -o /tmp/glow-rescan .
```

- [ ] **Step 2: Set up a temp workspace**

```bash
TMP=$(mktemp -d) && cd "$TMP" && echo "# initial" > a.md && echo $TMP
```

Note the printed path.

- [ ] **Step 3: Run the TUI**

In a new terminal, run:

```bash
/tmp/glow-rescan --refresh-interval 2s "$TMP"  # use the path printed above
```

You should see `a.md` in the stash list.

- [ ] **Step 4: Add a file and confirm it appears within ~2s**

In another terminal:

```bash
cd <the TMP path> && echo "# new" > b.md
```

Expected: within ~2 seconds, `b.md` appears in the stash list without pressing any key.

- [ ] **Step 5: Delete a file and confirm it disappears**

```bash
rm a.md
```

Expected: within ~2 seconds, `a.md` is gone from the list.

- [ ] **Step 6: Confirm `--refresh-interval 0` disables auto-refresh**

Quit (`q`), then re-run:

```bash
/tmp/glow-rescan --refresh-interval 0 "$TMP"
```

Add another file from the other terminal; confirm it does NOT appear automatically, but DOES appear when you press `F`.

- [ ] **Step 7: Confirm cursor preservation**

Restart with `--refresh-interval 2s`, create five files, navigate the cursor to the middle one, and from another terminal create a file with a name that sorts before the cursor's. The cursor should remain on the same document (its on-page index may shift by one).

- [ ] **Step 8: Confirm pager + filter don't churn**

- Open a document with Enter. Edit the underlying directory externally. The pager should not flicker, and re-entering the stash should reflect the new state. (Rescans skip while in the pager; the next tick after returning to the stash will pick up changes.)
- Start a filter with `/` and type a query. From another terminal, create/delete files. The list should not change while you are typing. After hitting Enter / Esc, the next tick incorporates the changes.

- [ ] **Step 9: Pause for review**

Summarize what you observed. If any step misbehaved, capture the discrepancy for the user before suggesting fixes.

Suggested final commit message after all tasks merged:

```
Auto-refresh the TUI stash list on a configurable interval
```

---

## Self-Review Notes

- **Spec coverage:** Config (Task 1) ✓. Diff algorithm steps 1–8 (Tasks 2–4) ✓. Commands & dispatch (Tasks 5–6) ✓. Guardrails — `RefreshInterval == 0`, `rescanInFlight`, `m.state != stateShowStash`, `filterState == filtering`, `viewState != stashStateReady` (Task 6, Step 2) ✓. Silent updates (no status message in Task 6) ✓. Manual `F` unchanged ✓ (no edits to that branch). Testing strategy (Tasks 2–4 unit, Task 7 manual) ✓.
- **No placeholders:** all steps have explicit file paths, full code, exact commands, expected output.
- **Type/name consistency:** `markdownFactory`, `applyRescan`, `restoreCursor`, `filterMarkdownsByPath`, `pathToMarkdown`, `rescanLocalFiles`, `scheduleRescan`, `rescanTickMsg`, `rescanFinishedMsg`, `rescanInFlight` — used identically across tasks.

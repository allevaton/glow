# Default Stash Sort Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--sort` flag / `sort` config key that sets the default ordering of the TUI stash (file list) view.

**Architecture:** Add a `parseSortMode` helper next to the `sortMode` enum, surface the choice through `ui.Config.DefaultSort`, wire a Cobra flag bound to Viper following the existing `refresh-interval` pattern, and initialize `stashModel.sortMode` from config. Invalid values print a warning and fall back to `name`.

**Tech Stack:** Go, Cobra, Viper, Bubble Tea.

---

### Task 1: `parseSortMode` helper + tests

**Files:**
- Modify: `ui/sort.go`
- Test: `ui/sort_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `ui/sort_test.go`:

```go
package ui

import "testing"

func TestParseSortMode(t *testing.T) {
	tests := []struct {
		in      string
		want    sortMode
		wantErr bool
	}{
		{"name", sortByName, false},
		{"modified", sortByModified, false},
		{"Name", sortByName, false},
		{" MODIFIED ", sortByModified, false},
		{"bogus", sortByName, true},
		{"", sortByName, true},
	}
	for _, tt := range tests {
		got, err := parseSortMode(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseSortMode(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("parseSortMode(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ui -run TestParseSortMode -v`
Expected: FAIL — `undefined: parseSortMode`.

- [ ] **Step 3: Write minimal implementation**

In `ui/sort.go`, add `fmt` and `strings` to the import block so it reads:

```go
import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)
```

Then add below the `const` block:

```go
// parseSortMode converts a config/flag string into a sortMode. Matching is
// case-insensitive and surrounding whitespace is ignored. On an unrecognized
// value it returns sortByName along with an error so callers can warn and fall
// back to the default.
func parseSortMode(s string) (sortMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "name":
		return sortByName, nil
	case "modified":
		return sortByModified, nil
	default:
		return sortByName, fmt.Errorf("invalid sort mode %q (valid: name, modified)", s)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ui -run TestParseSortMode -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/sort.go ui/sort_test.go
git commit -m "Add parseSortMode helper for default stash sort"
```

---

### Task 2: Add `DefaultSort` to `ui.Config`

**Files:**
- Modify: `ui/config.go`

- [ ] **Step 1: Add the field**

In `ui/config.go`, add `DefaultSort` to the `Config` struct, after `PreserveNewLines`:

```go
	EnableMouse      bool
	PreserveNewLines bool
	DefaultSort      string
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: builds with no error.

- [ ] **Step 3: Commit**

```bash
git add ui/config.go
git commit -m "Add DefaultSort to ui.Config"
```

---

### Task 3: Initialize `stashModel.sortMode` from config

**Files:**
- Modify: `ui/stash.go` (`newStashModel`, around line 478-499)

- [ ] **Step 1: Set sortMode from config in newStashModel**

In `ui/stash.go`, inside `newStashModel`, after the `m := stashModel{...}` struct literal is assigned, add:

```go
	if mode, err := parseSortMode(common.cfg.DefaultSort); err != nil {
		if common.cfg.DefaultSort != "" {
			fmt.Fprintf(os.Stderr, "glow: %v; defaulting to name\n", err)
		}
		m.sortMode = sortByName
	} else {
		m.sortMode = mode
	}
```

Note: an empty `DefaultSort` (config never set) silently falls back to `sortByName` without a warning; only an explicit invalid value warns.

- [ ] **Step 2: Ensure imports**

Confirm `ui/stash.go` imports both `fmt` and `os`. If either is missing from the import block, add it. (Check with `grep -n '"fmt"\|"os"' ui/stash.go`.)

- [ ] **Step 3: Verify it builds and existing ui tests pass**

Run: `go build ./... && go test ./ui`
Expected: builds; tests pass.

- [ ] **Step 4: Commit**

```bash
git add ui/stash.go
git commit -m "Initialize stash sortMode from config default"
```

---

### Task 4: Wire the `--sort` flag through Cobra + Viper

**Files:**
- Modify: `main.go` — var block (line ~48), `init()` flags/bindings (line ~424-436), `validateOptions` (line ~174-183), `runTUI` (line ~378)

- [ ] **Step 1: Add the package var**

In `main.go`, add to the `var (...)` block after `refreshInterval time.Duration`:

```go
	refreshInterval  time.Duration
	sortFlag         string
```

- [ ] **Step 2: Register the flag and Viper binding**

In `init()`, after the `refresh-interval` flag registration:

```go
	rootCmd.Flags().StringVar(&sortFlag, "sort", "name", "default sort order for the file list: name or modified (TUI-mode only)")
```

After the `refreshInterval` BindPFlag line:

```go
	_ = viper.BindPFlag("sort", rootCmd.Flags().Lookup("sort"))
```

After the existing `viper.SetDefault(...)` calls (near `viper.SetDefault("all", true)`):

```go
	viper.SetDefault("sort", "name")
```

- [ ] **Step 3: Read it in validateOptions**

In `validateOptions`, after `refreshInterval = viper.GetDuration("refreshInterval")`:

```go
	sortFlag = viper.GetString("sort")
```

- [ ] **Step 4: Pass it to the TUI config**

In `runTUI`, after `cfg.RefreshInterval = refreshInterval`:

```go
	cfg.DefaultSort = sortFlag
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: builds with no error.

- [ ] **Step 6: Manually verify the flag is recognized**

Run: `go run . --help`
Expected: output lists `--sort string   default sort order for the file list: name or modified (TUI-mode only) (default "name")`.

- [ ] **Step 7: Commit**

```bash
git add main.go
git commit -m "Add --sort flag and sort config key for default stash sort"
```

---

### Task 5: Document the `sort` config key

**Files:**
- Modify: `README.md` (config example, line ~201-216)

- [ ] **Step 1: Add to the example config**

In the `glow.yml` example block in README.md, add after the `preserveNewLines` line:

```yaml
# preserve newlines in the output
preserveNewLines: false
# default sort order for the file list: "name" or "modified" (TUI-mode only)
sort: "name"
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Document sort config key in README"
```

---

### Task 6: Full verification

- [ ] **Step 1: Run the full suite and lint**

Run: `go build ./... && go test ./... && golangci-lint run`
Expected: build succeeds, all tests pass, lint clean.

- [ ] **Step 2: Manual smoke test**

Run: `go run . --sort modified` in a directory with several markdown files of differing modtimes.
Expected: TUI opens with files ordered most-recently-modified first; pressing `s` toggles to name order.

Run: `go run . --sort bogus`
Expected: a `glow: invalid sort mode "bogus" (valid: name, modified); defaulting to name` warning prints, and the TUI opens sorted by name.

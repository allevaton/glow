# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Glow is a terminal-based markdown reader written in Go. It has two execution modes that share the same binary:

- **CLI mode** — renders markdown to stdout (or through `$PAGER`) via `charmbracelet/glamour`.
- **TUI mode** — a Bubble Tea application that browses and reads local markdown files.

## Common commands

```bash
go build                  # build the `glow` binary
go test ./...             # run all tests
go test ./... -run TestRemoveFrontmatter   # run a single test by name
go test ./ui -v                            # verbose run of a single package
golangci-lint run         # lint (config in .golangci.yml)
task lint | task test     # equivalents via Taskfile.yaml
task log                  # tail the TUI debug log (~/Library/Caches/glow/glow.log on macOS, ~/.cache/glow/glow.log on Linux/Windows)
```

Go 1.25+ (per `go.mod`; the README still says 1.21+ but `go.mod` is authoritative). The repo uses `gofumpt` + `goimports` (configured under `formatters` in `.golangci.yml`).

## Architecture

### Entry & source resolution (`main.go`)

`main.go` wires up the Cobra root command, Viper config, and dispatches between CLI and TUI execution. `sourceFromArg` is the key fan-in: it accepts stdin (`-`), GitHub/GitLab shorthand (handled in `github.go` / `gitlab.go` / `url.go`), HTTP(S) URLs, a directory (walked for a README), or a plain file path, and returns a `source{reader, URL}`. Everything downstream consumes that abstraction.

The `Version` and `CommitSHA` package vars in `main.go` are empty in source — goreleaser injects them via `-ldflags` at release time (see `.goreleaser.yml`).

Mode selection in `execute`:
- stdin piped → CLI
- no args → TUI on CWD
- single dir arg → TUI rooted at that dir
- file/URL/stdin-dash → CLI (with `--tui` / `--pager` flags overriding output)

### Config layering

Three sources, merged by Viper in `tryLoadConfigFromDefaultPlaces` (called from `init`):
1. `glow.yml` discovered via `muesli/go-app-paths`, with `XDG_CONFIG_HOME` and `GLOW_CONFIG_HOME` overrides prepended.
2. Environment variables (`GLOW_*` prefix via Viper; additional `env` struct tags on `ui.Config` for things like `GLAMOUR_STYLE`, `GLOW_HIGH_PERFORMANCE_PAGER`, `GLOW_ENABLE_GLAMOUR`).
3. Cobra flags (bound via `viper.BindPFlag`).

`validateOptions` runs as `PersistentPreRunE` and pulls final values from Viper — when adding a new flag, bind it to Viper here, not just on the flag itself, or it will silently ignore the config file.

### TUI (`ui/`)

Bubble Tea `model` in `ui/ui.go` with a top-level `state` switching between two sub-models:

- `stashModel` (`ui/stash.go`, `stashitem.go`, `stashhelp.go`) — the file browser. Discovers local markdown via `muesli/gitcha` (respects `.gitignore`; `--all` widens the search). Files arrive asynchronously over a channel as `foundLocalFileMsg`.
- `pagerModel` (`ui/pager.go`) — renders a single document inside a `bubbles/viewport`. Watches the file with `fsnotify` for live reload, and (optionally) uses Bubble Tea's high-performance rendering path.

`ui/markdown.go` defines the `markdown` value type that flows between the two sub-models. `ui/keys.go` centralizes key bindings; `ui/styles.go` and `ui/sort.go` hold styling and ordering helpers. `ui/editor.go` shells out to `$EDITOR` via `charmbracelet/x/editor`.

The TUI mode receives its config through `ui.Config` (see `ui/config.go`), which is parsed from env vars via `caarlos0/env` in `main.runTUI` and supplemented with values resolved from flags/Viper.

### Rendering

Markdown rendering goes through `glamour.NewTermRenderer` configured with:
- `utils.GlamourStyle(style, isCode)` — see `utils/utils.go`. Non-markdown files are wrapped as code blocks (`utils.WrapCodeBlock`) and rendered with the code-only style.
- `utils.RemoveFrontmatter` strips YAML/TOML frontmatter before rendering.
- A `baseURL` derived from the source URL so relative links/images resolve correctly when reading remote READMEs.

When stdout is not a TTY and no `--style` was passed, style is forced to `notty` so output is plain.

### Where to make changes

- New CLI flag → `main.go` `init()` + bind to Viper + read in `validateOptions`.
- New source type (e.g. another host shorthand) → extend `sourceFromArg` in `main.go` and add a resolver alongside `github.go` / `gitlab.go`.
- New TUI key → `ui/keys.go`, then handle in the relevant sub-model's `Update`.
- Changes to file discovery rules → `ui/stash.go` (`findLocalFiles`) and the gitcha integration.

## Lint expectations

`golangci-lint` runs with a strict set including `wrapcheck`, `gosec`, `nestif`, `nolintlint`, `unparam`, `revive`, and friends. Existing code uses `//nolint:<linter>` annotations sparingly and with reasons attached — match that style if you need to silence a check.

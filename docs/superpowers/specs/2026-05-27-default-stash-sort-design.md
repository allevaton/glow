# Configurable default stash sort

## Goal

Let users set the default sort order for the TUI stash (file list) view, so it
starts in their preferred order instead of always starting alphabetically by
name. The runtime `s` toggle continues to work, simply starting from the
configured default.

## Configuration surface

- New CLI flag `--sort` (string), default `"name"`.
- Bound to Viper key `sort` so it also works from `glow.yml`, matching the
  existing `refresh-interval` pattern.
- `viper.SetDefault("sort", "name")`.
- Accepted values: `name`, `modified`.

## Error handling

Invalid values are non-fatal. When the configured value does not parse, print a
warning to stderr and fall back to `name`. Glow still launches normally.

## Components & data flow

1. **`ui/sort.go`** — add `parseSortMode(string) (sortMode, error)` mapping
   `"name" → sortByName`, `"modified" → sortByModified`, returning an error for
   anything else. Matching is case-insensitive (the input is lowercased before
   comparison) and surrounding whitespace is trimmed. Keeps the string↔enum
   mapping next to the enum and makes it unit-testable.
2. **`main.go`**
   - Declare a `sortFlag string` package var alongside the other flag vars.
   - Register `--sort` flag (default `"name"`) and `viper.BindPFlag("sort", ...)`.
   - `viper.SetDefault("sort", "name")`.
   - In `validateOptions`, read `sortFlag = viper.GetString("sort")`.
3. **`ui/config.go`** — add `DefaultSort string` to `ui.Config`.
4. **`main.go runTUI`** — `cfg.DefaultSort = sortFlag` (alongside
   `cfg.RefreshInterval`).
5. **`ui/stash.go newStashModel`** — initialize `m.sortMode` from
   `common.cfg.DefaultSort` via `parseSortMode`. On parse error, print a warning
   and use `sortByName`.

## Testing

- Unit test `parseSortMode`:
  - `"name"` → `sortByName`, no error.
  - `"modified"` → `sortByModified`, no error.
  - case-insensitivity / trimming (`"Name"`, `" MODIFIED "`) parse correctly.
  - invalid value (e.g. `"bogus"`) → error returned.

## Docs

- Add `--sort` to the README flags list and note the `sort` config key with its
  accepted values.

## Out of scope

- Additional sort modes (size, path, etc.).
- Ascending/descending control beyond the existing per-mode ordering.
- Persisting a sort chosen at runtime back to config.

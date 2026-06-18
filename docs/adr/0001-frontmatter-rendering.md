# Render frontmatter as a header-stripped two-pass glamour table

Glow now renders YAML frontmatter as a two-column key/value block at the top of a document (default on, opt out with `frontmatter: false` / `--frontmatter=false`) instead of stripping it. We feed the metadata to glamour as a markdown table because, empirically, a table is the *only* markdown construct in which a wrapped value's continuation lines stay aligned under the value column — every other construct (bold-key list, list item, blockquote, code block) strips the leading padding and wraps ragged to the margin.

GFM tables force a header row, which we don't want (frontmatter is just loose key/value pairs). So we render the table in its *own* glamour pass and strip the leading blank-header line and the divider line off the output, then concatenate the result above the separately-rendered body (a single blank line in the seam, no rule). Rendering the table in isolation — rather than one combined pass over `table + body` — keeps the header region trivially locatable, so a stray table inside the body can't be mis-stripped.

## Considered Options

- **Bold-key key/value list with self-computed padding** — themed and header-less, but long values wrap ragged to the margin; leading whitespace on continuation lines is trimmed by markdown. Rejected: ragged wrap was a deal-breaker.
- **Markdown table, keep the empty header** — clean wrapping, zero coupling, but a blank header bar + divider sits on top. Rejected for aesthetics.
- **Custom lipgloss rendering** — full control, but means re-theming a table component per glamour style and re-solving notty; "too much custom rendering."

## Consequences

- The header-strip couples to glamour's table output. Mitigated by: glow pins its glamour version in `go.mod`; the divider is a regex-simple line (`^\s*[─-]+[┼+|]`); golden tests across the built-in styles guard against silent regressions on upgrade.
- Two glamour passes per document instead of one.
- **Values render as prose, not monospace.** An early version wrapped cells in code spans to render them literally, but that made every value monospace. Instead, cell content escapes every `.` to `\.`: glamour draws it as a plain `.`, but it breaks glamour's linkify patterns for emails / URLs / `www.`, which would otherwise be auto-linked into a numbered footer reference that *empties the cell*. The table pass also sets `WithInlineTableLinks(true)` as a safety net (scoped to the table only, so in-body tables are unaffected) so a rare dot-less URL like `http://localhost` degrades to a visible inline link rather than a lost cell. Other markdown in a value is left intact and honored as prose.
- A leading blank line is prepended so the frontmatter has the top padding the body would otherwise have provided.
- v1 is YAML-only; the parser dispatches on fence type and yields a format-agnostic ordered entry list, leaving room for TOML (`+++`) / JSON later without touching the call sites.
- Malformed YAML, a non-mapping top level, or empty frontmatter silently fall back to the old behavior (render body only) — this is a viewing experience, not a linter.

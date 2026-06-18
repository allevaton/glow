# Glow

A terminal markdown reader. This context covers how documents are sourced, parsed, and rendered for display in CLI and TUI modes.

## Language

### Frontmatter

**Frontmatter**:
The metadata block at the very top of a markdown file, delimited by fences. For v1, YAML between `---` fences; the design leaves room for TOML (`+++`) and JSON later.
_Avoid_: Front matter (two words), header, preamble

**Frontmatter table**:
The rendered representation of frontmatter as a two-column key/value markdown table, themed by glamour. It is rendered in its own glamour pass so that its (label-less) header row and divider can be stripped, leaving a clean header-less aligned block, which is then concatenated above the rendered body. The default presentation as of the frontmatter-rendering work.

**Body**:
The document content that follows the frontmatter block, rendered in its own glamour pass. Historically the only thing glow rendered (frontmatter was stripped).

**Entry**:
A single key/value pair from the frontmatter, preserved in document order, format-agnostic so the table builder doesn't care whether it came from YAML, TOML, or JSON.

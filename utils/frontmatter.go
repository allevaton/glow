package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	yaml "go.yaml.in/yaml/v3"
)

// errNotMapping is returned when a document's frontmatter parses but its
// top-level value is a sequence or scalar rather than a key/value mapping —
// there are no keys to lay out in a two-column table.
var errNotMapping = errors.New("frontmatter top level is not a mapping")

// fmEntry is one flattened key/value pair from frontmatter, in document order.
// Nested mappings are flattened into dotted keys; sequences are joined onto a
// single line. The representation is format-agnostic so a future TOML/JSON
// parser can feed the same table builder.
type fmEntry struct {
	key   string
	value string
}

// FrontmatterTableMarkdown turns the YAML frontmatter at the top of content
// into a markdown table (with an empty header row, stripped later from the
// rendered output) and returns it alongside the remaining body.
//
// ok is false when there is nothing useful to render — no frontmatter, an empty
// block, malformed YAML, or a non-mapping top level. In every case the returned
// body has any frontmatter block removed, so callers can render it directly:
// glow is a reader, not a linter, and silently degrades to body-only.
func FrontmatterTableMarkdown(content []byte) (table string, body []byte, ok bool) {
	inner, rest, found := frontmatterParts(content)
	if !found {
		return "", content, false
	}

	entries, err := parseFrontmatter(inner)
	if err != nil || len(entries) == 0 {
		return "", rest, false
	}

	return buildFrontmatterTable(entries), rest, true
}

// frontmatterParts splits content into the inner YAML (between the fences) and
// the body that follows. found is false unless a fence opens at byte 0.
func frontmatterParts(content []byte) (inner, body []byte, found bool) {
	matches := yamlPattern.FindAllIndex(content, 2)
	if len(matches) < 2 || matches[0][0] != 0 {
		return nil, nil, false
	}
	return content[matches[0][1]:matches[1][0]], content[matches[1][1]:], true
}

func parseFrontmatter(inner []byte) ([]fmEntry, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(inner, &doc); err != nil {
		return nil, err //nolint:wrapcheck
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, nil // empty frontmatter
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errNotMapping
	}
	return flattenMapping("", root), nil
}

// flattenMapping walks a mapping node into ordered entries, expanding nested
// mappings into dotted-key rows and collapsing sequences onto one line.
func flattenMapping(prefix string, node *yaml.Node) []fmEntry {
	var entries []fmEntry
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := joinKey(prefix, node.Content[i].Value)
		val := node.Content[i+1]
		switch val.Kind {
		case yaml.MappingNode:
			entries = append(entries, flattenMapping(key, val)...)
		default:
			entries = append(entries, fmEntry{key: sanitizeCell(key), value: sanitizeCell(inlineNode(val))})
		}
	}
	return entries
}

func joinKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// inlineNode renders any node onto a single line: scalars verbatim, sequences
// comma-joined, and (nested-in-a-sequence) mappings as "k: v" pairs.
func inlineNode(node *yaml.Node) string {
	switch node.Kind {
	case yaml.SequenceNode:
		parts := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			parts = append(parts, inlineNode(item))
		}
		return strings.Join(parts, ", ")
	case yaml.MappingNode:
		parts := make([]string, 0, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			parts = append(parts, node.Content[i].Value+": "+inlineNode(node.Content[i+1]))
		}
		return strings.Join(parts, ", ")
	default:
		return node.Value
	}
}

var whitespaceRun = regexp.MustCompile(`\s+`)

// sanitizeCell collapses whitespace (newlines included), escapes the pipe so a
// value can't break the table grid, and escapes dots. The dot escape is what
// keeps values rendering as prose: glamour draws "\." as a plain ".", but it
// breaks the linkify patterns for emails / URLs / "www.", which would otherwise
// be auto-linked into a numbered footer reference that empties the cell. Other
// markdown in a value is left intact (honored as prose).
func sanitizeCell(s string) string {
	s = whitespaceRun.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, ".", `\.`)
	return strings.TrimSpace(s)
}

func buildFrontmatterTable(entries []fmEntry) string {
	var b strings.Builder
	b.WriteString("|  |  |\n| --- | --- |\n")
	for _, e := range entries {
		b.WriteString("| " + e.key + " | " + e.value + " |\n")
	}
	return b.String()
}

var (
	ansiPattern    = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	dividerPattern = regexp.MustCompile(`^\s*[-─]{2,}[┼+|][-─]{2,}\s*$`)
)

// StripRenderedTableHeader removes the empty header row and the divider line
// that glamour emits for the (label-less) frontmatter table, plus any leading
// blank lines, leaving just the aligned key/value rows. If no divider is found
// the input is returned unchanged.
func StripRenderedTableHeader(rendered string) string {
	lines := strings.Split(rendered, "\n")

	divider := -1
	for i, line := range lines {
		if dividerPattern.MatchString(ansiPattern.ReplaceAllString(line, "")) {
			divider = i
			break
		}
	}
	if divider < 1 {
		return rendered
	}

	// Drop the header row (the line above the divider) and the divider itself.
	kept := append(lines[:divider-1:divider-1], lines[divider+1:]...)

	// Trim leading blank lines left behind by the removed header.
	for len(kept) > 0 && strings.TrimSpace(ansiPattern.ReplaceAllString(kept[0], "")) == "" {
		kept = kept[1:]
	}
	return strings.Join(kept, "\n")
}

// RenderWithFrontmatter renders content, turning YAML frontmatter into a
// header-stripped key/value table above the body. The table and body are
// rendered in separate glamour passes (so the table's header region is trivially
// locatable) and joined with a single blank line. When there is no usable
// frontmatter, the body is rendered alone.
//
// opts are the glamour options used for the body; the table pass adds
// WithInlineTableLinks so a bare email/URL value renders inline in its cell
// rather than being pulled out into a numbered footer reference (which would
// leave the cell empty). Scoping it to the table pass keeps in-body table links
// rendering as they always have.
func RenderWithFrontmatter(content []byte, opts []glamour.TermRendererOption) (string, error) {
	table, body, ok := FrontmatterTableMarkdown(content)

	bodyR, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return "", fmt.Errorf("unable to create renderer: %w", err)
	}
	if !ok {
		out, err := bodyR.Render(string(body))
		if err != nil {
			return "", fmt.Errorf("unable to render markdown: %w", err)
		}
		return out, nil
	}

	tableOpts := append(append([]glamour.TermRendererOption{}, opts...), glamour.WithInlineTableLinks(true))
	tableR, err := glamour.NewTermRenderer(tableOpts...)
	if err != nil {
		return "", fmt.Errorf("unable to create frontmatter renderer: %w", err)
	}

	tableOut, err := tableR.Render(table)
	if err != nil {
		return "", fmt.Errorf("unable to render frontmatter: %w", err)
	}
	bodyOut, err := bodyR.Render(string(body))
	if err != nil {
		return "", fmt.Errorf("unable to render markdown: %w", err)
	}
	// A leading blank line gives the frontmatter the same top padding the body
	// would otherwise have had; the single blank line before bodyOut separates
	// the table from the document.
	table = strings.TrimRight(StripRenderedTableHeader(tableOut), "\n")
	return "\n" + table + "\n" + bodyOut, nil
}

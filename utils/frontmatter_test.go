package utils

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
)

func TestFrontmatterTableMarkdown(t *testing.T) {
	tt := []struct {
		name     string
		input    string
		wantOK   bool
		wantRows []string // substrings expected in the table markdown
		wantBody string
	}{
		{
			name:     "scalars",
			input:    "---\nname: grilling\ncount: 3\n---\n# Body\n",
			wantOK:   true,
			wantRows: []string{"| name | grilling |", "| count | 3 |"},
			wantBody: "# Body\n",
		},
		{
			name:     "list joins onto one line",
			input:    "---\ntags:\n  - go\n  - cli\n---\nbody",
			wantOK:   true,
			wantRows: []string{"| tags | go, cli |"},
			wantBody: "body",
		},
		{
			name:     "nested map flattens to dotted keys; dots escaped to disarm autolink",
			input:    "---\nauthor:\n  name: Nick\n  email: a@b.com\n---\nbody",
			wantOK:   true,
			wantRows: []string{`| author\.name | Nick |`, `| author\.email | a@b\.com |`},
			wantBody: "body",
		},
		{
			name:     "multiline value collapses to spaces",
			input:    "---\ndesc: >\n  one\n  two\n---\nbody",
			wantOK:   true,
			wantRows: []string{"| desc | one two |"},
			wantBody: "body",
		},
		{
			name:     "pipe is escaped so it can't break the grid",
			input:    "---\nk: \"a|b\"\n---\nbody",
			wantOK:   true,
			wantRows: []string{`| k | a\|b |`},
			wantBody: "body",
		},
		{
			name:     "no frontmatter",
			input:    "# Just a body\n",
			wantOK:   false,
			wantBody: "# Just a body\n",
		},
		{
			name:     "empty frontmatter strips to body",
			input:    "---\n---\n# Body\n",
			wantOK:   false,
			wantBody: "# Body\n",
		},
		{
			name:     "malformed yaml strips to body",
			input:    "---\nname: : : bad\n  - nope\n---\n# Body\n",
			wantOK:   false,
			wantBody: "# Body\n",
		},
		{
			name:     "non-mapping top level strips to body",
			input:    "---\n- one\n- two\n---\n# Body\n",
			wantOK:   false,
			wantBody: "# Body\n",
		},
		{
			name:     "fence not at byte zero is not frontmatter",
			input:    "intro\n---\nname: x\n---\nbody",
			wantOK:   false,
			wantBody: "intro\n---\nname: x\n---\nbody",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			table, body, ok := FrontmatterTableMarkdown([]byte(tc.input))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if string(body) != tc.wantBody {
				t.Errorf("body = %q, want %q", string(body), tc.wantBody)
			}
			for _, want := range tc.wantRows {
				if !strings.Contains(table, want) {
					t.Errorf("table missing %q\ngot:\n%s", want, table)
				}
			}
		})
	}
}

// TestStripRenderedTableHeader guards the coupling to glamour's table layout:
// across every built-in style the empty header row and divider must be removed,
// leaving the data rows and no leading blank line.
func TestStripRenderedTableHeader(t *testing.T) {
	table := buildFrontmatterTable([]fmEntry{
		{key: "name", value: "grilling"},
		{key: "tags", value: "go, cli"},
	})

	for _, style := range []string{"ascii", "notty", "light", "dark", "dracula"} {
		t.Run(style, func(t *testing.T) {
			r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(60))
			if err != nil {
				t.Fatal(err)
			}
			out, err := r.Render(table)
			if err != nil {
				t.Fatal(err)
			}

			stripped := StripRenderedTableHeader(out)
			clean := ansiPattern.ReplaceAllString(stripped, "")

			if dividerPattern.MatchString(firstNonBlank(clean)) {
				t.Errorf("divider/header survived stripping:\n%s", clean)
			}
			if strings.HasPrefix(stripped, "\n") {
				t.Error("stripped output starts with a blank line")
			}
			for _, want := range []string{"name", "grilling", "tags"} {
				if !strings.Contains(clean, want) {
					t.Errorf("stripped output missing %q:\n%s", want, clean)
				}
			}
		})
	}
}

// TestRenderWithFrontmatterNoLinkLeak ensures an email value is shown literally
// and never auto-linked into a trailing numbered reference.
func TestRenderWithFrontmatterNoLinkLeak(t *testing.T) {
	opts := []glamour.TermRendererOption{glamour.WithStandardStyle("ascii"), glamour.WithWordWrap(80)}
	out, err := RenderWithFrontmatter([]byte("---\nemail: nick@example.com\n---\n# Body\n"), opts)
	if err != nil {
		t.Fatal(err)
	}
	clean := ansiPattern.ReplaceAllString(out, "")
	if !strings.Contains(clean, "nick@example.com") {
		t.Errorf("email value missing from output:\n%s", clean)
	}
	if strings.Contains(clean, "[1]:") {
		t.Errorf("email was auto-linked into a reference:\n%s", clean)
	}
}

func firstNonBlank(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

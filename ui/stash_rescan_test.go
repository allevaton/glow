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

func TestApplyRescan_AddsNewPaths(t *testing.T) {
	m := newTestStash()
	// Seed with /b.md so a missing sortMarkdowns call would leave the
	// result in insertion order (/b.md, /a.md) instead of name order.
	m.addMarkdowns(stubFactory("/b.md"))

	m.applyRescan([]string{"/a.md", "/b.md"}, stubFactory)

	got := pathsOf(m.markdowns)
	want := []string{"/a.md", "/b.md"}
	if len(got) != len(want) {
		t.Fatalf("len(markdowns) = %d, want %d (paths=%v)", len(got), len(want), got)
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("markdowns[%d].localPath = %q, want %q", i, got[i], p)
		}
	}
}

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

	if got := m.cursor(); got != 1 {
		t.Errorf("cursor = %d after removal of selection; want 1 (clamped to last valid index)", got)
	}
	if got := m.selectedMarkdown(); got == nil {
		t.Errorf("selectedMarkdown is nil; want a valid entry after clamp")
	}
}

func TestApplyRescan_RemovesBoundaryElements(t *testing.T) {
	cases := []struct {
		name string
		gone string
		want []string
	}{
		{"first", "/a.md", []string{"/b.md", "/c.md"}},
		{"last", "/c.md", []string{"/a.md", "/b.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestStash()
			m.addMarkdowns(
				stubFactory("/a.md"),
				stubFactory("/b.md"),
				stubFactory("/c.md"),
			)
			var remaining []string
			for _, p := range []string{"/a.md", "/b.md", "/c.md"} {
				if p == tc.gone {
					continue
				}
				remaining = append(remaining, p)
			}

			m.applyRescan(remaining, stubFactory)

			got := pathsOf(m.markdowns)
			if len(got) != len(tc.want) {
				t.Fatalf("len(markdowns) = %d, want %d (paths=%v)", len(got), len(tc.want), got)
			}
			for i, p := range tc.want {
				if got[i] != p {
					t.Errorf("markdowns[%d].localPath = %q, want %q", i, got[i], p)
				}
			}
		})
	}
}

func TestApplyRescan_EmptyBeforeAddsAll(t *testing.T) {
	m := newTestStash()

	m.applyRescan([]string{"/a.md", "/b.md"}, stubFactory)

	got := pathsOf(m.markdowns)
	want := []string{"/a.md", "/b.md"}
	if len(got) != len(want) {
		t.Fatalf("len(markdowns) = %d, want %d (paths=%v)", len(got), len(want), got)
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("markdowns[%d].localPath = %q, want %q", i, got[i], p)
		}
	}
}

func TestApplyRescan_AddsAndRemovesInOneTick(t *testing.T) {
	m := newTestStash()
	m.addMarkdowns(
		stubFactory("/a.md"),
		stubFactory("/b.md"),
	)

	// /a.md gone, /c.md new, /b.md stays.
	m.applyRescan([]string{"/b.md", "/c.md"}, stubFactory)

	got := pathsOf(m.markdowns)
	want := []string{"/b.md", "/c.md"}
	if len(got) != len(want) {
		t.Fatalf("len(markdowns) = %d, want %d (paths=%v)", len(got), len(want), got)
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("markdowns[%d].localPath = %q, want %q", i, got[i], p)
		}
	}
}

func TestApplyRescan_BuildsFilterValueOnAddUnderFilter(t *testing.T) {
	m := newTestStash()
	m.addMarkdowns(stubFactory("/a.md"))
	for _, md := range m.markdowns {
		md.buildFilterValue()
	}
	m.filterState = filterApplied
	m.filteredMarkdowns = append([]*markdown{}, m.markdowns...)

	m.applyRescan([]string{"/a.md", "/b.md"}, stubFactory)

	// The new /b.md should have its filterValue populated so future
	// filtering can rank it.
	var newMd *markdown
	for _, md := range m.markdowns {
		if md.localPath == "/b.md" {
			newMd = md
			break
		}
	}
	if newMd == nil {
		t.Fatalf("/b.md missing from markdowns after rescan: %v", pathsOf(m.markdowns))
	}
	if newMd.filterValue == "" {
		t.Errorf("buildFilterValue not called on /b.md after add under active filter")
	}
}

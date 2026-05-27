package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// sortMode controls how the stash list is ordered.
type sortMode int

const (
	sortByName     sortMode = iota // alphabetical by note (default)
	sortByModified                 // most-recently modified first
)

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

func sortMarkdowns(mds []*markdown, mode sortMode) {
	switch mode {
	case sortByModified:
		slices.SortStableFunc(mds, func(a, b *markdown) int {
			// Descending: newer modtimes come first.
			return b.Modtime.Compare(a.Modtime)
		})
	default:
		slices.SortStableFunc(mds, func(a, b *markdown) int {
			return cmp.Compare(a.Note, b.Note)
		})
	}
}

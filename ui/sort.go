package ui

import (
	"cmp"
	"slices"
)

// sortMode controls how the stash list is ordered.
type sortMode int

const (
	sortByName     sortMode = iota // alphabetical by note (default)
	sortByModified                 // most-recently modified first
)

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

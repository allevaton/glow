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

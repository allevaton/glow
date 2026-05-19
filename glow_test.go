package main

import (
	"testing"

	"github.com/charmbracelet/glamour/styles"
)

func TestGlowFlags(t *testing.T) {
	tt := []struct {
		args  []string
		check func() bool
	}{
		{
			args: []string{"-p"},
			check: func() bool {
				return pager
			},
		},
		{
			args: []string{"-s", "light"},
			check: func() bool {
				return style == "light"
			},
		},
		{
			args: []string{"-w", "40"},
			check: func() bool {
				return width == 40
			},
		},
	}

	for _, v := range tt {
		err := rootCmd.ParseFlags(v.args)
		if err != nil {
			t.Fatal(err)
		}
		if !v.check() {
			t.Errorf("Parsing flag failed: %s", v.args)
		}
	}
}

func TestValidateStyle(t *testing.T) {
	tt := []struct {
		name       string
		in         string
		wantStyle  string
		wantErr    bool
	}{
		{name: "auto passes through", in: "auto", wantStyle: "auto"},
		{name: "named built-in passes through", in: "dark", wantStyle: "dark"},
		{name: "missing style file falls back to auto", in: "/nonexistent/path/to/style.json", wantStyle: styles.AutoStyle},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateStyle(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateStyle(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if got != tc.wantStyle {
				t.Errorf("validateStyle(%q) = %q, want %q", tc.in, got, tc.wantStyle)
			}
		})
	}
}

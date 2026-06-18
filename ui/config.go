package ui

import "time"

// Config contains TUI-specific configuration.
type Config struct {
	ShowAllFiles     bool
	ShowLineNumbers  bool
	Gopath           string `env:"GOPATH"`
	HomeDir          string `env:"HOME"`
	GlamourMaxWidth  uint
	GlamourStyle     string `env:"GLAMOUR_STYLE"`
	EnableMouse      bool
	PreserveNewLines bool
	ShowFrontmatter  bool
	DefaultSort      string

	// Working directory or file path
	Path string

	RefreshInterval time.Duration

	// For debugging the UI
	GlamourEnabled bool `env:"GLOW_ENABLE_GLAMOUR" envDefault:"true"`
}

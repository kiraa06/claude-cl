package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	themeDark  = "dark"
	themeLight = "light"
)

var (
	canvas   lipgloss.Style
	canvasBg lipgloss.TerminalColor
)

func applyTheme(name string) {
	if name == themeLight {
		bg := lipgloss.Color("255")
		dim = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Background(bg)
		faint = lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Background(bg)
		header = lipgloss.NewStyle().Foreground(lipgloss.Color("25")).Background(bg).Bold(true)
		title = lipgloss.NewStyle().Foreground(lipgloss.Color("235")).Background(bg)
		selected = lipgloss.NewStyle().Foreground(lipgloss.Color("17")).Background(lipgloss.Color("153")).Bold(true)
		marker = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(bg).Bold(true)
		accent = lipgloss.NewStyle().Foreground(lipgloss.Color("28")).Background(bg)
		warn = lipgloss.NewStyle().Foreground(lipgloss.Color("130")).Background(bg)
		aiMark = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Background(bg)
		paneBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("245")).
			BorderBackground(bg).
			Background(bg)
		canvas = lipgloss.NewStyle().Foreground(lipgloss.Color("235")).Background(bg)
		canvasBg = bg
		return
	}

	// Dark: original foreground-only palette. No cell backgrounds — the
	// terminal's own background is the canvas, which is what made it read well.
	dim = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "246", Dark: "243"})
	faint = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "239"})
	header = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "24", Dark: "74"}).Bold(true)
	title = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"})
	selected = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "17", Dark: "231"}).Bold(true)
	marker = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "84"}).Bold(true)
	accent = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "84"})
	warn = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "214"})
	aiMark = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "62", Dark: "141"})
	paneBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "248", Dark: "238"})
	canvas = lipgloss.NewStyle()
	canvasBg = lipgloss.NoColor{}
}

func themeFile() string {
	if p := os.Getenv("CL_CONFIG_DIR"); p != "" {
		return filepath.Join(p, "theme")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "cl", "theme")
}

func loadTheme() string {
	if v := os.Getenv("CL_THEME"); v == themeLight || v == themeDark {
		return v
	}
	p := themeFile()
	if p == "" {
		return themeDark
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return themeDark
	}
	s := strings.TrimSpace(string(data))
	if s == themeLight || s == themeDark {
		return s
	}
	return themeDark
}

func saveTheme(name string) {
	p := themeFile()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte(name+"\n"), 0o600)
}

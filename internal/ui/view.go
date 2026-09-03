package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"

	"github.com/kiraa06/claude-cl/internal/group"
)

// Colours are adaptive so the picker reads correctly on light and dark
// terminals without asking which one it is on.
var (
	dim      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "246", Dark: "243"})
	faint    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "239"})
	header   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "24", Dark: "74"}).Bold(true)
	title    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"})
	selected = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "17", Dark: "231"}).Bold(true)
	marker   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "84"}).Bold(true)
	accent   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "84"})
	warn     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "214"})
	aiMark   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "62", Dark: "141"})
)

const (
	previewWidth = 42
	minPreviewAt = 104 // terminal width below which the preview is hidden
	ageColumn    = 5
	modelColumn  = 8
)

// listHeight is how many rows fit, given the fixed chrome above and below.
func (m Model) listHeight() int {
	chrome := 6 // title line, blank, model row, hint row, and padding
	if m.searching || m.status != "" || m.confirming {
		chrome++
	}
	if h := m.height - chrome; h > 3 {
		return h
	}
	return 3
}

// clampOffset scrolls the window the minimum needed to keep the cursor visible.
func (m *Model) clampOffset() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if maxOffset := len(m.rows) - h; m.offset > maxOffset {
		m.offset = max(maxOffset, 0)
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// View renders the picker.
func (m Model) View() string {
	listWidth := m.width
	preview := m.showPreview && m.width >= minPreviewAt
	if preview {
		listWidth = m.width - previewWidth - 3
	}

	var b strings.Builder
	b.WriteString(m.renderTitleBar(listWidth))
	b.WriteString("\n\n")
	b.WriteString(m.renderList(listWidth))

	body := b.String()
	if preview {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(listWidth).Render(body),
			lipgloss.NewStyle().Width(previewWidth).MarginLeft(2).Render(m.renderPreview()),
		)
	}
	return body + "\n" + m.renderFooter()
}

func (m Model) renderTitleBar(width int) string {
	left := accent.Render("cl") + dim.Render(fmt.Sprintf("  %d sessions", len(m.sessions)))
	if m.query != "" {
		shown := 0
		for _, r := range m.rows {
			if r.kind == rowSession {
				shown++
			}
		}
		left += dim.Render(fmt.Sprintf(" · %d matching", shown))
	}
	return left
}

func (m Model) renderList(width int) string {
	if len(m.rows) == 0 {
		return faint.Render("  no sessions")
	}
	h := m.listHeight()
	start := m.offset
	end := min(start+h, len(m.rows))

	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(i, width))
		b.WriteByte('\n')
	}
	if end < len(m.rows) {
		b.WriteString(faint.Render(fmt.Sprintf("  ↓ %d more", len(m.rows)-end)))
	}
	return b.String()
}

func (m Model) renderRow(i, width int) string {
	r := m.rows[i]
	switch r.kind {
	case rowHeader:
		line := header.Render(r.text)
		if i > 0 {
			return "\n" + line
		}
		return line
	case rowNote:
		return faint.Render("    " + r.text)
	case rowNew:
		return m.renderNewRow(i)
	default:
		return m.renderSessionRow(i, width)
	}
}

func (m Model) renderNewRow(i int) string {
	if i == m.cursor {
		return marker.Render("  ▸ ") + selected.Render("New session") +
			dim.Render("  in this directory")
	}
	return "    " + accent.Render("New session")
}

func (m Model) renderSessionRow(i, width int) string {
	r := m.rows[i]
	s := r.session

	// Right-hand metadata is fixed width so titles line up. The path only
	// appears where it adds information, which the HERE section does not.
	meta := fmt.Sprintf("%*s %-*s", ageColumn, age(s.Modified), modelColumn, shortModel(s.Model))
	var path string
	if r.section != group.KindCwd {
		path = group.Abbreviate(s.Cwd)
	}

	titleWidth := width - lipgloss.Width(meta) - 8
	if path != "" {
		titleWidth -= 26
	}
	titleWidth = max(titleWidth, 12)

	name := pad(clip(s.Title, titleWidth), titleWidth)
	line := "    " + title.Render(name)
	if i == m.cursor {
		line = marker.Render("  ▸ ") + selected.Render(name)
	}

	// A dot marks titles Claude wrote itself, as opposed to a first prompt.
	if s.AITitled {
		line += aiMark.Render(" ·")
	} else {
		line += "  "
	}
	if path != "" {
		line += " " + faint.Render(pad(clip(path, 24), 25))
	}
	return line + " " + dim.Render(meta)
}

func (m Model) renderPreview() string {
	r, ok := m.current()
	if !ok || r.kind != rowSession {
		return faint.Render("New session\n\nStarts claude in\n" + group.Abbreviate(m.cwd))
	}
	s := r.session

	var b strings.Builder
	b.WriteString(header.Render(clip(s.Title, previewWidth)))
	b.WriteString("\n")
	b.WriteString(faint.Render(group.Abbreviate(s.Cwd)))
	if s.Branch != "" && s.Branch != "HEAD" {
		b.WriteString(faint.Render("  " + s.Branch))
	}
	b.WriteString("\n")
	b.WriteString(dim.Render(s.Modified.Format("Mon 2 Jan 15:04") + "  " + shortModel(s.Model)))
	b.WriteString("\n\n")

	if len(s.Preview) == 0 {
		b.WriteString(faint.Render("(no preview)"))
		return b.String()
	}
	for _, t := range s.Preview {
		who, style := "you", accent
		if t.Role == "assistant" {
			who, style = "claude", aiMark
		}
		b.WriteString(style.Render(who))
		b.WriteString("\n")
		b.WriteString(dim.Render(indent(wrap(clip(t.Text, 240), previewWidth-2), "  ")))
		b.WriteString("\n\n")
	}
	return b.String()
}

func (m Model) renderFooter() string {
	var lines []string

	if m.confirming {
		if r, ok := m.current(); ok {
			lines = append(lines, warn.Render("delete ")+
				selected.Render(clip(r.session.Title, 48))+
				warn.Render("?  y / n")+
				dim.Render("  (moved to trash, not erased)"))
		}
	} else if m.searching {
		lines = append(lines, accent.Render("search ")+selected.Render(m.query)+marker.Render("▌"))
	} else if m.status != "" {
		lines = append(lines, warn.Render(m.status))
	}

	var models []string
	for i, name := range m.models {
		if i == m.modelIdx {
			models = append(models, marker.Render("‹ ")+selected.Render(name)+marker.Render(" ›"))
			continue
		}
		models = append(models, faint.Render(name))
	}
	lines = append(lines, dim.Render("model  ")+strings.Join(models, "  "))

	hints := "↑↓ move · ←→ model · ⏎ start · f fork · d delete · / search · p preview · q quit"
	if m.searching {
		hints = "type to filter · ↑↓ move · ⏎ start · esc clear"
	}
	lines = append(lines, faint.Render(hints))
	return strings.Join(lines, "\n")
}

// age renders a compact relative time: the list is scanned, not read.
func age(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}

// shortModel reduces a recorded model id to something that fits a column.
// Transcripts also carry pseudo-models for internal records (compaction writes
// "<synthetic>"); those name nothing launchable, so the column stays blank.
func shortModel(id string) string {
	if id == "" || !isModelName(id) {
		return ""
	}
	s := strings.TrimPrefix(id, "claude-")
	if i := strings.Index(s, "-"); i > 0 {
		if _, err := fmt.Sscanf(s[i+1:], "%d", new(int)); err == nil {
			s = s[:i]
		}
	}
	return clip(s, modelColumn)
}

// isModelName reports whether an id looks like a real model identifier.
func isModelName(id string) bool {
	for _, r := range id {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '.' &&
			r != '[' && r != ']' && r != '_' {
			return false
		}
	}
	return true
}

// clip truncates to a rune budget, marking the cut.
func clip(s string, width int) string {
	r := []rune(s)
	if width <= 1 || len(r) <= width {
		return s
	}
	return strings.TrimRight(string(r[:width-1]), " ") + "…"
}

// pad right-pads to a display width.
func pad(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// wrap breaks text into lines of at most width.
func wrap(s string, width int) string {
	if width < 8 {
		width = 8
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// indent prefixes every line, so a wrapped block stays visually one unit.
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

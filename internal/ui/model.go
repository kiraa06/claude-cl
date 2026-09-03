// Package ui implements the session picker.
package ui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kiraa06/claude-cl/internal/group"
	"github.com/kiraa06/claude-cl/internal/launch"
	"github.com/kiraa06/claude-cl/internal/scan"
)

// allCap bounds the global section; the rest stay reachable through search.
const allCap = 40

// Choice is what the picker decided to launch.
type Choice struct {
	Mode    launch.Mode
	Session scan.Session
	Model   string
}

// rowKind distinguishes the flattened list's entries.
type rowKind int

const (
	rowHeader rowKind = iota
	rowNew
	rowSession
	rowNote
)

// row is one line of the flattened list. Headers and notes are rendered but
// never selected, which keeps navigation a simple scan for the next selectable
// row instead of a special case per section.
type row struct {
	kind    rowKind
	text    string
	section group.Kind
	session scan.Session
}

func (r row) selectable() bool { return r.kind == rowNew || r.kind == rowSession }

// Model is the picker's state.
type Model struct {
	sessions  []scan.Session // everything found, unfiltered
	cwd       string
	repoRoot  string
	claudeDir string

	rows   []row
	cursor int
	offset int // first visible row, for scrolling

	models   []string
	modelIdx int
	// modelPinned records that the user chose a model explicitly, which stops
	// the selection following whichever row is highlighted.
	modelPinned bool

	query     string
	searching bool

	showPreview bool
	confirming  bool
	status      string

	width, height int

	// Choice is set when the user picks something; the program then quits and
	// the caller execs it.
	Choice *Choice
}

// New builds a picker over the given sessions.
func New(sessions []scan.Session, cwd, claudeDir string, models []string) Model {
	m := Model{
		sessions:    sessions,
		cwd:         cwd,
		repoRoot:    group.RepoRoot(cwd),
		claudeDir:   claudeDir,
		models:      models,
		showPreview: true,
		width:       100,
		height:      30,
	}
	m.rebuild()
	m.syncModelToCursor()
	return m
}

// rebuild recomputes the flattened rows from the current query, keeping the
// cursor on a selectable row.
func (m *Model) rebuild() {
	visible := scan.Filter(m.sessions, m.query)
	sections := group.Build(visible, m.cwd, m.repoRoot, allCap)

	rows := make([]row, 0, len(visible)+len(sections)+2)
	for _, sec := range sections {
		rows = append(rows, row{kind: rowHeader, text: sec.Label(), section: sec.Kind})
		if sec.Kind == group.KindCwd {
			rows = append(rows, row{kind: rowNew, text: "New session", section: sec.Kind})
		}
		for _, s := range sec.Sessions {
			rows = append(rows, row{kind: rowSession, session: s, section: sec.Kind})
		}
		if sec.Hidden > 0 {
			rows = append(rows, row{
				kind:    rowNote,
				text:    plural(sec.Hidden, "more session") + " — press / to search",
				section: sec.Kind,
			})
		}
	}
	m.rows = rows

	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	if m.cursor < 0 || !m.rows[max(m.cursor, 0)].selectable() {
		m.cursor = m.firstSelectable()
	}
	m.clampOffset()
}

func (m Model) firstSelectable() int {
	for i, r := range m.rows {
		if r.selectable() {
			return i
		}
	}
	return 0
}

// move steps the cursor by delta, skipping headers and notes and stopping at
// the ends rather than wrapping.
func (m *Model) move(delta int) {
	i := m.cursor
	for {
		i += delta
		if i < 0 || i >= len(m.rows) {
			return // no selectable row that way; stay put
		}
		if m.rows[i].selectable() {
			m.cursor = i
			m.syncModelToCursor()
			m.clampOffset()
			return
		}
	}
}

// syncModelToCursor points the footer at the highlighted session's own model,
// so resuming keeps the model the conversation was using. An explicit choice by
// the user takes precedence.
func (m *Model) syncModelToCursor() {
	if m.modelPinned || m.cursor >= len(m.rows) {
		return
	}
	r := m.rows[m.cursor]
	if r.kind != rowSession {
		m.modelIdx = 0 // a new session uses the configured default
		return
	}
	if alias := launch.Alias(r.session.Model); alias != "" {
		if i := launch.IndexOf(m.models, alias); i >= 0 {
			m.modelIdx = i
		}
	}
}

func (m *Model) cycleModel(delta int) {
	if len(m.models) == 0 {
		return
	}
	m.modelIdx = (m.modelIdx + delta + len(m.models)) % len(m.models)
	m.modelPinned = true
}

func (m Model) currentModel() string {
	if m.modelIdx < len(m.models) {
		return m.models[m.modelIdx]
	}
	return ""
}

// current returns the highlighted row.
func (m Model) current() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func (m Model) Init() tea.Cmd { return nil }

// Update handles one message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Some terminals report zero before settling; keep the last good size
		// rather than collapsing the list to its minimum.
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		m.clampOffset()
		return m, nil
	case tea.KeyMsg:
		if m.confirming {
			return m.updateConfirm(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

// updateConfirm handles the delete confirmation prompt, which swallows every
// other key so a stray keystroke cannot both confirm and act.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirming = false
		r, ok := m.current()
		if !ok || r.kind != rowSession {
			return m, nil
		}
		dest, err := scan.Trash(m.claudeDir, r.session)
		if err != nil {
			m.status = "could not delete: " + err.Error()
			return m, nil
		}
		m.removeSession(r.session.ID)
		m.status = "moved to " + filepath.Join(scan.TrashDir, filepath.Base(dest))
		return m, nil
	default:
		m.confirming = false
		m.status = ""
		return m, nil
	}
}

// updateSearch handles keys while the search field has focus.
func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching, m.query, m.status = false, "", ""
		m.rebuild()
		return m, nil
	case "enter":
		return m.launch(launch.Resume)
	case "up", "ctrl+p":
		m.move(-1)
		return m, nil
	case "down", "ctrl+n":
		m.move(1)
		return m, nil
	case "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.rebuild()
		}
		return m, nil
	case "ctrl+u":
		m.query = ""
		m.rebuild()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.cycleModel(1)
		return m, nil
	case "shift+tab":
		m.cycleModel(-1)
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.query += string(msg.Runes)
		if msg.Alt {
			return m, nil
		}
		m.rebuild()
	}
	return m, nil
}

// updateNormal handles keys in the default navigation mode.
func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k", "ctrl+p":
		m.move(-1)
	case "down", "j", "ctrl+n":
		m.move(1)
	case "left", "h", "shift+tab":
		m.cycleModel(-1)
	case "right", "l", "tab":
		m.cycleModel(1)
	case "g", "home":
		m.cursor = m.firstSelectable()
		m.syncModelToCursor()
		m.clampOffset()
	case "G", "end":
		for i := len(m.rows) - 1; i >= 0; i-- {
			if m.rows[i].selectable() {
				m.cursor = i
				break
			}
		}
		m.syncModelToCursor()
		m.clampOffset()
	case "/":
		m.searching, m.status = true, ""
	case "p":
		m.showPreview = !m.showPreview
	case "enter":
		return m.launch(launch.Resume)
	case "f":
		if r, ok := m.current(); ok && r.kind == rowSession {
			return m.launch(launch.Fork)
		}
		m.status = "fork needs an existing session"
	case "d":
		if r, ok := m.current(); ok && r.kind == rowSession {
			m.confirming = true
		} else {
			m.status = "nothing to delete"
		}
	}
	return m, nil
}

// launch records the choice and stops the program. The row decides between a
// new session and a resume, so Enter always does the obvious thing.
func (m Model) launch(mode launch.Mode) (tea.Model, tea.Cmd) {
	r, ok := m.current()
	if !ok {
		return m, nil
	}
	if r.kind == rowNew {
		mode = launch.New
	}
	m.Choice = &Choice{Mode: mode, Session: r.session, Model: m.currentModel()}
	return m, tea.Quit
}

// removeSession drops a deleted session and rebuilds the list.
func (m *Model) removeSession(id string) {
	kept := make([]scan.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.ID != id {
			kept = append(kept, s)
		}
	}
	m.sessions = kept
	m.rebuild()
}

// SetQuery opens the picker already filtered, as when a query is passed on the
// command line.
func (m *Model) SetQuery(q string) {
	m.query = q
	m.searching = true
	m.rebuild()
	m.syncModelToCursor()
}

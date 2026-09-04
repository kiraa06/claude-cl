package scan

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Meta is the raw result of reading a transcript's windows.
type Meta struct {
	Title        string // best human prompt, cleaned
	AITitle      string // Claude's own ai-title record, if any
	Cwd          string
	Branch       string
	Model        string
	Preview      []Turn
	ForkedFrom   string // parent session id, when this transcript was forked
	CustomTitle  string // user-renamed title, if any
	AgentName    string // Claude's branch/clone label
	ContinuedIn  string // session this one was continued in
	RelocatedCwd string // cwd after a move, newer than the recorded cwd
}

// record is the subset of a transcript line the picker cares about.
type record struct {
	Type         string `json:"type"`
	AITitle      string `json:"aiTitle"`
	CustomTitle  string `json:"customTitle"`
	AgentName    string `json:"agentName"`
	Cwd          string `json:"cwd"`
	GitBranch    string `json:"gitBranch"`
	RelocatedCwd string `json:"relocatedCwd"`
	ContinuedIn  string `json:"continuedInSessionId"`
	Origin       *struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	ForkedFrom struct {
		SessionID string `json:"sessionId"`
	} `json:"forkedFrom"`
	Message struct {
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// Byte prefilters. Unmarshalling every line of a 63MB transcript would dominate
// startup, so a line is only parsed when it contains a marker that could matter.
var (
	markAITitle   = []byte(`"type":"ai-title"`)
	markAssistant = []byte(`"type":"assistant"`)
	markUser      = []byte(`"type":"user"`)
	markCwd       = []byte(`"cwd":"`)
	markForked    = []byte(`"forkedFrom"`)
	markCustom    = []byte(`"type":"custom-title"`)
	markAgent     = []byte(`"type":"agent-name"`)
	markRelocated = []byte(`"type":"relocated"`)
	markContinued = []byte(`"type":"continued-in"`)
)

// cloneMark is the character Claude appends to cloned/branched session names.
const cloneMark = '\u2442'

// Parse extracts display metadata from a transcript's head and tail windows.
// tail may be nil, meaning head holds the whole file.
//
// Head supplies the first human prompt and the working directory; tail supplies
// the newest ai-title, the model last used, and the preview turns.
func Parse(head, tail []byte) Meta {
	var m Meta

	var candidates []string
	forEachLine(head, func(line []byte) {
		if m.Cwd == "" && bytes.Contains(line, markCwd) {
			if r, ok := decode(line); ok {
				m.Cwd, m.Branch = r.Cwd, r.GitBranch
			}
		}
		if len(candidates) < titleCandidates && bytes.Contains(line, markUser) {
			if r, ok := decode(line); ok && isHumanTurn(r) {
				if t := CleanTitle(textOf(r.Message.Content)); t != "" {
					candidates = append(candidates, t)
				}
			}
		}
		if m.ForkedFrom == "" && bytes.Contains(line, markForked) {
			if r, ok := decode(line); ok && r.ForkedFrom.SessionID != "" {
				m.ForkedFrom = r.ForkedFrom.SessionID
			}
		}
	})
	m.Title = bestTitle(candidates)

	// The end of the conversation lives in tail, or in head when read whole.
	end := tail
	if end == nil {
		end = head
	}
	forEachLine(end, func(line []byte) {
		switch {
		case bytes.Contains(line, markAITitle):
			if r, ok := decode(line); ok && r.AITitle != "" {
				m.AITitle = r.AITitle // keep the last, it is the freshest
			}
		case bytes.Contains(line, markCustom):
			if r, ok := decode(line); ok && r.CustomTitle != "" {
				m.CustomTitle = r.CustomTitle
			}
		case bytes.Contains(line, markAgent):
			if r, ok := decode(line); ok && r.AgentName != "" {
				m.AgentName = r.AgentName
			}
		case bytes.Contains(line, markRelocated):
			if r, ok := decode(line); ok && r.RelocatedCwd != "" {
				m.RelocatedCwd = r.RelocatedCwd
			}
		case bytes.Contains(line, markContinued):
			if r, ok := decode(line); ok && r.ContinuedIn != "" {
				m.ContinuedIn = r.ContinuedIn
			}
		case bytes.Contains(line, markAssistant):
			if r, ok := decode(line); ok && r.Type == "assistant" {
				if m.Cwd == "" {
					m.Cwd, m.Branch = r.Cwd, r.GitBranch
				}
				if r.Message.Model != "" {
					m.Model = r.Message.Model
				}
				if t := CleanTitle(textOf(r.Message.Content)); t != "" {
					m.Preview = appendTurn(m.Preview, Turn{"assistant", t})
				}
			}
		case bytes.Contains(line, markUser):
			if r, ok := decode(line); ok && isHumanTurn(r) {
				if t := CleanTitle(textOf(r.Message.Content)); t != "" {
					m.Preview = appendTurn(m.Preview, Turn{"user", t})
				}
			}
		}
	})
	return m
}

// appendTurn keeps only the most recent previewTurns exchanges.
func appendTurn(turns []Turn, t Turn) []Turn {
	turns = append(turns, t)
	if len(turns) > previewTurns {
		turns = turns[1:]
	}
	return turns
}

// skillName reduces a skill's directory to the skill's own name. The manual can
// begin on the same line, so anything from a markdown heading onwards is cut
// before taking the final path segment.
func skillName(dir string) string {
	if i := strings.Index(dir, " #"); i >= 0 {
		dir = dir[:i]
	}
	dir = strings.TrimRight(strings.TrimSpace(dir), string(filepath.Separator))
	if dir == "" || dir == "." {
		return ""
	}
	return filepath.Base(dir)
}

// injectedPrompts are phrases Claude Code writes into a user record on the
// user's behalf — interruptions, skill-reload notices, resumed-conversation
// banners. They read as prose, so isPasted does not catch them, and they say
// nothing about what the session was for.
var injectedPrompts = []string{
	"[Request interrupted by user",
	"is already loaded above; instructions unchanged",
	"Caveat: The messages below were generated by the user",
	"This session is being continued from a previous conversation",
	"Please continue the conversation from where we left it off",
}

func isInjected(s string) bool {
	for _, p := range injectedPrompts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// structuredLabels are the prefixes CleanTitle uses for prompts it recognised
// and rewrote. They describe the session even when they are short, so they skip
// the minimum-length rule.
var structuredLabels = []string{"skill: ", "scheduled: "}

func isStructured(s string) bool {
	for _, p := range structuredLabels {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// bestTitle picks the prompt that best describes a session.
//
// Sessions often open with an ack or a one-word nudge ("ok", "resume", "Yoo!")
// and carry the real request a turn or two later, so short prompts are passed
// over. Candidates are ranked, never dropped: a typed prose prompt first, then
// a recognised label such as "skill: prom", then pasted material — credentials,
// URLs, stack traces — then text Claude Code injected itself, and finally the
// very first candidate, so a title is empty only when there were no prompts.
func bestTitle(candidates []string) string {
	var structured, pasted, injected string
	for _, c := range candidates {
		// A recognised label ("skill: prom") describes the session, but a
		// prompt the user actually typed describes it better, so keep looking.
		if isStructured(c) {
			if structured == "" {
				structured = c
			}
			continue
		}
		if len([]rune(c)) < minTitleLen {
			continue
		}
		if isInjected(c) {
			if injected == "" {
				injected = c
			}
			continue
		}
		if !isPasted(c) {
			return c
		}
		if pasted == "" {
			pasted = c
		}
	}
	// Nothing is discarded, only ranked: a session always keeps some title.
	for _, fallback := range []string{structured, pasted, injected} {
		if fallback != "" {
			return fallback
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// isPasted reports whether a prompt looks like pasted material rather than
// something typed as a sentence.
func isPasted(s string) bool {
	if strings.Contains(s, "://") || strings.Contains(s, "=") {
		return true
	}
	var letters, spaces int
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters++
		case unicode.IsSpace(r):
			spaces++
		}
	}
	n := len([]rune(s))
	// Prose is mostly letters and broken up by spaces; config dumps, paths and
	// logs are neither.
	return letters*10 < n*6 || spaces*20 < n
}

func decode(line []byte) (record, bool) {
	var r record
	if err := json.Unmarshal(line, &r); err != nil {
		return record{}, false
	}
	return r, true
}

// isHumanTurn reports whether a user record is a real prompt.
//
// Claude Code started writing origin.kind:"human" around 2.1.156. Older
// transcripts omit origin or set it to null; those are still human turns.
// Known non-human kinds (task-notification and the rest) stay excluded.
func isHumanTurn(r record) bool {
	if r.Type != "user" {
		return false
	}
	if r.Origin == nil || r.Origin.Kind == "" || r.Origin.Kind == "human" {
		return true
	}
	return false
}

func forEachLine(b []byte, fn func([]byte)) {
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			fn(b)
			return
		}
		if i > 0 {
			fn(b[:i])
		}
		b = b[i+1:]
	}
}

// textOf pulls plain text out of a message content field, which is either a
// bare string or an array of typed blocks.
func textOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == "text" && blk.Text != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

// Claude Code injects wrapper elements into prompts (system reminders, command
// output, hook results) that carry no human intent. Their whole contents are
// dropped. RE2 has no backreferences, so each wrapper gets its own pattern;
// content inside unrecognised tags is kept rather than guessed at.
var wrapperBlocks = func() []*regexp.Regexp {
	names := []string{
		"system-reminder", "local-command-caveat", "local-command-stdout",
		"command-name", "command-args", "command-message", "command-stdout",
		"user-prompt-submit-hook", "task-notification", "function_calls",
		"function_results", "thinking", "EXTREMELY_IMPORTANT",
	}
	rs := make([]*regexp.Regexp, len(names))
	for i, n := range names {
		rs[i] = regexp.MustCompile(`(?s)<` + n + `(?:\s[^>]*)?>.*?</` + n + `>`)
	}
	return rs
}()

// loneTag strips any tag markup left over once wrapper blocks are gone.
var loneTag = regexp.MustCompile(`</?[a-zA-Z][\w-]*(?:\s[^>]*)?/?>`)

// scheduledTask captures the name of a scheduled run, whose prompt is one big
// wrapper element. The task name describes the session better than anything in
// the body does.
var scheduledTask = regexp.MustCompile(`<scheduled-task[^>]*\bname="([^"]+)"`)

// skillPreamble captures the directory from the block Claude Code prepends when
// a skill is invoked. The whole manual follows on later lines, so only the first
// line is considered.
var skillPreamble = regexp.MustCompile(`^\s*Base directory for this skill:[ \t]*([^\n]+)`)

// CleanTitle reduces a prompt to a single readable line fit for a list row.
func CleanTitle(s string) string {
	if m := scheduledTask.FindStringSubmatch(s); m != nil {
		return "scheduled: " + truncate(m[1], titleMaxLen-11)
	}
	if m := skillPreamble.FindStringSubmatch(s); m != nil {
		if name := skillName(m[1]); name != "" {
			return "skill: " + truncate(name, titleMaxLen-7)
		}
	}
	for _, re := range wrapperBlocks {
		s = re.ReplaceAllString(s, " ")
	}
	s = loneTag.ReplaceAllString(s, " ")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	return truncate(s, titleMaxLen)
}

// stripCloneMark removes the trailing branch marker Claude adds to clones.
func stripCloneMark(s string) string {
	return strings.TrimSpace(strings.TrimRightFunc(s, func(r rune) bool {
		return r == cloneMark || unicode.IsSpace(r)
	}))
}

func hasCloneMark(s string) bool {
	for _, r := range s {
		if r == cloneMark {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimRight(string(r[:max-1]), " ") + "…"
}

// deepScan streams further into a transcript than the head window reaches,
// looking for a prompt worth using as a title. Only sessions whose opening
// prompts were missing or too short pay this cost. Bounded so a huge
// transcript cannot stall startup.
func deepScan(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, deepScanLimit)
	n, _ := io.ReadFull(f, buf)
	if n == 0 {
		return ""
	}

	var candidates []string
	forEachLine(dropPartialTail(buf[:n]), func(line []byte) {
		if len(candidates) >= titleCandidates {
			return
		}
		if !bytes.Contains(line, markUser) {
			return
		}
		if r, ok := decode(line); ok && isHumanTurn(r) {
			if t := CleanTitle(textOf(r.Message.Content)); t != "" {
				candidates = append(candidates, t)
			}
		}
	})
	return bestTitle(candidates)
}

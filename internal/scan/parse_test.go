package scan

import (
	"strings"
	"testing"
)

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "why did the pod restart?", "why did the pod restart?"},
		{"newlines collapse", "fix the\n\nbuild\tplease", "fix the build please"},
		{"strips system reminder",
			"<system-reminder>ignore me</system-reminder>real question",
			"real question"},
		{"strips command block",
			"<command-name>/model</command-name><command-args></command-args>set opus",
			"set opus"},
		{"strips unclosed tag", "<local-command-caveat>text after", "text after"},
		{"unicode preserved", "why is café.js failing? 日本語", "why is café.js failing? 日本語"},
		{"only tags yields empty", "<system-reminder>all of it</system-reminder>", ""},
		{"empty", "", ""},
		{"control chars dropped", "hello\x00\x07world", "helloworld"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CleanTitle(tc.in); got != tc.want {
				t.Errorf("CleanTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCleanTitleTruncates(t *testing.T) {
	got := CleanTitle(strings.Repeat("a", 200))
	if len([]rune(got)) != titleMaxLen {
		t.Errorf("length = %d, want %d", len([]rune(got)), titleMaxLen)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("want ellipsis suffix, got %q", got)
	}
}

// Records mirror the real transcript shapes observed on disk.
const (
	recCwd       = `{"type":"user","origin":{"kind":"human"},"message":{"role":"user","content":"first prompt"},"cwd":"/Users/kj/repo","gitBranch":"main"}`
	recAssistant = `{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-5","content":[{"type":"thinking","thinking":"hm"},{"type":"text","text":"here is the answer"}]}}`
	recAITitle   = `{"type":"ai-title","aiTitle":"Pod restart investigation","sessionId":"x"}`
	recAITitle2  = `{"type":"ai-title","aiTitle":"Newer title wins","sessionId":"x"}`
	recNoise     = `{"type":"mode","mode":"normal"}`
)

func TestParseWholeFile(t *testing.T) {
	head := []byte(strings.Join([]string{
		recNoise, recCwd, recAssistant, recAITitle, recAITitle2,
	}, "\n") + "\n")

	m := Parse(head, nil)

	if m.AITitle != "Newer title wins" {
		t.Errorf("AITitle = %q, want the last record's title", m.AITitle)
	}
	if m.Title != "first prompt" {
		t.Errorf("Title = %q, want %q", m.Title, "first prompt")
	}
	if m.Cwd != "/Users/kj/repo" || m.Branch != "main" {
		t.Errorf("Cwd/Branch = %q/%q", m.Cwd, m.Branch)
	}
	if m.Model != "claude-sonnet-5" {
		t.Errorf("Model = %q", m.Model)
	}
	if len(m.Preview) != 2 {
		t.Fatalf("Preview has %d turns, want 2: %+v", len(m.Preview), m.Preview)
	}
	if m.Preview[1].Role != "assistant" || m.Preview[1].Text != "here is the answer" {
		t.Errorf("preview tail = %+v, want assistant text block extracted", m.Preview[1])
	}
}

func TestParseSeparateWindows(t *testing.T) {
	head := []byte(recCwd + "\n")
	tail := []byte(recAITitle + "\n" + recAssistant + "\n")

	m := Parse(head, tail)

	if m.Title != "first prompt" {
		t.Errorf("Title = %q, want head-derived prompt", m.Title)
	}
	if m.AITitle != "Pod restart investigation" {
		t.Errorf("AITitle = %q, want tail-derived title", m.AITitle)
	}
}

func TestParseNoAITitleFallsBackToPrompt(t *testing.T) {
	m := Parse([]byte(recCwd+"\n"), []byte(recAssistant+"\n"))
	if m.AITitle != "" {
		t.Errorf("AITitle = %q, want empty", m.AITitle)
	}
	if m.Title != "first prompt" {
		t.Errorf("Title = %q, want fallback prompt", m.Title)
	}
}

func TestParseNoHumanTurn(t *testing.T) {
	m := Parse([]byte(recNoise+"\n"+recAssistant+"\n"), nil)
	if m.Title != "" || m.AITitle != "" {
		t.Errorf("want no title for a session with no human turn, got %q/%q", m.Title, m.AITitle)
	}
}

func TestParseIgnoresMalformedLines(t *testing.T) {
	head := []byte("{not json at all\n" + recCwd + "\n" + `{"type":"user",` + "\n")
	m := Parse(head, nil)
	if m.Title != "first prompt" {
		t.Errorf("Title = %q; malformed neighbours must not break parsing", m.Title)
	}
}

func TestParseHandlesEmptyInput(t *testing.T) {
	if m := Parse(nil, nil); m.Title != "" || m.Model != "" {
		t.Errorf("empty input should yield zero Meta, got %+v", m)
	}
}

func TestPreviewCapped(t *testing.T) {
	var lines []string
	for range previewTurns * 3 {
		lines = append(lines, recAssistant)
	}
	m := Parse([]byte(strings.Join(lines, "\n")+"\n"), nil)
	if len(m.Preview) != previewTurns {
		t.Errorf("Preview = %d turns, want cap of %d", len(m.Preview), previewTurns)
	}
}

func TestDropPartialWindows(t *testing.T) {
	if got := string(dropPartialTail([]byte("a\nb\ncut"))); got != "a\nb\n" {
		t.Errorf("dropPartialTail = %q", got)
	}
	if got := string(dropPartialHead([]byte("cut\nb\nc"))); got != "b\nc" {
		t.Errorf("dropPartialHead = %q", got)
	}
	if got := dropPartialTail([]byte("no newline")); got != nil {
		t.Errorf("dropPartialTail with no newline = %q, want nil", got)
	}
	if got := dropPartialHead([]byte("no newline")); got != nil {
		t.Errorf("dropPartialHead with no newline = %q, want nil", got)
	}
}

func TestBestTitleSkipsAcks(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"skips greeting", []string{"Yoo!", "set up the Jenkins batch job credentials"},
			"set up the Jenkins batch job credentials"},
		{"skips ack chain", []string{"ok", "hi", "why is the deployment failing on stage?"},
			"why is the deployment failing on stage?"},
		{"first is already good", []string{"why did the pod restart?", "and the logs?"},
			"why did the pod restart?"},
		{"all short falls back to first", []string{"ls", "hi"}, "ls"},
		{"empty", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := bestTitle(tc.in); got != tc.want {
				t.Errorf("bestTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseCwdFromTailWhenHeadIsAllHooks(t *testing.T) {
	// A session opening with large hook attachments pushes every cwd-bearing
	// record out of the head window; resume still needs the directory.
	head := []byte(recNoise + "\n")
	tail := []byte(`{"type":"assistant","cwd":"/Users/kj/repo","gitBranch":"main","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"hi"}]}}` + "\n")

	m := Parse(head, tail)
	if m.Cwd != "/Users/kj/repo" || m.Branch != "main" {
		t.Errorf("Cwd/Branch = %q/%q, want them recovered from tail", m.Cwd, m.Branch)
	}
}

func TestParsePrefersHeadCwd(t *testing.T) {
	head := []byte(recCwd + "\n")
	tail := []byte(`{"type":"assistant","cwd":"/somewhere/else","message":{"role":"assistant","model":"m","content":"x"}}` + "\n")
	if m := Parse(head, tail); m.Cwd != "/Users/kj/repo" {
		t.Errorf("Cwd = %q, want the head value to win", m.Cwd)
	}
}

func TestIsPasted(t *testing.T) {
	pasted := []string{
		"JENKINS_INFRA_URL=https://jenkins.moveinsync.com JENKINS_USER=tools",
		"https://jenkins.moveinsync.com/job/DevOps/job/MIS-Deployer/job/batch/",
		"/Users/kj/repo/internal/scan/parse.go:117:22:undefined:foo",
	}
	for _, s := range pasted {
		if !isPasted(s) {
			t.Errorf("isPasted(%q) = false, want true", s)
		}
	}
	prose := []string{
		"why did the ets deployment pod restart last night?",
		"add a retry with backoff to the webhook sender please",
		"can you look into the failing stage deploy",
	}
	for _, s := range prose {
		if isPasted(s) {
			t.Errorf("isPasted(%q) = true, want false", s)
		}
	}
}

func TestBestTitlePrefersProseOverPaste(t *testing.T) {
	in := []string{
		"Yoo!",
		"JENKINS_INFRA_URL=https://jenkins.moveinsync.com JENKINS_USER=tools",
		"look into the deployment jobs that were triggered yesterday",
	}
	want := "look into the deployment jobs that were triggered yesterday"
	if got := bestTitle(in); got != want {
		t.Errorf("bestTitle = %q, want %q", got, want)
	}
}

func TestBestTitleFallsBackToPasteWhenNoProse(t *testing.T) {
	in := []string{"ok", "https://jenkins.moveinsync.com/job/DevOps/job/batch/"}
	want := "https://jenkins.moveinsync.com/job/DevOps/job/batch/"
	if got := bestTitle(in); got != want {
		t.Errorf("bestTitle = %q, want %q", got, want)
	}
}

func TestCleanTitleHandlesTagsWithAttributes(t *testing.T) {
	// The stripper must cope with attributes, not just bare tags.
	in := `<system-reminder priority="high">noise here</system-reminder>the real ask`
	if got := CleanTitle(in); got != "the real ask" {
		t.Errorf("CleanTitle = %q, want %q", got, "the real ask")
	}
}

func TestCleanTitleNamesScheduledTasks(t *testing.T) {
	in := `<scheduled-task name="daily-prod-cost-routine" id="7">run the cost routine now</scheduled-task>`
	want := "scheduled: daily-prod-cost-routine"
	if got := CleanTitle(in); got != want {
		t.Errorf("CleanTitle = %q, want %q", got, want)
	}
}

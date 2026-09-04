package scan

import (
	"os"
	"path/filepath"
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
	recForked    = `{"type":"attachment","forkedFrom":{"sessionId":"e8bcd8da-8714-4c09-b450-0b9f0e97e994","messageUuid":"76fd993e-bdc1-442c-9153-8e763c4730ef"},"sessionId":"1f87bae3-e778-4aa6-9f70-fb6e94c5e916"}`
	recParentMsg = `{"type":"user","parentUuid":"not-a-session","origin":{"kind":"human"},"message":{"role":"user","content":"forked prompt that is long enough"},"cwd":"/Users/kj/repo"}`
	recCustom    = `{"type":"custom-title","customTitle":"tldraw offline access","sessionId":"x"}`
	recAgent     = `{"type":"agent-name","agentName":"Fix malformed JSON in Claude settings ⑂","sessionId":"x"}`
	recRelocated = `{"type":"relocated","relocatedCwd":"/new/place","sessionId":"x"}`
	recContinued = `{"type":"continued-in","continuedInSessionId":"child-id","sessionId":"x"}`
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
	if m.ForkedFrom != "" {
		t.Errorf("ForkedFrom = %q, want empty on an unforked transcript", m.ForkedFrom)
	}
}

func TestParseForkedFrom(t *testing.T) {
	head := []byte(strings.Join([]string{recForked, recCwd, recParentMsg}, "\n") + "\n")
	m := Parse(head, nil)
	if m.ForkedFrom != "e8bcd8da-8714-4c09-b450-0b9f0e97e994" {
		t.Errorf("ForkedFrom = %q, want the parent session id", m.ForkedFrom)
	}
}

func TestParseCustomTitleAndRelocated(t *testing.T) {
	head := []byte(strings.Join([]string{recCwd, recCustom, recRelocated, recContinued, recAgent}, "\n") + "\n")
	m := Parse(head, nil)
	if m.CustomTitle != "tldraw offline access" {
		t.Errorf("CustomTitle = %q", m.CustomTitle)
	}
	if m.RelocatedCwd != "/new/place" {
		t.Errorf("RelocatedCwd = %q", m.RelocatedCwd)
	}
	if m.ContinuedIn != "child-id" {
		t.Errorf("ContinuedIn = %q", m.ContinuedIn)
	}
	if m.AgentName != "Fix malformed JSON in Claude settings ⑂" {
		t.Errorf("AgentName = %q", m.AgentName)
	}
}

func TestDisplayTitlePrefersCustomThenStripsCloneMark(t *testing.T) {
	got, titled := displayTitle(Meta{CustomTitle: "my name", AITitle: "ai ⑂", Title: "prompt"})
	if got != "my name" || titled {
		t.Errorf("custom: %q %v, want untitled (user-renamed)", got, titled)
	}
	got, titled = displayTitle(Meta{AITitle: "Fix malformed JSON in Claude settings ⑂", Title: "prompt"})
	if got != "Fix malformed JSON in Claude settings" || !titled {
		t.Errorf("ai clone: %q %v", got, titled)
	}
}

func TestLinkClonesNestsMarkedUnderUnmarked(t *testing.T) {
	sessions := []Session{
		{ID: "c1", Path: "/p/c1.jsonl", Title: "Fix JSON", Clone: true},
		{ID: "orig", Path: "/p/orig.jsonl", Title: "Fix JSON", Clone: false},
		{ID: "c2", Path: "/p/c2.jsonl", Title: "Fix JSON", Clone: true},
		{ID: "other", Path: "/p/o.jsonl", Title: "unrelated", Clone: false},
	}
	linkClones(sessions)
	if sessions[0].ParentID != "orig" || sessions[2].ParentID != "orig" {
		t.Errorf("clone parents = %q %q, want orig", sessions[0].ParentID, sessions[2].ParentID)
	}
	if sessions[3].ParentID != "" {
		t.Errorf("unrelated nested under %q", sessions[3].ParentID)
	}
}

func TestLinkContinuesNestsTarget(t *testing.T) {
	sessions := []Session{
		{ID: "child", Path: "/p/c.jsonl", Title: "next"},
		{ID: "parent", Path: "/p/p.jsonl", Title: "orig", ContinuedIn: "child"},
	}
	linkContinues(sessions)
	if sessions[0].ParentID != "parent" {
		t.Errorf("child parent = %q, want parent", sessions[0].ParentID)
	}
}

func TestLinkClonesIgnoresIdenticalTitlesWithoutMark(t *testing.T) {
	sessions := []Session{
		{ID: "a", Path: "/p/a.jsonl", Title: "same title"},
		{ID: "b", Path: "/p/b.jsonl", Title: "same title"},
	}
	linkClones(sessions)
	if sessions[0].ParentID != "" || sessions[1].ParentID != "" {
		t.Fatal("identical titles without a clone mark must stay peers")
	}
}

func TestParseIgnoresParentUuid(t *testing.T) {
	head := []byte(recParentMsg + "\n")
	m := Parse(head, nil)
	if m.ForkedFrom != "" {
		t.Errorf("ForkedFrom = %q, parentUuid is a message id not a session", m.ForkedFrom)
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

func TestParseOriginNullIsHuman(t *testing.T) {
	// Transcripts from Claude Code ~2.1.156 and earlier have origin:null.
	line := `{"type":"user","origin":null,"message":{"role":"user","content":"please look at the flaky auth tests"},"cwd":"/repo"}`
	m := Parse([]byte(line+"\n"+recAITitle+"\n"), nil)
	if m.Title != "please look at the flaky auth tests" {
		t.Errorf("Title = %q, want the prompt from an origin-null user record", m.Title)
	}
	if m.Cwd != "/repo" {
		t.Errorf("Cwd = %q", m.Cwd)
	}
}

func TestParseOriginAbsentIsHuman(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"please look at the flaky auth tests"},"cwd":"/repo"}`
	m := Parse([]byte(line+"\n"), nil)
	if m.Title != "please look at the flaky auth tests" {
		t.Errorf("Title = %q, want the prompt from a user record with no origin", m.Title)
	}
}

func TestParseTaskNotificationIsNotHuman(t *testing.T) {
	line := `{"type":"user","origin":{"kind":"task-notification"},"message":{"role":"user","content":"scheduled ping that should not title the row"},"cwd":"/repo"}`
	m := Parse([]byte(line+"\n"+recAITitle+"\n"), nil)
	if m.Title != "" {
		t.Errorf("Title = %q, task-notification must not count as a human prompt", m.Title)
	}
	if m.AITitle != "Pod restart investigation" {
		t.Errorf("AITitle = %q", m.AITitle)
	}
}

func TestOneKeepsPreOriginSession(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"user","origin":null,"message":{"role":"user","content":"please look at the flaky auth tests"},"cwd":"/repo"}`,
		recAssistant,
		recAITitle,
	}, "\n") + "\n"
	path := filepath.Join(proj, "old.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, ok := one(path)
	if !ok {
		t.Fatal("one() dropped a real pre-origin session")
	}
	if s.Title != "Pod restart investigation" && s.Title != "please look at the flaky auth tests" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Cwd != "/repo" {
		t.Errorf("Cwd = %q", s.Cwd)
	}
}

func TestOneDropsTitleOnlyStub(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	body := recAITitle + "\n" + recAssistant + "\n"
	path := filepath.Join(proj, "stub.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := one(path); ok {
		t.Fatal("one() kept an ai-title stub with no human prompt")
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

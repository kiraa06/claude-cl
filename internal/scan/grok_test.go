package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGrokReadsSummaryNotHistory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "%2Ftmp%2Frepo", "01abc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := `{
  "generated_title": "Fix the picker columns",
  "current_model_id": "grok-4.6",
  "updated_at": "2026-09-04T01:00:00Z",
  "head_branch": "main",
  "info": {"id": "01abc", "cwd": "/tmp/repo"}
}`
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), []byte(sum), 0o644); err != nil {
		t.Fatal(err)
	}
	// A huge history file must not be required to list.
	if err := os.WriteFile(filepath.Join(dir, "chat_history.jsonl"), []byte(`{"type":"system","content":"ignore"}\n`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Grok(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sessions, want 1", len(got))
	}
	s := got[0]
	if s.ID != "01abc" || s.Title != "Fix the picker columns" || s.Cwd != "/tmp/repo" {
		t.Errorf("session = %+v", s)
	}
	if s.Tool != "grok" || s.Model != "grok-4.6" {
		t.Errorf("tool/model = %s %s", s.Tool, s.Model)
	}
}

func TestGrokTitleFallsBackToUserPrompt(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "%2Ftmp%2Frepo", "sess2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := `{"info":{"id":"sess2","cwd":"/tmp/repo"},"updated_at":"2026-09-04T01:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), []byte(sum), 0o644); err != nil {
		t.Fatal(err)
	}
	hist := `{"type":"system","content":"ignore"}
{"type":"user","content":[{"type":"text","text":"please implement the login form validation"}]}
`
	if err := os.WriteFile(filepath.Join(dir, "chat_history.jsonl"), []byte(hist), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Grok(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "please implement the login form validation" {
		t.Errorf("title = %v", got)
	}
}

func TestGrokDecodesCwdFromDirectoryName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "%2FUsers%2Fkj%2Frepo", "sess")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := `{"info":{"id":"sess"},"session_summary":"untitled work","updated_at":"2026-09-04T01:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), []byte(sum), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Grok(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Cwd != "/Users/kj/repo" {
		t.Errorf("cwd = %v", got)
	}
}

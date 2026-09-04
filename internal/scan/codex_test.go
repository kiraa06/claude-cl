package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexParsesSessionMetaAndUserPrompt(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "09", "04")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session_meta","payload":{"id":"abc-1","cwd":"/repo"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions\nskip me"}]}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix the flaky auth test please"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"looking at the test now"}]}}
`
	path := filepath.Join(dir, "rollout-2026-09-04T01-00-00-abc-1.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Codex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sessions, want 1", len(got))
	}
	s := got[0]
	if s.ID != "abc-1" || s.Cwd != "/repo" {
		t.Errorf("id/cwd = %s %s", s.ID, s.Cwd)
	}
	if s.Title != "fix the flaky auth test please" {
		t.Errorf("title = %q, want the human prompt not AGENTS.md", s.Title)
	}
	if s.Tool != "codex" {
		t.Errorf("tool = %q", s.Tool)
	}
}

func TestCodexSkipsArchivedSessions(t *testing.T) {
	root := t.TempDir()
	arch := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(arch, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session_meta","payload":{"id":"old","cwd":"/repo"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"old archived chat"}]}}
`
	if err := os.WriteFile(filepath.Join(arch, "rollout-old.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Codex(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("archived sessions leaked: %+v", got)
	}
}

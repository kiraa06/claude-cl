package scan

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Grok lists Grok Build sessions under root (~/.grok/sessions).
//
// Each session is a directory <urlencoded-cwd>/<uuid>/ with a small
// summary.json. Listing reads only that file — chat_history.jsonl is left
// alone until the preview pane asks for a tail.
func Grok(root string) ([]Session, error) {
	projects, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var sessions []Session
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		dir := filepath.Join(root, p.Name())
		cwd := grokCwdFromDir(p.Name())
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if s, ok := grokOne(filepath.Join(dir, e.Name()), cwd); ok {
				sessions = append(sessions, s)
			}
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})
	markMissing(sessions)
	return sessions, nil
}

type grokSummary struct {
	GeneratedTitle string `json:"generated_title"`
	SessionSummary string `json:"session_summary"`
	CurrentModelID string `json:"current_model_id"`
	UpdatedAt      string `json:"updated_at"`
	LastActiveAt   string `json:"last_active_at"`
	HeadBranch     string `json:"head_branch"`
	Info           struct {
		ID  string `json:"id"`
		Cwd string `json:"cwd"`
	} `json:"info"`
}

func grokOne(dir, fallbackCwd string) (Session, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		return Session{}, false
	}
	var sum grokSummary
	if json.Unmarshal(data, &sum) != nil {
		return Session{}, false
	}
	id := sum.Info.ID
	if id == "" {
		id = filepath.Base(dir)
	}
	title := strings.TrimSpace(sum.GeneratedTitle)
	if title == "" {
		title = strings.TrimSpace(sum.SessionSummary)
	}
	cwd := sum.Info.Cwd
	if cwd == "" {
		cwd = fallbackCwd
	}
	if cwd == "" {
		return Session{}, false
	}
	mod := grokTime(sum.LastActiveAt)
	if mod.IsZero() {
		mod = grokTime(sum.UpdatedAt)
	}
	if mod.IsZero() {
		if fi, err := os.Stat(dir); err == nil {
			mod = fi.ModTime()
		}
	}
	preview := grokPreview(filepath.Join(dir, "chat_history.jsonl"))
	aiTitled := sum.GeneratedTitle != ""
	if title == "" {
		title = grokTitleFromPreview(preview)
		aiTitled = false
	}
	if title == "" {
		title = "untitled"
	}
	return Session{
		ID:       id,
		Path:     dir,
		Title:    truncate(title, titleMaxLen),
		AITitled: aiTitled,
		Cwd:      cwd,
		Branch:   sum.HeadBranch,
		Model:    sum.CurrentModelID,
		Modified: mod,
		Preview:  preview,
		Tool:     "grok",
	}, true
}

func grokTitleFromPreview(turns []Turn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role != "user" {
			continue
		}
		t := turns[i].Text
		if len([]rune(t)) < minTitleLen {
			continue
		}
		return t
	}
	return ""
}

func grokCwdFromDir(name string) string {
	if u, err := url.PathUnescape(name); err == nil && strings.HasPrefix(u, "/") {
		return u
	}
	return ""
}

func grokTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Time{}
	}
	return t
}

func grokPreview(path string) []Turn {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == 0 {
		return nil
	}
	head, tail, err := windows(path, fi.Size())
	if err != nil {
		return nil
	}
	end := tail
	if end == nil {
		end = head
	}
	var turns []Turn
	forEachLine(end, func(line []byte) {
		var rec struct {
			Type    string          `json:"type"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(line, &rec) != nil {
			return
		}
		if rec.Type != "user" && rec.Type != "assistant" {
			return
		}
		text := CleanTitle(textOf(rec.Content))
		if text == "" || grokInjected(text) {
			return
		}
		turns = appendTurn(turns, Turn{Role: rec.Type, Text: text})
	})
	return turns
}

func grokInjected(t string) bool {
	if strings.HasPrefix(t, "OS Version:") || strings.HasPrefix(t, "The following skills") {
		return true
	}
	if strings.HasPrefix(t, "<") {
		return true
	}
	return false
}

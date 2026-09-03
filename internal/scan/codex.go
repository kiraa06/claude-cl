package scan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	markSessionMeta  = []byte(`"type":"session_meta"`)
	markResponseItem = []byte(`"type":"response_item"`)
)

// Codex lists Codex CLI rollouts under root (~/.codex/sessions).
// Archived sessions are skipped. Only a head window is parsed.
func Codex(root string) ([]Session, error) {
	var paths []string
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if fi.IsDir() && fi.Name() == "archived_sessions" {
			return filepath.SkipDir
		}
		if fi.IsDir() {
			return nil
		}
		name := fi.Name()
		if strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]Session, 0, len(paths))
	for _, p := range paths {
		if s, ok := codexOne(p); ok {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Modified.After(out[j].Modified)
	})
	markMissing(out)
	return out, nil
}

func codexOne(path string) (Session, bool) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == 0 {
		return Session{}, false
	}
	head, tail, err := windows(path, fi.Size())
	if err != nil {
		return Session{}, false
	}
	meta := parseCodex(head, tail)
	if meta.ID == "" || meta.Cwd == "" || meta.Title == "" {
		return Session{}, false
	}
	return Session{
		ID:       meta.ID,
		Path:     path,
		Title:    meta.Title,
		AITitled: false,
		Cwd:      meta.Cwd,
		Model:    meta.Model,
		Modified: fi.ModTime(),
		Preview:  meta.Preview,
		Tool:     "codex",
	}, true
}

type codexMeta struct {
	ID      string
	Cwd     string
	Title   string
	Model   string
	Preview []Turn
}

type codexRecord struct {
	Type    string `json:"type"`
	Payload struct {
		ID      string          `json:"id"`
		Cwd     string          `json:"cwd"`
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
	} `json:"payload"`
}

func parseCodex(head, tail []byte) codexMeta {
	var m codexMeta
	var candidates []string
	forEachLine(head, func(line []byte) {
		switch {
		case m.ID == "" && bytes.Contains(line, markSessionMeta):
			if r, ok := decodeCodex(line); ok {
				m.ID, m.Cwd = r.Payload.ID, r.Payload.Cwd
			}
		case bytes.Contains(line, markResponseItem):
			r, ok := decodeCodex(line)
			if !ok || r.Payload.Type != "message" {
				return
			}
			if r.Payload.Role == "user" && len(candidates) < titleCandidates {
				if t := CleanTitle(codexText(r.Payload.Content)); t != "" && !codexInjected(t) {
					candidates = append(candidates, t)
				}
			}
		}
	})
	m.Title = bestTitle(candidates)

	end := tail
	if end == nil {
		end = head
	}
	forEachLine(end, func(line []byte) {
		if !bytes.Contains(line, markResponseItem) {
			return
		}
		r, ok := decodeCodex(line)
		if !ok || r.Payload.Type != "message" {
			return
		}
		if r.Payload.Model != "" {
			m.Model = r.Payload.Model
		}
		t := CleanTitle(codexText(r.Payload.Content))
		if t == "" || codexInjected(t) {
			return
		}
		role := r.Payload.Role
		if role != "user" && role != "assistant" {
			return
		}
		m.Preview = appendTurn(m.Preview, Turn{Role: role, Text: t})
	})
	return m
}

func decodeCodex(line []byte) (codexRecord, bool) {
	var r codexRecord
	if json.Unmarshal(line, &r) != nil {
		return r, false
	}
	return r, true
}

func codexText(raw json.RawMessage) string {
	if t := textOf(raw); t != "" {
		return t
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
		if (blk.Type == "text" || blk.Type == "input_text" || blk.Type == "output_text") && blk.Text != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

func codexInjected(t string) bool {
	if strings.HasPrefix(t, "<") {
		return true
	}
	if strings.HasPrefix(t, "# AGENTS.md") || strings.HasPrefix(t, "AGENTS.md") {
		return true
	}
	return false
}

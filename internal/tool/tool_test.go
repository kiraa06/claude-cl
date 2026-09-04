package tool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextCycles(t *testing.T) {
	tools := []Detected{{Kind: Claude}, {Kind: Grok}, {Kind: Codex}}
	if got := Next(tools, Claude); got != Grok {
		t.Errorf("next claude = %q, want grok", got)
	}
	if got := Next(tools, Codex); got != Claude {
		t.Errorf("next codex = %q, want claude", got)
	}
}

func TestTomlValueLastMatchWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := "name = \"other\"\n[models]\ndefault = \"grok-4.6\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tomlValue(path, "default"); got != "grok-4.6" {
		t.Errorf("default = %q", got)
	}
}

func TestTomlValueStripsTrailingComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := "model = \"gpt-5.1-codex\"  # the default\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tomlValue(path, "model"); got != "gpt-5.1-codex" {
		t.Errorf("model = %q, want gpt-5.1-codex", got)
	}
}

func TestLoadPreferred(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CL_CONFIG_DIR", dir)
	if got := LoadPreferred(); got != Claude {
		t.Errorf("empty config = %q, want claude", got)
	}
	SavePreferred(Grok)
	if got := LoadPreferred(); got != Grok {
		t.Errorf("saved = %q, want grok", got)
	}
}

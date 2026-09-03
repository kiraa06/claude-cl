package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// projectsRoot is the real transcript store. The live tests below read it, so
// they skip cleanly on a machine that has never run Claude Code.
func projectsRoot(t testing.TB) string {
	root := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no transcript store at %s", root)
	}
	return root
}

// TestLiveScan is a smoke test over the real store: it reports what the picker
// would show and asserts the scan stays fast enough to feel instant.
func TestLiveScan(t *testing.T) {
	root := projectsRoot(t)

	start := time.Now()
	sessions, err := All(root)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(sessions) == 0 {
		t.Skip("store has no resumable sessions")
	}

	var aiTitled, withModel, withCwd int
	for _, s := range sessions {
		if s.AITitled {
			aiTitled++
		}
		if s.Model != "" {
			withModel++
		}
		if s.Cwd != "" {
			withCwd++
		}
		if s.Title == "" {
			t.Errorf("session %s has an empty title", s.ID)
		}
	}

	t.Logf("%d sessions in %v (ai-titled=%d fallback=%d model=%d cwd=%d)",
		len(sessions), elapsed, aiTitled, len(sessions)-aiTitled, withModel, withCwd)
	for i, s := range sessions {
		if i >= 10 {
			break
		}
		t.Logf("  %-58s %-16s %s", s.Title, s.Model, s.Cwd)
	}

	if withCwd < len(sessions)*9/10 {
		t.Errorf("only %d/%d sessions resolved a cwd; resume would chdir blind",
			withCwd, len(sessions))
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("scan took %v, too slow for an interactive launcher", elapsed)
	}
}

func BenchmarkLiveScan(b *testing.B) {
	root := projectsRoot(b)
	for b.Loop() {
		if _, err := All(root); err != nil {
			b.Fatal(err)
		}
	}
}

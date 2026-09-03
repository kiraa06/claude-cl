package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TrashDir is where deleted transcripts are kept.
const TrashDir = ".trash-cl"

// Trash moves a session's transcript out of the store, together with the
// subagent directory that belongs to it.
//
// Transcripts are moved rather than unlinked: they hold hours of conversation,
// sometimes tens of megabytes of it, and a single keystroke should not destroy
// that irreversibly. The returned path is where the transcript now lives.
func Trash(claudeDir string, s Session) (string, error) {
	dest := filepath.Join(claudeDir, TrashDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}

	// Namespace by project directory so identically named files from different
	// projects cannot collide in the trash.
	project := filepath.Base(filepath.Dir(s.Path))
	target := filepath.Join(dest, project+"__"+filepath.Base(s.Path))
	if err := os.Rename(s.Path, target); err != nil {
		return "", err
	}

	// Subagent transcripts live in a sibling directory named for the session.
	if sidecar := strings.TrimSuffix(s.Path, ".jsonl"); sidecar != s.Path {
		if fi, err := os.Stat(sidecar); err == nil && fi.IsDir() {
			if err := os.Rename(sidecar, filepath.Join(dest, project+"__"+s.ID)); err != nil {
				return target, fmt.Errorf("transcript trashed, subagents left behind: %w", err)
			}
		}
	}
	return target, nil
}

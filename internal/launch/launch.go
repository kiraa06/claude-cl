// Package launch turns a picker choice into a claude invocation.
package launch

import (
	"errors"
	"os"
	"os/exec"
)

// Mode is what to do with the chosen row.
type Mode int

const (
	// New starts a fresh session in the current directory.
	New Mode = iota
	// Resume continues an existing session under its original id.
	Resume
	// Fork continues an existing session under a new id, leaving the
	// original transcript untouched.
	Fork
)

// Spec is a fully resolved claude invocation.
type Spec struct {
	Dir  string // directory to run in
	Args []string
}

// Build assembles the claude invocation for a choice.
//
// Resuming chdirs into the session's own directory. claude locates a session by
// id from anywhere, but resuming elsewhere would run the conversation against
// the wrong working directory — a different CLAUDE.md, different relative
// paths, a different git repository — and appends records carrying that other
// directory to the transcript. Changing directory first puts the session back
// where its work was happening.
func Build(bin string, mode Mode, sessionID, sessionDir, cwd, model string) Spec {
	args := []string{bin}
	dir := cwd

	if mode != New {
		args = append(args, "--resume", sessionID)
		if mode == Fork {
			args = append(args, "--fork-session")
		}
		if sessionDir != "" {
			dir = sessionDir
		}
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return Spec{Dir: dir, Args: args}
}

// BuildFor assembles an invocation for claude, grok, or codex.
func BuildFor(kind, bin string, mode Mode, sessionID, sessionDir, cwd, model string) Spec {
	switch kind {
	case "grok":
		return buildGrok(bin, mode, sessionID, sessionDir, cwd, model)
	case "codex":
		return buildCodex(bin, mode, sessionID, sessionDir, cwd, model)
	default:
		return Build(bin, mode, sessionID, sessionDir, cwd, model)
	}
}

func buildGrok(bin string, mode Mode, sessionID, sessionDir, cwd, model string) Spec {
	args := []string{bin}
	dir := cwd
	if mode != New {
		args = append(args, "--resume", sessionID)
		if mode == Fork {
			args = append(args, "--fork-session")
		}
		if sessionDir != "" {
			dir = sessionDir
		}
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	return Spec{Dir: dir, Args: args}
}

func buildCodex(bin string, mode Mode, sessionID, sessionDir, cwd, model string) Spec {
	args := []string{bin}
	dir := cwd
	switch mode {
	case Resume:
		args = append(args, "resume", sessionID)
		if sessionDir != "" {
			dir = sessionDir
		}
	case Fork:
		args = append(args, "fork", sessionID)
		if sessionDir != "" {
			dir = sessionDir
		}
	}
	if model != "" {
		args = append(args, "-c", "model="+model)
	}
	return Spec{Dir: dir, Args: args}
}

// ErrNoClaude reports that the claude binary is not on PATH.
var ErrNoClaude = errors.New("claude not found on PATH")

// Binary resolves the claude executable.
func Binary() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", ErrNoClaude
	}
	return path, nil
}

// Exec hands control to claude, replacing this process where the platform
// allows it. It only returns on failure.
func Exec(s Spec) error {
	if len(s.Args) == 0 {
		return errors.New("empty invocation")
	}
	if s.Dir != "" {
		if err := os.Chdir(s.Dir); err != nil {
			return err
		}
	}
	return handOff(s)
}

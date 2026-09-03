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

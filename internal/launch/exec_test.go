package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecReplacesProcess runs Exec for real against a stand-in claude, in a
// child process. It confirms the two things a unit test on Build cannot: that
// the working directory is changed before the exec, and that the argv actually
// arrives at the binary.
func TestExecReplacesProcess(t *testing.T) {
	if os.Getenv("CL_EXEC_CHILD") == "1" {
		execChild()
		return
	}

	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "session")
	if err := os.Mkdir(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A stand-in claude that reports how it was invoked.
	fake := filepath.Join(dir, "claude")
	script := "#!/bin/sh\necho \"ARGS:$*\"\necho \"PWD:$(pwd)\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestExecReplacesProcess")
	cmd.Env = append(os.Environ(),
		"CL_EXEC_CHILD=1",
		"CL_EXEC_BIN="+fake,
		"CL_EXEC_DIR="+sessionDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, "ARGS:--resume abc-123 --model sonnet") {
		t.Errorf("argv did not reach the binary:\n%s", got)
	}
	// macOS resolves temp dirs through /private, so compare the suffix.
	if !strings.Contains(got, "PWD:") || !strings.Contains(got, filepath.Base(sessionDir)) {
		t.Errorf("exec did not run in the session's directory:\n%s", got)
	}
}

// execChild runs inside the forked child and never returns on success.
func execChild() {
	spec := Build(os.Getenv("CL_EXEC_BIN"), Resume, "abc-123", os.Getenv("CL_EXEC_DIR"), "/unused", "sonnet")
	if err := Exec(spec); err != nil {
		os.Stderr.WriteString("exec failed: " + err.Error() + "\n")
		os.Exit(1)
	}
}

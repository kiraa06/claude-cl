//go:build windows

package launch

import (
	"os"
	"os/exec"
)

// handOff runs claude as a child process and exits with its status.
//
// Windows has no exec that replaces the running image, so the picker stays
// alive as a thin parent, passing the console straight through. Untested:
// development and testing happen on macOS and Linux.
func handOff(s Spec) error {
	cmd := exec.Command(s.Args[0], s.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}

//go:build !windows

package launch

import (
	"os"
	"syscall"
)

// handOff replaces this process with claude.
//
// Replacing rather than spawning leaves no wrapper in the process tree, so the
// terminal, signals and exit status all belong to claude itself.
func handOff(s Spec) error {
	return syscall.Exec(s.Args[0], s.Args, os.Environ())
}

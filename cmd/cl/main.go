// Command cl picks a Claude Code session to resume, or starts a new one.
//
// It lists sessions in three sections — the current directory first, then the
// rest of the current git repository, then everywhere else — with the title
// Claude gave each conversation, and hands the choice to claude by replacing
// itself with it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kiraa06/claude-cl/internal/launch"
	"github.com/kiraa06/claude-cl/internal/scan"
	"github.com/kiraa06/claude-cl/internal/ui"
)

// version is stamped by the release pipeline. When it is not — someone ran
// `go install`, or built from a checkout — the module version Go embeds in the
// binary is the honest answer.
var version = ""

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

const usage = `cl — pick a Claude Code session

usage:
  cl [query]      pick a session; a query opens the picker filtered
  cl --version    print the version
  cl --help       print this help

keys:
  ↑↓ / jk   move            ⏎   start or resume
  pgup/pgdn page            f   fork into a new session
  ←→ / hl   choose model    d   delete (moves to trash)
  /         search          y   copy session id
  p         preview         q   quit
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cl:", err)
		os.Exit(1)
	}
}

func run() error {
	query, action := parseArgs(os.Args[1:])
	switch action {
	case actionHelp:
		fmt.Print(usage)
		return nil
	case actionVersion:
		fmt.Printf("cl %s\n", buildVersion())
		return nil
	}

	// Resolve claude before scanning: with no claude to launch there is
	// nothing the picker could usefully do.
	bin, err := launch.Binary()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cwd, err := workingDir()
	if err != nil {
		return err
	}

	claudeDir := filepath.Join(home, ".claude")
	sessions, err := scan.All(filepath.Join(claudeDir, "projects"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	models, _ := launch.Models(filepath.Join(claudeDir, "settings.json"))
	model := ui.New(sessions, cwd, claudeDir, models)
	if query != "" {
		model.SetQuery(query)
	}

	final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}

	choice := final.(ui.Model).Choice
	if choice == nil {
		return nil // the user quit
	}
	spec := launch.Build(bin, choice.Mode, choice.Session.ID, choice.Session.Cwd, cwd, choice.Model)
	return launch.Exec(spec)
}

// workingDir returns the current directory with symlinks resolved.
//
// Claude records the resolved path, so an unresolved one would file the
// sessions belonging to this directory under "all projects" instead. On macOS
// this is the difference between /tmp and /private/tmp.
func workingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		return resolved, nil
	}
	return cwd, nil
}

// action is what the command line asked for.
type action int

const (
	actionPick action = iota
	actionHelp
	actionVersion
)

// parseArgs interprets the command line. Anything that is not a recognised
// flag is treated as a search query.
func parseArgs(args []string) (query string, a action) {
	var terms []string
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "help":
			return "", actionHelp
		case "-v", "--version", "version":
			return "", actionVersion
		default:
			terms = append(terms, arg)
		}
	}
	if len(terms) == 0 {
		return "", actionPick
	}
	return strings.Join(terms, " "), actionPick
}

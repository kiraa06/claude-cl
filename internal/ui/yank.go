package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// copyText puts s on the clipboard. OSC 52 covers terminals that speak it
// (including over ssh); pbcopy / wl-copy / xclip cover the rest.
func copyText(s string) error {
	var oscErr error
	if _, err := fmt.Fprintf(os.Stdout, "\033]52;c;%s\a", base64.StdEncoding.EncodeToString([]byte(s))); err != nil {
		oscErr = err
	}
	for _, spec := range [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
	} {
		path, err := exec.LookPath(spec[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, spec[1:]...)
		cmd.Stdin = strings.NewReader(s)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return oscErr
}

// Package util provides shared utilities used across claix packages.
package util

import (
	"fmt"
	"os/exec"
	"runtime"
)

// MakeHyperlink creates an OSC 8 terminal hyperlink.
// The label is rendered as clickable text — clicking opens the URL in a browser.
// Supported by: iTerm2, Warp, Ghostty, VS Code terminal, Kitty, WezTerm.
// Terminals that don't support OSC 8 just show the label text (graceful fallback).
func MakeHyperlink(url, label string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, label)
}

// OpenBrowser opens a URL in the user's default browser.
// Cross-platform: uses `open` on macOS, `xdg-open` on Linux, `cmd /c start` on Windows.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// RepoURL is the GitHub repository URL for claix.
const RepoURL = "https://github.com/sayantanghosh-in/claix"

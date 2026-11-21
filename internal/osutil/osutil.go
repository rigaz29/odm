// Package osutil opens files and folders with the host's default handler.
package osutil

import (
	"os/exec"
	"runtime"
)

// Open launches the file with the default application.
func Open(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// Shutdown powers the machine off, as the scheduler's "when done" action.
func Shutdown() error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("shutdown", "/s", "/t", "0").Start()
	case "darwin":
		return exec.Command("osascript", "-e", `tell app "System Events" to shut down`).Start()
	default:
		return exec.Command("systemctl", "poweroff").Start()
	}
}

// Reveal shows the file in the OS file manager, selecting it when supported.
func Reveal(path string) error {
	return revealCommand(path).Start()
}

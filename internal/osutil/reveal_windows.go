//go:build windows

package osutil

import (
	"os/exec"
	"syscall"
)

// revealCommand opens Explorer with the file selected. Explorer reads its
// command line itself and wants /select, followed by the path in quotes.
// Go quotes an argument with a space as a whole, so "/select,C:\a b\c.txt"
// was not taken as the switch, and a comma in a name split the path:
// either way Explorer opened Documents with nothing selected. The command
// line is written here instead (a Windows path cannot hold a quote).
func revealCommand(path string) *exec.Cmd {
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + path + `"`}
	return cmd
}

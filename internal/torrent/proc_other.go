//go:build !windows

package torrent

import (
	"io"
	"os"
	"os/exec"
)

func hide(cmd *exec.Cmd) {}

// bind does nothing here: the program stops when its input closes, which
// also happens when ODM crashes.
func bind(p *os.Process) io.Closer { return nil }

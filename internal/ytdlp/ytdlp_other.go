//go:build !windows

package ytdlp

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// hide starts yt-dlp in a process group of its own, so killTree reaches
// the processes it starts too.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err != nil {
		return p.Kill()
	}
	return nil
}

// localText is what yt-dlp wrote to its error output: UTF-8 here.
func localText(b []byte) string { return strings.ToValidUTF8(string(b), "�") }

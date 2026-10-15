package ytdlp

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// createNoWindow keeps the console of a console program from appearing
// when a GUI program starts it.
const createNoWindow = 0x08000000

func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree ends p and every process it started: yt-dlp runs as a child of
// the program started (pip's launcher, or the standalone build's loader).
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid))
	hide(kill)
	if err := kill.Run(); err != nil {
		return p.Kill()
	}
	return nil
}

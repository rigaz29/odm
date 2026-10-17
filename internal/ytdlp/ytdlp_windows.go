package ytdlp

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/windows"
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

// cpACP is the system's ANSI code page, for MultiByteToWideChar.
const cpACP = 0

// localText is what yt-dlp wrote to its error output. The standalone
// yt-dlp.exe ignores PYTHONIOENCODING and writes in the ANSI code page
// ("youre" in Western Europe), which is read as such when it is not
// UTF-8.
func localText(b []byte) string {
	if utf8.Valid(b) || len(b) == 0 {
		return string(b)
	}
	n, err := windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), nil, 0)
	if err != nil || n <= 0 {
		return string(b)
	}
	wide := make([]uint16, n)
	if _, err := windows.MultiByteToWideChar(cpACP, 0, &b[0], int32(len(b)), &wide[0], n); err != nil {
		return string(b)
	}
	return windows.UTF16ToString(wide)
}

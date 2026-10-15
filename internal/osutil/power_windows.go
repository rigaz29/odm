//go:build windows

package osutil

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var procSetSuspendState = windows.NewLazySystemDLL("powrprof.dll").NewProc("SetSuspendState")

// Suspend puts the computer to sleep, or into hibernation when hibernate is
// set. SetSuspendState is called directly: "rundll32 powrprof.dll,
// SetSuspendState" passes its arguments wrongly and hibernates when
// hibernation is on, even when sleep was asked for.
func Suspend(hibernate bool) error {
	if err := procSetSuspendState.Find(); err != nil {
		return err
	}
	h := uintptr(0)
	if hibernate {
		h = 1
	}
	// bHibernate, bForce (no: applications may object), bWakeupEventsDisabled
	// (no: a scheduled start can still wake the computer).
	r, _, err := procSetSuspendState.Call(h, 0, 0)
	if r == 0 {
		return fmt.Errorf("SetSuspendState: %w", err)
	}
	return nil
}

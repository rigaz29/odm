//go:build !fyne && !gui

package ui

import (
	"context"
	"errors"

	"odm/internal/engine"
	"odm/internal/tools"
)

// Available reports whether this build includes the desktop UI.
const Available = false

// Options mirrors the desktop build's Run options.
type Options struct {
	Show   <-chan struct{}
	Prompt <-chan engine.AddOptions
	Hidden bool
	Tools  *tools.Keeper
}

// Run is unavailable in headless builds (built without the fyne tag); the web
// UI is used there instead.
func Run(context.Context, *engine.Manager, Options) error {
	return errors.New("this build has no desktop UI; rebuild with -tags fyne")
}

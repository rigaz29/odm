//go:build gui && !fyne

package ui

import (
	"context"

	"odm/internal/engine"
	"odm/internal/gui"
	"odm/internal/tools"
)

// Available reports whether this build includes the desktop UI: the
// new one (internal/gui), with -tags gui.
const Available = true

// Options mirrors the Fyne build's Run options.
type Options struct {
	Show   <-chan struct{}
	Prompt <-chan engine.AddOptions
	Hidden bool
	Tools  *tools.Keeper
}

// Run starts the new UI and blocks until ODM quits or ctx ends.
func Run(ctx context.Context, mgr *engine.Manager, opts Options) error {
	return gui.Run(ctx, mgr, gui.Options{Show: opts.Show, Prompt: opts.Prompt, Hidden: opts.Hidden, Tools: opts.Tools})
}

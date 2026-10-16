//go:build fluent && !fyne

package ui

import (
	"context"

	"odm/internal/engine"
	"odm/internal/fluent"
	"odm/internal/tools"
)

// Available reports whether this build includes the desktop UI: the
// Fluent one (internal/fluent), with -tags fluent.
const Available = true

// Options mirrors the Fyne build's Run options.
type Options struct {
	Show   <-chan struct{}
	Prompt <-chan engine.AddOptions
	Hidden bool
	Tools  *tools.Keeper
}

// Run starts the Fluent UI and blocks until ODM quits or ctx ends.
func Run(ctx context.Context, mgr *engine.Manager, opts Options) error {
	return fluent.Run(ctx, mgr, fluent.Options{Show: opts.Show, Prompt: opts.Prompt, Hidden: opts.Hidden})
}

package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// Modes of Settings.ToolUpdates: whether ODM may download the tools.
const (
	ModeAsk  = ""     // not asked yet: asked the first time one is needed
	ModeAuto = "auto" // installed when needed and kept up to date silently
	ModeOff  = "off"  // never downloaded
)

// ErrNeedsConsent is returned while the person has not said whether ODM
// may download the tools; the question is on its way to them.
var ErrNeedsConsent = errors.New("ODM can download it for you: answer the question in ODM's window, or turn on automatic downloads in Options → Downloads")

// Keeper decides when the tools are installed: when one is needed and the
// person allowed it, and in the background to keep them current.
type Keeper struct {
	Installer *Installer
	Mode      func() string // Settings.ToolUpdates
	// Ask asks the person once whether ODM may download the tools; nil
	// where nobody can be asked (no desktop window).
	Ask func()
	// OnInstalled is told about each install or update that happened.
	OnInstalled func(t Tool, version string, first bool)
	Log         func(format string, args ...any)

	asked sync.Once
	kick  chan struct{}
}

// NewKeeper returns a keeper installing with in; mode reads the setting.
func NewKeeper(in *Installer, mode func() string) *Keeper {
	return &Keeper{Installer: in, Mode: mode, kick: make(chan struct{}, 1)}
}

func byName(name string) (Tool, bool) {
	for _, t := range All() {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

func (k *Keeper) logf(format string, args ...any) {
	if k.Log != nil {
		k.Log(format, args...)
	}
}

// Ensure returns the program of the tool named name ("yt-dlp", "ffmpeg")
// from ODM's folder, installing it first when the person allowed that.
func (k *Keeper) Ensure(ctx context.Context, name string) (string, error) {
	t, ok := byName(name)
	if !ok || !t.Supported() {
		return "", fmt.Errorf("ODM cannot download %s on this system", name)
	}
	switch k.Mode() {
	case ModeAuto:
		if _, err := k.install(ctx, t, nil); err != nil {
			return "", err
		}
		return filepath.Join(k.Installer.Dir, t.Files[0]), nil
	case ModeAsk:
		if k.Ask != nil {
			k.asked.Do(func() { go k.Ask() })
			return "", ErrNeedsConsent
		}
	}
	return "", fmt.Errorf("downloading %s is turned off in Options → Downloads", name)
}

// Install installs or updates the tool named name now, whatever the mode:
// the person asked for it (the Setup window). progress, which may be nil,
// follows the download.
func (k *Keeper) Install(ctx context.Context, name string, progress Progress) (string, error) {
	t, ok := byName(name)
	if !ok || !t.Supported() {
		return "", fmt.Errorf("ODM cannot download %s on this system", name)
	}
	return k.install(ctx, t, progress)
}

func (k *Keeper) install(ctx context.Context, t Tool, progress Progress) (string, error) {
	before := k.Installer.Installed(t)
	v, err := k.Installer.Install(ctx, t, progress)
	if err != nil {
		k.logf("tools: %s: %v", t.Name, err)
		return "", err
	}
	if v != before {
		k.logf("tools: %s %s installed", t.Name, v)
		if k.OnInstalled != nil {
			k.OnInstalled(t, v, before == "")
		}
	}
	return v, nil
}

// UpdateAll installs or updates every tool this system supports and
// returns the first error.
func (k *Keeper) UpdateAll(ctx context.Context) error {
	var first error
	for _, t := range All() {
		if !t.Supported() {
			continue
		}
		if _, err := k.install(ctx, t, nil); err != nil && first == nil {
			first = fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	return first
}

// Kick makes Run check now, as when automatic downloads were just allowed.
func (k *Keeper) Kick() {
	select {
	case k.kick <- struct{}{}:
	default: // a check is already due
	}
}

// Run keeps the tools current while automatic downloads are on: a minute
// after start, whenever kicked, and then every interval.
func (k *Keeper) Run(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-k.kick:
		}
		if k.Mode() == ModeAuto {
			_ = k.UpdateAll(ctx)
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(interval)
	}
}

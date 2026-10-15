//go:build fyne

package ui

import (
	"image/color"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// probeCard shows, under the New Download form, what the check of the
// address found: the file's name, size and type and whether it can be
// resumed, a stream, or why the check failed.
type probeCard struct {
	root     fyne.CanvasObject
	icon     *widget.Icon
	spinner  *widget.Activity
	title    *widget.Label
	detail   *widget.Label
	resume   *widget.Label // "Resumable" or "Cannot resume", on the right
	actions  *fyne.Container
	checking bool
}

// newProbeCard makes the card; action (the ZIP preview button) sits on its
// right and is shown by the form when it applies.
func newProbeCard(action fyne.CanvasObject) *probeCard {
	c := &probeCard{
		icon:    widget.NewIcon(theme.FileIcon()),
		spinner: widget.NewActivity(),
		title:   widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		detail:  widget.NewLabel(""),
		resume:  widget.NewLabel(""),
	}
	c.title.Truncation = fyne.TextTruncateEllipsis
	c.detail.Truncation = fyne.TextTruncateEllipsis
	c.detail.Importance = widget.LowImportance
	c.resume.Importance = widget.LowImportance
	c.spinner.Hide()
	iconBox := container.NewGridWrap(fyne.NewSize(32, 32), container.NewStack(c.icon, c.spinner))
	c.actions = container.NewHBox(c.resume, action)
	text := container.New(layout.NewCustomPaddedVBoxLayout(-6), c.title, c.detail)
	body := container.NewBorder(nil, nil, container.NewCenter(iconBox), container.NewCenter(c.actions), text)
	c.root = container.NewStack(newCardBackground(), container.New(layout.NewCustomPaddedLayout(4, 4, 8, 8), body))
	c.idle()
	return c
}

func (c *probeCard) show(icon fyne.Resource, title, detail, resume string) {
	if c.checking {
		c.checking = false
		c.spinner.Stop()
		c.spinner.Hide()
		c.icon.Show()
	}
	c.icon.SetResource(icon)
	c.title.SetText(title)
	c.detail.SetText(detail)
	c.resume.Importance = widget.LowImportance
	c.resume.SetText(resume)
}

// idle is the card before an address is typed.
func (c *probeCard) idle() {
	c.show(theme.SearchIcon(), T("No address yet"),
		T("Paste or type one: ODM checks the file's size and type before adding it."), "")
}

// check shows that the address is being checked.
func (c *probeCard) check() {
	c.show(theme.SearchIcon(), T("Checking the address…"), "", "")
	c.checking = true
	c.icon.Hide()
	c.spinner.Show()
	c.spinner.Start()
}

// file shows a checked file: name, then size, type and server.
func (c *probeCard) file(p *engine.Probe, name string) {
	if name == "" {
		name = T("Unnamed file")
	}
	size := T("Size unknown")
	if p.Total > 0 {
		size = humanBytes(p.Total)
	}
	parts := []string{size}
	if ct := strings.TrimSpace(strings.SplitN(p.ContentType, ";", 2)[0]); ct != "" {
		parts = append(parts, ct)
	}
	if u, err := url.Parse(p.FinalURL); err == nil && u.Host != "" {
		parts = append(parts, u.Hostname())
	}
	resume, look := T("Resumable"), widget.SuccessImportance
	if !p.RangeOK {
		resume, look = T("Cannot resume"), widget.WarningImportance
	}
	c.show(iconForCategory(engine.Category(name)), name, strings.Join(parts, " · "), resume)
	c.resume.Importance = look
	c.resume.Refresh()
}

// stream shows an HLS or DASH playlist; info names its kind and note.
func (c *probeCard) stream(title, info string) {
	c.show(theme.MediaVideoIcon(), title, info, "")
}

// warn shows a check that went wrong in a way the download can live with.
func (c *probeCard) warn(title, detail string) {
	c.show(theme.NewWarningThemedResource(theme.WarningIcon()), title, detail, "")
}

// problem shows why the address cannot be downloaded or checked.
func (c *probeCard) problem(title, detail string) {
	c.show(theme.NewErrorThemedResource(theme.ErrorIcon()), title, detail, "")
}

// cardBackground is a rounded panel in the theme's input colour of the
// moment, behind the probe card.
type cardBackground struct {
	widget.BaseWidget
}

func newCardBackground() *cardBackground {
	b := &cardBackground{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *cardBackground) CreateRenderer() fyne.WidgetRenderer {
	r := &cardBackgroundRenderer{rect: canvas.NewRectangle(color.Transparent)}
	r.rect.StrokeWidth = 1
	r.Refresh()
	return r
}

type cardBackgroundRenderer struct{ rect *canvas.Rectangle }

func (r *cardBackgroundRenderer) Layout(size fyne.Size) { r.rect.Resize(size) }

func (r *cardBackgroundRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (r *cardBackgroundRenderer) Refresh() {
	r.rect.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.rect.StrokeColor = theme.SeparatorColor()
	r.rect.CornerRadius = theme.Size(theme.SizeNameInputRadius) + 2
	r.rect.Refresh()
}

func (r *cardBackgroundRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }

func (r *cardBackgroundRenderer) Destroy() {}

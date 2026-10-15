//go:build fyne

package ui

import (
	"image"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// graphSamples is how many seconds of transfer rate the graph shows.
const graphSamples = 60

// speedGraph draws the transfer rate of the last minute as a filled line,
// newest on the right, scaled to the fastest second shown.
type speedGraph struct {
	widget.BaseWidget
	mu      sync.Mutex
	samples []int64 // bytes per second, oldest first
	last    time.Time
}

func newSpeedGraph() *speedGraph {
	g := &speedGraph{}
	g.ExtendBaseWidget(g)
	return g
}

// add records the rate at now. Samples closer together than most of a
// second are dropped, so extra refreshes (a button press) do not squeeze
// the time axis.
func (g *speedGraph) add(rate int64, now time.Time) {
	g.mu.Lock()
	if !g.last.IsZero() && now.Sub(g.last) < 900*time.Millisecond {
		g.mu.Unlock()
		return
	}
	g.last = now
	g.samples = append(g.samples, max(rate, 0))
	if n := len(g.samples); n > graphSamples {
		g.samples = append(g.samples[:0], g.samples[n-graphSamples:]...)
	}
	g.mu.Unlock()
	g.Refresh()
}

func (g *speedGraph) snapshot() (samples []int64, peak int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	samples = append([]int64(nil), g.samples...)
	for _, s := range samples {
		peak = max(peak, s)
	}
	return samples, peak
}

func (g *speedGraph) CreateRenderer() fyne.WidgetRenderer {
	r := &speedGraphRenderer{g: g, bg: canvas.NewRectangle(color.Transparent), peak: canvas.NewText("", color.Transparent)}
	r.raster = canvas.NewRaster(r.draw)
	r.peak.TextSize = theme.CaptionTextSize()
	r.Refresh()
	return r
}

type speedGraphRenderer struct {
	g      *speedGraph
	bg     *canvas.Rectangle
	raster *canvas.Raster
	peak   *canvas.Text
}

func (r *speedGraphRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.raster.Resize(size)
	r.peak.Move(fyne.NewPos(6, 3))
	r.peak.Resize(r.peak.MinSize())
}

func (r *speedGraphRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 64) }

func (r *speedGraphRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.bg.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	r.bg.Refresh()
	_, peak := r.g.snapshot()
	r.peak.Text = ""
	if peak > 0 {
		r.peak.Text = "Peak " + humanBytes(peak) + "/s"
	}
	r.peak.Color = theme.PlaceHolderColor()
	r.peak.Refresh()
	r.Layout(r.g.Size())
	r.raster.Refresh()
}

// draw paints the graph w by h pixels: the area under the rate in the
// accent colour, fading towards the bottom, and the rate itself as a line.
func (r *speedGraphRenderer) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	samples, peak := r.g.snapshot()
	if len(samples) < 2 || peak <= 0 || w < 2 || h < 8 {
		return img
	}
	ac := color.NRGBAModel.Convert(theme.PrimaryColor()).(color.NRGBA)
	top := h / 4 // room for the peak caption
	span := float64(h - 1 - top)
	step := float64(w-1) / float64(graphSamples-1)
	first := graphSamples - len(samples) // the series starts here when short
	prevY := -1
	for x := 0; x < w; x++ {
		pos := float64(x)/step - float64(first)
		if pos < 0 {
			continue
		}
		i := int(pos)
		v := float64(samples[min(i, len(samples)-1)])
		if i+1 < len(samples) {
			f := pos - float64(i)
			v = v*(1-f) + float64(samples[i+1])*f
		}
		y := h - 1 - int(v/float64(peak)*span+0.5)
		for yy := y; yy < h; yy++ {
			a := 0x70 - 0x58*(yy-y)/max(h-y, 1) // fades out downwards
			img.SetNRGBA(x, yy, color.NRGBA{R: ac.R, G: ac.G, B: ac.B, A: uint8(a)})
		}
		lo, hi := y, y+1 // a 2px line, joined to the previous column
		if prevY >= 0 {
			lo, hi = min(lo, prevY), max(hi, prevY)
		}
		for yy := max(lo, 0); yy <= min(hi, h-1); yy++ {
			img.SetNRGBA(x, yy, ac)
		}
		prevY = y
	}
	return img
}

func (r *speedGraphRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.raster, r.peak}
}

func (r *speedGraphRenderer) Destroy() {}

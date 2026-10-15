//go:build fyne

package ui

import (
	"image"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

func TestSpeedGraphSamples(t *testing.T) {
	test.NewTempApp(t)
	g := newSpeedGraph()
	now := time.Now()
	g.add(100, now)
	g.add(999, now.Add(300*time.Millisecond)) // too soon: dropped
	for i := 1; i <= graphSamples+10; i++ {
		g.add(int64(i*10), now.Add(time.Duration(i)*time.Second))
	}
	samples, peak := g.snapshot()
	if len(samples) != graphSamples {
		t.Fatalf("%d samples kept, want %d", len(samples), graphSamples)
	}
	if peak != int64((graphSamples+10)*10) || samples[len(samples)-1] != peak {
		t.Fatalf("peak %d, newest %d", peak, samples[len(samples)-1])
	}
}

func TestSpeedGraphDraws(t *testing.T) {
	test.NewTempApp(t)
	g := newSpeedGraph()
	r := test.TempWidgetRenderer(t, g).(*speedGraphRenderer)
	if img := r.draw(120, 60).(*image.NRGBA); !blank(img) {
		t.Fatal("an empty graph drew something")
	}
	now := time.Now()
	for i, v := range []int64{0, 50, 100, 50} {
		g.add(v, now.Add(time.Duration(i)*time.Second))
	}
	img := r.draw(120, 60).(*image.NRGBA)
	// The newest samples sit at the right edge; the left part is empty.
	if blank(img.SubImage(image.Rect(110, 0, 120, 60)).(*image.NRGBA)) {
		t.Fatal("the right edge, the newest second, is empty")
	}
	if !blank(img.SubImage(image.Rect(0, 0, 60, 60)).(*image.NRGBA)) {
		t.Fatal("the first half of the minute should be empty with four samples")
	}
	if r.peak.Text != "Peak 100 B/s" {
		t.Fatalf("caption %q", r.peak.Text)
	}
}

func blank(img *image.NRGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A != 0 {
				return false
			}
		}
	}
	return true
}

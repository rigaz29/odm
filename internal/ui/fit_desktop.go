//go:build fyne

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// dragScroll is a vertical scroll container that also scrolls when its
// content is dragged, as a finger does on a touch screen. Fyne's desktop
// driver turns a touch into mouse drags, and its own scroll container only
// follows the wheel and its thin bar, so on a tablet a long page could not
// be moved. A drag that starts on a widget that takes drags itself (an
// entry selecting text) still goes to that widget.
type dragScroll struct {
	widget.BaseWidget
	scroll *container.Scroll
}

func newDragScroll(content fyne.CanvasObject) *dragScroll {
	s := &dragScroll{scroll: container.NewVScroll(content)}
	s.ExtendBaseWidget(s)
	return s
}

func (s *dragScroll) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.scroll)
}

// Dragged moves the content with the pointer: dragging down shows what is
// above, as the wheel turned up does.
func (s *dragScroll) Dragged(e *fyne.DragEvent) {
	s.scroll.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: e.Dragged.DY}})
}

func (s *dragScroll) DragEnd() {}

// fitWindow shows w at size want, or smaller where the screen's work area
// (the screen without the taskbar) is smaller, so the window and its
// buttons are never cut off on a small laptop or a tablet. Fyne measures
// in its own units, derived from the monitor (not Windows' scaling), so
// the window is shown first and fitted at the scale it got.
func fitWindow(w fyne.Window, want fyne.Size) {
	w.Resize(want)
	w.Show()
	fitShown(w, want)
}

// fitShown shrinks a window already shown to the work area, keeping want
// where it fits.
func fitShown(w fyne.Window, want fyne.Size) {
	pw, ph, ok := workArea()
	scale := w.Canvas().Scale()
	if !ok || scale <= 0 {
		return
	}
	got := fitSize(want, float32(pw)/scale, float32(ph)/scale)
	if got != want {
		w.Resize(got)
		w.CenterOnScreen()
	}
}

// titleRoom is the height kept for the window's title bar and frame.
const titleRoom = 48

// fitSize is want made to fit a work area of the given size (Fyne units).
func fitSize(want fyne.Size, areaW, areaH float32) fyne.Size {
	if maxW := areaW - 16; want.Width > maxW {
		want.Width = maxW
	}
	if maxH := areaH - titleRoom; want.Height > maxH {
		want.Height = maxH
	}
	return want
}

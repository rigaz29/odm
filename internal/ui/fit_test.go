//go:build fyne

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

func TestFitSize(t *testing.T) {
	want := fyne.NewSize(680, 600)
	// A 1920x1080 laptop at Fyne's scale 1.3125 with a taskbar: 1020 px
	// of height is 777 units, room for the window as asked.
	if got := fitSize(want, 1920/1.3125, 1020/1.3125); got != want {
		t.Fatalf("roomy screen: %v", got)
	}
	// A 1366x768 laptop at 1.25: the window gets the height there is.
	got := fitSize(want, 1366/1.25, 728/1.25)
	if got.Width != 680 || got.Height != 728/1.25-titleRoom {
		t.Fatalf("small screen: %v", got)
	}
	// A narrow tablet in portrait.
	if got := fitSize(want, 600, 1000); got.Width != 584 {
		t.Fatalf("narrow screen: %v", got)
	}
}

// The Options window's pages scroll, so however many settings a page gets,
// the window does not have to be taller than a small laptop's screen.
func TestOptionsWindowStaysShort(t *testing.T) {
	a := newTestApp(t, engine.Settings{})
	a.openOptionsTab(0)
	w := a.optionsWin
	if w == nil {
		t.Fatal("no Options window")
	}
	defer w.Close()
	if h := w.Content().MinSize().Height; h > 480 {
		t.Fatalf("the Options window needs %v units of height", h)
	}
}

func TestDragScroll(t *testing.T) {
	test.NewTempApp(t)
	rows := container.NewVBox()
	for i := 0; i < 60; i++ {
		rows.Add(widget.NewLabel("row"))
	}
	s := newDragScroll(rows)
	s.Resize(fyne.NewSize(200, 150))

	// A finger moving up shows what is below.
	s.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: -80}})
	if s.scroll.Offset.Y != 80 {
		t.Fatalf("offset after dragging up: %v", s.scroll.Offset.Y)
	}
	// Moving down past the top stops at the top.
	s.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: 500}})
	if s.scroll.Offset.Y != 0 {
		t.Fatalf("offset after dragging past the top: %v", s.scroll.Offset.Y)
	}
	// The page asks for little height: the window may be short.
	if h := s.MinSize().Height; h >= rows.MinSize().Height {
		t.Fatalf("min height %v is the content's %v", h, rows.MinSize().Height)
	}
}

package gui

import (
	"bytes"
	"image/png"
	"testing"
)

func TestAppIcon(t *testing.T) {
	for _, size := range []int{16, 32, 256} {
		img, err := png.Decode(bytes.NewReader(appIcon(size)))
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Fatalf("size %d: %v", size, b)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("size %d: the corner is not clear", size)
		}
		// The arrow's shaft is white, the background beside it the accent.
		if r, g, b, _ := img.At(size/2, size*3/10).RGBA(); r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
			t.Errorf("size %d: the shaft is %d,%d,%d", size, r>>8, g>>8, b>>8)
		}
		if r, _, b, a := img.At(size/5, size/2).RGBA(); a>>8 < 250 || b>>8 < 200 || r>>8 > 200 {
			t.Errorf("size %d: the background is %d,%d a%d", size, r>>8, b>>8, a>>8)
		}
	}
}

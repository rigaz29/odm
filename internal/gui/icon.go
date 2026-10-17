package gui

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// appIcon draws ODM's icon as a PNG of size×size pixels: the title bar's
// mark, a white arrow down to a line on a rounded square in the accent
// gradient (frontend/src/styles.css, accent-gradient). It is drawn rather
// than shipped as a file: the tray, the windows and the taskbar take it at
// any size. Each pixel averages a 4×4 grid of samples, for smooth edges.
func appIcon(size int) []byte {
	const ss = 4 // samples per pixel side
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	stops := []struct {
		at      float64
		r, g, b float64
	}{{0, 0x8b, 0x5c, 0xf6}, {0.45, 0x63, 0x66, 0xf1}, {1, 0x22, 0xd3, 0xee}}
	gradient := func(t float64) (float64, float64, float64) {
		for i := 1; i < len(stops); i++ {
			if t <= stops[i].at {
				a, b := stops[i-1], stops[i]
				f := (t - a.at) / (b.at - a.at)
				return a.r + (b.r-a.r)*f, a.g + (b.g-a.g)*f, a.b + (b.b-a.b)*f
			}
		}
		last := stops[len(stops)-1]
		return last.r, last.g, last.b
	}
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					// The sample in units of the icon: 0..1 across.
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					if !inRoundedRect(x, y, 0.04, 0.04, 0.96, 0.96, 0.22) {
						continue
					}
					cr, cg, cb := gradient(clamp01((x*0.94 + y*0.34) / 1.28)) // 110° as in the CSS
					if inArrow(x, y) {
						cr, cg, cb = 255, 255, 255
					}
					r, g, b, a = r+cr, g+cg, b+cb, a+1
				}
			}
			if a == 0 {
				continue
			}
			n := float64(ss * ss)
			img.SetNRGBA(px, py, color.NRGBA{R: uint8(r / a), G: uint8(g / a), B: uint8(b / a), A: uint8(math.Round(255 * a / n))})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// IconFile is ODM's icon as a Windows .ico, drawn at each size Windows
// asks for, each one a PNG: the exe's icon (scripts/winres.sh puts it in
// cmd/odm/rsrc_windows_amd64.syso), which Explorer, the taskbar and the
// system notifications show.
func IconFile() []byte {
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 96, 128, 256}
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		images[i] = appIcon(size)
	}
	var buf bytes.Buffer
	le := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	le([3]uint16{0, 1, uint16(len(sizes))}) // reserved, type 1 (icon), count
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		side := uint8(size) // 256 is written as 0
		le([4]uint8{side, side, 0, 0})
		le([2]uint16{1, 32}) // planes, bits per pixel
		le([2]uint32{uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, img := range images {
		buf.Write(img)
	}
	return buf.Bytes()
}

// inArrow reports whether (x, y) is in the arrow: a shaft, a head and the
// line it points down to.
func inArrow(x, y float64) bool {
	const cx = 0.5
	switch {
	case math.Abs(x-cx) <= 0.055 && y >= 0.22 && y <= 0.56: // shaft
		return true
	case y >= 0.4 && y <= 0.64 && math.Abs(x-cx) <= (0.64-y)*0.95: // head
		return true
	case inRoundedRect(x, y, 0.25, 0.72, 0.75, 0.81, 0.045): // the line
		return true
	}
	return false
}

func inRoundedRect(x, y, x0, y0, x1, y1, radius float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	dx := math.Max(math.Max(x0+radius-x, 0), x-(x1-radius))
	dy := math.Max(math.Max(y0+radius-y, 0), y-(y1-radius))
	return dx*dx+dy*dy <= radius*radius
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

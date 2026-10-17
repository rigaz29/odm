package gui

import (
	"strconv"
	"strings"
)

// formatBytes writes a size as the page does (frontend/src/format.ts,
// humanBytes): it steps up at 1000 with a binary divisor, so 1000 MiB reads
// 0.98 GB; Indonesian uses the decimal comma. "" for nothing.
func formatBytes(n int64, lang string) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1024
		i++
	}
	digits := 1
	switch {
	case i == 0:
		digits = 0
	case v < 10:
		digits = 2
	}
	s := strconv.FormatFloat(v, 'f', digits, 64)
	if lang == "id" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s + " " + units[i]
}

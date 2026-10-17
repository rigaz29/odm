package gui

// fitSize is width×height, or smaller where the screen's work area (the
// screen without the taskbar), areaW×areaH, cannot hold it with margin to
// spare: at 150% a 1920×1080 screen is 1280×720 for windows, and the main
// window asks for 1280×780. A window and its buttons are then never cut
// off on a small screen or at a large scale (as the Fyne UI's fitWindow,
// internal/ui/fit_desktop.go). An unknown area (0) leaves the size be.
func fitSize(width, height, areaW, areaH, margin int) (int, int) {
	if areaW > 0 {
		width = min(width, max(areaW-margin, areaW/2))
	}
	if areaH > 0 {
		height = min(height, max(areaH-margin, areaH/2))
	}
	return width, height
}

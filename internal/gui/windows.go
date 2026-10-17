//go:build gui

package gui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// appWindows opens ODM's other windows: New Download for a download the
// browser handed over, a progress window per download, the download
// complete window. Each loads the same page, which shows the view its
// address names (?view=…). They keep Windows' own frame and title bar:
// only the main window draws its own.
type appWindows struct {
	app        *application.App
	background application.RGBA
	theme      application.Theme
}

func (w *appWindows) open(name, title, view string, query url.Values, width, height int, onTop bool) {
	if win, ok := w.app.Window.GetByName(name); ok {
		win.Show()
		win.Focus()
		return
	}
	query.Set("view", view)
	// Smaller on a screen too small for it, with room for Windows' frame
	// (workarea.go).
	areaW, areaH, _ := workArea()
	width, height = fitSize(width, height, areaW, areaH, 64)
	win := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             name,
		Title:            title,
		Width:            width,
		Height:           height,
		MinWidth:         min(width, 420),
		MinHeight:        min(height, 240),
		URL:              "/?" + query.Encode(),
		BackgroundColour: w.background,
		AlwaysOnTop:      onTop,
		InitialPosition:  application.WindowCentered,
		Windows:          application.WindowsWindow{Theme: w.theme},
	})
	win.Focus()
}

// openAdd shows the New Download window for the hand-off token. It stays
// above other windows: the browser keeps the focus (Windows does not let
// a background program take it), and the window would open behind it.
func (w *appWindows) openAdd(token string) {
	w.open("add-"+token, "New Download", "add", url.Values{"token": {token}}, 640, 600, true)
}

func (w *appWindows) openProgress(id string) {
	w.open("progress-"+id, "Download", "progress", url.Values{"id": {id}}, 580, 620, false)
}

func (w *appWindows) closeProgress(id string) {
	if win, ok := w.app.Window.GetByName("progress-" + id); ok {
		win.Close()
	}
}

func (w *appWindows) openComplete(id string) {
	w.open("complete-"+id, "Download complete", "complete", url.Values{"id": {id}}, 540, 330, true)
}

// openOptions opens the Options window, or brings it forward, on tab.
func (w *appWindows) openOptions(tab string) {
	w.open("options", "ODM Options", "options", url.Values{"tab": {tab}}, 820, 640, false)
}

// openScheduler opens the Scheduler, asking for a new queue's name first
// with newQueue.
func (w *appWindows) openScheduler(newQueue bool) {
	q := url.Values{}
	if newQueue {
		q.Set("new", "1")
	}
	w.open("scheduler", "Scheduler", "scheduler", q, 860, 640, false)
}

// openNotice shows a scheduler notice above other windows: ODM may be
// out of sight when a queue finishes.
func (w *appWindows) openNotice(q url.Values) {
	w.open("notice-"+q.Get("kind"), "ODM", "notice", q, 480, 240, true)
}

// pickOpen asks for a file to read, of the type pattern names.
func (w *appWindows) pickOpen(filterName, pattern string) string {
	file, err := w.app.Dialog.OpenFile().CanChooseFiles(true).AddFilter(filterName, pattern).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return file
}

// pickSave asks where to save a file, offering name.
func (w *appWindows) pickSave(name string) string {
	file, err := w.app.Dialog.SaveFile().SetFilename(name).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return file
}

// pickFolder asks for a folder with Windows' own dialog, starting in
// current. The dialog wants backslashes, and reports a folder that is not
// there as an error of its own, so such a start is left out.
func (w *appWindows) pickFolder(current string) string {
	d := w.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false)
	if current = strings.TrimSpace(current); current != "" {
		current = filepath.Clean(filepath.FromSlash(current))
		if fi, err := os.Stat(current); err == nil && fi.IsDir() {
			d.SetDirectory(current)
		}
	}
	dir, err := d.PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return dir
}

// pickFile asks for a program, starting in the folder of current.
func (w *appWindows) pickFile(current string) string {
	d := w.app.Dialog.OpenFile().CanChooseFiles(true).CanChooseDirectories(false)
	if current = strings.TrimSpace(current); current != "" {
		dir := filepath.Dir(filepath.Clean(filepath.FromSlash(current)))
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			d.SetDirectory(dir)
		}
	}
	file, err := d.PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return file
}

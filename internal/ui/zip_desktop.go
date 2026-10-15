//go:build fyne

package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"odm/internal/engine"
)

// showZipPreview lists what a ZIP archive holds, reading only its end from
// the server, so the person can check it before downloading all of it.
func (a *application) showZipPreview(parent fyne.Window, url, name string, headers map[string]string) {
	status := widget.NewLabel(T("Reading the list of contents…"))
	status.Wrapping = fyne.TextWrapWord
	var entries []engine.ZipEntry
	table := widget.NewTableWithHeaders(
		func() (int, int) { return len(entries), 3 },
		func() fyne.CanvasObject { return newTableCell() },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*tableCell)
			if id.Row < 0 || id.Row >= len(entries) {
				c.setText("")
				return
			}
			e := entries[id.Row]
			switch id.Col {
			case 0:
				label := e.Name
				if e.Encrypted {
					label += T("  (encrypted)")
				}
				c.setText(label)
			case 1:
				if e.Dir {
					c.setText(T("folder"))
				} else {
					c.setText(humanBytes(e.Size))
				}
			case 2:
				if !e.Modified.IsZero() {
					c.setText(e.Modified.Local().Format("2006-01-02 15:04"))
				}
			}
		},
	)
	table.ShowHeaderColumn = false
	table.CreateHeader = func() fyne.CanvasObject { return newHeaderLabel(nil) }
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		o.(*headerLabel).SetText([]string{T("Name"), T("Size"), T("Modified")}[id.Col])
	}
	for i, w := range []float32{380, 90, 130} {
		table.SetColumnWidth(i, w)
	}
	table.Hide()

	title := Tf("Contents of %s", name)
	d := dialog.NewCustom(title, T("Close"), container.NewBorder(status, nil, nil, nil, framed(table)), parent)
	d.Resize(fyne.NewSize(660, 440))
	d.Show()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		list, err := a.mgr.PreviewZip(ctx, url, headers)
		fyne.Do(func() {
			if err != nil {
				status.SetText(err.Error())
				return
			}
			entries = list.Entries
			format := markT("%d files, %s when extracted · %s read to list them")
			if list.Files == 1 {
				format = markT("%d file, %s when extracted · %s read to list them")
			}
			status.SetText(Tf(format, list.Files, dash(humanBytes(list.Size)), dash(humanBytes(list.Fetched))))
			table.Show()
			table.Refresh()
		})
	}()
}

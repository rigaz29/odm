// Package gui is ODM's desktop UI on Windows (docs/GUI.md): Wails v3
// windows whose page (frontend/: React, Tailwind and shadcn/ui) calls the
// engine through Service, with the tray and Windows notifications. Linux
// and macOS keep the Fyne UI (internal/ui). The windows are compiled with
// -tags gui; Service and the helpers it needs build without it, so their
// tests run with the rest.
package gui

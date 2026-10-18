// Package gui is ODM's desktop UI (docs/GUI.md): Wails v3 windows whose
// page (frontend/: React, Tailwind and shadcn/ui) calls the engine through
// Service, with the tray and the system's notifications. The page is drawn
// by WebView2 on Windows, WebKitGTK on Linux and WebKit on macOS. The
// previous Fyne UI (internal/ui) stays as a build for Linux without
// WebKitGTK 6.0. The windows are compiled with -tags gui; Service and the
// helpers it needs build without it, so their tests run with the rest.
package gui

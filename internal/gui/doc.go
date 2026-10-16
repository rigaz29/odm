// Package gui is ODM's new desktop UI (docs/GUI.md): a Wails v3 window
// whose page (frontend/: React, Tailwind and shadcn/ui) calls the engine
// through Service. The window is compiled with -tags gui; Service and the
// helpers it needs build without it, so their tests run with the rest.
//
// Stage 1: the main window on the real engine (the list, the running
// downloads' cards, the sidebar, the toolbar's actions, the status bar).
// The dialogs come in stage 3: until then a download a browser hands over
// is added without a window.
package gui

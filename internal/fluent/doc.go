// Package fluent is ODM's Windows 11 (Fluent) desktop UI, built with Wails
// v3: a WebView2 window whose page (frontend/, React and Fluent UI) calls
// the engine through Service. The window itself is compiled with
// -tags fluent; Service and the helpers it needs build without it, so
// their tests run with the rest. docs/FLUENT-UI.md has the plan.
//
// Stage 0 (prototype): the main window only. The list, the category pane,
// the toolbar's queue and transfer actions and the status bar work on the
// real engine; dialogs that stage 3 brings (New Download with its probe,
// Options, Scheduler, progress windows) are not here yet, and downloads a
// browser hands over are added without a window.
package fluent

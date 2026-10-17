//go:build !windows

package gui

// The WebView2 Runtime is Windows' (webview2_windows.go).
func webView2Installed() bool { return true }

func askForWebView2(func(string) string) bool { return false }

const webView2Page = ""

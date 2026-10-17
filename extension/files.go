// Package extension holds the browser extension's files, so ODM can write
// the Chromium extension out for "Load unpacked" (the Setup window) without
// a download. The extension itself is the JavaScript beside this file.
package extension

import "embed"

// Files are the extension's own files; extpack.Unpack writes them out with
// the manifest for a browser.
//
//go:embed manifest.json *.js *.html
var Files embed.FS

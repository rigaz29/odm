# ODM browser integration

The extension hands downloads to ODM through Chrome/Firefox **native messaging**.
The browser starts `ODM.exe` as the native host; ODM recognises that launch and
relays each request to the running ODM instance over its control socket,
`<config>/odm/odm.sock` (override with `ODM_SOCKET`). `<config>` is
`%AppData%` on Windows, `~/Library/Application Support` on macOS and
`~/.config` on Linux.

## 1. Register the native host

ODM does this itself when it starts, if no browser is registered yet or the
registration points at an ODM that was moved or deleted. A registration of
another ODM copy that still exists is left alone. **Options → General**
shows which ODM each browser starts, and **Register** points them all at
this one. After **Unregister** (there or with `-unregister-browser`), ODM
stops registering itself until you register again.

To register by hand, from a shell:

```
ODM.exe -register-browser      # Windows
./odm -register-browser        # macOS / Linux
```

This writes the host manifests to `<config>/odm/native/` and registers them for
the current user:

- **Windows** — `HKCU\Software\{Google\Chrome, Microsoft\Edge, Chromium, Mozilla}\NativeMessagingHosts\com.ochi.odm`.
- **macOS / Linux** — copies the manifest into each installed browser's
  `NativeMessagingHosts` folder (Chrome, Chromium, Edge, Firefox).

Moving `ODM.exe` needs nothing: the next start registers the new place.
`-unregister-browser` removes the registration.

## 2. Install the extension

Each release ships both packages:

- **Chrome, Edge, Brave, Opera** — `odm-extension-chromium.zip`. Unzip it to
  a folder you keep, open `chrome://extensions` (or `edge://extensions`),
  enable *Developer mode*, choose *Load unpacked* and pick that folder. The
  `key` in the manifest pins the ID to `fnondfibgeiffjplpokfgiacjelmbcbk`,
  the origin the host manifest allows; check that the page shows it. To
  update, replace the folder's contents and click *Reload*. Chrome and Edge
  do not install `.crx` files from outside their stores on Windows and macOS,
  hence the unpacked route until the extension is in the stores.
- **Firefox** — `odm-extension-firefox.xpi`, signed by Mozilla: open it in
  Firefox (drag it onto a window, or *File → Open File*) and confirm. Its
  add-on ID is `odm@troke.id`. Pre-releases ship
  `odm-extension-firefox-unsigned.xpi`, which only Firefox Developer Edition
  or Nightly accept (with `xpinstall.signatures.required` set to `false`).

Restart the browser once after registering so it re-reads the host list.

Developers can load the source folder directly: *Load unpacked* on this
`extension/` folder in Chromium browsers, or `about:debugging` → *Load
Temporary Add-on* in Firefox (removed when Firefox closes).

### How the packages differ

`go run ./cmd/extpack` builds both from this folder. Firefox does not support
`background.service_worker`, so its manifest uses `background.scripts`, drops
`key`, and keeps `browser_specific_settings` (add-on ID and the data
collection declaration: the download URL and the page it came from are sent
to ODM on this computer). The Chromium manifest drops the Firefox-only keys.

## 3. Use it

ODM need not be open: when the browser hands it a download, it starts in
the system tray first. Then:

- **Click a file link.** Downloads whose type is in ODM's *Options → File
  types* go to ODM, which opens a *New Download* window to confirm (turn it
  off in *Options → Downloads*). Sites listed there under "leaves downloads
  from these sites alone" are skipped. **Alt-click** a link to let the
  browser download it this once. If ODM cannot be reached or started, the
  browser keeps the download.
- **Click "Download this video"** over a player. The button appears when the
  page loads a video or audio file of at least 512 KB; with several, it
  lists them. An HLS/DASH stream is listed with each quality its playlist
  offers: resolution, bitrate, codecs and the expected size. ODM reads the
  playlist when the menu opens (starting first if it is not running), with
  the site's cookies. A playlist the player loaded that is only one of those
  qualities is left out. ODM's New Download window opens with the quality
  picked and offers the audio tracks; with that window turned off, ODM
  downloads the picked quality.
- **YouTube** (watch pages, Shorts, YouTube Music, players embedded in other
  pages): YouTube's player fetches signed pieces no extension can hand over,
  so the button offers the video itself, and ODM asks
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) for its qualities (resolution,
  bitrate, codecs, container, expected size, and audio alone). ODM
  downloads the one picked through yt-dlp. yt-dlp must be installed: next
  to `ODM.exe`, on PATH, or chosen in ODM's *Options → Downloads*. It reads
  the page itself; the browser's cookies are not given to it, so videos
  that need a sign-in are not available. The pieces the player fetches
  (`googlevideo.com`) are no longer listed.
- **Right-click** a link, video, audio or image → **Download with ODM**.
- **Toolbar popup:** the icon's badge counts the media the page has loaded
  (up to 15, from every frame), and the popup lists them, streams with
  their qualities, with how long ago each was seen. Use it when there is no button: audio-only pages, players
  drawn on a canvas, a player in another frame, or after closing the
  button with ✕.

The popup also toggles takeover and tests the connection, starting ODM
when it is not running.

With each hand-off the extension sends ODM the URL, the page it came from,
the page title, the browser's user agent and the site's cookies for that URL,
so downloads that need a signed-in session work. Everything stays on this
computer: it goes to ODM over native messaging, not to any server.

### Troubleshooting

- **No video button in Firefox:** open `about:addons` → ODM Integration →
  *Permissions* and allow access to all websites. Firefox 127 and later grant
  it at install, but it can be revoked there.
- **Downloads stay in the browser:** check *Options → General* in ODM. If a
  browser starts another ODM, or none, click *Register*, then restart the
  browser.
- **Another download manager takes the download, in one browser only:** some
  hook into the browser process itself (Internet Download Manager with
  *Options → General → Use advanced browser integration*, for one) and take
  a download before the browser has created it, so this extension never
  sees it, even with that manager's own extension turned off. Untick the
  browser in that manager's list of browsers, or turn the option off.

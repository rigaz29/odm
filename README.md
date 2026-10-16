# Ochi Download Manager (ODM)

A segmented, resumable download manager for Windows, Linux and macOS, written
in Go. ODM opens several connections per file, resumes at the exact byte where
a transfer stopped, and organizes downloads into categories and scheduled
queues. It runs as a desktop app, or headless with a JSON API.

![ODM main window with a download in progress](docs/screenshot.png)

## Features

**Downloading**
- Up to 32 connections per file using `Range` requests (8 by default), with
  per-server overrides. The first connection opens immediately. Another opens
  after every connection already running has stored file bytes, up to the
  configured limit. Several connections use HTTP/1.1, one socket each.
  A single stream can still use HTTP/2.
- An idle connection takes half of the chunk that will take the longest to
  finish, when that chunk will take at least twice as long as the fastest
  one. Until a pace is known, it takes half of the largest remaining chunk.
- Resume after a pause, crash or restart from the exact byte offset. If a
  link expires, give the download a new address and it continues where it
  stopped, as long as the new address serves the same file.
- Range requests ask for the raw bytes (`Accept-Encoding: identity`), so a
  compressed response is not written in place of the file. A header set on
  the download itself is left as it is.
- A connection that closes before its chunk is finished retries from the
  last byte written, and stops after the retry limit.
- A saved `ETag` or `Last-Modified` is sent as `If-Range` on resume. If the
  file on the server has changed, the partial download is discarded and
  started again from the first byte. A `200` for a range that already covers
  the whole file is kept when that validator still matches.
- A later chunk re-reads a short stretch of the chunk before it. If the two
  copies differ, the rest of the file is fetched on one connection. If that
  copy still differs, the partial file is discarded and the download starts
  again from the first byte.
- A server that answers a smaller range with the whole file keeps bytes
  already stored. The remainder is fetched on one connection.
- Servers that do not support ranges are fetched on a single stream.
- Global and per-download speed limits, plus an optional data cap per time
  window. The global limit is switched off and on from the Downloads menu
  or the tray, and keeps its value while off.
- HTTP and SOCKS5 proxies, or the system proxy (the default).

**Organizing**
- Files are sorted into category folders (`Video/`, `Audio/`, `Documents/`, …),
  and each category can have its own destination folder. Categories and the
  file types they take can be added, deleted and changed.
- The New Download window remembers the save folder chosen last time, and
  falls back to the default folder when that one is gone.
- Named queues with a run order. Each queue can download a set number of files
  at once. A synchronization queue checks its finished files against the
  server and downloads the ones that changed again, replacing the local copy.
- A scheduler per queue: start with ODM, start at a set time (once or on chosen
  weekdays), start again every so many minutes until stopped, stop at a set
  time, retry failed files, and open a file, exit, sleep, hibernate or shut
  down when the queue finishes (after a 30-second countdown that can be
  cancelled).
- ZIP preview: the files inside a ZIP archive are listed before it is
  downloaded, reading only the end of the archive.
- Site logins (HTTP Digest or Basic, sent only to a host that asks), import
  and export of the download list, and a site
  grabber that crawls a page and queues the files it links to.
- A new download never replaces an existing file: if the name is taken, it is
  saved as `name (2).ext`. Adding a link that is already in the list asks
  first, or follows a setting for the browser, clipboard and grabber.

**Desktop**
- A main window with a toolbar, category and queue tree, sortable list and
  multi-select.
- A progress window per download that shows each connection, and a dialog when
  a download completes.
- Runs from the system tray. ODM can launch at login, and starting it again
  brings the running window forward.
- Checks GitHub Releases for new versions. It only notifies; it never updates
  itself.

**Integration**
- A browser extension (Chrome, Edge, Firefox) hands downloads to ODM through
  native messaging, and opens them in a *New Download* window where you
  choose the folder and name, and for a stream the quality and audio track. ODM can register itself with each browser.
- A **Download this video** button appears over a video player when the
  extension sees the page load a video or audio file. It names the file
  after the page title. Streams split into many segments (HLS/DASH) are
  downloaded segment by segment in the quality you pick (the best by
  default); separate video and
  audio tracks are joined with [ffmpeg](https://ffmpeg.org/) if it is
  installed (next to ODM, on PATH, or chosen in Options > Downloads, which
  also shows the ffmpeg version ODM found). A live HLS stream is recorded
  until you stop the download (Pause or Stop), the broadcast ends, or
  nothing new arrives for a while; live DASH is not supported. A result in
  MPEG-TS becomes MP4 when it finishes (copied, not re-encoded; Matroska
  when the streams do not fit MP4). DRM-protected video is not supported
  and never will be.
- On YouTube (watch pages, Shorts, embedded players) the button lists the
  video's qualities and audio-only formats as
  [yt-dlp](https://github.com/yt-dlp/yt-dlp) reads them, and ODM downloads
  the one you pick through yt-dlp, with its progress, pause and resume.
  A YouTube address pasted into *New Download* works the same way. yt-dlp
  reads the page itself; the browser's cookies are not given to it.
- A broadcast on such a page (YouTube Live, Twitch and others yt-dlp reads,
  where they come as HLS) shows "● LIVE" with a Record row per quality.
  ODM records the playlist yt-dlp finds with its own recorder, from the
  click until you stop it, and asks yt-dlp for a fresh playlist when the
  old address expires. The plan for TikTok (FLV) and Instagram/Facebook
  (live DASH) is in [docs/ROADMAP.md](docs/ROADMAP.md), phase 6.
- yt-dlp and ffmpeg need no installing. The first time one is needed, ODM
  asks whether it may download them (about 20 MB): yt-dlp from its own
  releases, and a small ffmpeg built for ODM that only copies streams
  between containers (`scripts/ffmpeg-min.sh`, published by the
  `ffmpeg-min` workflow, LGPL). Each is checked against the SHA-256 its
  release lists, kept in `%LocalAppData%\odm\tools`, and updated silently
  once a day. Options > Downloads turns this off, updates them now, or
  points at copies of your own, which ODM otherwise also finds next to it
  or on PATH.
- The extension's toolbar icon counts the media a page has loaded, and its
  popup lists them (from every frame) for players the button cannot find.
- A clipboard monitor queues downloadable URLs you copy. You can turn it on
  or off at any time.

## Install

Once a version is tagged, prebuilt archives are attached to
[GitHub Releases](https://github.com/troke12/odm/releases):

| Platform | Archive |
| -------- | ------- |
| Windows x64 | `odm-windows-amd64.zip` |
| Linux x64 | `odm-linux-amd64.tar.gz` |
| Linux ARM64 | `odm-linux-arm64.tar.gz` |
| macOS (Apple silicon) | `odm-darwin-arm64.tar.gz` |
| macOS (Intel) | `odm-darwin-amd64.tar.gz` |

Unpack the archive and run `ODM.exe` (Windows) or `./odm` (Linux, macOS).
Settings and download state are stored in `<config>/odm/`. `<config>` is
`%AppData%` on Windows, `~/Library/Application Support` on macOS and `~/.config`
on Linux.

### Build from source

ODM needs Go 1.23 or later. The engine uses only the standard library. The
desktop UI is built with [Fyne](https://fyne.io) behind the `fyne` build tag,
so it needs cgo.

| Target | Command | Requirements |
| ------ | ------- | ------------ |
| Windows | `./scripts/build.sh windows` | mingw-w64 (`CC_WINDOWS`, default `x86_64-w64-mingw32-gcc`) |
| Linux | `./scripts/build.sh linux` | `libgl1-mesa-dev xorg-dev libxkbcommon-dev` |
| macOS | `./scripts/build.sh darwin` (or `darwin-intel`) | Xcode command line tools |
| Headless | `./scripts/build.sh cli` | none |

```bash
go test ./...                          # engine tests, no UI needed
go test -tags fyne ./internal/ui       # UI tests (cgo)
go build -tags fyne -o odm ./cmd/odm   # desktop build for the current OS
```

## Usage

```bash
ODM.exe                                  # open the app
ODM.exe -hidden                          # start in the system tray
ODM.exe -add https://host/big.iso -no-ui # fetch one URL in the terminal, then exit
```

| Flag | Meaning |
| ---- | ------- |
| `-dir`, `-conns`, `-files` | Override the download folder, connections per file, and simultaneous downloads |
| `-add URL` | Download one URL headless and exit |
| `-no-ui`, `-open` | Run without a window / control whether a window opens |
| `-hidden` | Start in the tray (used by launch-at-login) |
| `-serve`, `-addr` | Also serve the web UI and API on `-addr` (default `127.0.0.1:8721`) |
| `-register-browser`, `-unregister-browser` | Register or remove the browser native host |
| `-version` | Print the version |

**Keyboard shortcuts** (main list): <kbd>Ctrl</kbd>/<kbd>Shift</kbd>+click to
select several downloads, <kbd>Ctrl</kbd>+<kbd>A</kbd> to select all,
<kbd>Del</kbd> to remove, <kbd>Enter</kbd> to open the progress window, arrow
keys to move, <kbd>Ctrl</kbd>+<kbd>N</kbd> to add a URL, and
<kbd>Ctrl</kbd>+<kbd>F</kbd> to search. The search box next to the toolbar
keeps the downloads whose file name, address or description contains every
word typed; <kbd>Esc</kbd> clears it. On macOS, use <kbd>Cmd</kbd> instead of
<kbd>Ctrl</kbd>.

**Notifications.** A system notification says when a download finishes or
fails, unless its complete dialog was just shown while ODM is in front. On
Windows, ODM can also play the system sound for these events. Both are in
*Options → Downloads*.

**Tray and start-up.** Closing the window hides ODM in the tray, and downloads
keep running. *Options → General* controls close-to-tray and *Launch ODM on
startup*. Launch-at-login is registered for the current user only: the `Run`
key under `HKCU` on Windows, an XDG autostart entry on Linux, and a LaunchAgent
on macOS. On Linux desktops without a tray (for example GNOME without an
AppIndicator extension), the icon may not appear. Starting ODM again still
shows the window.

**Proxy.** *Options → Proxy / Socks* offers three choices:
- No proxy.
- System settings (the default): the `HTTP(S)_PROXY` variables, and on Windows
  also the proxy set in Internet Options. Automatic configuration (PAC)
  scripts are not evaluated.
- A manual HTTP or SOCKS5 proxy.

## Browser integration

1. Start ODM once: it registers itself with Chrome, Edge, Chromium and
   Firefox. *Options → General* shows the status. (`ODM.exe
   -register-browser` does the same from a shell.)
2. Install the extension from the release:
   - Chrome, Edge, Brave, Opera: unzip `odm-extension-chromium.zip`, then
     *Developer mode* → *Load unpacked* in `chrome://extensions`.
   - Firefox: open `odm-extension-firefox.xpi` (signed by Mozilla).
3. Restart the browser once.

[extension/README.md](extension/README.md) has the details, including where
the registration is written on each OS.

## Security model

ODM opens **no TCP port by default**. A running instance listens on a Unix
domain socket in the user's config directory (`odm.sock`). The browser relay
uses this socket, and it also stops a second copy of ODM from starting. Web
pages can send requests to `127.0.0.1` but cannot reach a socket file, so
websites you visit cannot reach the API. The socket needs Windows 10 version
1803 or later. Without it, ODM still runs, but browser hand-off is disabled.

`-serve` exposes the web UI and API over TCP. That listener requires a random
token generated at each start and printed at start-up. Opening `/?token=…`
sets a `SameSite=Strict` cookie; scripts send the `X-ODM-Token` header.
Requests from any other `Origin` are refused.

## Headless mode and HTTP API

Builds without the desktop UI (`./scripts/build.sh cli`), or runs with
`-serve`, serve a basic web UI and this JSON API. The web UI covers
adding, pausing and removing downloads. Scheduler and most Options are desktop
only.

| Method | Path | Purpose |
| ------ | ---- | ------- |
| GET / PUT | `/api/settings` | Read or update settings (applied immediately) |
| GET / POST | `/api/downloads` | List downloads, or add `{"url","dir","connections","headers","speed_limit"}` |
| GET / DELETE | `/api/downloads/{id}` | One download; `DELETE ?file=true` also deletes the file |
| POST | `/api/downloads/{id}/pause` \| `/resume` \| `/retry` \| `/open` \| `/reveal` | Control a download |
| POST | `/api/downloads/{id}/limit` \| `/options` | Per-download speed limit / open-when-done |
| GET | `/api/probe?url=` | Size, type and file name of a URL without adding it |
| POST | `/api/probe/stream` | Qualities of an HLS/DASH playlist `{"url","headers"}`; an add with `"stream"` takes one as `"quality"` |
| POST | `/api/probe/site` | Qualities yt-dlp finds at a video page `{"url"}` (503 `"missing"` without yt-dlp); an add with `"site": true` takes one's `"format"` |
| POST | `/api/actions/pause-all` \| `/stop-all` \| `/resume-all` \| `/clear-completed` | Batch actions (`stop-all` also stops started queues) |
| GET | `/api/events` | Server-Sent Events stream of the list |

## How a download works

1. **Probe.** A ranged `GET bytes=0-0` returns the file size, range support,
   file name, cookies and validators.
2. **Plan.** The file is split into up to N chunks of at least 64 KiB. A tiny
   last chunk is merged into the one before it.
3. **Transfer.** Workers write each chunk into the `.odmpart` file at its
   absolute offset. Each range request asks for the raw bytes. With more than
   one connection the requests use HTTP/1.1, so each connection has its own
   socket. The first connection opens immediately. Another opens after every
   connection already running has stored file bytes, up to the configured
   limit. A worker that finishes early takes half of the chunk with the most
   time left, when that chunk will take at least twice as long as the fastest
   one. Until a pace is known, that is the largest remaining chunk. Bytes past
   that chunk's new end are left for the connection that took them. A
   connection that receives no bytes for 30 seconds, including while it waits
   for response headers, is closed and that chunk is retried from the last
   byte written. The other connections keep going. If the server answers a
   smaller range with the whole file, bytes already stored are kept and the
   remainder is fetched on one connection.
4. **Finish.** The `.odmpart` file is renamed to the final name.

Each chunk's progress (`{id,start,end,done}`) is saved as JSON, together with
the `ETag` and `Last-Modified` from the probe, so a restart continues from
the exact byte while the file on the server is unchanged. If `If-Range` fails,
or a later probe reports a different validator or size, ODM deletes the partial
file and downloads the new one. A `200` for a range that already covers the
whole file is kept when the validator still matches. A chunk that starts after
another one also reads 64 bytes of that earlier chunk and compares the two
copies. When they differ, the other connections stop and one connection
finishes from the last matching byte. If that copy differs as well, the
partial file is deleted and downloaded again from the first byte, still on
one connection. A resume after this fallback stays on one connection.
Segmented ranges, byte-offset resume and chunk splitting are standard
download-manager techniques. ODM implements them in its own code.

## Project layout

```
cmd/odm             entry point: desktop app, native-messaging relay, headless mode
internal/engine     probe, chunk planner, transfers, queues, scheduler, settings, state
internal/ui         desktop UI (Fyne): main window, progress, Scheduler, Options
internal/server     JSON + SSE API, token guard, embedded web UI
internal/ipc        Unix-socket control channel and single-instance lock
internal/native     native-messaging framing and browser-launch detection
internal/browserreg native-host registration for Chrome, Edge, Chromium, Firefox
internal/sysproxy   OS proxy lookup (environment, Windows Internet Options)
internal/autostart  launch at login (Run key, XDG autostart, LaunchAgent)
internal/update     GitHub Releases version check
internal/clipmon    clipboard watcher (native on Windows, via Fyne elsewhere) and URL detection
internal/osutil     open, reveal and shutdown helpers
cmd/extpack         builds the Chromium (.zip) and Firefox (.xpi) extension packages
internal/extpack    per-browser manifest derivation and packaging
extension/          MV3 browser extension source and native-host manifests
```

## Roadmap

- A properties dialog for each download.
- A credentials prompt for HTTP 401 responses from a host with no site login.
- A virus-scan hook, grabber projects, and translations.
- Live DASH streams.
- Signed self-update, and code-signed Windows builds.

## License

[MIT](LICENSE) © 2026 Ochi Download Manager contributors.

ODM is an independent implementation. It ships no code, binaries, assets or
data from any other product, and it does not bypass copy protection or
activation. Copyright protects expression, not ideas. This project claims
rights only over its own source and asserts none over any idea, algorithm or
behavior. See [DISCLAIMER.md](DISCLAIMER.md) for the full statement.

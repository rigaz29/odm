# Changelog

All notable changes to ODM are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/) and the project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- A search box next to the toolbar, also reached with Ctrl+F, keeps the
  downloads whose file name, address or description contains every word
  typed. It works together with the category or queue picked in the
  sidebar, and Esc clears it.
- The desktop window speaks Indonesian. It follows the system language by
  default; Options → General → Language can keep it in English or
  Indonesian (after ODM starts again). Fyne's own texts (dialog buttons,
  the file dialog, the copy and paste menu) follow too. Category and queue
  names, which are the user's own, and messages from the download engine
  stay as they are.
- A download can be given the checksum its publisher lists (SHA-256,
  SHA-1, SHA-512 or MD5, with or without the type: the length of the hex
  tells). The finished file is hashed before it takes its name; a file
  that does not match stays a `.odmpart`, the download ends with an error
  naming both hashes, and Resume fetches every byte again. The New
  Download window, the web UI, `POST /api/downloads` (`checksum`) and
  `-add` with `-checksum` take one; the complete dialog says whether it
  matched.
- The web UI looks and works like the desktop window: the same colours,
  icons, toolbar, category and queue tree, columns, search box, status
  bar and keys (Ctrl+N, Ctrl+F, Ctrl+A, Enter, Space, Del, Shift+Del), a
  New Download card with the result of the check, and a speed graph in
  the progress window. It follows the browser's light or dark setting, or
  keeps one (the palette button, or Options). On a narrow screen the
  categories move into a drawer and the list becomes cards.
- `POST /api/queues/{name}/start` and `/stop` start and stop a queue
  (`*` for all of them), for the web UI's Start Queue and Stop Queue.
- The New Download window shows what the check of the address found as a
  card under the form: the file's type icon, name, size, type and server,
  and whether it can be resumed, or a stream, or why the check failed,
  with a spinner while it runs. The ZIP Preview… button moved onto it.
- The progress window graphs the transfer rate of the last minute under
  the download status, with the peak rate.
- A light theme. ODM follows the system's light or dark app mode, also
  when it changes while ODM runs, and Options → General → Appearance can
  keep it light or dark and pick one of eight accent colours. The window
  and tray icons take the accent colour, so they show on a dark or a light
  taskbar.
- Columns of the download list can be hidden and shown (right-click a
  column header, or View → Columns) and resized by dragging a header's
  border. The shown columns, their widths and the sort order are kept for
  the next start; Reset columns brings back the defaults.
- Space pauses or resumes the selected downloads, and Shift+Del removes
  them and deletes their files. The menus show their keys, Help → Keyboard
  shortcuts lists them all, and Ctrl+N and Ctrl+F now also work while the
  search box has the keyboard.
- `POST /api/actions/stop-all`, which the web UI's Stop All button now uses.
- Speed limiter on/off in the Downloads menu and the tray menu, with
  Downloads → Speed limiter settings… to change the value. Switching it off
  keeps the value; with none saved yet, ODM asks for one. The status bar
  names the limit while it applies, and Options → Connection has an On box
  next to the global limit.
- Synchronization queues (Scheduler → Schedule): starting such a queue also
  checks each finished file against the server (ETag, else Last-Modified,
  else size) and downloads it again when it changed there. The new copy
  replaces the local file, under the same name, once it is complete; the
  old one stays until then. With "Start the queue again every N minutes"
  the check is periodic. A synchronization queue shows a refresh icon.
- ZIP preview: for a ZIP archive on a server that sends parts of files, the
  New Download window has a Preview… button that lists the files inside
  (name, size, date, and whether they are encrypted) before anything is
  downloaded. Only the end of the archive, where ZIP keeps its list of
  contents, is read.
- Categories can be added, deleted and given other file types in Options →
  Save to (New…, Delete, Defaults). The file types decide which category,
  and so which subfolder, a download gets; Other, always last, takes the
  rest. A download whose category was deleted is filed by its file name.
- A queue can start again every so many minutes until it is stopped (by
  hand, at its stop time or by Stop all), which picks up files added or
  failed since. A repeating queue never counts as finished, so it runs no
  finish action. Scheduler → Schedule.
- A queue can put the computer to sleep or into hibernation when it
  finishes, next to exiting ODM and shutting down; the choices are now one
  list, and when queues finish together the strongest is carried out. ODM
  keeps running after the computer wakes.
- The New Download window offers the save folder chosen there last time,
  for downloads added by hand and those handed over by the browser. A
  folder that no longer exists, a new default folder, or turning the option
  off (Options → Save to) brings back the default folder. Downloads added
  from the clipboard, the site grabber or the API still use the default.
- A system notification when a download finishes or fails, on by default.
  A finished download is not notified when its complete dialog was shown
  while ODM is in front. On Windows, ODM can also play the system sound for
  these events (off by default). Both are in Options → Downloads.
- The extension's toolbar icon shows how many videos, audio files and
  streams the page has loaded, and its popup lists them (from every frame,
  with how long ago each was seen) to send one to ODM. This covers pages
  where the in-page button does not appear.
- Live HLS streams are recorded: ODM keeps reloading the playlist and
  saving new segments until the download is stopped (Pause or Stop
  finishes the file), the broadcast ends, or nothing new arrives for a
  while. A segment that cannot be fetched before it leaves the playlist is
  skipped. If ODM closes mid-way, Resume continues the recording after a
  gap. The New Download window marks such a stream as live. Live DASH is
  still refused.
- A connection that receives no bytes for 30 seconds is closed and its chunk
  is retried from the last written offset. The same limit covers the wait
  for response headers. Other connections of the same file keep going.
- Resumed downloads send the saved `ETag` or `Last-Modified` as `If-Range`.
  When the file on the server has changed, the partial file is discarded and
  the download starts again from the first byte. A `200` response whose range
  already covers the whole file is kept when that validator still matches.
- When two connections receive different bytes for the same offset, the
  download continues on one connection from the last matching byte. If that
  copy still differs, the partial file is discarded and the download starts
  again from the first byte.

### Changed

- A connection four or more times slower than a freed one hands it all the
  work it has left, instead of keeping the last piece that halving left it
  while every other connection had finished; a freed connection waits for
  such a chance while others still work. A connection far slower than the
  others no longer takes half of a fast one's work. With one of eight
  connections crawling at 16 KB/s, a 24 MB file took 3.1 s instead of
  9.1 s in a local test.
- The status bar is split in two. The left names the list: how many
  downloads, how many match the search, and how many are selected. The
  right names what is going on: how many downloads run and at what total
  speed, how many wait in a queue, and the speed limit while it applies.

### Security

- On Windows a finished download is handed to Attachment Services, as
  browsers hand theirs: it gets the mark of the web (the `Zone.Identifier`
  stream, with the address it came from) as system policy asks, so
  SmartScreen checks a downloaded program before it runs and Office opens a
  downloaded document in Protected View, and the registered antivirus
  program scans it. A file the scan removes ends as an error. Where the
  service is unavailable the mark is written directly; a drive that cannot
  hold it (FAT) does not fail the download.
- File names lose the characters that change the direction text is shown
  in (such as RIGHT-TO-LEFT OVERRIDE), which a server could use to make
  `photo‮gpj.exe` look like `photoexe.jpg`.
- A stream whose playlist is on a public server can no longer make ODM
  request addresses on the user's own machine or network (localhost,
  private and link-local ranges, including after a redirect). Browsers
  block such requests from web pages; ODM fetched them on the page's
  behalf. Playlists that are themselves on the local network still work.
  A public name that resolves to a private address (some company networks
  do this for caches) is refused as well.

### Fixed

- In Chrome, Edge and Brave the browser extension let the browser keep a
  download whose link does not end in the file's type (`download.php?id=7`,
  `/get/1234`), even when the server named it `report.pdf`. These browsers
  tell extensions about a download before they read that name; the
  extension now waits for it before deciding.
- Downloads from HTTPS servers that speak HTTP/2 (most of them) failed with
  "malformed HTTP response" as soon as they used more than one connection.
  The connections for byte ranges, which use HTTP/1.1, still offered HTTP/2
  when connecting, and the server took it.
- A file replaced on a server that sends no ETag or Last-Modified is
  downloaded again from the start when its size changes mid-download. Each
  connection now checks the total size in `Content-Range` (and that the
  range is well formed), not only where the range starts; the new file's
  bytes used to be spliced into the old one.
- Queue names are the same queue whatever their case. A download added,
  moved or imported into "night" joins the listed queue Night, so it gets
  Night's schedule and runs and stops with it; deleting "night" deletes
  Night; and settings no longer keep "Night" and "night" as two queues.
- The extension forgets an Alt-click (download in the browser) once it no
  longer applies; the marks used to pile up until the browser closed.
- A stream's playlist or decryption key whose server stops sending part
  way fails after the 30-second stall limit, as a segment does, instead of
  holding the download until it is paused.
- A long toolbar label goes on two lines instead of pushing the last
  buttons out of the window, and a column header too long for its column
  ends with an ellipsis instead of running into the next one.
- Saving Options in the web UI no longer wipes the settings it does not
  show (queues and their schedules, site logins, the proxy, category
  folders and more). `PUT /api/settings` now changes only the fields its
  body names, and `PATCH /api/settings` does the same.
- Toolbar buttons and column headers now light up under the mouse pointer.
- The Tasks menu no longer shows a second exit item, Quit, below Exit.
- Finishing a download waits up to three seconds for a file that another
  program holds open for a moment (an antivirus scanning it, a player with
  the old copy), instead of failing at the final rename on Windows.
- Site logins now work for files that need them, and support Digest as
  well as Basic. The first size check used to go out without the login, so
  a protected file failed with 401 before any byte was fetched. A login is
  now sent only to a host that asks for one, with the scheme it names
  (Digest with SHA-256 or MD5, or Basic), and then with every request to
  that host; it is not sent after a redirect to another host, and a wrong
  password fails once instead of retrying. The site grabber and the New
  Download window's size check use the logins too. A login's host may be
  typed with `https://`, a port or a path.
- A file whose size the server does not give (no `Content-Length`, as with
  a generated archive or a chunked response) is downloaded instead of
  refused. It is fetched on one connection, its size is known when the
  response ends, and a response cut off part way is fetched again.
- A server that answers 429 Too Many Requests or 503 Service Unavailable
  is asked again after the time its `Retry-After` gives (at most two
  minutes per try), or after the usual pause when it gives none. This
  covers the first size check, which used to fail the download at once,
  file connections and stream segments.
- After a restart the download list keeps the order downloads were added
  in (it followed their random IDs), the Last attempt column keeps its
  time (it showed the restart), and a download added afterwards joins the
  end of its queue instead of taking a position a restored download
  already has. Imported downloads join the end of their queue too.
- File names that Windows reserves for devices (`CON`, `NUL`, `COM1`, …,
  with any extension) get a leading `_`. Before, a page titled "nul" or
  "con" could make the download fail or write into a device on Windows.
- The size probe requests the first byte (`Range: bytes=0-0`). A server that
  answers 200 for a range covering the whole file is still segmented when
  that one-byte request returns 206.
- Range requests send `Accept-Encoding: identity`, so the bytes written to
  disk are the file itself. An `Accept-Encoding` header set on the download
  is left as it is.
- A connection that closes before its chunk is finished retries from the
  last byte written. The retry uses the usual pause and stops after the
  retry limit.
- Splitting a chunk while its connection is still reading no longer counts
  those bytes twice. Progress stays within the file size.
- A server that answers a smaller range with the whole file no longer fails
  that chunk. Bytes already stored are kept, and the rest of the file is
  fetched on one connection.
- A speed limit below 64 KB/s no longer stops a download for good. One
  read can be larger than a second's worth of the limit; it is now let
  through and the next read waits until the limit is met again.
- A download from a server without range support no longer ends up
  corrupted after a dropped connection or a resume. Such a server always
  sends the file from the first byte, so the download now starts counting
  from zero again instead of writing that data after the bytes it had.
- A download removed while it was running no longer comes back the next
  time ODM starts. Its last progress update could be saved after the
  removal had deleted its state.
- Saved download states are written one at a time, so two updates of one
  download cannot leave a state file that fails to load (the download
  used to disappear from the list).
- Resume all no longer crashes when a download is removed at the same time.
- Reading the settings (Options, the web UI, the browser extension) while
  they are being saved can no longer crash ODM. Each reader now gets its
  own copy of the lists and tables in the settings.
- `settings.json` is written to a temporary file and renamed into place,
  one save at a time. A half-written or interleaved file used to fail to
  load, and ODM then started with default settings, without its logins,
  queues and schedules.
- File names are made safe for Windows in more cases: control characters
  (for example from a `%0A` in the address) become `_`, and a name longer
  than 200 bytes is shortened without cutting a character in half and keeps
  its extension.
- A server that accepts the connection and then never answers no longer
  keeps a download waiting forever. Connecting is limited to 30 seconds,
  the TLS handshake to 15 seconds and the wait for the response headers to
  30 seconds, for file probes, stream playlists, keys and the site grabber
  as well as for transfers.
- A proxy changed in Options now applies to downloads that are already
  in the list. They used to keep the old proxy until ODM was restarted.

### Changed

- Stop All (toolbar, Downloads menu, web UI) now also stops the queues that
  were started, instead of doing the same as Pause All. A file resumed later
  no longer carries its queue on, and a queue's exit or shut-down action can
  no longer fire after everything was stopped. Pause All still leaves
  started queues started.
- The extension sends a media address as the page requested it, without
  removing any site's byte-range parameters; only the `#fragment` is
  dropped.
- A download that uses several connections fetches each part over HTTP/1.1,
  so each connection has its own socket. A single stream can still use HTTP/2.
- An idle connection takes the tail of the chunk that will take the longest
  to finish, once that chunk is at least twice as slow as the fastest one.
  Until a pace is known, the largest remainder is used.
- Connections open one at a time. The next connection starts after every
  connection already open has stored file bytes, up to the configured limit.

## [0.3.1] - 2026-10-15

### Fixed

- A paused or failed download closes its partial file. Resuming it in the
  same session no longer leaves an earlier handle open, which on Windows
  made the final rename fail ("being used by another process"). This also
  affected HLS and DASH streams. (#12, by @rigaz29)

## [0.3.0] - 2026-10-15

### Added

- HLS and DASH streams: ODM reads the playlist, picks the best quality,
  downloads the segments over several connections (each segment is kept,
  so a resume skips the finished ones) and joins them into one file.
  AES-128 encrypted HLS is decrypted. Separate video and audio tracks are
  joined with ffmpeg when it is found next to ODM, on PATH, or at the path
  chosen in Options > Downloads, which shows the version ODM found. Expired links are refreshed once from
  the playlist. Live and DRM-protected streams are refused.
- The extension's "Download this video" button now sends HLS/DASH streams
  to ODM.
- The New Download window reads a stream's playlist and offers its
  qualities (resolution, bitrate, codecs, estimated size) and audio tracks,
  best first. This also works for a playlist address typed or pasted by
  hand. A single-quality playlist says so, and live or DRM-protected
  streams are refused before anything is queued; if the playlist cannot be
  read just then, the stream can still be queued and ODM picks the best
  quality when it runs.
- The progress window of a stream counts its segments ("12 of 340")
  instead of showing an empty connection map.

### Changed

- Removing a download together with its file waits for the transfer to
  stop first, so the file is not still open (Windows refused to delete it).

## [0.2.0] - 2026-10-13

### Added

- "Download this video" button over video players: the extension notes video
  and audio files the page loads (512 KB and up, Google video byte-range
  requests turned back into the whole file) and hands the chosen one to ODM
  named after the page title (site name after the last " | " dropped,
  spaces as underscores: `Episode_11.mp4`). HLS/DASH streams are listed as
  unsupported.
- Downloads handed over by the browser open a New Download window (its own
  window, so it works while ODM is in the tray) prefilled with the name,
  cookies, referrer and user agent, which are also used for the size probe.
  *Options → Downloads* can turn it off.
- The extension reads its file types and a new "leave these sites alone"
  list from ODM's Options; Alt-click lets the browser download a link.

- Releases include the browser extension: `odm-extension-chromium.zip` for
  Chrome, Edge, Brave and Opera (load unpacked) and a Mozilla-signed
  `odm-extension-firefox.xpi` (unsigned on pre-releases).
- `cmd/extpack` derives a manifest per browser from `extension/`.

### Changed

- The Firefox add-on ID is now `odm@troke.id`, and the add-on declares the
  data it hands to ODM (Firefox's data collection permissions).
- The native host allows a list of Chromium extension IDs, ready for store
  IDs alongside the pinned unpacked ID.

### Fixed

- A download the extension took over was lost when ODM was closed or not
  registered: the browser's copy was cancelled before ODM confirmed. It is
  now paused, then cancelled only after ODM accepts, and resumed otherwise.
- The Add dialog probed the size without the download's headers, so sites
  needing cookies or a referrer showed an error instead of the size.
- The extension did not run in Firefox at all: its manifest only had
  `background.service_worker`, which Firefox does not support. The Firefox
  package uses `background.scripts`.

## [0.1.1] - 2026-10-13

### Added

- Refresh download address: point an unfinished download at a new URL when
  its link expires. ODM checks that the new address serves the same file
  (same size, resumable once bytes are on disk) before switching.
- Duplicate links: the Add dialog asks whether to show the existing entry or
  download again; browser, clipboard, grabber and API adds follow a setting
  (skip, default, or download again).
- Clipboard monitor on Linux and macOS (desktop build).
- Release archives for Linux ARM64 and macOS Intel.

### Changed

- The clipboard monitor's on/off switch applies immediately instead of after
  a restart. A link already on the clipboard when ODM starts, or when the
  monitor is switched on, is not grabbed.

### Fixed

- A new download no longer replaces an existing file with the same name;
  finishing used to delete it. Downloads now claim a free name
  (`name (2).ext`), and two downloads of the same name no longer share one
  partial file.
- The Windows clipboard reader is bounded by the clipboard block's real size.

## [0.1.0] - 2026-10-13

### Added

- Segmented, resumable download engine: multiple sockets per file using
  `Range: bytes=`, dynamic chunk splitting, per-chunk resume state, retry with
  backoff, shared global and per-download speed limits.
- Graceful single-stream fallback for servers without `Accept-Ranges`.
- Category folders by file type, automatic subfolder placement.
- Queues: named queues, `Start queue` / `Stop queue`, move a download between
  queues, `Q` column. A started queue runs one download at a time in queue
  order; items can be moved up and down.
- Scheduler: start/stop queues at set times on chosen weekdays; optionally
  exit ODM or shut the computer down when started queues finish, after a
  cancellable 30-second countdown.
- Extra file types for the clipboard monitor and site grabber; per-category
  destination folders.
- Site logins: per-host credentials applied as HTTP Basic auth.
- HTTP proxy support.
- Download list export / import as JSON.
- Site grabber: breadth-first crawl with depth/page/file limits that queues
  every recognised file link.
- Browser takeover: MV3 extension plus a native-messaging relay host.
- Windows clipboard monitor for downloadable URLs.
- Desktop UI (Fyne, Windows/macOS/Linux with the `fyne` build tag): menu bar,
  icon toolbar, category/queue sidebar, sortable report table, Add/Options/
  progress/grabber dialogs.
- Headless mode: JSON + SSE API and an embedded web UI (`-no-ui`).
- Progress window opens when a download is started or resumed (status,
  speed limiter, completion options, connection map); completion window
  with Open / Open folder when a download finishes.
- System tray: close to tray, tray menu, launch on startup (hidden), and a
  second launch brings the running window forward.
- Multi-select in the list (Ctrl/Shift-click, Ctrl+A) with Delete, Enter,
  arrow keys and Ctrl+N shortcuts.
- Scheduler window with per-queue schedules (start with ODM, at a time once
  or on weekdays, stop at a time, retries, open file / exit / shut down when
  done, files at the same time) and download limits.
- Options window with File types, Save to, Downloads, Connection (per-server
  connection exceptions, download limits), Proxy / Socks (system, manual
  HTTP or SOCKS5) and Site logins tabs; configurable User-Agent.
- `-register-browser` and an Options button register the native host for
  Chrome, Edge, Chromium and Firefox; the extension's ID is pinned.
- Update check against GitHub Releases at start-up and from the Help menu
  (notify only, can be disabled in Options).
- Local control channel over a Unix domain socket (browser relay, single
  instance); no TCP port is opened unless `-serve` is given, and that
  listener requires a per-run token and rejects foreign origins.

[Unreleased]: https://github.com/troke12/odm/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/troke12/odm/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/troke12/odm/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/troke12/odm/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/troke12/odm/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/troke12/odm/releases/tag/v0.1.0

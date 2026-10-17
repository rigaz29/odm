# Changelog

All notable changes to ODM are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/) and the project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- A right-click in ODM's windows no longer shows the browser's own menu
  (Back, Refresh, Save as, Print, Inspect): the Windows build is Wails'
  release build now. Text boxes and selected text keep Cut, Copy and
  Paste.
- A long file name no longer runs out of the box of the question before
  removing a download (or of any other dialog): it wraps inside it.
- A TikTok live recording whose connection dropped and was opened again
  no longer comes out hours long, mostly still picture: a few minutes
  showed as four hours. Each part's timestamps run on from the
  broadcast's start, and ffmpeg's join took that for the length of the
  part before; the parts are now copied one by one before they are
  joined. Once a broadcast is over, TikTok's server keeps sending its
  last seconds on every connection: those repeats are dropped, so the
  recording ends instead of adding the same seconds again and again.
- A download resumed after a power cut (or Windows crashing) no longer
  ends with runs of zeros in its file: a broken video or archive, with no
  error unless a checksum was given. The progress saved could count bytes
  still in Windows' cache, which the cut lost. A running download's file
  is now flushed to the disk before its progress is saved, every 5
  seconds, and a download's state file is flushed before it replaces the
  old one, so a cut cannot leave one that drops the download from the
  list.

### Added

- A Setup window on Windows, opened at ODM's first start (and once for
  someone who used ODM before when yt-dlp or ffmpeg is missing), or from
  ⋯ → Help → Set up ODM… and Options → General → Install the extension….
  It installs yt-dlp and ffmpeg in one go, with progress and the choice to
  keep them updated, and lists the installed browsers with ODM's extension
  in each: Firefox and Firefox-based browsers get the add-on Mozilla
  signed, opened in them so they ask to add it; Chrome, Edge, Brave and
  Opera get the extension ODM carries, written to a folder, with the steps
  of "Load unpacked" (the extensions page's address and the folder copied
  to paste). A browser shows as connected once its extension talks to
  ODM; the extension now says which browser it runs in. ODM brings that
  folder up to date at every start, so the extension follows ODM's
  releases after a browser restart. Browsers take no extension from
  outside their store by themselves, so these steps stay the person's.

## [0.5.0] - 2026-10-17

### Added

- yt-dlp signs in to video sites, set under Options → Programs ("Sign in
  to video sites with the cookies of"; by default with the browser you
  download from, or "No browser").
  YouTube asks some visitors to "sign in to confirm you're not a bot",
  and then lists no qualities: the menu offered only "best quality", and
  the download failed. "The browser you download from (ODM's extension)"
  works in Chrome too: the extension sends the site's cookies with the
  page (`siteCookies` in its messages, `cookies` for `POST
  /api/probe/site`, `site_cookies` for `POST /api/downloads`), ODM keeps
  them in memory per site, and yt-dlp gets them in a cookie file removed
  after the run (`--cookies`). Firefox, Chrome, Edge, Brave, Opera,
  Vivaldi or Chromium instead have yt-dlp read the browser's cookies
  itself (`--cookies-from-browser`), which Chrome does not allow: it
  locks its cookies while open and, since version 127, encrypts them. Used
  for the page's qualities, the download, and a live recording's fresh
  address. The setting is `ytdlp_cookies`: `extension` (the default, also
  for an empty value), `none`, or a browser.
- When yt-dlp cannot list a video's qualities, the extension's menu says
  why next to its one "best quality" row ("Could not copy Chrome cookie
  database", "Sign in to confirm you're not a bot").
- Live broadcasts on video sites can be recorded: YouTube Live, Twitch,
  and others yt-dlp reads, where they come as HLS. yt-dlp finds the
  broadcast's playlist and the headers it needs; ODM records it with its
  own recorder (Stop finishes and joins the file, as for any live stream)
  and asks yt-dlp for a fresh playlist when the signed address expires,
  also on a Resume hours later. The extension's menu on such a page shows
  "● LIVE" with a "Record" row per quality, and its button reads
  "● Record live" once ODM has answered; a live stream a page's player
  loads is marked the same way. A YouTube live address pasted into New
  Download is recorded likewise. `POST /api/probe/site` answers `live`,
  `manifest`, `headers` and the playlist's qualities; `POST
  /api/downloads` takes `page`.
- Live streams sent as one endless response, as TikTok's player reads its
  broadcasts (FLV), are recorded: from the click until Stop, opened again
  into a new part when the connection drops, and joined into one MP4 with
  ffmpeg (Matroska when the streams do not fit; without ffmpeg the parts
  are kept as they came). The extension lists such a response (an FLV
  without a size) as "● LIVE · FLV stream". TikTok's video and live pages
  go to yt-dlp like YouTube's; a broadcast is recorded from its HLS
  playlist, else its DASH manifest, else its FLV, and yt-dlp is asked for
  a fresh address when the signed one is refused. The small ffmpeg reads
  FLV and joins parts (`ffmpeg-min-n8.1-2`). `POST /api/downloads` takes
  `"stream": "http-live"`; `POST /api/probe/site` answers `kind`.
- Live DASH streams are recorded, such as the ones Facebook's and
  Instagram's players read: the extension sees the manifest the signed-in
  player loads and its menu shows "● LIVE" with a Record row per quality;
  a live manifest pasted into New Download works too. ODM reloads the
  manifest as it asks, takes each new segment once (by when it starts:
  the addresses are signed anew on every reload), starts at the live edge,
  and finishes when stopped, when the manifest turns static or when it is
  gone; video and audio are joined into one MP4. Templates counted from
  the clock (`availabilityStartTime`) are expanded up to now. No sign-in
  is handed to any other program: the browser's cookies go with the
  manifest's requests, as for any download.
- Stream and recording results in MPEG-TS, video and audio in one track,
  become MP4 when they finish, copied with ffmpeg without re-encoding, or
  Matroska when their streams do not fit MP4; without ffmpeg the TS file
  stays.

- yt-dlp and ffmpeg no longer need installing. The first time one is
  needed (a YouTube video, a stream whose video and audio are apart), ODM
  asks once whether it may download them; then it downloads yt-dlp from
  its releases and a small ffmpeg built for ODM (`scripts/ffmpeg-min.sh`,
  LGPL: it only copies streams between MP4, WebM/Matroska, MPEG-TS and
  audio containers), checks each against the SHA-256 its release lists,
  keeps them in `%LocalAppData%\odm\tools`, and updates them silently once
  a day, a running copy included. Options → Downloads turns this on or
  off and updates them now; a path chosen there still wins. The
  `ffmpeg-min` workflow builds that ffmpeg from FFmpeg's signed release
  and publishes it with its checksum, script and licence.
- YouTube videos, through [yt-dlp](https://github.com/yt-dlp/yt-dlp), an
  optional program like ffmpeg (next to ODM, on PATH, or chosen in
  Options → Downloads, which shows the version found). On YouTube's watch
  pages, Shorts, YouTube Music and embedded players, the extension's
  "Download this video" menu and popup list the qualities yt-dlp reads:
  resolution, bitrate, codecs, container and expected size, the most
  widely playable codec per resolution, and the audio alone. ODM runs
  yt-dlp for the one picked and shows its progress; Pause stops yt-dlp and
  Resume continues its partial files. A YouTube address pasted into New
  Download is read the same way. yt-dlp reads the page itself: the
  browser's cookies are neither given to it nor kept. The pieces YouTube's
  player fetches are no longer listed as media. `POST /api/probe/site`
  lists a page's qualities, and `POST /api/downloads` takes `site` and
  `format`.
- The browser extension's "Download this video" menu and popup list each
  quality of an HLS/DASH stream: resolution, bitrate, codecs and the
  expected size, read by ODM from the playlist with the site's cookies.
  A playlist the player loaded that is only one of those qualities is left
  out. The New Download window opens with the quality picked, and with
  that window turned off ODM downloads it. `POST /api/probe/stream` lists
  a playlist's qualities, and `POST /api/downloads` takes `quality`.
- A download the browser hands over starts ODM, in the system tray, when it
  is not running. The browser used to keep such downloads, so taking them
  over needed ODM to be open first. The extension popup's connection test
  starts it the same way.
- ODM registers itself with the browsers when it starts, so the extension
  works without first running `-register-browser` or clicking *Register*.
  This happens when no browser is registered yet, and again when ODM has
  been moved. A registration of another ODM copy that still exists is left
  alone. Unregister (in Options or `-unregister-browser`) turns it off
  until the next Register.
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

- The Firefox add-on's ID is `odm@rigaz29.github.io`, so Mozilla signs it
  on this repository's AMO account when a release is tagged (the release
  workflow's `AMO_JWT_ISSUER` and `AMO_JWT_SECRET`). `odm@troke.id` was
  signed on the account of the project this fork comes from. ODM's native
  host manifest allows both, so an add-on already installed keeps working;
  after installing the new one, register ODM again (Options → General →
  Register) and restart Firefox.
- Windows builds have a new desktop UI (`internal/gui`, see
  `docs/GUI.md`): a page drawn by Microsoft Edge WebView2 through Wails,
  built with React and shadcn/ui, in place of the Fyne UI, which Linux and
  macOS keep. It does all the Fyne UI did, in the same windows (New
  Download, progress, download complete, Options, Scheduler), and adds:
  - a main window with a card for each running download (speed over the
    last minute, connections, time left), a list that sorts by its
    headers, resizes, hides and restores columns, and stays quick with
    thousands of downloads, and a search box (Ctrl+K) that also takes a
    pasted address and runs any command by name;
  - light and dark themes that follow Windows or are chosen in Options;
  - Narrator and other screen readers, which read every window, and a
    keyboard that reaches all of it (the Menu key or Shift+F10 opens a
    download's menu);
  - menus and dialogs that come and go smoothly (and stay still with
    Windows' "reduce motion"), and windows fitted to small screens and
    large scales.

  Settings carry over, the Fyne UI's theme and language included. The
  exe carries ODM's icon and name, which Explorer, the taskbar, Task
  Manager and the Windows notifications now show. The Windows build needs
  no cgo (Node.js builds the page) and needs the Microsoft Edge WebView2
  Runtime, which Windows 11 and updated Windows 10 have; where it is
  missing, ODM says so and offers Microsoft's download page.
  `./scripts/build.sh windows-fyne` still builds the Fyne UI for Windows.
- The update check, and the About window's link, read this repository's
  releases (rigaz29/odm) instead of troke12/odm's. A fork still sets its
  own with `-X odm/internal/version.Repo=<owner/name>`.
- The Options window fits small laptops and tablets. It was as tall as its
  longest page (Downloads), taller than a 1080p screen at 125% with its
  OK button off the bottom. Pages now scroll instead, also by dragging
  them, as a finger does on a touch screen (Fyne's own scrolling only
  followed the wheel and its thin bar there); the window opens at a size
  that fits the screen's work area; tabs that do not fit go into a "⋯"
  menu. yt-dlp and ffmpeg moved to a Programs tab of their own, and the
  sites the browser extension leaves alone to its section on General. The
  main window is likewise made to fit the screen the first time it shows.
- The New Download window that opens for a download from the browser stays
  above other windows until it is answered. The browser keeps the focus,
  as Windows does not let ODM take it from there, so the window used to
  open behind the browser.
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

- TikTok videos sent from the browser were all named
  `TikTok_-_Make_Your_Day.mp4`: TikTok keeps that page title while its
  player moves from one video to the next, and the extension named site
  videos after the page. They are now named after yt-dlp's title of the
  video (on TikTok, its caption), which the extension already had from the
  quality menu. A name that is only a site's title ("TikTok - Make Your
  Day", "YouTube", none at all) is replaced with yt-dlp's title when the
  download starts, so older extensions benefit too, and New Download shows
  yt-dlp's title instead of it.
- A download added without starting it (New Download → Download Later, or
  `"start": false` from the API) was gone after ODM closed: it was saved
  only once its state changed. It is saved when it is added.
- Recording a YouTube broadcast failed when it was stopped ("merge
  failed … Invalid data found"). YouTube's live playlists now list the
  last hour, with video and audio apart: the recording began an hour in
  the past and fetched all the video before any audio, so a recording
  stopped early had no audio to join. A live recording now starts at the
  live edge (the last three segments, as a player does; live DASH did
  already), fetches video and audio pieces in turn, and keeps the one
  track alone when the other has nothing.
- A left click selects a download in the list again, and a double click
  opens its progress window: since right-clicks picked their row, the
  cells took every click and left clicks were lost. Ctrl+click adds a
  download to the selection or takes it out, Shift+click selects the
  range from the last one clicked, and Ctrl+Shift+click adds that range.
  The selection is made as the button goes down, with the keys held then,
  as in Explorer. Help → Keyboard shortcuts lists the clicks.
- Restarting the browser no longer sends its whole download history to
  ODM. Chromium reports every download in its history to extensions as a
  new one when it loads the history (at start, once the extension has
  run), so each past download of a listed type opened a New Download
  window, one after another, and was erased from the browser's download
  list. The extension now only takes downloads that began in the last 30
  seconds.
- The extension in Firefox no longer sends the frame a stream was found in
  as the stream's page: ODM keeps a page to ask yt-dlp for a live
  stream's fresh address, and would have asked it about that frame.
- Right-clicking a download in the list opens its menu at once. The click
  used to reach the list rather than the row, so the menu acted on what
  was selected before, and a row had to be left-clicked first. Now a row
  outside the selection becomes the selection, as in Explorer; within a
  selection of several, the selection stays for the menu and the clicked
  row becomes the current one.
- The extension popup's connection test said "ODM is reachable" while ODM
  was closed: the native host answered it without asking ODM.
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

[Unreleased]: https://github.com/rigaz29/odm/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/rigaz29/odm/releases/tag/v0.5.0
[0.3.1]: https://github.com/troke12/odm/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/troke12/odm/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/troke12/odm/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/troke12/odm/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/troke12/odm/releases/tag/v0.1.0

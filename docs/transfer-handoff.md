# Handoff for continuing ODM

Written 2026-10-14. This file replaces the earlier Indonesian engine-only note on the same path. The branch is `docs/transfer-handoff`, created from `engine-local-test` at `35b963ba7bd012dd2ff8e6d4ec15fdd0c69ba146`. This commit does not change Go code.

The note is for Ryan Bagas and for an AI continuing the work in `C:\Users\rigaz\odm`. It records observed behavior, the state of the ODM tree, and the working rules. It has no address lists, no decompiler listings, and no copied product source, binaries, artwork, or data files.

The program that was analyzed is the installed Internet Download Manager 6.43 build (file version 6.43.10.3, Tonec Inc.) at `C:\Program Files (x86)\Internet Download Manager`. It is a behavior reference only. ODM is a separate program. Pull requests to `troke12/odm` must not name this product, Gopeed, or grab.

## Read this first

- ODM is an independent implementation. `DISCLAIMER.md` forbids copying another product's code, binaries, resources, artwork, or data, and forbids working around protection. License, trial, serial, and registration checks were not analyzed and must not be traced, patched, or bypassed. A registration dialog and Registration / Order menu items exist. Name them as excluded. Do not document how the checks work, and do not publish the purchase, register, or license tooltip strings.
- Pull requests to `troke12/odm` are in English, one behavior per PR, branched from upstream `main`. The body is Summary, Behavior, Test plan. Do not mention Internet Download Manager, Gopeed, or grab in a PR body.
- Do not merge the eleven open pull requests. Do not stack feature branches. A bug that exists only on an already-open PR is fixed on that PR's branch.
- `engine-local-test` already contains all eleven engine changes for a private Windows build. It is pushed to the fork (`origin`) and is not a pull request.
- This docs branch sits on `engine-local-test` so the notes match the tree that was tested. Do not open it as a pull request to `troke12/odm`. New engine work still starts from upstream `main` at `5350f79` until upstream moves.
- Analysis artifacts under `C:\rea-work\` stay on this machine. Do not commit them. `odm.exe` in the repo root is a local build. Do not commit it.
- FTP, FTP command pipelining, RTMP, MMS, HLS, DASH, and the sub-65-byte tail takeover stay out of the ODM queue unless a later request says otherwise. Protected media streams were not analyzed. The product states that it will not download a protected stream because it must not bypass technical protection. Do not investigate or implement that path.
- The bundled help (`idman.chm` and the related CHM files) is older than the 6.43 resources. Where they disagree, prefer the 6.43 menus, dialog titles, string table, and `tips.txt`. The clearest case is the takeover key: the old help still says Ctrl for Internet Explorer, while the current "Using special keys" dialog and `tips.txt` say the key is chosen by the user.

## Repository

| Role | Location |
| --- | --- |
| Checkout | `C:\Users\rigaz\odm` |
| Fork | `origin` = `https://github.com/rigaz29/odm.git` |
| Upstream | `upstream` = `https://github.com/troke12/odm.git` |
| Git identity | Ryan Bagas `<ryanbagas27@gmail.com>` |
| `gh` | `C:\Program Files\GitHub CLI\gh.exe`, account `rigaz29` |
| Module | `odm`, Go 1.23. Direct requires: `fyne.io/fyne/v2` v2.8.2, `golang.org/x/sys` v0.30.0 |
| Upstream and fork `main` | `5350f79` (`chore(release): v0.2.0`) |

Upstream `main` does not contain the feature commits below. Each feature branch documents itself in `README.md` and in `CHANGELOG.md` under `## [Unreleased]`. Merging two of those branches always conflicts in both files. Keep every bullet. Do not duplicate the same bullet.

## Commands on this machine

Go is not on `PATH`. The portable SDK is `C:\Users\rigaz\AppData\Local\go-sdk\go\bin`. Every PowerShell starts in `C:\Users\rigaz`. Enter the repo first. PowerShell 5.1 has no `&&`. The `>` operator writes UTF-16. Write commit and PR bodies with `[System.IO.File]::WriteAllText` and `UTF8Encoding $false` (no BOM). In PowerShell `@{u}` is a hashtable. Quote it when it is meant for git.

```powershell
Set-Location C:\Users\rigaz\odm
$env:GOTOOLCHAIN = "local"
$env:PATH = "C:\Users\rigaz\AppData\Local\go-sdk\go\bin;" + $env:PATH
go test ./... -count=1 -timeout 240s
go vet ./...
```

Do not use `go test -race`. cgo is off by default, so the race detector fails.

The desktop build needs a C compiler because Fyne uses cgo:

```powershell
go build -tags fyne -o odm.exe ./cmd/odm
```

Without a C compiler, the headless binary is:

```powershell
go build -o odm.exe ./cmd/odm
```

The desktop Fyne build has not been run on this machine. Do not treat a Fyne binary as proven. Flags the source already supports include `.\odm.exe`, `-hidden`, and `-add URL -no-ui`.

On `35b963b`, before the first docs commit, `go test ./... -count=1` passed 99 tests in 15 packages and `go vet ./...` passed. Docs commits do not change Go packages, so that result still describes the engine.

## How the 2026-10-14 pass was done

The analyzed copy is `C:\rea-work\IDMan.exe`, 6,189,056 bytes, SHA-256 `2aab3f864f0df739e2c41642f1607195d18c173014f54ec914c8388d77ad1981`. The installed file is `C:\Program Files (x86)\Internet Download Manager\IDMan.exe`. It is a native 32-bit PE. REA reports format `pe32`, machine 332 (`0x14C`). It is not Electron and not a packaged script.

Tools:

- REA 6.4.0. The launcher is `C:\Users\rigaz\AppData\Roaming\npm\rea.ps1`. `rea` itself is not on `PATH`. The script is `C:\Users\rigaz\AppData\Roaming\npm\node_modules\rea-agents\scripts\rea.mjs`.
- Ghidra `12.1.4_PUBLIC` at `C:\tools\ghidra_12.1.4_PUBLIC`.
- JDK `21.0.12.1` at `C:\Program Files\Java\jdk-21.0.12.1`.
- Environment: `GHIDRA_INSTALL_DIR`, `JAVA_HOME`, `REA_ANALYSIS_PROVIDER=ghidra`, `REA_GHIDRA_STARTUP_TIMEOUT_MS=900000`.
- Headless Ghidra is `support\analyzeHeadless.bat`. The parent of the project directory must already exist. Do not invoke it through `cmd /c "... < nul"`.
- Saved project: `C:\rea-work\ghidra-proj\IDMan`. Later runs use `-process IDMan.exe -noanalysis` against that project. Do not reimport while the project is still there.
- Existing scripts: `C:\rea-work\scripts\DumpEngine.java`, `DumpMore.java`, `DumpMore2.java`, `DumpMore3.java`, `DumpCaller.java`. Their output is `engine-decomp.txt`, `engine-more.txt`, `engine-more2.txt`, `engine-more3.txt`, `engine-caller.txt`. REA snapshot: `C:\rea-work\idman-snapshot.json`. A snapshot can contain a decompiled procedure. Never copy that into the repo or into an answer.

This pass added three static views. None of them are committed.

1. `rea inspect-pe-resources` on `IDMan.exe`. Coverage was reported complete: 887 entries examined, 436 resources. Local output: `C:\rea-work\ui\rea-pe.json`. Resource offsets stay in that file. Do not copy them here. `inspect-binary-layout` is ELF-only and does not apply.
2. Two `rea search` runs with `--mode regex --provider ghidra --format json` and `--pattern` only. Do not also pass a positional pattern. Do not combine `--token-limit` with `--format json` (REA returns `UNSUPPORTED_OUTPUT_COMBINATION`). Redirect stdout in-process as UTF-8. PowerShell `>` writes UTF-16 and corrupts the JSON. Each search imports the PE into an ephemeral Ghidra session. That import is separate from the saved project. The first search returned 838 hits, 820 unique values, confidence `observed`, authority `shipped-artifact`. Local file: `C:\rea-work\ui\rea-search.json`. The second search returned 35 hits, 34 unique values. Local file: `C:\rea-work\ui\rea-search2.json`. REA only reports strings Ghidra has defined. An empty match does not prove the bytes are absent.
3. A local resource walker loaded each PE as a data file and dumped dialog titles, classic menus, and the string table to `C:\rea-work\ui\res\`. Dialog control captions from that walker are misaligned and are not evidence. Titles, classic menus, and string-table entries are usable. The same walker was run on the companion binaries. Most of those have a version resource and no dialogs.

The bundled help was decompiled locally with `hh.exe -decompile` into `C:\rea-work\ui\chm\idman`, `scheduler`, and `grabber`. `tutor.chm` was not extracted. Help pages were paraphrased. Do not paste the HTML, `tips.txt`, or `defexclist.txt` into the repo.

The 29-second stall figure comes from an earlier reading of the select loop and was not measured again. What this pass reconfirmed is the restart log text and that the check compares elapsed time with a deadline stored on that connection. Log lines distinguish timeout restart variants (1), (2), and (3).

Kernel drivers `idmtdi*.sys` and `idmwfp*.sys` were not opened. Advanced browser integration is a driver plus a helper, and ODM uses a browser extension instead. Do not deepen the driver analysis for ODM work.

## Install layout

`IDMan.exe` is the main window, the transfer engine, the scheduler, and the grabber. Companion modules below are identified from version resources, file names, and a few user-facing strings. Do not invent behavior for a module whose only evidence is a version string.

| File | Role that was actually evidenced |
| --- | --- |
| `IDMMsgHost.exe` | Native messaging host. `IDMMsgHost.json` names `com.tonec.idm` and allow-lists two Chromium extension origins. `IDMMsgHostMoz.json` is the Firefox counterpart. |
| `IDMIECC.dll` | Browser helper object for Internet Explorer. |
| `downlWithIDM.dll` | Internet Explorer context-menu handler. Uses `InternetGetCookieA` for cookie capture. |
| `IDMNetMon.dll` | Network monitor. Sockets plus a named pipe (`IDMNetworkMonitor`). Separate from the transfer engine. |
| `idmcchandler2.dll` | Click catcher for browsers. |
| `IDMGetAll.dll` | Module present. No dialog behavior recovered. |
| `idmbrbtn.dll` | Download panel. |
| `IDMShellExt.dll` | Shell extension. |
| `idmBroker.exe` | Broker described as reading settings. |
| `IDMIntegrator64.exe` | Integration loader. |
| `IEMonitor.exe` | Agent for click monitoring in IE-based browsers. Has a dialog of that title. |
| `IDMGrHlp.exe` | Manual-login helper. Embeds the Internet Explorer web browser control and waits until the user presses OK. |
| `MediumILStart.exe`, `idmfsa.dll`, `idmindex.dll`, `IDMVMPrs.dll`, `idmvconv.dll` | Present. Version strings only. Do not assign them a design. |
| `IEExt.htm` | Registers the IE link processor. |
| `idman.chm`, `scheduler.chm`, `grabber.chm`, `tutor.chm` | Help. `tutor.chm` was not extracted. |
| `IDMGCExt.crx`, `IDMGCExt59.crx`, `IDMEdgeExt.crx`, `IDMOpExt.nex`, `idmmzcc.xpi`, `idmmzcc2.xpi`, `idmmzcc3.xpi` | Shipped browser packages. |
| `idm_*.lng`, `inst_*.lng` | Language packs, plus translated tips. |
| `Toolbar\` and `3d_style_3.tbi` | Toolbar skins. The binary looks for `*.tbi`. |
| `defexclist.txt` | URL patterns the integrator skips (noise, ads, short game sounds, update payloads). Do not copy the list. ODM's exclude box is the counterpart. |
| `tips.txt` | Current user tips: takeover key, Alt to leave the download in the browser, F1, queue retry, dialog toggles, clipboard separate from browser capture, site logins, command line, drag-out. Paraphrase only. |
| `idmantypeinfo.tlb` | COM type library. Not dissected. |
| `libssl.dll`, `libcrypto.dll` | Bundled OpenSSL used by the transfer path. |

Settings live in the user profile and in a `DownloadManager` registry tree (queues, scheduler, grabber projects, proxy). Queue files also sit under a `Scheduler` directory as `q_*.dt` and `s_*.dt`. Grabber projects sit under a `Grabber\Projects` directory. Do not clone those formats. ODM keeps `settings.json`, a `state` directory, and `odm.log` under the user config directory.

## Transfer engine

### Network layer

Transfer orchestration is inside `IDMan.exe` and uses its own sockets (`send`, `recv`, `select`). There is no `InternetOpen` import, so the body of a download is not WinINet. `WinHttpOpen`, `WinHttpGetProxyForUrl`, `WinHttpGetIEProxyConfigForCurrentUser`, and `WinHttpDetectAutoProxyConfigUrl` are used for Internet Explorer and PAC proxy discovery, not for the file body. PAC handling also references the WinINet auto-proxy DLL exports and a local proxy-script window. Both paths are proxy configuration. The download body stays on the socket client.

TLS uses the bundled OpenSSL: `TLS_method`, `SSL_connect`, `SSL_read`, `SSL_write`, `SSL_CTX_set_alpn_protos`. ALPN failure for HTTP/2 has its own message. Near the `h2` token is a version token `1.60.0`, and the HTTP/2 session text (client magic, flooding, the `PRI * HTTP/2.0` preface) matches a bundled HTTP/2 stack. Treat HTTP/1.1 as the program's own socket client, and HTTP/2 plus TLS as libraries shipped beside it. Do not assert a library name the binary does not spell out.

HTTP proxying uses `CONNECT host:port HTTP/1.1` with `Proxy-Authorization` Basic or Digest. SOCKS has its own path, including authorization failure and an unknown address type. RTTI names a proxy connection, an FTP proxy connection, and separate HTTP, FTP, RTMP, and MMS connection types.

Connection reuse is logged ("Reusing connection"). A finished neighbor can be told `QUIT` (the log string is misspelled "Sendig QUIT"). That `QUIT` is the same path as the short-tail takeover below.

### HTTP request shape

Visible request lines use `%s %s HTTP/1.1`, plus `Host`, `User-Agent`, `Accept`, and `Connection: Keep-Alive`. `HEAD` exists.

Four 64-bit `Range` templates:

- `bytes=0-%I64d`
- `bytes=0-`
- `bytes=%I64d-`
- `bytes=%I64d-%I64d`

`Accept-Encoding` has three forms: `identity`, an inserted value (`%s`), and `gzip, deflate`. `identity` is what keeps a ranged body as file bytes. The response parser knows `Content-Range`, `ETag`, `Content-Length`, `Content-Encoding` (`gzip`, `deflate`, `br`), `Transfer-Encoding: chunked`, and `multipart/byteranges`.

`If-Range`, `If-Match`, `If-None-Match`, `If-Modified-Since`, and `Range` are in the client header vocabulary. The string scan did not re-prove the exact condition that sends `If-Range`. Earlier behavior reading ties the validator (ETag or modification time) to resume and to the decision when new bytes disagree with stored bytes. The options help has a separate checkbox to ignore modification-time changes on resume, because some servers change that time on every request and would otherwise restart the file.

Origin authorization includes Digest (`Authorization: Digest username=...`). ODM today only fills Basic from the per-host login table.

A user message says that when newly received bytes on several connections do not match bytes already stored, the download continues on one connection. A second reaction, also user-visible, is to start again from the first byte. ODM implements both reactions with its own 64-byte overlap.

### Writing and joining bytes

Downloaded pieces are raw file bytes. There is no compression step that joins them. `Accept-Encoding: identity` on a ranged request is what keeps the body as file bytes rather than a gzip payload. A completion log says the parts are appended into one file. The UI warns not to close the download dialog while that assembly runs. An "access denied" during assembly is attributed to another process, often antivirus, still holding the file. A CRC error during download is attributed to the disk, not to a checksum the transfer computed. Do not treat that CRC string as a recovered integrity algorithm.

The live writer uses `SetFilePointer`, `SetEndOfFile`, `WriteFile`, `LockFile` / `UnlockFile`, `GetOverlappedResult`, `CreateFileMappingA`, and `MapViewOfFile`. A chunk-order violation can roll the chunk size back. The size formula was not recovered.

There is a second path for old downloads. Before resume, the program rereads numbered part files. Registry records store a part id, a 64-bit start, and the next id. The parts are copied raw into one file. Errors on this path talk about opening the local file, reading the next part's size, allocating memory, and failing to read or write while the parts are joined. This is a compatibility path for an old layout, not the live writer, and not an encoder.

When a piece finishes and the piece queue is empty, the log names the last piece. The end position is compared with the expected size.

Log lines also speak of optional or alternative connections ("Need optional connections", "Add alternative connection", "Optional connections have been created"). The rule that creates them was not recovered. Do not invent a port of it.

### Connection schedule

The connection count is a user setting ("Max. connections number"), including a per-server cap. No single default constant was recovered from strings or from the existing dumps. Do not guess one and copy it into ODM. The help says to be careful above four connections. The binary's own warning says a high count can get the client blocked by a server, and that on a slow link or a weak machine more connections can make the transfer slower. ODM uses a default of 8 and refuses a value above 32 (`settings.go`). Those are ODM's numbers.

Two speed presets exist as labels: high speed (Ethernet, cable, Wi-Fi, mobile 4G) and low speed (dial-up, ISDN, Bluetooth, mobile Edge, IrDA). The older segmentation help says a remainder that is too small for the connection type is not split, that a finished connection is reused without a new login, that the largest remaining piece is split in half, and that positions are saved several times a minute. That help paragraph is the older manual. The binary's exact split formula was not recovered. Do not treat the paragraph as the algorithm.

When one connection finishes and the neighbor's remainder is shorter than 65 bytes (`0x41`), or that remainder is already marked, the remainder is given to the connection that just finished. The old socket is sent `QUIT` and marked done. A log line also says the last packet of a connection was skipped and the piece completed. Do not port the 65-byte threshold. ODM only gives up a tail when the remainder is at least twice `MinChunkSize` (64 KiB) and the slow piece's ETA is at least twice the fastest piece's ETA.

The readiness loop uses `__ftime32_s`. One connection that passes its deadline is restarted alone. The other connections are not restarted with it. An earlier reading placed that deadline a little above 29 seconds. This pass did not measure it again.

### Outside the ODM HTTP engine

FTP has `TYPE`, `CWD`, `SIZE`, `REST %I64d`, `RETR`, and `LIST`. A `SIZE` that does not match can keep the old size. Command pipelining was seen in an earlier reading. If passive mode is required and the server has no `PASV`, the data connection fails. All of that stays outside ODM.

`Change chunk size` is RTMP packet type 1 (big-endian size), not the HTTP segment size. RTMPT uses `POST /fcs/ident2`, `POST /open/1`, and `POST /%s/%s/%ld` with `Content-Type: application/x-fcs` and a Shockwave Flash user agent. Flash/Flex packet names (report, bandwidth, shared object, invoke) are present, along with `NetConnection.Connect.InvalidApp` and `NetConnection.Connect.Rejected`. An earlier reading also saw video reassignment after more than ten failed keyframe comparisons. Do not turn those numbers into an ODM specification.

MMS is a separate connection type and the `mms://` scheme is a defined string. It was not traced further.

HLS is a real parser (`CM3UParser` with master playlist, media playlist, media segment, and a decoding state). Defined playlist tags include the usual media-playlist and master-playlist tags, a byte-range tag, a map tag, a key tag, and end-of-list. Custom schemes `hlstoidm://`, `hls2toidm://`, `hls2toidmtomp4://`, `idmhlstomp4://`, and `idmhlstomp4v2://` exist. Completing an m3u can offer to download the mp3 links it contains. A string mentions mixing audio and video into one file. A class name for an MP4 DASH reader exists, plus a temporary-init marker. `SUBTITLES=` is a defined string. None of this was reduced to an algorithm. Recognizing a key tag is not permission to implement decryption. Protected streams stay excluded, as stated at the top of this note.

A download panel can offer "Download this video", the last requested FLV, and a list of up to ten recent FLV URLs. Host strings for `googlevideo.com` video playback and video-info URLs are present. A minimum size in kilobytes hides the panel button. The panel can be turned off per file type, and a checkbox stops automatic capture from web players. This is a UI and capture feature, not a recovered stream extractor. Do not port a video-site extractor.

`Hash: MD5` is a defined string. `md5-sess` belongs to Digest authentication. A support-page string mentions SHA-256 in the context of old Windows versions. None of those were traced to a download-integrity check. ODM's optional SHA-256 item remains an ODM idea and is not a port of a recovered check.

URL and cookie capture is separate from the transfer engine. `IDMNetMon.dll` uses sockets and a named pipe. `downlWithIDM.dll` uses `InternetGetCookieA`. Some Chromium profiles are named in strings. ODM must not read another browser's cookie database. The extension cookie API is the ODM path.

## ODM engine before the pull requests

Already present at `5350f79`, before the pull requests below:

- A range plan, default 8 connections, maximum 32, minimum piece 64 KiB. A tail smaller than that is merged into the previous piece (`NewPlan`).
- `Split` cuts half of the largest unfinished remainder for an idle worker.
- Resume from JSON. One stream when the server does not support ranges.
- `Content-Range` checks.
- A shared client in `buildClient`: `MaxIdleConns` 64, `MaxIdleConnsPerHost` 16, `IdleConnTimeout` 30 seconds, `ForceAttemptHTTP2` true. Proxy modes `system`, `none`, `http`, and `socks5`.
- Fyne UI behind `-tags fyne`. `golang.org/x/sys` is for the OS, not for the transfer.
- No grab library, no Gopeed library, and no code from the analyzed product. The engine is `net/http` plus `internal/engine`.

## Eleven pull requests

All are from `main`. All are still open. None are merged on GitHub. Do not merge them upstream.

| PR | Branch | HEAD | What it does |
| --- | --- | --- | --- |
| [#1](https://github.com/troke12/odm/pull/1) | `engine-stall-timeout` | `57e7ff5` | A 30-second limit covers the wait for headers and every body read. A stall retries from `chunk.Next()`. A user pause is not a stall. |
| [#2](https://github.com/troke12/odm/pull/2) | `engine-if-range` | `86521ce` | Resume sends `If-Range`. A different validator discards the partial file and starts once from byte 0. A `200` that already covers the whole file is kept when the validator still matches. |
| [#3](https://github.com/troke12/odm/pull/3) | `engine-conflict-fallback` | `a831a0d` | A 64-byte overlap. On a mismatch, keep the matching prefix and continue on one connection. A second mismatch starts once from byte 0. A further mismatch fails. |
| [#4](https://github.com/troke12/odm/pull/4) | `engine-http1-ranges` | `0a4f8e6` | Several range connections use a cloned HTTP/1.1 transport. The probe and a single stream stay on the shared client. |
| [#5](https://github.com/troke12/odm/pull/5) | `engine-range-identity` | `8f2548e` | A range sets `Accept-Encoding: identity` only when that header is still empty. A caller-supplied value is kept. A single stream is not forced to identity. |
| [#6](https://github.com/troke12/odm/pull/6) | `engine-slow-chunk` | `10d7a5e` | The tail moves when its ETA is at least 2x the fastest, the window is 200 ms, and the remainder is at least 2x 64 KiB. Until a pace is known, the largest remainder is used. |
| [#7](https://github.com/troke12/odm/pull/7) | `engine-short-body` | `774597e` | EOF before the piece is done is `errShortBody`. Bytes already written stay. Retry from `Next()` up to 8 times. |
| [#8](https://github.com/troke12/odm/pull/8) | `engine-probe-byte` | `5093e62` | The probe sends `Range: bytes=0-0` plus identity. A `206` with a total in `Content-Range` means ranges work. A `200` means one stream. |
| [#9](https://github.com/troke12/odm/pull/9) | `engine-clamp-end` | `cf9e76e` | `ownedSpan` clamps a buffer to the piece's current `End` before the write and before `Done` grows. |
| [#10](https://github.com/troke12/odm/pull/10) | `engine-ignored-range` | `45eeab5` | A `200` for a smaller range keeps the contiguous prefix and continues on one connection. |
| [#11](https://github.com/troke12/odm/pull/11) | `engine-ramp-connections` | `ffd4737` | Connections open one at a time, after every connection already open has stored file bytes. |

The pull-request command that has worked:

```powershell
gh pr create --repo troke12/odm --base main --head rigaz29:<branch> --body-file <path>
```

A push has failed once on DNS and then succeeded when retried. `gh pr edit` can change a PR body before the commit is on the remote. Check the remote SHA after a push.

## Combined tree on `engine-local-test`

`engine-local-test` tracks `origin/engine-local-test` and is pushed. It is not a pull request. HEAD is `35b963b`.

First-parent order:

1. `5093e62` probe, fast-forward
2. `ee87b10` identity
3. `0a2b162` HTTP/1.1
4. `dc08885` short body
5. `373c2c5` clamp
6. `3791521` stall, including the header wait
7. `3eff102` slow tail
8. `f8e6d66` ramp
9. `6e84d81` If-Range
10. `716eabb` tests: a one-byte probe is not a transfer
11. `f086a3d` conflict
12. `35b963b` ignored range

Three test-only adjustments (`716eabb` and the fixes inside the conflict merge and the ignored-range merge) exist only on the combined branch. They are not on the GitHub pull requests. Do not "restore" those tests to the standalone PR shape.

### One `Start` loop

`Download.Start` allows up to 5 passes. There is no second worker loop. When the loop ends unfinished, the download fails with `errConflictGaveUp`.

`prepare` runs only on the first pass and after a representation change. A representation change restarts from byte 0 and clears `single`, `restarted`, and `alone`. A second change in the same `Start` fails with `errRepresentationChanged` ("file on the server changed").

`connLimit` is 1 when ranges are unsupported, or `single` is set, or `alone` is set, or the limit is below 1.

After a worker stops, the decision order is: pause, then representation change, then ignored range, then conflict, then success or failure. A pause still records the intent: `planAlone` when a range was ignored, `adoptSingle` when there was a conflict and `single` is not set yet.

Conflict and ignored-range do not call `prepare` again. Only a representation change probes again.

### Flags that must stay separate

| Flag | Meaning | JSON |
| --- | --- | --- |
| `single` | Two connections disagreed. The next pass and a later resume stay on one connection. | `single` |
| `restarted` | That single connection also disagreed, and the partial file was discarded. Once per `Start`. | not stored |
| `alone` | The server answered a small range with the whole file. | `alone` |
| `sawIgnored` | This pass saw that answer. | not stored |
| `pass` | Incremented when that answer is seen, so a read that still holds the old plan does not write. | not stored |
| `ETag`, `LastModified` | Representation validators. | `etag`, `last_modified` |

`single` is not cleared at the start of `Start`. Old state without `single` or `alone` stays multi-connection. `Readdress` copies the new probe validators.

There is one `transferCancel` field. Do not declare it again while merging a branch.

### 206, 200, and the overlap

`boundaryOverlap` is 64. It applies only to HTTP 206. The request rewinds 64 bytes from `chunk.Next()` and those bytes are compared at `reqStart`. A 200 body does not use those 64 bytes.

A `206` whose `Content-Range` does not start at `reqStart` is a mismatch error.

A `200` with a known `Content-Length` that is greater than 0 and is not the file size stays an error ("server ignored Range").

A `200` whose range is already the whole file is written from byte 0 only when `wholeFileOK` holds: start is 0, end is `total-1`, `total > 0`, the ETag (or `Last-Modified` when there is no ETag) still matches, and `Content-Length` is negative or equal to the total.

A validator that is present on the response and different from the saved one is `errRepresentationChanged`, including for a whole-file 200. A missing response validator is not a mismatch.

A `200` for a smaller range, with no validator mismatch:

- Not yet `alone`: return `errIgnoredRange` without writing that body at the range offset. `noteIgnoredRange` sets `alone` and `sawIgnored`, increments `pass`, and cancels the pass. After workers stop, `planAlone` keeps the contiguous prefix (`contiguousDone`; a hole ends the prefix) as one piece.
- Already `alone`: `discardPrefix` drops the prefix that is already stored, then the rest is written at that offset. `discardPrefix` uses `body.Read`, not `readWithIdle`. A large prefix skip is not stall-bounded.

If that prefix is already the whole file, `finish` is called.

`trySplit` returns nil while `alone` is set. A conflict `single` may still split until the next pass forces one connection.

### Bytes that count

Only `ownedSpan` bytes after the write, and after `End` is read again, increase `Done`, `notePace`, `limiter.Wait`, and `noteReady`. Bytes past `End` do not count. `notePace` runs only when the count is greater than 0.

If `d.pass` differs from the `pass` captured at the start of `fetch`, return `errIgnoredRange` without writing.

`fetchWithRetry` returns immediately on nil, `errIgnoredRange`, `errRepresentationChanged`, and `errConflict`. Other errors back off from 300 ms, doubling, up to 8 seconds, at most `MaxRetries` = 8. A `retryBase` of zero uses 300 ms. A clean EOF before the piece is done is `errShortBody`. EOF on the last byte completes the piece.

`os.File.WriteAt` is safe to call concurrently. `Limiter.Wait` is safe when the limiter is nil. `doneLocked` sums each piece's `Done`, so the 64-byte overlap is counted twice in progress. State is written about every 250 ms from `notify`, after `WriteAt` and before a required fsync.

### Stall and the segment client

`exchange(ctx, client, req)` uses the client from `segmentClient`, not always `d.client`. A nil client panics the way `client.Do` used to. The header wait and each body idle gap use `DefaultStallTimeout` of 30 seconds through `time.AfterFunc` and `readWithIdle`. A successful `Do` must not cancel the request context. Cancellation is on `Body.Close` (`cancelOnClose`). Cancelling the parent context is a pause, not a stall.

`segmentTransport` clones the transport, sets `ForceAttemptHTTP2` false, and sets `TLSNextProto` to a non-nil empty map. Idle-pool sizes are unchanged. A transport that is not `*http.Transport` is left alone. The probe and a single stream stay on the shared client, which may use HTTP/2.

### Ramp

One worker starts first. `noteReady` runs after that worker stores its first file bytes. The next worker starts only when every worker already opened has stored bytes, up to `Connections`. A refused response stores nothing, so the extra connections stay closed. Ramp counters are not saved. The next `Start` ramps again.

On an instant `httptest`, a 64 KiB piece can finish in one read before the next goroutine runs, so the peak in flight can stay 1. Conflict tests must not require a peak of at least 2.

### Probe

`Inspect` sends `Range: bytes=0-0` and `Accept-Encoding: identity`. The probe itself is not stall-bounded. A `416` on an empty file fails. An unknown `Content-Length` fails the probe. Probe `Set-Cookie` overwrites the caller's `Cookie` when the probe cookies are non-empty. A test that answers every request must treat `bytes=0-0` as a probe and must not count it as a transfer. A test that matches a `Range` prefix must allow the 64-byte rewind.

The helper in `probe_test.go` is named `requestedSpan`. `wholeFileRange` belongs to `download.go`.

## Browser extension, as it feeds the engine

This is already on `main`. It is not part of the engine pull requests.

`extension/background.js` `handOff` sends:

- cookies from `chrome.cookies.getAll` for that URL, rebuilt, not the live `Cookie` header of the request that is in flight
- `referrer`
- `navigator.userAgent`

`chrome.webRequest.onHeadersReceived` reads response headers only (`Content-Type`, `Content-Length`, `Content-Range`) to notice media. The media threshold is `MIN_MEDIA_BYTES`, 512 KiB. There is no `onBeforeSendHeaders` and no `onSendHeaders`.

Manifest permissions: `downloads`, `contextMenus`, `storage`, `nativeMessaging`, `cookies`, `webRequest`. The native host name is `com.ochi.odm`. The Chromium id is pinned by the manifest `key`. The Firefox id is `odm@troke.id`.

`cmd/odm/native.go` `relayDownload` can accept `m.Headers` plus `Referer`, `Cookie`, and `User-Agent`. The extension does not fill the `headers` map. In the engine, `applyHeaders` runs first, and then a non-empty `d.cookies` overwrites the `Cookie` header.

So `Authorization`, site-specific request headers, and the live page `Cookie` do not reach the download. What reaches it is the cookie jar for that URL, the referrer, and the user agent.

The analyzed product has user-visible text for the same gap. Some sites allow the file to be requested only once. If the browser already requested it, the manager's second request receives a web page. The product's advice to the user is to hold the force key so the manager issues the request first, or to open the page again so a new address or new session data can be captured. Another message says a browser VPN extension can put the request on a proxy the manager never sees. ODM does not have a force key and does not forward live request headers. That is a recorded gap, not a request to imitate one-time URL capture.

## Main window

The main dialog title is "Internet Download Manager 6.43". It hosts a list and a category tree, with classic menu 129.

Tasks: Add new download, Add batch download, Add batch download from clipboard, Run site grabber, Show drop target, Export (to the product's export file, or to text), Import (from that export file, or from text), Exit.

A File section in the same menu: Stop Download, Remove, Download Now, Redownload.

Downloads: Pause All, Stop All, Delete All Completed, Find (Ctrl-F), Find Next (F3), Scheduler, Start queue, Stop queue, Speed Limiter (turn on, turn off, settings), Options.

View: Hide categories, arrange by order added, file name, size, status, time left, transfer rate, last try date, description, save path, referrer, or parent page. Toolbar: customize, large buttons, small buttons, look for new toolbars, Neon. Tray icon: 3D, classic, or hidden. Customize URL list. Dark mode (the string table says it follows the Windows dark-app setting). Font: choose or reset. Language, with English as a menu item and many `idm_*.lng` packs on disk.

Help: contents, tutorials, scheduler and queues, grabber help, tip of the day, home page, support, check for updates, about, tell a friend. Registration and Order are present and excluded.

The older main-window help describes the list columns (size, status, time left, rate, description), sorting by a column header, and a toolbar with Add URL, Start/Resume, Stop, Stop All, Delete, Delete All Completed, Options, Scheduler, Start Queue, Stop Queue, and Tell A Friend. Add URL can take a new address or pick an existing one, plus optional server authorization. Save-as leads to a progress dialog with a per-segment chart. Pause postpones. Cancel stops and closes. Resume requires the server to support it. Delete is enabled for a complete or paused download and confirms when the file is incomplete. The "Q" column marks the queue. The help's pictures use three yellow files for the main download queue, three green files for the main synchronization queue, two yellow files for an extra download queue, two green files for an extra synchronization queue, and a small clock when the queue is scheduled.

ODM's desktop columns are File Name, Q, Size, Status, Time left, Transfer rate, Last attempt, Description (`columnTitles` in `internal/ui/desktop.go`). The View menu sorts by name, size, status, time left, rate, and description. It does not list every column, and columns cannot be hidden. There is no Find. The theme in `theme_desktop.go` is a fixed dark palette. There is no light theme, no Windows-dark toggle, no language pack, and no toolbar skin.

ODM's desktop menus are thinner. Tasks: Add new download, Grab site, Export download list, Import download list, Exit. Downloads: Pause all, Stop all, Clear completed, Start queue, Stop queue, New queue, Scheduler, Options. The desktop "Stop all" item calls `PauseAll`, the same function as "Pause all". Help: Check for updates, About. Export and import are ODM's own JSON list, not a foreign export format. The web UI (`internal/server/web/index.html`) has Add URL, Resume, Stop, Stop All, Delete, Clear Done, Options, Pause All, Resume All, and Download Later. The progress window has tabs Download status, Speed limiter, and Options on completion.

A second launch of ODM finds the control socket and posts `/api/show` instead of starting another copy. Config flags in `cmd/odm/main.go`: `-addr` default `127.0.0.1:8721`, `-serve`, `-dir`, `-conns`, `-files`, `-add`, `-native-messaging`, `-open`, `-no-ui`, `-version`, `-hidden`, `-register-browser`, `-unregister-browser`.

## Options

Dialog titles that correspond to option pages: General, File types, Save to, Downloads, Connection, Proxy / Socks, Sites Logins, Site login, Dial Up / VPN, Sounds, Browsers integration, Browser details. Related dialogs: Max. connections number for a server, Using special keys, Advanced proxy settings, Download panel exceptions, Virus checking, Customize download progress dialog, Speed Limiter settings, the address-exception list, and per-browser integration pages (Chrome, Edge, IE, Firefox and other Mozilla-based browsers).

What those pages do, from the help plus the strings that confirm the 6.43 UI:

- General. Launch at startup. Clipboard monitoring, which is separate from browser capture. Basic integration versus advanced integration. The advanced path installs a network driver and needs the Windows Base Filtering Engine service. It can fail when that service is missing or stopped, when a Kaspersky TDI driver conflicts, or when the helper is not allowed to run elevated. The user can turn it off. An IE click-monitor agent can be enabled. Keys open the special-keys dialog. A download panel can be shown on a web player, with a minimum size and a switch that stops automatic capture from players.
- File types. Extensions that are taken over automatically. A site exception list accepts a leading asterisk (`*.example.com` style). A separate address-exception list exists. Unchecking a type that is still on the file-type list does not by itself stop capture.
- Connection. A link-speed choice, the max connection count, per-server exceptions, and a rolling quota (megabytes in a time window). Hitting the quota stops downloads and resumes them at a later clock time. The binary warns that a high connection count can get the machine blocked.
- Save to. A default directory per category, an option to remember the last save path, and a separate temporary directory. "Change folder for this category on last selected" is a string.
- Downloads. Show or hide the start dialog, show the progress dialog or show it minimized, show the complete dialog. Silent mode is the combination of those being off. One option starts the transfer while the file-info dialog is still open, so bytes can arrive while the user picks a folder. Another ignores file-modification-time changes on resume. An optional antivirus executable and parameters run after completion. The product's own text says a real-time scanner already sees the file, and that the external scanner is for the case where real-time protection is off. Assembly can fail with access denied if something else holds the file. Do not turn that into advice to exclude a process from antivirus.
- Proxy / Socks. Copy settings from the system browser, FTP passive mode, proxy username and password, and bypass patterns. The program can notice an IE proxy or a PAC script and ask to reuse it.
- Site logins. Username and password per site or directory. The help ties part of this page to a registered install. Do not analyze that side. The behavior ODM needs is only "store a login and send Basic".
- Dial Up / VPN. Windows phone-book entries, redial count, and interval, via the RAS API. The program can notice that Windows dial-up is in use and offer to adopt it.

ODM's options tabs are General, File types, Save to, Downloads, Connection, Proxy / Socks, and Site logins.

- General: register or unregister the native host, launch at startup into the tray, keep running in the tray when the window closes, clipboard monitor, update check.
- File types: the takeover list, restored by `DefaultFileTypes()`. The same list feeds the clipboard monitor and the grabber.
- Save to: one default folder, optional category subfolders, and a folder override per category. No separate temporary directory and no "remember the last path" switch were found.
- Downloads: show the new-download window for browser items, show the progress window, show the complete dialog. Those three do not apply to queue processing. A duplicate from the browser, clipboard, or grabber is skipped or downloaded again. Adding a duplicate by hand always asks. User-Agent. Exclude sites, space-separated, with a leading `*.` for subdomains.
- Connection: default max connections 1 through 32, simultaneous downloads, a global KB/s limit (0 is off), a per-host exception table, and a rolling limit of N megabytes every N hours with an optional warning. No high/low link preset and no "resume at clock time" were found.
- Proxy / Socks: no proxy, system, or manual HTTP or SOCKS5, with host, port, user, and password. No FTP passive switch, because ODM has no FTP.
- Site logins: a host, user, and password table. The engine sends Basic.

ODM has no sounds page, no dial-up or VPN page, no virus-scan page, no download-panel page, and no per-browser integration page.

## Scheduler and queues

The scheduler opens from the toolbar, from Downloads, or from a queue's context menu (edit, schedule, create). Dialog titles: Scheduler, Schedule, Files in the queue, Enter queue name, IDM Queues.

There are two built-in queue kinds plus any number of extra queues. The main download queue is one-time: a file leaves the queue when it finishes. The main synchronization queue is periodic: it checks whether the server file changed, replaces the local file, and keeps the item. Extra queues exist. A periodic sync can repeat every N minutes or hours until "Stop download at" or the end of the day. The stop time is independent of the start checkbox. A failed file goes to the end of the queue. If it is the only one left, it is retried forever every 30 seconds unless "Number of retries for each file if downloading failed" caps that. `tips.txt` states the 30-second retry. On a normal finish, not when the user stops the queue, the scheduler can hang up the modem, exit, turn the computer off, sleep, or hibernate. One option forces processes to terminate. Its own tooltip says that can lose data. ODM does not have that option. Another option opens a file or runs a program. A `.bat` is how several programs are run. The grabber scheduler waits 10 seconds after a project finishes before it runs the completion action.

The files tab reorders with arrows or drag, deletes, and can drag an item onto another queue. A completed file is the only kind that may be dropped onto a synchronization queue. Each queue has its own simultaneous-file count. A site that allows one connection is supposed to get its own queue set to one file, plus a connection-tab exception of one connection.

A scheduled start can ask Windows to wake the machine. The product tells the user to enable wake timers. It will say so when wake timers are off.

New files enter the main download queue and leave it when complete. The user can move items from a context menu or by dragging onto the queue list. "Don't add to any queue" exists.

ODM queues are a flat name list. The default name is `Main` and it cannot be removed. Any extra queue is the same kind of queue. There is no synchronization queue that rechecks the server and replaces the local file. `QueueSchedule` stores: start on ODM startup, start at `HH:MM` once on a date or daily on Sunday through Saturday, stop at `HH:MM`, a retry cap, a file to open when the queue runs out of work, exit, shut down the computer, and how many of that queue's files run at once. There is no every-N-minutes period, no modem hang-up, no sleep, no hibernate, and no force-terminate. Failed files requeue only when the retry cap is enabled.

## Site grabber

The grabber is a project with its own window ("IDM Site Grabber", menu 256). Project: New, Load, Save, save current settings as a template, recent projects, Close. Options: Grabber Settings, Schedule the project.

The wizard, from the grabber help:

1. Start page. `http` is assumed. `https` and `ftp` must be typed. Optional login, a "enter login and password manually" path that opens a browser after step 4 (the helper embeds the IE browser control), logout URLs that must not be opened, a project name shown in the main category tree, and templates such as pictures, video, audio, whole site, or custom.
2. Where to save. Per-file category folders, one chosen category, or the site's relative folders. For a saved site, links in downloaded pages can be rewritten to local relative links, and links that were not downloaded become absolute. Overwrite, or rename to `index_2.html` style. Optionally add the checked files to the main list when the grabber closes.
3. Page filters. Levels on this site and on other sites are about pages, not about the files linked from a page. Level 0 on both means only the start page. Ignore popups. The grabber does not open browser windows except for the manual login. Do not walk parent directories. Explore every site in the main domain. Optional JavaScript, which the help says to use only on a site the user trusts, because a script can do more than reveal a link. With that option off, the grabber still scans script text and recovers most links, except those built by a complicated expression. Include and exclude filters on domain and path, with an asterisk.
4. File filters. Include and exclude types, comma-separated, no spaces, asterisks allowed, and a token for the start page. Search this site only. Hide duplicates that share a name and a size, which is disabled when original relative folders are on. Start every match at once, or review first. Path and domain filters. Minimum and maximum size.

The main action window has a site tree and a file list. Toolbar actions, confirmed by the string table: Start Exploring, Stop Exploring, Check All Files, Uncheck All Files, Start Downloading the checked files, Stop downloading the checked files, Add checked files to the main list and to the download queue, Show grabber statistics. Statistics stay on top. A file's context menu can open the file or its folder, and a properties item can rename the save name or copy the URL and referrer. The user can check a shift-selected group. Going back in the wizard and forward again resumes explore or download.

Grabber settings choose how many files to explore at once and how many to download at once. Link text can become the description filed on the main list. A default checkbox looks in the Internet Explorer cache and redownloads only when the cached file changed.

Scheduling a project can explore only, explore and download matches, or download the checked files, once. Periodic synchronization checks for changes and forces the overwrite option from step 2. The project must be closed, or open on the main action dialog and stopped. A project left inside the wizard is not started by the clock. The grabber itself retries a file only twice and then stops. The help tells the user to move a difficult file to the main queue, which retries without that cap. Completion can hang up, exit, shut down, sleep, hibernate, or open a file, after a 10-second wait.

Projects and templates are saved. Deleting a project is confirmed. A corrupt project file is rejected.

ODM's grabber is one dialog: start page, depth, max pages, max files, stay on the same host, start downloads immediately. The dialog defaults are depth 1, 50 pages, and 100 files. `Grab` in `internal/engine/grabber.go` uses depth 1, 100 pages, and 200 files when those values are not positive. It walks breadth-first, reads `href` with a regular expression, keeps `http` and `https`, and queues a link whose extension is on the file-type list. Pages are fetched with the current user agent and a 2 MiB body cap. There are no projects, templates, saved filters, JavaScript execution, size limits, duplicate hiding, login browser, cache lookup, relative-folder rewrite, or a scheduled project.

## Clipboard, drop target, batch, command line

Clipboard monitoring is its own option. One address prompts once. Several addresses prompt to download them as a set. An empty clipboard reports that no web address was found. ODM has a clipboard monitor tied to the file-type list. It does not have a batch-from-clipboard command.

The drop target is a small always-available window ("IDM drop target. Drop web-links for downloading here"), toggled from Tasks. Closing it hides it. The complete dialog can drag the finished file out so a player adds it to a playlist instead of opening it as a replacement. ODM has neither the floating target nor that drag-out.

Batch download is its own dialog, and Tasks can also start a batch from the clipboard. ODM has no numeric or wildcard batch.

The older command-line help, paraphrased: the executable accepts `/d` plus a URL, `/p` for a local directory, `/f` for a file name, `/q` to exit after success, `/h` to hang up after success, `/n` for silent mode, `/a` to queue without starting, and `/s` to start the scheduler queue. `/q` applies only to the first copy. `/a`, `/h`, `/n`, `/q`, `/f`, and `/p` apply only together with `/d`. ODM's flags are the Go flags listed under the main window. There is no `/d` style switch and no hang-up switch.

Import from a text file finds every URL and lets the user edit the list before adding. The 6.43 menu also imports and exports the product's own list format. ODM imports and exports its JSON list.

## Categories, find, and the list

Categories have a name, a default directory, and associated extensions. The help's built-in set includes Music, Video, Programs, and Documents, and the user can add, edit, and delete. If start dialogs are off, a file goes to the category folder on its own. The queues node in the same tree filters the list.

ODM's categories are fixed: Video, Audio, Documents, Programs, Compressed, Images, Disk Images, and Other (`categoryExt` in `settings.go`). The user can turn category subfolders on and can override each folder. The user cannot add a category or edit its extension list. The file-type list is a separate takeover list and does not rewrite `categoryExt`.

Find and Find Next search the list. ODM has no list search. File properties in the analyzed product can change the path, the URL, and the description, and can open the file. ODM can replace a URL while keeping progress when the server can resume. That dialog tells the user to paste the new address of the same file.

Zip preview is a dialog shown before a download of an archive. ODM does not preview archives.

Duplicate-link handling asks what to do, or resumes a partial file and shows the complete dialog when the existing file is already complete. ODM's rule is skip or download again for automatic adds, and always ask for a hand add.

## Sounds, virus check, updates, dial-up

Sounds are per-event wave files copied into a sounds folder and played with `PlaySoundW`. ODM plays no event sounds.

Virus checking launches an external program after the download completes (`CreateProcess` is named in the error text). Failure to start it is its own error. ODM does not launch a scanner.

Updates are a help-menu check, a "new version" dialog, and a quick-update dialog. A string says the program is updated often because browsers and sites change. ODM has its own update check. Do not point ODM at the other product's update server.

Dial-up progress strings cover port open, authentication, callback, and modem reset. Queue completion can hang up the modem. ODM does not dial or hang up.

## ODM surfaces that already match, and gaps that do not

Already in ODM, so do not file them as missing:

- Segmented HTTP with resume, a shared `net/http` client, system or manual proxy, per-host connection caps, a global speed limit, and a rolling megabyte budget.
- The eleven engine behaviors on `engine-local-test`, with the semantics in the combined `Start` section.
- A desktop list with categories, queues, a scheduler, a grabber, clipboard monitoring, a tray, browser native messaging, and a small web UI.
- Add, pause, stop, resume, delete, clear completed, progress window, complete dialog, duplicate policy, site logins, and exclude sites.

Recorded gaps. They are not a request to implement them in this note:

- Synchronization queues, periodic every-N-minutes sync, hang-up, sleep, hibernate, and force-terminate.
- Dial-up and VPN phone book.
- Event sounds.
- An external virus scan after completion.
- Zip preview before download.
- Numeric or wildcard batch, and batch from the clipboard.
- A floating drop target and drag-out from the complete dialog.
- Column and toolbar customization, skins, a light theme, a Windows dark-mode toggle, and language packs.
- A shell extension, a network-monitor capture driver, and a per-browser video download panel.
- Grabber projects, templates, JavaScript, size filters, duplicate hiding, a manual-login browser, cache lookup, and a scheduled project.
- Ignore-mtime-on-resume as its own switch. ODM uses If-Range, which restarts when the validator changes.
- Start storing file bytes while the add dialog is still open. ODM probes in that dialog and transfers after the user confirms.
- Remember the last save path automatically, and a separate temporary directory.
- FTP passive mode, and FTP at all.
- Find in the list.
- Live browser request headers. See the extension section.
- The desktop "Stop all" item currently calls `PauseAll`.

## Known engine limits

Not started, and do not start it unless asked:

- Review item 8: optional user SHA-256. A mismatch downloads once more from byte 0. It does not replace the 64-byte overlap check.

Seen, and not yet a pull request:

- A worker that fails hard can leave a sibling piece unfinished. The bounded retry already exists.
- `429` and `Retry-After` are not special.
- A probe without `Content-Length` fails.
- The probe is not stall-bounded.
- `discardPrefix` is not stall-bounded.
- Probe cookies can replace the caller's `Cookie`.
- State saved about every 250 ms does not mean those bytes have been fsynced.
- Live browser request headers are not forwarded.

## How to add one behavior

1. `git fetch upstream`.
2. Branch from `upstream/main` (`5350f79` until upstream moves). Do not branch from `engine-local-test` or from this docs branch.
3. One behavior. Tests in `internal/engine`. `go test ./...` and `go vet ./...` with `GOTOOLCHAIN=local`.
4. Changelog and README only for that behavior, under `## [Unreleased]`.
5. Push to `origin`, then `gh pr create` to `troke12/odm` when asked. English. Do not name the analyzed product.
6. For a combined Windows test, merge that branch into `engine-local-test` locally. A `download.go` conflict is usually both sides adding a helper. Keep both. The write loop counts only `ownedSpan` bytes. `exchange` must keep taking the segment client. Do not put back `ch.Done += int64(n)` without the clamp.
7. Do not open `engine-local-test` or this docs branch as an upstream pull request.

Files the engine work touches most: `internal/engine/download.go`, `plan.go`, `probe.go`, `manager.go`, `types.go`, `http.go`, plus `*_test.go`, `README.md`, and `CHANGELOG.md`.

UI work that is not the transfer engine belongs in `internal/ui` (`desktop.go`, `dialogs_desktop.go`, `scheduler_desktop.go`, `theme_desktop.go`), `internal/engine/grabber.go`, `internal/engine/settings.go`, `cmd/odm`, and `extension/`. One behavior per pull request still applies. A UI pull request is also branched from upstream `main`, not from this docs branch.

## File map

| File | Role |
| --- | --- |
| `internal/engine/download.go` | `Start`, `fetch`, stall, overlap, conflict, ignored range, ramp, state |
| `internal/engine/plan.go` | `NewPlan`, `Split` |
| `internal/engine/probe.go` | `Inspect`, `ParseContentRange` |
| `internal/engine/manager.go` | `buildClient`, `segmentTransport`, queues, `Readdress` |
| `internal/engine/types.go` | constants, `Chunk`, pace |
| `internal/engine/http.go` | `applyHeaders` |
| `internal/engine/limiter.go` | rate limit |
| `internal/engine/store.go` | saved state |
| `internal/engine/grabber.go` | breadth-first `href` crawl |
| `internal/engine/settings.go` | options, categories, `QueueSchedule`, `DefaultFileTypes` |
| `internal/ui/desktop.go` | main window, menus, columns, tray |
| `internal/ui/dialogs_desktop.go` | add, options, grabber, progress |
| `internal/ui/scheduler_desktop.go` | scheduler window |
| `internal/ui/theme_desktop.go` | fixed dark palette |
| `internal/server/web/index.html` | browser UI |
| `cmd/odm/main.go` | flags, single instance |
| `cmd/odm/native.go` | extension bridge |
| `extension/background.js` | handoff from the browser |
| `extension/manifest.json` | permissions and host name |

ODM constants that were chosen on purpose. Do not change them so they "match another program":

- stall 30 seconds
- overlap 64 bytes
- minimum piece 64 KiB
- slow tail 2x and a 200 ms window
- default 8 connections, cap 32
- at most 8 retries, starting at 300 ms, capped at 8 seconds
- ramp with no extra delay

## Local artifacts

These stay on disk and out of git.

| Path | What it is |
| --- | --- |
| `C:\rea-work\IDMan.exe` | The analyzed copy |
| `C:\rea-work\ghidra-proj\IDMan` | Saved Ghidra project. Reuse it. Do not reimport |
| `C:\rea-work\idman-snapshot.json` | REA snapshot. May contain a decompiled procedure. Do not copy it |
| `C:\rea-work\scripts\Dump*.java` and `C:\rea-work\engine-*.txt` | Earlier engine dumps |
| `C:\rea-work\ui\rea-pe.json` | Resource inventory. Contains offsets. Do not copy them |
| `C:\rea-work\ui\rea-search.json` and `rea-search2.json` | String searches. Contains addresses. Do not copy them |
| `C:\rea-work\ui\res\` | Dialog titles, menus, and string tables. Control captions in those dumps are misaligned |
| `C:\rea-work\ui\chm\` | Decompiled help. Do not paste it into the repo |

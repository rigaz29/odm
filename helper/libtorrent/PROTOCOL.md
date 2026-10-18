# odm-libtorrent protocol, version 1

ODM starts `odm-libtorrent [--state <file>]` and talks to it over its
standard input and output: one JSON object per line, UTF-8, ending in `\n`.
ODM sends **commands**. The program sends a **reply** to each command, and
**events** whenever something happens. Diagnostics go to standard error.

`--state` names a file that keeps the session's DHT nodes between runs, so
that magnet links find peers sooner.

The program exits when its input closes, which includes ODM crashing, or
on `quit`. Before exiting it pauses every torrent, writes out its cache and
then its resume data, waiting for that for at most 10 seconds. Then it
sends `bye`.

## Conventions

- A torrent is named by a **key**, a string ODM chooses (its download's ID).
- Paths are absolute, UTF-8, with `/` or the system's separator. Paths the
  program sends back always use `/`.
- File **indexes** are positions in the torrent's file list, as the
  `metadata` event and `inspect` give them. Pad files (`"pad": true`) keep
  their index.
- Speed limits are in bytes per second; `0` means no limit.
- File **priorities** are 0 (do not download) to 7. 4 is the default.

## Commands and replies

A command has a `cmd` and, optionally, an `id`. The reply carries the same
`id` (`null` when the command had none or the line was not JSON) and `ok`:

```json
{"id": 7, "cmd": "pause", "key": "a1b2"}
{"id": 7, "ok": true}
{"id": 8, "ok": false, "error": "no torrent \"zz\""}
```

A reply only says that the command was taken. What follows from it, such
as a rename or a move, comes later as events.

### settings

Changes the session. Every field is optional; the program starts with
nothing listening and DHT, LSD and port mapping off.

| Field | |
|---|---|
| `listen` | libtorrent's `listen_interfaces`, e.g. `"0.0.0.0:6881,[::]:6881"` |
| `incoming` | accept incoming connections (TCP and uTP) |
| `port_forwarding` | UPnP and NAT-PMP |
| `dht`, `lsd` | DHT and local service discovery |
| `download_limit`, `upload_limit` | for the whole session |
| `user_agent`, `peer_fingerprint` | what trackers and peers are told |
| `proxy` | `{"type": "none" \| "socks5" \| "http", "host", "port", "user", "password", "peers"}`; `peers` (default true) sends peer connections through it too |
| `status_interval` | milliseconds between `status` events, 0 for none (default 1000) |

### inspect

Reads a torrent without adding it: `{"torrent": "<path>"}` or
`{"magnet": "<link>"}`. For a .torrent the reply has the fields of the
`metadata` event; for a magnet link only `name`, `trackers`, `web_seeds`
and the info hashes.

### add

Adds a torrent. The reply has `info_hash` (v1, hex), `info_hash_v2` (when
the torrent has one) and `resumed`.

| Field | |
|---|---|
| `key` | required, unique |
| `torrent` / `magnet` | a .torrent file or a magnet link |
| `resume` | where the torrent's resume data is kept. If it can be read, the torrent continues from it (`"resumed": true`), and its metadata comes from it too; neither `torrent` nor `magnet` is then needed. The program rewrites it whenever the torrent's state changes |
| `save_path` | required: where the files go |
| `names` | `{"<index>": "<relative path>"}`: files under other names. Needs the metadata (a .torrent or resume data); for a magnet link, use `rename` |
| `priorities` | a priority for each file |
| `hold` | for a magnet link: download no file until `priorities` gives them |
| `paused` | add it paused |
| `download_limit`, `upload_limit` | for this torrent |
| `peers` | `["ip:port", …]` to connect to |
| `drop_trackers`, `drop_web_seeds` | URLs to leave out of the torrent's trackers and web seeds (ODM leaves out those on its own network for a torrent from the internet) |

ODM runs its own queue: the program never starts or stops torrents by
itself.

Pieces that a wanted file shares with a file not downloaded are kept in
`.<info hash>.parts` in `save_path`, and move with the files. Only
`remove` with `delete_files` deletes it; otherwise it is ODM's to delete
once the torrent is removed.

### remove

`{"key", "delete_files": false}`. Also deletes the resume data. `removed`
follows.

### pause, resume

`{"key"}`. `resume` also clears an error the torrent stopped on.

### priorities

`{"key", "priorities": [4, 0, …]}`: a priority for each file.

### rename

`{"key", "names": {"<index>": "<relative path>"}}`. A `renamed` or
`rename_failed` event follows for each file.

### move

`{"key", "save_path"}`: moves the torrent's files there; fails if a file
exists at the target. `moved` or `move_failed` follows.

### limits

`{"key", "download_limit", "upload_limit"}`, both optional.

### connect

`{"key", "peer": "ip:port"}`.

### status

`{"key"}`, or nothing for every torrent. The reply has `torrents`, a list
of status objects (see the `status` event).

### pieces

`{"key"}`. The reply has `count`, the number of pieces, and `have`: base64
of a bitfield with one bit per piece the torrent has, the first piece in the
high bit of the first byte.

### file_progress

`{"key"}`. The reply has `files`: the bytes downloaded of each file.

### quit

Like closing the input: the program saves resume data, sends `bye` and
exits.

## Events

| Event | Fields |
|---|---|
| `hello` | `protocol`, `version`, `libtorrent`; the first line, before any command |
| `listening` | `address`, `port`, `type` (`tcp`, `utp`, …) |
| `listen_failed` | `address`, `port`, `type`, `error` |
| `portmap` | `port` (external), `via` (`upnp` or `natpmp`) |
| `portmap_failed` | `via`, `error` |
| `metadata` | `key`, `name`, `size`, `piece_size`, `pieces`, `private`, `files` (`path`, `size`, `pad`), `trackers`, `web_seeds`, `info_hash`, `info_hash_v2`, `magnet`. Right after `add` for a torrent with metadata; for a magnet link when it arrives. Paths are the torrent's own, before `names` |
| `status` | `torrents`: status objects of the torrents that changed, every `status_interval` |
| `finished` | `key`: every wanted file is downloaded, checked and written to the disk. Not sent while nothing is wanted (`hold`) |
| `renamed` | `key`, `index`, `path` |
| `rename_failed` | `key`, `index`, `error` |
| `moved` | `key`, `path` |
| `move_failed` | `key`, `error`, `file` |
| `resume_saved` | `key` |
| `removed` | `key`, and `error` when deleting files failed |
| `error` | `key`, `error`, and `file`, `op` when known |
| `bye` | the last line |

A status object:

| Field | |
|---|---|
| `key` | |
| `state` | `checking_resume_data`, `checking`, `metadata`, `downloading`, `finished` (wanted files done) or `seeding` (every file done) |
| `paused`, `has_metadata`, `finished`, `seeding`, `moving` | |
| `size`, `done` | bytes wanted and bytes of those downloaded and checked |
| `total`, `total_done` | the same for every file |
| `download_rate`, `upload_rate` | payload bytes per second |
| `downloaded`, `uploaded` | payload bytes over the torrent's life |
| `failed_bytes` | bytes that failed their hash check |
| `peers`, `seeds` | connected |
| `swarm_peers`, `swarm_seeds` | as trackers report them, -1 when unknown |
| `active_time`, `seeding_time` | seconds |
| `error` | when the torrent stopped on an error |

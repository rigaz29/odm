# odm-libtorrent

The companion program ODM runs to download torrents, on
[libtorrent](https://www.libtorrent.org/) (libtorrent-rasterbar 2.1).
ODM downloads it the first time a torrent is added and talks to it over its
standard input and output ([PROTOCOL.md](PROTOCOL.md)). It is a separate
program so that `ODM.exe` stays free of C++ and cgo, and so that a crash in
it stops the torrents, not ODM (docs/ROADMAP.md, Fase 8).

## Building

The workflow `.github/workflows/odm-libtorrent.yml` builds it for Windows
x64, Linux x64 and ARM64, and macOS (Apple silicon and Intel), with
libtorrent, Boost, OpenSSL and nlohmann/json built statically by
[vcpkg](https://vcpkg.io/) at the commit `vcpkg.json` names. By hand:

```bash
git clone https://github.com/microsoft/vcpkg && ./vcpkg/bootstrap-vcpkg.sh
cmake -S helper/libtorrent -B build -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_TOOLCHAIN_FILE=$PWD/vcpkg/scripts/buildsystems/vcpkg.cmake
cmake --build build --config Release
```

The first build compiles Boost and OpenSSL, which takes 7 to 20 minutes.

## Testing

The tests are Go, in this folder. They run the program, with every peer on
127.0.0.1, and skip without it:

```bash
ODM_LIBTORRENT=$PWD/build/odm-libtorrent go test ./helper/libtorrent/
```

`odm-libtorrent create <file or folder> <out.torrent>` makes the torrents
they use.

## Releasing

A tag `odm-libtorrent-p<protocol>-<libtorrent version>-<build>`, such as
`odm-libtorrent-p1-2.1.2-1`, publishes a pre-release that ODM finds by the
prefix of its protocol. Change `kProtocol` in `src/version.hpp`, and the
prefix ODM looks for, when a change would break an ODM that knows the
previous protocol.

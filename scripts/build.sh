#!/usr/bin/env bash
# Build ODM.
#
# Windows builds the desktop UI of internal/gui (build tag "gui", see
# docs/GUI.md): the page first (needs Node and npm), then the exe, with no
# cgo. Linux and macOS build the Fyne UI (build tag "fyne"), which needs cgo:
#   Linux    -> libgl1-mesa-dev xorg-dev
#   macOS    -> Xcode command line tools
# The "windows-fyne" target builds the Fyne UI for Windows too, with
# mingw-w64 (CC_WINDOWS, default x86_64-w64-mingw32-gcc).
# The "cli" target builds the engine + web UI with no cgo and no desktop UI.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${OUT:-dist}"
mkdir -p "$OUT"

# nomanifest prints -extldflags that keep the toolchain's default manifest
# out of the exe, or nothing when the toolchain has none. mingw-w64 builds
# such as WinLibs and MSYS2 link default-manifest.o into every program; ODM
# embeds its own (cmd/odm/rsrc_windows_amd64.syso), and ld then keeps both
# under the same ID, so Windows may load the one without DPI awareness. An
# empty default-manifest.o that gcc finds first through -B replaces it.
nomanifest() {
  local cc="$1" found stub
  found="$("$cc" -print-file-name=default-manifest.o 2>/dev/null || true)"
  case "$found" in
    */*|*\\*) ;;          # a path: the toolchain ships one
    *) return 0 ;;
  esac
  stub="$OUT/.nomanifest"
  mkdir -p "$stub"
  echo "" | "$cc" -x c -c - -o "$stub/default-manifest.o"
  # gcc on Windows needs a Windows path; pwd -W gives one under Git Bash.
  stub="$(cd "$stub" && (pwd -W 2>/dev/null || pwd))"
  printf ' -extldflags=-B%s/' "$stub"
}

desktop() {
  local goos="$1" goarch="$2" name="$3" ldflags="-s -w"
  [ "$goos" = "windows" ] && ldflags="-s -w -H=windowsgui"
  echo "building desktop $goos/$goarch -> $OUT/$name"
  if [ "$goos" = "windows" ]; then
    local cc="${CC_WINDOWS:-x86_64-w64-mingw32-gcc}"
    ldflags="$ldflags$(nomanifest "$cc")"
    CGO_ENABLED=1 CC="$cc" GOOS="$goos" GOARCH="$goarch" \
      go build -tags fyne -trimpath -ldflags "$ldflags" -o "$OUT/$name" ./cmd/odm
  else
    CGO_ENABLED=1 GOOS="$goos" GOARCH="$goarch" \
      go build -tags fyne -trimpath -ldflags "$ldflags" -o "$OUT/$name" ./cmd/odm
  fi
  echo "built $OUT/$name"
}

cli() {
  local goos="$1" goarch="$2" name="$3"
  echo "building cli $goos/$goarch -> $OUT/$name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "-s -w" -o "$OUT/$name" ./cmd/odm
  echo "built $OUT/$name"
}

gui() {
  local name="$1"
  echo "building gui windows/amd64 -> $OUT/$name"
  (cd internal/gui/frontend && npm ci --no-audit --no-fund && npm run build)
  # production: Wails' release mode, without WebView2's own right-click
  # menu (Back, Refresh, Print, Inspect) and its developer tools.
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
    go build -tags gui,production -trimpath -ldflags "-s -w -H=windowsgui" -o "$OUT/$name" ./cmd/odm
  echo "built $OUT/$name"
}

case "${1:-windows}" in
  windows) gui ODM.exe ;;
  windows-fyne) desktop windows amd64 ODM.exe ;;
  linux) desktop linux amd64 odm ;;
  darwin) desktop darwin arm64 odm ;;
  darwin-intel) desktop darwin amd64 odm ;;
  cli) cli linux amd64 odm-cli ;;
  all)
    gui ODM.exe
    desktop linux amd64 odm
    desktop darwin arm64 odm
    ;;
  *) echo "usage: $0 [windows|windows-fyne|linux|darwin|darwin-intel|cli|all]" >&2; exit 2 ;;
esac

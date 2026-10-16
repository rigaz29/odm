#!/usr/bin/env bash
# Rebuild cmd/odm/rsrc_windows_amd64.syso, the Windows resources linked into
# every Windows build of ODM (the Fyne UI and the new one alike): the
# manifest (cmd/odm/odm.manifest), the icon (drawn by internal/gui/icon.go)
# and the name Explorer and Task Manager show (scripts/winres.json). The
# icon is the exe's first, ID 3, the one Windows notifications show.
# Needs the wails3 CLI (go install github.com/wailsapp/wails/v3/cmd/wails3).
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
go run ./scripts/appicon "$tmp/odm.ico"
wails3 generate syso -arch amd64 -icon "$tmp/odm.ico" \
  -manifest cmd/odm/odm.manifest -info scripts/winres.json \
  -out cmd/odm/rsrc_windows_amd64.syso
echo "wrote cmd/odm/rsrc_windows_amd64.syso"

#!/usr/bin/env bash
# Build the small ffmpeg (and ffprobe) ODM downloads for Windows.
#
# ODM never re-encodes: it joins a video track with an audio track (HLS/DASH
# streams, and yt-dlp's merges for YouTube) and copies the codecs into
# another container. So this build has no encoders and no filters beyond
# the few the ffmpeg program requires, only:
#   - containers: MP4/M4A/MOV, Matroska/WebM, MPEG-TS, Ogg, and raw AAC,
#     MP3, FLAC, AC-3 audio; FLV read only (live streams such as TikTok's),
#     and the concat demuxer, which joins a recording's parts;
#   - parsers and bitstream filters for H.264, H.265, VP8/VP9, AV1, AAC,
#     Opus, Vorbis, MP3, AC-3 and FLAC, which copying between those
#     containers needs;
#   - audio decoders, which reading a file's stream parameters uses.
# The result is LGPL: no --enable-gpl and no external libraries. The
# release ODM downloads carries this script, the FFmpeg version and its
# licence, which is what the LGPL asks of a binary.
#
# usage: scripts/ffmpeg-min.sh <ffmpeg source dir> <out dir> [cross prefix]
#   cross prefix: "x86_64-w64-mingw32-" on Linux; empty when building with
#   MinGW on Windows itself (Git Bash with WinLibs).
set -euo pipefail

# pwd -W gives Git Bash's paths in Windows form ("C:/..."), which a native
# make (mingw32-make) can open; elsewhere it fails and pwd is used.
abs() { (cd "$1" && (pwd -W 2>/dev/null || pwd)); }
src="$(abs "$1")"
mkdir -p "$2"
out="$(abs "$2")"
cross="${3:-}"
make="${MAKE:-make}"
jobs="${JOBS:-$(nproc 2>/dev/null || echo 4)}"

list() { local IFS=,; echo "$*"; }

protocols=(file pipe)
demuxers=(mov matroska mpegts ogg aac mp3 flac ac3 eac3 wav flv live_flv concat)
muxers=(mp4 mov ipod matroska webm mpegts ogg opus adts mp3 flac null)
parsers=(h264 hevc vp8 vp9 av1 mpeg4video mpegvideo aac aac_latm ac3 mpegaudio opus vorbis flac dca)
bsfs=(aac_adtstoasc h264_mp4toannexb hevc_mp4toannexb extract_extradata
  vp9_superframe vp9_superframe_split av1_frame_merge av1_frame_split null)
decoders=(aac aac_latm ac3 eac3 mp3 opus vorbis flac)

args=(
  --disable-everything --disable-autodetect --disable-network
  --disable-doc --disable-debug --disable-ffplay --disable-avdevice
  --disable-swscale --enable-small --disable-x86asm
  --enable-ffmpeg --enable-ffprobe --enable-w32threads
  --enable-static --disable-shared
  --target-os=mingw32 --arch=x86_64
  --extra-ldflags=-static
  --enable-protocol="$(list "${protocols[@]}")"
  --enable-demuxer="$(list "${demuxers[@]}")"
  --enable-muxer="$(list "${muxers[@]}")"
  --enable-parser="$(list "${parsers[@]}")"
  --enable-bsf="$(list "${bsfs[@]}")"
  --enable-decoder="$(list "${decoders[@]}")"
)
if [ -n "$cross" ]; then
  args+=(--enable-cross-compile --cross-prefix="$cross")
fi

build="$out/build"
mkdir -p "$build"
cd "$build"
"$src/configure" "${args[@]}"
# configure records its own source path in Git Bash's form ("/tmp/..."),
# which a native make cannot open: write it the Windows way.
if win="$(cd "$src" && pwd -W 2>/dev/null)"; then
  msys="$(cd "$src" && pwd)"
  sed -i "s#$msys#$win#g" Makefile ffbuild/config.mak
fi
"$make" -j"$jobs"
strip=strip
[ -n "$cross" ] && strip="${cross}strip"
for p in ffmpeg ffprobe; do
  "$strip" "$p.exe"
  cp "$p.exe" "$out/"
done
echo "built $out/ffmpeg.exe $out/ffprobe.exe"

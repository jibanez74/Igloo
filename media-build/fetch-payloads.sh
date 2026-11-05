#!/usr/bin/env bash
# Installs the ffmpeg and ffprobe payloads pinned in payloads.lock for each
# platform given, as server/cmd/internal/<tool>/<tool>_<platform>. A payload
# that already matches its pin is left alone. Anything else is downloaded and
# moved into place only after its sha256 matches. Runs on macOS's bash 3.2.
#   fetch-payloads.sh <platform>...   (linux_amd64, darwin_arm64)
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(dirname "$here")
lock=$here/payloads.lock

if [ $# -eq 0 ]; then
  echo "usage: $0 <platform>..." >&2
  exit 2
fi

sha256() {
  if command -v sha256sum > /dev/null; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

for platform in "$@"; do
  found=0
  while read -r payload sum url member; do
    case $payload in '' | '#'*) continue ;; esac
    case $payload in *_"$platform") ;; *) continue ;; esac
    found=1
    dest=$root/server/cmd/internal/${payload%%_*}/$payload
    if [ -f "$dest" ] && [ "$(sha256 "$dest")" = "$sum" ]; then
      echo "$payload: up to date"
      continue
    fi

    # One download per url: both macOS payloads come from the same archive.
    download=$tmp/$(printf '%s' "$url" | cksum | cut -d' ' -f1)
    if [ ! -f "$download" ]; then
      echo "$payload: downloading $url"
      curl -fsSL --retry 3 -o "$download.part" "$url"
      mv "$download.part" "$download"
    fi
    file=$download
    if [ -n "$member" ]; then
      mkdir -p "$tmp/$payload"
      tar -xf "$download" -C "$tmp/$payload" "$member"
      file=$tmp/$payload/$member
    fi

    got=$(sha256 "$file")
    if [ "$got" != "$sum" ]; then
      echo "$payload: sha256 $got does not match payloads.lock ($sum)" >&2
      exit 1
    fi
    # Copy beside the destination first so the rename is atomic. The copy also
    # gets a fresh mtime, which tells make to recompress the .zst.
    cp "$file" "$dest.part"
    chmod 0755 "$dest.part"
    mv -f "$dest.part" "$dest"
    echo "$payload: installed"
  done < "$lock"
  if [ "$found" = 0 ]; then
    echo "payloads.lock pins nothing for $platform" >&2
    exit 1
  fi
done

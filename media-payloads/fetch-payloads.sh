#!/usr/bin/env bash
# Installs the ffmpeg and ffprobe payloads pinned in payloads.lock for each
# platform given, as server/cmd/internal/<tool>/<tool>_<platform>. A payload
# that already matches its pin is left alone. Anything else is downloaded and
# moved into place only after its sha256 matches. A download that stalls or
# drops resumes where it stopped. Runs on macOS's bash 3.2.
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

size() {
  if [ -f "$1" ]; then wc -c < "$1" | tr -d ' '; else echo 0; fi
}

# fetch <label> <url> <file>: GitHub's release CDN has been seen to crawl at a
# few KB/s and to drop connections mid-file, so a failed or stalled attempt
# resumes the partial file rather than starting over. It gives up once five
# attempts in a row add no bytes, or when a retry would start more than an
# hour after the download began: a link that keeps adding a trickle would
# otherwise retry for days. An hour leaves room for a 30 MB payload at about
# 10 KB/s. The caller still checks the sha256 of whatever lands in <file>.
fetch() {
  local label=$1 url=$2 out=$3 progress=-sS rc code before stuck=0
  local deadline=$((SECONDS + 3600))
  if [ -t 2 ]; then progress=--progress-bar; fi
  while :; do
    before=$(size "$out.part")
    rc=0
    code=$(curl -fL -C - --connect-timeout 30 --speed-limit 1024 --speed-time 60 \
      "$progress" -w '%{http_code}' -o "$out.part" "$url") || rc=$?
    # 416: the partial file already holds every byte.
    if [ "$rc" = 0 ] || [ "$code" = 416 ]; then break; fi
    case $rc in
      # The server ignored the range request, so the next attempt starts over.
      33) rm -f "$out.part" ;;
      22) case $code in
            408 | 429 | 5??) ;;
            *) echo "$label: $url returned HTTP $code" >&2; return 1 ;;
          esac ;;
    esac
    if [ "$(size "$out.part")" -gt "$before" ]; then stuck=0; else stuck=$((stuck + 1)); fi
    if [ "$stuck" -ge 5 ]; then
      echo "$label: download made no progress in 5 attempts, giving up" >&2
      return 1
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "$label: download still incomplete after an hour, giving up at $(size "$out.part") bytes" >&2
      return 1
    fi
    echo "$label: download interrupted (curl exit $rc), resuming at $(size "$out.part") bytes" >&2
    sleep 2
  done
  mv "$out.part" "$out"
}

tmp=$(mktemp -d)
stage=
trap 'rm -rf "$tmp"; [ -z "$stage" ] || rm -f "$stage"' EXIT

for platform in "$@"; do
  found=0
  while read -r payload sum url member; do
    case $payload in '' | '#'*) continue ;; esac
    case $payload in *_"$platform") ;; *) continue ;; esac
    found=1
    dest=$root/server/cmd/internal/${payload%%_*}/$payload
    if [ -f "$dest" ] && [ "$(sha256 "$dest")" = "$sum" ]; then
      chmod 0755 "$dest"
      echo "$payload: up to date"
      continue
    fi

    # One download per url: both macOS payloads come from the same archive.
    download=$tmp/$(printf '%s' "$url" | cksum | cut -d' ' -f1)
    if [ ! -f "$download" ]; then
      echo "$payload: downloading $url"
      fetch "$payload" "$url" "$download"
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
    # Copy to a file of this run's own beside the destination, so the rename is
    # atomic and concurrent runs never share one. The copy also gets a fresh
    # mtime, which tells make to recompress the .zst.
    stage=$(mktemp "$dest.XXXXXX")
    cp "$file" "$stage"
    chmod 0755 "$stage"
    mv -f "$stage" "$dest"
    stage=
    echo "$payload: installed"
  done < "$lock"
  if [ "$found" = 0 ]; then
    echo "payloads.lock pins nothing for $platform" >&2
    exit 1
  fi
done

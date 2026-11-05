#!/usr/bin/env bash
# Builds Igloo's ffmpeg and ffprobe for linux_amd64 inside the pinned toolchain
# container. Needs only Docker on the host. Output lands in out/linux_amd64/.
#
#   media-build/build.sh           reuse cached sources and finished dependencies
#   media-build/build.sh --clean   rebuild everything except the source cache
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
image=igloo-media-build:linux_amd64

if [[ ${1:-} == --clean ]]; then
  rm -rf "$root/work" "$root/out/linux_amd64"
fi

docker build -t "$image" "$root"
mkdir -p "$root/cache" "$root/work" "$root/out"
docker run --rm \
  -u "$(id -u):$(id -g)" \
  -e HOME=/build/work/home \
  -e JOBS="${JOBS:-$(nproc)}" \
  -v "$root:/build" \
  "$image" bash /build/scripts/build-linux.sh

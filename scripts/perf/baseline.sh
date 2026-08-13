#!/usr/bin/env bash
# Orchestrates a baseline performance capture against a running Igloo server.
#
# Usage:
#   baseline.sh startup <binary> [args...]   Measure exec -> /api/health 200 and
#                                            bytes written during boot, then stop.
#   baseline.sh idle <pid> [minutes]         Sample RSS/CPU/writes while idle (default 10).
#   baseline.sh browse <pid> [minutes]       k6 browse scenario + sampling (default 5).
#   baseline.sh stream <pid> <movie_id> [vus] [minutes]
#                                            k6 HLS scenario + sampling (default 1 VU, 10 min).
#   baseline.sh sizes                        Record binary and payload sizes.
#
# Env: BASE_URL (default http://localhost:8080), EMAIL, PASSWORD (see soak.js),
#      OUT_DIR (default scripts/perf/results).
#
# Results land in $OUT_DIR as CSV (samples) and .txt (summaries).

set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
BASE_URL="${BASE_URL:-http://localhost:8080}"
OUT_DIR="${OUT_DIR:-$here/results}"
mkdir -p "$OUT_DIR"
stamp="$(date +%Y%m%d-%H%M%S)"

sample() { # pid out.csv — starts sampler in background, echoes sampler pid
  "$here/sample-rss.sh" "$1" "$2" 2 &
  echo $!
}

case "${1:-}" in
startup)
  shift
  binary="${1:?usage: baseline.sh startup <binary> [args...]}"
  shift || true
  start_ns=$(date +%s%N)
  "$binary" "$@" &
  pid=$!
  trap 'kill -TERM "$pid" 2>/dev/null || true' EXIT
  until curl -sf -o /dev/null "$BASE_URL/api/health"; do
    kill -0 "$pid" 2>/dev/null || { echo "server exited during startup" >&2; exit 1; }
    sleep 0.1
  done
  ready_ms=$(( ($(date +%s%N) - start_ns) / 1000000 ))
  write_bytes=$(awk '/^write_bytes:/ {print $2}' "/proc/$pid/io" 2>/dev/null || echo "n/a")
  out="$OUT_DIR/startup-$stamp.txt"
  {
    echo "binary: $binary"
    echo "time_to_health_ms: $ready_ms"
    echo "boot_write_bytes: $write_bytes"
  } | tee "$out"
  kill -TERM "$pid"
  wait "$pid" 2>/dev/null || true
  trap - EXIT
  ;;

idle)
  pid="${2:?usage: baseline.sh idle <pid> [minutes]}"
  minutes="${3:-10}"
  out="$OUT_DIR/idle-$stamp.csv"
  echo "sampling PID $pid for $minutes min -> $out"
  sampler=$(sample "$pid" "$out")
  sleep $((minutes * 60))
  kill "$sampler" 2>/dev/null || true
  ;;

browse)
  pid="${2:?usage: baseline.sh browse <pid> [minutes]}"
  minutes="${3:-5}"
  command -v k6 >/dev/null || { echo "k6 is required (https://k6.io)"; exit 1; }
  out="$OUT_DIR/browse-$stamp"
  sampler=$(sample "$pid" "$out.csv")
  k6 run -e SCENARIO=browse -e VUS=3 -e DURATION="${minutes}m" -e BASE_URL="$BASE_URL" \
    --summary-export "$out-k6.json" "$here/soak.js" | tee "$out-k6.txt"
  kill "$sampler" 2>/dev/null || true
  ;;

stream)
  pid="${2:?usage: baseline.sh stream <pid> <movie_id> [vus] [minutes]}"
  movie_id="${3:?usage: baseline.sh stream <pid> <movie_id> [vus] [minutes]}"
  vus="${4:-1}"
  minutes="${5:-10}"
  command -v k6 >/dev/null || { echo "k6 is required (https://k6.io)"; exit 1; }
  out="$OUT_DIR/stream-${vus}vu-$stamp"
  sampler=$(sample "$pid" "$out.csv")
  k6 run -e SCENARIO=stream-hls -e MOVIE_ID="$movie_id" -e VUS="$vus" \
    -e DURATION="${minutes}m" -e BASE_URL="$BASE_URL" \
    --summary-export "$out-k6.json" "$here/soak.js" | tee "$out-k6.txt"
  kill "$sampler" 2>/dev/null || true
  ;;

sizes)
  out="$OUT_DIR/sizes-$stamp.txt"
  {
    for f in "$repo_root/server/dist/igloo-server" "$repo_root/server/dist/igloo-server-dev" \
      "$repo_root"/server/cmd/internal/ffmpeg/ffmpeg_* "$repo_root"/server/cmd/internal/ffprobe/ffprobe_*; do
      [ -f "$f" ] && printf "%12d  %s\n" "$(stat -c %s "$f")" "$f"
    done
    du -sh "$repo_root/server/cmd/api/webdist" 2>/dev/null || true
  } | grep -v '\.go$' | tee "$out"
  ;;

*)
  sed -n '2,16p' "$0"
  exit 1
  ;;
esac

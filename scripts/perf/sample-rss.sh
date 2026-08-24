#!/usr/bin/env bash
# Sample RSS, CPU and cumulative write bytes of a process to CSV.
#
# Usage: sample-rss.sh <pid> <out.csv> [interval_seconds]
#
# Columns: epoch_s, rss_kb, cpu_ticks (utime+stime, cumulative), write_bytes
# (cumulative, from /proc/<pid>/io). Convert cpu_ticks deltas to CPU% with:
#   cpu% = 100 * (ticks2 - ticks1) / (CLK_TCK * (t2 - t1))
# CLK_TCK is normally 100 (getconf CLK_TCK).

set -euo pipefail

pid="${1:?usage: sample-rss.sh <pid> <out.csv> [interval_seconds]}"
out="${2:?usage: sample-rss.sh <pid> <out.csv> [interval_seconds]}"
interval="${3:-2}"

if [ ! -d "/proc/$pid" ]; then
  echo "no such process: $pid" >&2
  exit 1
fi

echo "epoch_s,rss_kb,cpu_ticks,write_bytes" >"$out"

while [ -d "/proc/$pid" ]; do
  rss_kb=$(awk '/^VmRSS:/ {print $2}' "/proc/$pid/status" 2>/dev/null || echo "")
  [ -n "$rss_kb" ] || break
  # Fields 14 (utime) and 15 (stime); parse after the last ')' so the comm
  # field cannot shift positions.
  cpu_ticks=$(awk '{sub(/.*\) /, ""); print $12 + $13}' "/proc/$pid/stat" 2>/dev/null || echo "")
  write_bytes=$(awk '/^write_bytes:/ {print $2}' "/proc/$pid/io" 2>/dev/null || echo 0)
  echo "$(date +%s),$rss_kb,$cpu_ticks,$write_bytes" >>"$out"
  sleep "$interval"
done

echo "process $pid exited; samples written to $out" >&2

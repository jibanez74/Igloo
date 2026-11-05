#!/usr/bin/env bash
# Runs on a host with an NVIDIA GPU: Igloo's NVIDIA capability probes and HLS
# session chains (capabilities.go, ffmpeg_hls.go) with two ffmpeg builds,
# timing each chain and sampling its picture. Writes only to /dev/null and to
# temporary samples beside this script. Skips the benchmarks while NVENC is in
# use, so it never competes with someone's playback.
#   nvidia-parity.sh <reference ffmpeg> <candidate ffmpeg> <ffprobe> <sdr file> <hdr file>...
set -u
REF=$1; CAND=$2; PROBE=$3; SDR=$4; shift 4
here=$(cd "$(dirname "$0")" && pwd)
SP="setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709"

sessions=$(nvidia-smi --query-gpu=encoder.stats.sessionCount --format=csv,noheader 2>/dev/null | head -1)
echo "GPU: $(nvidia-smi --query-gpu=name,driver_version --format=csv,noheader); active NVENC sessions: $sessions"
if [[ ${sessions:-0} != 0 ]]; then
  echo "NVENC is in use (someone may be watching); not benchmarking now."
  exit 2
fi

# run_probe <command...>: runs a probe for at most 20 s, prints "exit <status>"
# and the first two lines of its output, and returns the probe's status.
run_probe() {
  local out status
  out=$(timeout 20 "$@" 2>&1)
  status=$?
  echo "exit $status $(head -2 <<< "$out")"
  return "$status"
}
cuda_probe() { # <bin> <vf>
  run_probe "$1" -v error -init_hw_device cuda=igloo_cuda -filter_hw_device igloo_cuda \
    -f lavfi -i testsrc2=s=256x144:d=0.1 -frames:v 1 -vf "$2" -c:v h264_nvenc -f null -
}
nvenc_failed=0
for b in REF CAND; do bin=${!b}
  echo "== $b: $("$bin" -hide_banner -version | head -1 | cut -d' ' -f1-3)"
  out=$(run_probe "$bin" -v error -f lavfi -i testsrc2=s=256x144:d=0.1 -frames:v 1 -c:v h264_nvenc -f null -) || nvenc_failed=1
  echo "  nvenc probe: $out"
  echo "  cuda scale probe: $(cuda_probe "$bin" "format=nv12,hwupload,scale_cuda=w=-2:h=144:format=yuv420p")"
  echo "  cuda tonemap probe: $(cuda_probe "$bin" "setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,format=p010le,hwupload,scale_cuda=w=-2:h=144:format=p010,tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0")"
done
if [[ $nvenc_failed == 1 ]]; then
  echo "NVENC does not open on this host (see the probe errors); not benchmarking."
  exit 3
fi

session() { # <bin> <file> <height> <bitrate M> <vf> <seconds> <output...>
  local bin=$1 f=$2 h=$3 rate=$4 vf=$5 secs=$6; shift 6
  nice -n 10 "$bin" -hide_banner -nostdin -y -fflags +genpts -analyzeduration 5000000 -probesize 5000000 \
    -init_hw_device cuda=igloo_cuda -filter_hw_device igloo_cuda -hwaccel cuda -i "$f" \
    -map 0:v:0 -map 0:a:0 -map_metadata -1 -map_chapters -1 \
    -c:v h264_nvenc -profile:v high -rc vbr -preset p4 -forced-idr 1 \
    -b:v "${rate}M" -maxrate "${rate}M" -bufsize "$((rate * 2))M" -vf "$vf" \
    -color_primaries bt709 -color_trc bt709 -colorspace bt709 \
    -g:v:0 96 -keyint_min:v:0 96 -force_key_frames:0 "expr:gte(t,n_forced*4)" \
    -c:a aac -ac 2 -b:a 320k -avoid_negative_ts make_zero -max_muxing_queue_size 1024 \
    -t "$secs" "$@"
}

run_case() { # <label> <file> <height> <bitrate> <vf>
  local label=$1 f=$2 h=$3 rate=$4 vf=$5 b bin speed
  session "$REF" "$f" "$h" "$rate" "$vf" 20 -f null - > /dev/null 2>&1  # warm the page cache
  for round in 1 2; do
    for b in REF CAND; do bin=${!b}
      speed=$(session "$bin" "$f" "$h" "$rate" "$vf" 60 -f null - 2>&1 | tr '\r' '\n' | grep -oE 'speed=[ 0-9.]+x|Error.*|Impossible.*' | tail -1)
      echo "  $label round$round $b: $speed"
    done
  done
  # 10 s samples for a picture check: tags and mean luma/chroma
  for b in REF CAND; do bin=${!b}
    session "$bin" "$f" "$h" "$rate" "$vf" 10 -f mp4 "$here/sample.$b.mp4" > /dev/null 2>&1
    echo "  $label $b sample: $("$PROBE" -v error -select_streams v:0 -show_entries stream=width,height,pix_fmt,color_primaries,color_transfer,color_space -of csv=p=0 "$here/sample.$b.mp4") $("$CAND" -v error -i "$here/sample.$b.mp4" -vf signalstats,metadata=print:file=- -f null - 2>/dev/null | awk -F= '/YAVG/{y+=$2;n++} /UAVG/{u+=$2} /VAVG/{v+=$2} END{if(n) printf "YAVG=%.2f UAVG=%.2f VAVG=%.2f", y/n, u/n, v/n}')"
  done
  rm -f "$here"/sample.*.mp4
}

echo "== SDR (CUDA scale chain): $(basename "$SDR")"
run_case sdr-1080 "$SDR" 1080 8 "format=nv12,$SP,hwupload,scale_cuda=w=-2:h=1080:format=yuv420p"
for HDR in "$@"; do
  echo "== HDR (CUDA tone-map chain): $(basename "$HDR")"
  for h in 2160 1080; do
    rate=16; [[ $h == 1080 ]] && rate=8
    run_case "hdr-$h" "$HDR" "$h" "$rate" "format=p010le,hwupload,scale_cuda=w=-2:h=$h:format=p010,tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0"
  done
done

# HLS cold-start: temp_file readiness — 2026-08-13

Measured on the same hardware, library, and protocol as
`baseline-2026-08-13.md` (dev build, k6 `stream-hls`, movie 59 for the 1-VU
run; movies 59/146/203 as three concurrent 1-VU runs). All numbers are k6
`time_to_first_segment`; server-side `ttfs_ms` log lines agree within a few
milliseconds.

## What changed

`fix/hls-cold-start`: `-hls_flags temp_file` plus the `segmentReady` predicate
(a segment is complete when its final name exists; init.mp4 once segment_0's
temp or final name appears), replacing the successor-file heuristic that gated
startup on ~two encoded segments. Also first-segment instrumentation
(`ttfs_ms`, `spawn_ms`, limiter rejection logging). See docs/ffmpeg.md
"Segment Serving and Readiness".

`temp_file` is capability-probed (`ffmpeg -h muxer=hls`), so these numbers are
the capable-binary path: this run used the embedded Jellyfin FFmpeg build,
which supports the flag. A swapped `IGLOO_FFMPEG_PATH` binary without it keeps
the legacy `segmentComplete` successor heuristic and would not show these
gains.

## Single stream (720p_3mbps transcode of the HEVC source, 5 min)

| Metric | Baseline | After | Delta |
|---|---|---|---|
| Cold TTFS (max) | **5,912 ms** | **1,363 ms** | **−77%** |
| Warm TTFS (med) | 14 ms | 14 ms | — |
| Segment p95 | 16 ms | 14 ms | — |
| Checks | 100% | 100% | — |

Server-side split for the cold session: `spawn_ms=0` (pre-encode overhead is
negligible), so the remaining ~1.4 s is ffmpeg's input probe + first segment
encode. In-browser (hls.js) the first file served is `init.mp4`, which now
goes out ~1.9 s into a 1080p session instead of after segment_0 finished
encoding.

## Three concurrent streams

Baseline was captured with the default 2-slot limiter (`NumCPU()/4`); the dev
`.env` now carries `HLS_MAX_CPU_TRANSCODES=5`, so both configurations were
measured:

**5 slots (no limiter pressure, pure CPU contention):** cold TTFS max
1.8 s / 6.3 s / 6.5 s — all three streams playing within ~6.5 s (baseline:
6.9 / 28.6 / 50.6 s, −87% on the worst stream).

**2 slots (baseline configuration):** the two streams that win slots start in
5.2 s / 5.5 s (baseline: 6.9 / 28.6 s). The third stream got **60 consecutive
503 rejections and never started within the 5-minute window** — worse than
the baseline's 50.6 s. At baseline, segment serving under saturation was so
slow (p95 8–14.6 s) that sessions regularly went >30 s idle and the same-owner
LRU reclaim let the queued stream in; with fast segment serving, sessions stay
hot and the instant-503 limiter starves later arrivals indefinitely. The new
`"hls transcode limiter rejected"` log line makes this visible.

**Consequence:** the limiter follow-up (park the acquisition on the permit
channel with a bounded wait instead of instant 503 — planned as its own PR) is
now a correctness fix for the 2-slot configuration, not just a latency
optimization.

## Manual playback matrix (dev server + Vite, Chromium via Playwright)

All passing after the change: SDR HEVC transcode (720p/1080p), rebased
seek/resume session (start>0), HDR tone-map (`tonemap_hdr=true`, cold TTFS
2.1 s on the CPU chain), copy-video remux (movie with `safe=1` verdict;
preflight passes with the new predicate; remux-unsafe files still fall back
to transcode), copy-video rebased seek (keyframe index), audio-track change
(new session per ordinal), text subtitles on a rebased session (1,353 cues
showing), watch-room playback (shared segment path). Not exercisable: no
interlaced source exists in the library (interlace gates untouched by this
change). Known pre-existing failure, unrelated: whole-file subrip extraction
over the Samba mount still exceeds the 60 s cap (`subtitle-extraction`
memory/backlog).

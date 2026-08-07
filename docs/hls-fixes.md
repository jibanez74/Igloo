# HLS: Remaining Recommended Work

This register carries forward the open items from the 2026-07-28 HLS playback audit (`docs/web-hls-playback-audit.md`, since removed) after the 2026-08-06 reliability pass, which closed H6 (remux-safety verdicts now persist in the `remux_safety_verdicts` table), H13 (`#EXT-X-INDEPENDENT-SEGMENTS` emitted in both playlist flavors; `#EXT-X-START` deliberately dropped), and H20 (failed temp-dir removals are logged). Items keep their audit IDs. "Verified" means the gap was confirmed in code or measured; "hypothesis" means it is plausible but has not been reproduced.

## Scan-time keyframe index (verified gap; highest-value performance item)

Keyframe positions are never persisted. Copy-video sessions that start mid-movie run a bounded ffprobe lookup per session (`KeyframeAtOrBefore`, `server/cmd/internal/ffprobe/ffprobe_keyframes.go`) with a 30-second lookback cap, and the result only feeds the advisory `X-Igloo-Actual-Start` header — sources with GOPs longer than the cap silently lose the measurement. Indexing keyframes at scan time (a new table keyed by video stream, populated by the movie scanner alongside `processMovieStreams` in `server/cmd/api/movies_scanner.go`) would make the actual-start measurement exact and free at session start, remove the per-session ffprobe, and is the prerequisite for ever reusing segments across seeks instead of discarding all output on every rebase. Sketch: one `ffprobe -select_streams v -show_entries packet=pts_time,flags` pass per file at scan time, stored as a compact blob or rows; `startHLSSession` reads the index instead of probing; a later phase can snap `-ss` to indexed keyframes and address per-segment output.

## H19 — interlaced / rotated / VFR sources (verified gap)

The transcode filter chain (`hlsVideoFilter`, `server/cmd/internal/ffmpeg/ffmpeg_hls.go`) only scales, converts pixel format, and tone-maps. Interlaced sources are not deinterlaced (combing artifacts), rotation metadata is not applied (sideways phone video), and variable-frame-rate sources get no fps normalization. Sketch: persist `field_order` and rotation side data at scan time (`video_streams` in `server/sqlc/schema.sql` has neither today), then conditionally prepend `yadif` (or the hardware equivalent), apply `transpose`, and consider `fps=` for pathological VFR. Needs careful per-device testing across the CPU/NVENC/QSV/VideoToolbox paths; see `docs/ffmpeg.md` for the chain variants.

## H15 — audio-copy gate ignores AAC profile (hypothesis)

The decision to copy audio is literally `copyAudio := audioCodec == "aac"` (`startHLSSession`, `server/cmd/api/hls_session.go`), so an HE-AAC or xHE-AAC stream would be copied even though browser support for those profiles inside fMP4 HLS is spotty. Unreproducible in the current library — every scanned AAC stream is LC — but the gate should read the stored `codec_profile` and only copy `LC` (falling back to the stereo AAC transcode otherwise) before a file with SBR/PS audio arrives. One-line fix plus tests.

## H16 — dialnorm ignored on downmix (hypothesis)

When AC-3/E-AC-3 is transcoded to stereo (`-c:a aac -ac 2 -b:a 320k` in `buildHLSArgs`), FFmpeg's default downmix does not apply dialnorm consistently, so perceived loudness can shift between sources or against direct-played tracks. Investigate `-drc_scale`/`loudnorm` options against real AC-3 material before changing anything; this needs listening tests, not just unit tests.

## H18 — no runtime hardware fallback (verified gap; needs a GPU host)

Encoder capabilities are probed once at startup (`probeCapabilities` / `ResolveHLSDevice`, `server/cmd/internal/ffmpeg/capabilities.go`). If the device fails later — driver reset, GPU contention — the session dies with a player error and every retry keeps failing until restart. Sketch: classify hardware-encoder failures in the session exit path (`onExit` in `startHLSSession`) and retry the session once with the CPU encoder, optionally demoting the cached capability. Cannot be developed or tested on this machine (no discrete GPU).

## H12 — in-player track/quality switching (product feature)

Changing audio track, subtitles, or quality mid-playback tears down the player and starts a new session (profile and audio ordinal are part of the HLS session key). A real fix spans the server (master playlist or session mutation endpoint) and the web player, and interacts with watch-room pinning. Largest item here; treat as a product decision, not a bug fix.

## Client-side stragglers

- **H10 (verified in code review, not reproduced):** a late `disposeHls` callback can null `hlsRef.current` while it already holds a newer hls.js instance (`web/src/components/movies/VideoPlayer.tsx`); guard the dispose with an identity check.
- **H4 browser confirmation:** the subtitle rebase fix (`helpers.ShiftWebVTT` serving cues shifted by `?start=`) has passing unit tests but has never been verified against a real browser on a rebased (resume/seek) session with sidecar subtitles.

## First-frame latency measurement (instrumentation)

Transcode startup is gated on roughly two encoded segments (~8 s of media): `init.mp4` is served once `segment_0` exists and `segment_0` once `segment_1` exists (`segmentComplete`, `server/cmd/api/hls_handler.go`). Fine on hardware encoders; on CPU-only 4K HDR tone-mapping this is the slowest path in the system and has never been measured. Log time-from-session-start-to-first-served-segment before optimizing anything.

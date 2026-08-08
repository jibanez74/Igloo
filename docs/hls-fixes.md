# HLS: Remaining Recommended Work

This register carries forward the open items from the 2026-07-28 HLS playback audit (`docs/web-hls-playback-audit.md`, since removed) after the 2026-08-06 reliability pass, which closed H6 (remux-safety verdicts now persist in the `remux_safety_verdicts` table), H13 (`#EXT-X-INDEPENDENT-SEGMENTS` handling; `#EXT-X-START` deliberately dropped — see R2 below for the 2026-08-07 correction to which playlists carry the tag), and H20 (failed temp-dir removals are logged). Items keep their audit IDs. "Verified" means the gap was confirmed in code or measured; "hypothesis" means it is plausible but has not been reproduced.

## ~~R1 — NVENC never forced IDR frames~~ (CLOSED 2026-08-07)

`buildHLSArgs` applied `-force_key_frames:0 expr:...` to every encoder, but the NVIDIA branch was only `-rc vbr -preset p4`. `h264_nvenc` defaults `-forced-idr` to false, and with that default FFmpeg requests a plain intra frame at each forced boundary instead of an IDR, so later frames could still reference across it — breaking segment-level random access for every client, not just native HLS players. `appendHLSNvidiaEncoderArgs` now appends `-forced-idr 1`, capability-gated on a new `recordEncoderOptions(bin, "h264_nvenc", ...)` probe, mirroring what QSV already did. Pinned by argument-level tests only (no GPU on the dev machine — same caveat as H18).

## ~~R2 — the independence tag was not gated on a proven guarantee~~ (CLOSED 2026-08-07)

`-hls_flags independent_segments` was passed unconditionally and the synthesized transcode playlist wrote `#EXT-X-INDEPENDENT-SEGMENTS` unconditionally, so both flavors advertised whole-playlist independence regardless of whether it held. Two holes: NVENC/QSV builds without their forced-IDR option (see R1), and copy-video output, where the remux validator only samples 4 fragments at the session's start offset and cannot rule out a later GOP-structure change in the source. Both playlist flavors are now gated on one predicate, `ffmpeg.HLSSegmentsAreIndependent`, surfaced to the API layer as `HLSSession.IndependentSegments`; copy-video never carries the tag. The old code comment claiming the flag was "inert" for transcodes was also wrong — `buildHLSPlaylistBody` serves FFmpeg's own finalized playlist once a transcode session exits. Deliberately *not* done: proving source-wide independence by walking every source keyframe's NAL type, which would add thousands of bounded reads per file to buy back a tag hls.js ignores.

## ~~R3 — persisted remux verdicts outlived their producer~~ (CLOSED 2026-08-07)

`remuxSafetyFingerprint` keyed verdicts on source metadata alone, but a verdict validates FFmpeg-generated fMP4 output. An embedded-payload upgrade, a swapped `IGLOO_FFMPEG_PATH` binary, or a change to the remux arguments left stale `safe=true` rows that skipped preflight against different muxer behavior. The fingerprint now carries the FFmpeg version (parsed from the startup `-version` banner, which `initializeCandidate` already runs — reused rather than probed twice) and `remuxVerdictProducerRevision`, a constant to bump when the remux arguments or `ValidateRemuxSafety` change. Like the field-order term in H19, this invalidates every previously persisted verdict once (one re-preflight per file).

## ~~Keyframe index~~ (CLOSED 2026-08-07)

Implemented as a **persisted, on-demand** index rather than the scan-time pass this entry originally sketched: the `keyframeindex` package extracts keyframe timestamps from the container's own seek tables (Matroska Cues for mkv/webm; stts/ctts/stss with single-edit `elst` handling for mp4/m4v/mov — the structures FFmpeg's `-ss` itself consults) on the first copy-video session of a file, persisting them in `keyframe_indexes` keyed by movie + stream + file fingerprint. Seeks are answered synchronously by binary search with no GOP-length cap; the bounded ffprobe probe survives only as the fallback for avi and index-less files, and its answers are never persisted. Deviation rationale: extraction is a few bounded index reads (milliseconds on local disks, low seconds on a network mount), so pre-computing at scan time bought no latency while adding scanner risk — the scanner hard-fails files on probe errors — and a full-library pass over slow storage; the fingerprint (`movies.size` + `movies.updated_at`) already re-extracts after any rescan. A scan-time backfill remains possible later if a future feature needs full coverage before first play.

**Unblocked follow-ups:** (a) keyframe-aligned playlist generation and segment reuse across seeks — the stored sorted-PTS array is exactly the input a segment planner needs; (b) optional post-scan backfill sweep for movies without an index row.

## ~~H19 — interlaced / rotated / VFR sources~~ (CLOSED 2026-08-07)

Shipped with three deliberate deviations from the original sketch. `field_order` and display-matrix rotation are persisted at scan time (`video_streams.field_order`/`rotation`; NULL on pre-existing rows, treated as progressive/unrotated until a rescan). Interlaced sources prepend a software `yadif` on every transcode chain — decoded frames are in system memory at each chain head, so no hardware deinterlacer variants were needed; the Apple HDR `scale_vt` chain (hardware frames) routes interlaced sources to the software tone-map chain instead. Interlaced streams are also rejected from remux and direct play (both static-gate copies updated), and the remux-safety fingerprint gained a field-order term, which invalidates all previously persisted verdicts once (one re-preflight per file). **No `transpose`:** FFmpeg's default autorotation handles rotation during transcode — proven against a real rotated source in the externalbin integration test — and copy paths pass the matrix through for browsers to honor; rotation is persisted and logged only. **No `fps=`:** VFR is detected (`isVFRStream`) and logged as `vfr_detected` only, since `-force_key_frames` already keeps segmentation correct. NVENC/QSV/VideoToolbox chains are pinned by argument-level tests only (no GPU/macOS on the dev machine — same caveat as H18); the CPU chain is proven end to end.

## ~~H15 — audio-copy gate ignores AAC profile~~ (CLOSED 2026-08-07)

Audio copy is now gated by `isCopySafeAACStream` (`server/cmd/api/hls_session.go`): AAC is copied only when the scanned `codec_profile` is a confirmed `LC` (case-insensitive); HE-AAC/xHE-AAC and unknown/NULL profiles fall back to the stereo AAC transcode. Deliberately strict — an unknown profile cannot prove safety, so the handful of library rows scanned without a profile now transcode instead of copy. The session-start log gained `audio_codec_profile` so a copy refusal is explainable.

## H16 — dialnorm ignored on downmix (hypothesis)

When AC-3/E-AC-3 is transcoded to stereo (`-c:a aac -ac 2 -b:a 320k` in `buildHLSArgs`), FFmpeg's default downmix does not apply dialnorm consistently, so perceived loudness can shift between sources or against direct-played tracks. Investigate `-drc_scale`/`loudnorm` options against real AC-3 material before changing anything; this needs listening tests, not just unit tests.

## H18 — no runtime hardware fallback (verified gap; needs a GPU host)

Encoder capabilities are probed once at startup (`probeCapabilities` / `ResolveHLSDevice`, `server/cmd/internal/ffmpeg/capabilities.go`). If the device fails later — driver reset, GPU contention — the session dies with a player error and every retry keeps failing until restart. Sketch: classify hardware-encoder failures in the session exit path (`onExit` in `startHLSSession`) and retry the session once with the CPU encoder, optionally demoting the cached capability. Cannot be developed or tested on this machine (no discrete GPU).

## H12 — in-player track/quality switching (product feature)

Changing audio track, subtitles, or quality mid-playback tears down the player and starts a new session (profile and audio ordinal are part of the HLS session key). A real fix spans the server (master playlist or session mutation endpoint) and the web player, and interacts with watch-room pinning. Largest item here; treat as a product decision, not a bug fix.

## Client-side stragglers

- ~~**H10:**~~ (CLOSED 2026-08-07) `disposeHls` now nulls `hlsRef.current` only when it still points at the instance being disposed (`web/src/components/playback/VideoPlayer.tsx` — the register's `components/movies` path was stale). The race was defensive rather than reproducible with the current single call site, but the guard makes the invariant local. The same fix was applied to the reachable variant in `useYouTubePlayer.ts`, whose cleanup nulled `playerRef` unconditionally across `reloadKey` rebuilds.
- **H4 browser confirmation:** the subtitle rebase fix (`helpers.ShiftWebVTT` serving cues shifted by `?start=`) has passing unit tests but has never been verified against a real browser on a rebased (resume/seek) session with sidecar subtitles.

## First-frame latency measurement (instrumentation)

Transcode startup is gated on roughly two encoded segments (~8 s of media): `init.mp4` is served once `segment_0` exists and `segment_0` once `segment_1` exists (`segmentComplete`, `server/cmd/api/hls_handler.go`). Fine on hardware encoders; on CPU-only 4K HDR tone-mapping this is the slowest path in the system and has never been measured. Log time-from-session-start-to-first-served-segment before optimizing anything.

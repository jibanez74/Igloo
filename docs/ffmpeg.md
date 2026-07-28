# FFmpeg and ffprobe in Igloo

This document explains how Igloo uses FFmpeg and ffprobe, why the current design exists, and what to keep in mind when changing media playback, scanning, subtitles, or deployment behavior.

Igloo uses these tools for three separate jobs:

- `ffprobe` reads media metadata during library scans.
- `ffmpeg` creates HLS output for browser playback when direct file playback is not enough.
- `ffmpeg` converts supported text subtitle streams to WebVTT.

Direct file streaming is separate from this flow. When the client can play a source file directly, Igloo can serve the original media without starting FFmpeg. FFmpeg is used when Igloo needs compatible HLS output, audio conversion, video transcoding, HDR tone mapping, or subtitle conversion.

## Binary Strategy

Release builds use embedded FFmpeg and ffprobe binaries. Platform-specific files under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/` use `//go:embed` to include the binary payload at compile time. At startup, Igloo extracts each binary into an operating-system temp directory such as `igloo-ffmpeg-*` or `igloo-ffprobe-*`, marks it executable, and keeps a singleton wrapper pointing at the extracted path.

Igloo uses Jellyfin FFmpeg builds for release payloads, not generic upstream FFmpeg builds. The Linux x64 payloads currently checked into this repository report `7.1.4-Jellyfin`. As of 2026-06-24, Jellyfin's current non-prerelease `jellyfin-ffmpeg` line is `7.1.4`, while its `8.1.1` line is a prerelease for the next major Jellyfin release. Upstream FFmpeg has newer stable branch releases, but embedded Igloo payloads should follow the stable Jellyfin FFmpeg line unless a change is intentionally testing a prerelease build.

When refreshing payloads, update both `ffmpeg_<platform>` and `ffprobe_<platform>` from the same Jellyfin FFmpeg release. Linux x64 builds use `ffmpeg_linux_amd64` and `ffprobe_linux_amd64`; macOS ARM64 builds expect matching `ffmpeg_darwin_arm64` and `ffprobe_darwin_arm64` payload files. `make build` checks that the payload files for the current native platform exist before compiling.

Development and CI can use the `externalbin` build tag instead. With that tag, the wrappers do not extract embedded binaries. They resolve tools in this order:

- `IGLOO_FFMPEG_PATH` or `IGLOO_FFPROBE_PATH`
- `ffmpeg` or `ffprobe` on `PATH`

The split exists for practical reasons:

- Release packages are self-contained on supported platforms.
- Development and tests do not require large ignored payload files.
- Hardware acceleration still depends on the host runtime, drivers, and FFmpeg build support.
- The wrappers give the application a stable internal interface even though the binary source differs by build mode.

Both wrappers are singletons. `ffmpeg.New()` and `ffprobe.New()` return the same instance after first initialization. Each wrapper verifies the resolved binary with `-version` before accepting it, so a corrupt, wrong-architecture, or non-executable binary fails during startup instead of during the first scan or transcode. On shutdown, `ffmpeg.Cleanup()` and `ffprobe.Cleanup()` remove extracted temp directories in embedded mode and reset the singleton state. In `externalbin` mode there is no extracted directory, so cleanup only resets the wrapper instance.

## Metadata Scanning

Movie and music scans treat ffprobe as required infrastructure.

For movies, Igloo calls `app.Ffprobe.GetMetadata(path)` while processing each movie file. The scanner uses ffprobe output for:

- duration and runtime
- container and stream metadata
- video, audio, and subtitle stream rows
- chapter information
- video dimensions, codec names, profiles, bit depth, pixel formats, frame rates, and color metadata
- audio codecs, language tags, channel layout, sample rate, bitrate, and the `default` disposition
- subtitle codecs, language tags, stream indices, titles, and the `default`/`forced` dispositions

Stream tag keys are normalized the same way format tags are (lowercased, separators stripped, `lang` accepted as a `language` alias), so Matroska muxers that write `TITLE`/`LANGUAGE` still produce labelled, preference-matchable streams.

For music, Igloo uses ffprobe to populate track metadata such as title, sort title, artist, album, genre, track number, disc number, release date, duration, bitrate, composer, and copyright. The library scan supplies each source file's size from the filesystem.

The scanner stores stream data in SQLite so playback does not need to run ffprobe on every HLS request. That is intentional. HLS session creation reads movie, video stream, and audio stream rows from the database and starts FFmpeg from that stored metadata. This keeps playback startup predictable and avoids probing the same file repeatedly while users are trying to watch something.

Movie scans use `ffprobe.GetMetadata`, which runs:

```bash
ffprobe -v quiet -print_format json -show_streams -show_format -show_chapters <file>
```

Music scans use `ffprobe.GetAudioMetadata`, which limits `-show_entries` to the audio format, stream, and tag fields needed by the music scanner.

The quiet JSON output keeps parsing deterministic and avoids mixing log text with structured data. Igloo rejects results with no streams because a scanned media item without streams cannot be played or indexed reliably.

## HLS Playback

Browser HLS playback is built around on-demand FFmpeg sessions.

When a client requests a personal HLS playlist:

```text
/api/movies/{id}/hls/{profile}/playlist.m3u8?playback_session=<uuid>&start=<seconds>&audio_track=<index>
```

`audio_track` is omitted for video-only movies. Igloo loads the movie duration, normalizes the requested start, reserves personal-session capacity, loads the remaining stream metadata, creates a temp directory, starts FFmpeg in the background, converts the reservation into a cached session, and returns a VOD-style playlist to the browser. Segment requests then read files from the session temp directory as FFmpeg produces them.

Personal HLS sessions are keyed by movie ID, requested profile, audio track, playback session ID, and effective normalized start time. If the same request arrives again, Igloo refreshes the cached session TTL and reuses the process. Before FFmpeg starts, Igloo evicts expired entries and removes only superseded windows for the same movie, user, and `playback_session` UUID; sessions from other playback UUIDs remain isolated. Different clients can therefore play the same movie concurrently unless the per-user cap requires an LRU replacement.

Cached personal sessions and in-flight creations share the `HLS_MAX_SESSIONS_PER_USER` cap (default 3). Admission reserves capacity before FFmpeg starts. At the cap, the owner's cached personal sessions are evicted in least-recently-used order until the new reservation fits; rooms and other users' sessions are never candidates. If every slot is already held by an in-flight reservation, the manifest request returns `503 Service Unavailable` with `Retry-After`. Remux and transcode creations both participate in this cap. A successful creation atomically exchanges its reservation for the cache entry, while every failure path releases the reservation. Concurrent creation of the same effective key is still deduplicated with singleflight. Clients can also tear a playback session's HLS sessions down explicitly with `POST /api/movies/{id}/hls/session/stop`; the stop endpoint stays scoped to its own playback session ID so a late stop from a closing tab cannot remove a session the user just created after reopening.

Personal sessions use a 5-minute idle TTL that every manifest and segment request refreshes. Because hls.js stops fetching once its buffer is full and a paused tab fetches nothing, the web player refetches the manifest every 2 minutes while HLS playback is ready and the video player is rendered. A fatal playback error removes the player and stops the timer immediately; a successful retry renders the player and enables it again. A client that skips the keepalive (or wakes from OS sleep after eviction) recovers transparently because a manifest request recreates the session at the same start offset. Watch-room sessions keep a 30-minute TTL — rooms have no per-client keepalive and always warm from the beginning, so evicting an idle room would restart playback for every participant. The cache sweep runs every minute, so an abandoned personal session is fully reclaimed (FFmpeg killed, temp dir removed, transcode permit released) within about six minutes even without an explicit stop.

HLS requests additionally accept an optional `reload` query parameter. It is an opaque client-supplied value that is echoed into the rewritten playlist asset URLs; it is not part of the session cache key.

FFmpeg runs with `context.Background()` after session creation. This is deliberate: an HLS process must outlive the HTTP request that created it, because the browser will request the manifest and segments as separate requests. The session cache owns the lifecycle. Expiration, eviction, room cleanup, or server shutdown stops the process and removes the temp directory.

HLS temp directories are created under the transcode directory stored in Settings. On first launch that value is seeded from `TRANSCODE_DIR`, or from `./transcode` when `TRANSCODE_DIR` is unset. This keeps heavy temporary media output in Igloo's configured transcode workspace instead of the operating-system temp directory.

## HLS Output Format

Igloo writes fragmented MP4 HLS, not MPEG-TS HLS.

The FFmpeg HLS command uses:

- `-f hls`
- `-hls_segment_type fmp4`
- `-hls_fmp4_init_filename init.mp4`
- `-hls_segment_filename segment_%d.m4s`
- `-hls_segment_options movflags=+frag_discont`
- `-hls_playlist_type event`
- `-hls_list_size 0`
- `-hls_time 4`

The generated files match the HTTP handlers:

- `init.mp4`
- `segment_0.m4s`
- `segment_1.m4s`
- `playlist.m3u8`

fMP4 HLS is used because modern browser players handle it well and it works naturally with copied H.264 video, transcoded H.264 video, and AAC audio. A short 4-second target segment gives acceptable startup and seek behavior while keeping the number of segment files manageable. FFmpeg also receives `movflags=+frag_discont` for fMP4 segment output so independent fragments tolerate discontinuities across rebased sessions and copy-video boundaries.

FFmpeg writes an event playlist while encoding. Igloo exposes a VOD-style playlist to clients. During encoding, Igloo generates a complete VOD playlist from the known movie duration so hls.js sees a seekable on-demand asset instead of a live/event stream. Generated VOD playlists use a target duration of 8 seconds for transcodes and 30 seconds for copy-video sessions, because copied video can only split on source keyframe boundaries. After FFmpeg exits successfully, Igloo finalizes the FFmpeg playlist by switching it to VOD and appending `#EXT-X-ENDLIST` when needed.

The manifest handler rewrites playlist asset URLs so each `init.mp4` and `segment_N.m4s` URL includes the selected audio track and session query parameters. The rewritten `start` is the effective normalized start used by the cache key and FFmpeg, not an out-of-range value from the original request. This keeps HLS asset requests tied to the same session configuration that created the manifest.

HLS responses use `Cache-Control: no-store`. Transcode output is session-scoped, temporary, and can vary by profile, audio track, start time, and playback context. Browser or proxy caching would make stale segment and playlist behavior harder to reason about.

## Remux, Transcode, and Fallback

Igloo supports a special HLS profile named `remux`. Remux mode copies the video stream with `-c:v copy` and only transcodes audio when the selected audio codec is not already AAC.

Remux exists because it preserves source video quality and avoids expensive video encoding when the source video is browser-compatible. It is much cheaper than transcoding and is the best path for compatible H.264 sources.

Remux is also reached without the user selecting it. Because direct play cannot select an audio track, choosing any track other than the container's first one resolves the playback mode to `remux` (see Audio Handling). Users who pick a non-default soundtrack on an otherwise direct-playable file therefore land on remux rather than direct play.

Remux is only attempted for browser-compatible H.264 codec names:

- `h264`
- `h.264`
- `avc`
- `avc1`

If the source video is not browser-compatible H.264, Igloo immediately falls back to the best-fit transcode profile. Igloo also falls back for H.264 streams that are not a safe browser remux target, including 10-bit, 4:2:2, or 4:4:4 sources identified from stored codec profile, bit depth, or pixel format metadata. The pixel-format rule is an allowlist of the 8-bit 4:2:0 formats browsers decode (`yuv420p`, `yuvj420p`, `nv12`, `nv21`), so an unrecognised format falls back rather than being assumed safe. This avoids serving copied video that browsers are unlikely to play through HLS.

Even H.264 remux can be unsafe. Some copied fMP4 fragments can start at samples that are not independently decodable by browser players. To avoid that, Igloo preflights remux output before committing to it:

- wait for `init.mp4`
- wait for the first 4 complete segments
- inspect the generated fMP4 fragments
- verify sync samples in the video track start with IDR frames
- cache the safe or unsafe verdict for 24 hours

If preflight times out or FFmpeg exits before enough output is available, Igloo falls back to transcoding without caching an unsafe verdict, because that kind of failure may be transient. If validation proves the fragments are unsafe, Igloo caches the unsafe verdict and falls back immediately for later sessions using the same movie, stream index, file size, and update timestamp.

The fallback profile is chosen with `BestFitHLSFallbackProfile`. Igloo picks the highest configured transcode profile whose target height fits within the source height. If the source is smaller than every configured profile, it falls back to `720p_3mbps` so playback still has a reliable transcode path.

## Direct Play Eligibility and Fallback

Direct play serves the original file over HTTP range requests with no FFmpeg process. Whether the web client offers it is decided from the scanned metadata plus one browser probe (`web/src/lib/playback.ts`, `getAvailableModes`; background in `docs/web-direct-playback-audit.md`):

- **Container.** Only MP4 (`mp4`/`m4v`) is eligible. The container→MIME mapping is pinned in `helpers.VideoMimeTypes` — never derived from the host's MIME tables — and MKV must never be added: Chrome and Firefox fail Matroska in a `<video>` element silently at 0ms with no `MediaError`.
- **Video.** H.264 codec names only, and the stream must be browser-safe: 10-bit, 4:2:2 and 4:4:4 sources are refused using the same profile / bit-depth / pixel-format rules as the server's remux gate (`isBrowserSafeH264RemuxCandidate`). Pixel formats are checked against an allowlist of the 8-bit 4:2:0 formats (`yuv420p`, `yuvj420p`, `nv12`, `nv21`) — the two copies of the list must stay in sync.
- **Audio.** The first audio stream's codec must be browser-playable, and the stream the browser will pick must be unambiguous: with two or more audio streams, a `default` disposition on a non-first stream or multiple `default` flags refuse direct play (no flags at all stays eligible — browsers follow container track order). Selecting any non-first track resolves the mode to `remux` (see Audio Handling).
- **Browser probe.** After the static rules pass, the client asks `canPlayType` with an RFC 6381 string built from the stored codec profile and level. The probe can only narrow eligibility, never widen it: watch-room creation enforces the same rules server-side and cannot probe.

The client never requests `/stream` before technical details have resolved, so a bookmarked `?mode=direct` link to an ineligible file resolves to an HLS mode without touching the raw stream. If an affirmatively eligible direct play still fails — `MEDIA_ERR_DECODE`, `MEDIA_ERR_SRC_NOT_SUPPORTED`, or no `loadedmetadata` within 10 seconds (the silent-stall case) — the player switches to `remux` exactly once per stream window, preserving position and track selection, and announces the switch.

## Transcode Profiles

Allowed HLS profiles are centralized in `server/cmd/internal/helpers/hls_profiles.go`:

| Profile | Video bitrate | Buffer size | Target height |
| --- | ---: | ---: | ---: |
| `2160p_16mbps` | `16M` | `32M` | `2160` |
| `1080p_8mbps` | `8M` | `16M` | `1080` |
| `1080p_6mbps` | `6M` | `12M` | `1080` |
| `1080p_4mbps` | `4M` | `8M` | `1080` |
| `720p_3mbps` | `3M` | `6M` | `720` |

Transcode mode sets `-b:v`, `-maxrate`, and `-bufsize` from the selected profile. It scales video to the profile height with width `-2`, which preserves aspect ratio while keeping the output width divisible by two for H.264 encoders.

All video transcodes set `-profile:v high`, profile bitrate, maxrate, bufsize, SDR color metadata, and an H.264 encoder. CPU transcode uses:

```text
-c:v libx264 -preset fast -sc_threshold:v:0 0
```

The `fast` preset is a practical default for self-hosted playback: it improves stream quality over faster x264 presets while keeping CPU use reasonable for home servers. Scene-cut insertion is disabled for CPU transcodes because Igloo aligns keyframes on the HLS segment cadence. Predictable keyframes make HLS segmentation and seeking more reliable.

When the source frame rate is known, Igloo sets a fixed 4-second GOP:

```text
-g:v:0 <ceil(segment_time*fps)> -keyint_min:v:0 <ceil(segment_time*fps)>
```

Every transcode, regardless of encoder, also uses a forced keyframe expression:

```text
-force_key_frames:0 expr:gte(t,n_forced*4)
```

The GOP flags make the GOP the right size, while `-force_key_frames` is what actually pins keyframes to the exact segment timestamps so every HLS segment starts on an IDR frame. GOP counting alone drifts on VFR sources and non-integer frame rates (23.976 fps rounds to a 96-frame GOP, which is about 4.004 seconds), splitting segments later and later. Without predictable keyframes, HLS segments can drift, seek behavior gets worse, and browsers may wait longer for independently decodable frames.

FFmpeg also runs with:

- `-fflags +genpts` to generate timestamps when sources have missing or awkward presentation timestamps.
- `-analyzeduration 5000000` and `-probesize 5000000` to give FFmpeg enough input data to identify streams without making startup unbounded.
- `-readrate 4` and `-readrate_initial_burst 60`, when the FFmpeg build supports those CLI options, so a session reads input at most 4x realtime after an initial 60-second burst instead of racing arbitrarily far ahead of playback.
- `-map_metadata -1` and `-map_chapters -1` to keep source metadata and chapter markers out of HLS output.
- `-avoid_negative_ts make_zero` to normalize output timestamps.
- `-max_muxing_queue_size 1024` to tolerate sources with stream timing that would otherwise overflow FFmpeg's muxing queue.

Transcodes also tag output color explicitly: the output gets `-color_primaries bt709 -color_trc bt709 -colorspace bt709`, every video filter chain ends with a matching `setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709`, and `-pix_fmt yuv420p` is set for all encoders except `h264_qsv` and the CUDA filter paths, which control their pixel format inside the filter chain.

Igloo does not pass `-threads` to FFmpeg. libx264 and the hardware encoders choose their own per-process thread behavior. CPU encoding pressure on a home server is bounded by the HLS transcode limiter: `HLS_MAX_CPU_TRANSCODES` sets the maximum number of concurrent HLS transcode sessions, and the default is `max(1, runtime.NumCPU()/4)`. Copy-video (remux) sessions bypass this CPU limiter because they do not encode video, but they still require a per-user personal-session reservation.

If CPU permits are exhausted, personal playback may reclaim the owner's least-recently-used non-remux session only when it has been idle for at least 30 seconds and FFmpeg is still running. Reclaim skips completed sessions, rooms, other users' sessions, copy-video sessions, and fresh active sessions, continuing through LRU candidates until it finds an eligible running transcode. Igloo then retries the new transcode once; otherwise it returns the normal `503` plus `Retry-After` response.

## Audio Handling

HLS sessions always map one video stream and map one audio stream when the movie has audio:

```text
-map 0:<video_stream_index>
-map 0:<audio_stream_index>
```

For video-only movies, Igloo omits the audio map and audio codec options. The stream indices are absolute ffprobe stream indices stored during scanning. Igloo does not rely on FFmpeg's relative stream numbering at playback time.

If the selected audio codec is AAC, Igloo copies it:

```text
-c:a copy
```

Otherwise, Igloo converts audio to stereo AAC at `320k`:

```text
-c:a aac -ac 2 -b:a 320k
```

AAC is the safest baseline for browser HLS playback. Downmixing to stereo avoids playback failures on clients that do not support the source channel layout.

### Audio Track Selection and Direct Play

The `audio_track` request parameter is an ordinal into the movie's audio streams ordered by `stream_index`, which is the same order the client's audio picker renders. It is not the ffprobe stream index. Igloo resolves the ordinal to the stored absolute index at session creation and uses that for `-map`.

Direct play has no equivalent mechanism. It serves the original file with range requests and no FFmpeg process, so the browser always decodes the container's first audio track and any other selection would be silently ignored. Igloo therefore treats the audio choice as authoritative: selecting any track other than the first resolves the playback mode from `direct` to `remux`, which copies the video stream and maps the requested audio track. Selecting the first track keeps direct play.

The web client enforces this in `resolvePlaybackSettings`, so the rule applies to saved settings, user language preferences, and deep links alike. Direct play is only ever paired with the first audio track, and it is refused outright when the container's `default` dispositions make the browser's own pick ambiguous (see Direct Play Eligibility and Fallback).

## Hardware Acceleration

The hardware acceleration setting is stored as one of:

- `cpu`
- `apple`
- `nvidia`
- `intel`

The default value is `cpu`. Set `HARDWARE_ACCELERATION_DEVICE` in `.env` when testing host hardware acceleration.

The FFmpeg encoder mapping is:

| Igloo device | FFmpeg decode flag | FFmpeg video encoder | Primary environment |
| --- | --- | --- | --- |
| `cpu` | none | `libx264` | Any supported runtime |
| `apple` | `-hwaccel videotoolbox` | `h264_videotoolbox` | macOS with VideoToolbox-capable FFmpeg |
| `nvidia` | `-hwaccel cuda` when the `cuda` hwaccel is probed; CUDA filter device only when `scale_cuda`/`tonemap_cuda` probes pass | `h264_nvenc` | Linux with NVIDIA driver/runtime support |
| `intel` | software decode by default; QSV filter device only when SDR `scale_qsv` is probed usable | `h264_qsv` | Linux with Intel QSV support |

NVIDIA adds:

```text
-rc vbr -preset p4
```

Intel adds:

```text
-preset veryfast
-look_ahead 1
-forced_idr 1
```

Igloo only sends those Intel encoder options when the probed FFmpeg build lists them for `h264_qsv`.

At startup, after the `-version` executability check, Igloo probes FFmpeg for encoders, filters, hardware acceleration methods, key filter options, encoder options, and selected runtime filter chains. CPU, unknown devices, missing hardware encoders, failed NVENC runtime probes, failed QSV runtime probes, and missing Apple VideoToolbox encoder support fall back to `libx264`. This is intentional: an invalid or unavailable hardware mode should not create a new unsupported encoder path inside the argument builder. The settings API validates known device names, but the HLS builder still has a safe CPU fallback.

NVIDIA encode is checked with a short runtime encode probe, not just by looking for `h264_nvenc` in `ffmpeg -encoders`. When the probed build supports the `cuda` hwaccel, NVIDIA transcodes add `-hwaccel cuda` without `-hwaccel_output_format`: FFmpeg decodes on the GPU when the source codec is supported and transparently falls back to software decode otherwise, and decoded frames land in system memory either way, so the same filter chains work in both cases. For SDR transcodes, NVIDIA normally uses software scaling into `yuv420p` frames before `h264_nvenc` encode:

```text
scale=-2:<height>,format=yuv420p
```

If NVENC is usable and FFmpeg also exposes `cuda`, `hwupload`, `scale_cuda`, the `scale_cuda` `format` option, and a successful CUDA scale runtime probe, SDR transcodes use an explicit CUDA upload and CUDA scaling:

```text
-init_hw_device cuda=igloo_cuda -filter_hw_device igloo_cuda
-vf format=nv12,hwupload,scale_cuda=w=-2:h=<height>:format=yuv420p
```

Intel QSV encode is checked with a short runtime encode probe, not just by looking for `h264_qsv` in `ffmpeg -encoders`. Unlike CUDA, QSV decode is intentionally not enabled: FFmpeg's generic `-hwaccel qsv` does not fall back to software decode as reliably across driver stacks. For SDR transcodes, Igloo uses software decode and normally software scaling into `nv12` frames before `h264_qsv` encode:

```text
scale=-2:<height>,format=nv12
```

If QSV encode is usable and FFmpeg also exposes `qsv`, `scale_qsv`, the `scale_qsv` `format` option, and a successful `scale_qsv` runtime probe, SDR transcodes use QSV scaling:

```text
-init_hw_device qsv=igloo_qsv -filter_hw_device igloo_qsv
-vf format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=<height>:format=nv12
```

NVIDIA and Intel hardware acceleration require host drivers, device access, and an FFmpeg build with the matching encoder support. Apple VideoToolbox is available only on macOS builds with a VideoToolbox-capable FFmpeg binary.

## HDR Tone Mapping

Igloo uses ffprobe video stream metadata to detect HDR sources. The current HDR checks look at `color_transfer`:

- `smpte2084` for HDR10/PQ
- `arib-std-b67` for HLG

Remux does not tone-map. If a user requests `remux`, Igloo copies video when remux is safe. Tone mapping only applies when Igloo transcodes HDR video into SDR HLS profiles.

Apple uses:

```text
scale_vt=w=-2:h=<height>:color_matrix=bt709:color_primaries=bt709:color_transfer=bt709
```

That keeps hardware decode enabled because VideoToolbox can handle scaling and HDR-to-SDR conversion in the GPU path.

CPU, NVIDIA, and Intel use a software tone-mapping filter chain:

```text
zscale=w=-2:h=<height>:t=linear:npl=100,
format=gbrpf32le,
zscale=p=bt709,
tonemap=tonemap=hable:desat=0,
zscale=t=bt709:m=bt709:r=tv,
format=<output_pixel_format>
```

CPU and NVIDIA software tone mapping output `yuv420p`; Intel outputs `nv12` for `h264_qsv`. For NVIDIA HDR tone mapping, Igloo uses an explicit CUDA upload plus `tonemap_cuda` only when the probed FFmpeg build exposes the CUDA scale/tone-map filters, the options Igloo needs, and successful CUDA scale and tone-map runtime probes:

```text
-init_hw_device cuda=igloo_cuda -filter_hw_device igloo_cuda
-vf format=p010le,hwupload,scale_cuda=w=-2:h=<height>:format=p010,tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0
```

If CUDA tone mapping is unavailable, NVIDIA falls back to software `zscale`/`tonemap` while still using `h264_nvenc` when the encoder is usable. Intel HDR tone mapping also uses the software filter chain with the hardware encoder, and never uses `scale_qsv`. The software filter chain needs software frames; forcing hardware decode there would complicate or break the filter pipeline. Keeping hardware encode still reduces CPU load on the final encode step.

The Hable tone curve is a practical default that gives reasonable SDR output for HDR movies without exposing tone-map tuning to users yet.

## Segment Serving and Readiness

FFmpeg writes segments sequentially while the browser is already requesting them. Igloo deliberately does not serve a segment as soon as the file appears.

A segment is treated as complete when:

- the next segment exists, which means FFmpeg has moved on, or
- FFmpeg has exited and the segment file exists.

`init.mp4` is treated as ready only after the first media segment exists. This avoids giving hls.js an init segment before any media is available.

This design prevents browsers from reading partially written `.m4s` files, which can cause decode errors, retry loops, or broken playback state. Segment requests wait up to `hlsSegmentWait` and poll every `hlsSegmentPoll`. If FFmpeg exits with an error before a requested segment exists, Igloo returns a transcode failure instead of hanging.

The poll interval is deliberately short. A segment that lands just after a check waits a full interval before it is served, and that wait sits directly on the startup and post-seek path; the readiness check itself is one or two stats against page-cached directory entries, so polling tightly costs far less than the latency it removes. The wait also ends as soon as the request context is cancelled — a seek abandons the in-flight segment request, and without that the goroutine would keep polling for the full `hlsSegmentWait`, so scrubbing would accumulate them.

Segments and whole media files are served through the kernel's `sendfile(2)` path. The session middleware wraps the response writer in a type that does not implement `io.ReaderFrom`, which would silently force every byte through a userspace copy, so `restoreSendfile` re-exposes the capability for the whole router. See §4.4 of `docs/web-direct-playback-audit.md`.

FFmpeg stderr is not streamed to clients. The HLS runner keeps the last 20 stderr lines and passes them to the session exit handler for logging. That gives enough context for server-side troubleshooting without storing unbounded FFmpeg output.

## Seeking and Resume Behavior

The HLS manifest accepts a `start` query parameter. When `start` is greater than zero, FFmpeg starts from that source offset with `-ss`. A raw API start offset at or past the movie's duration — stale saved progress after a re-scan, or rounding at the very end — is clamped to five seconds before the end instead of failing (or to zero when the movie is shorter than five seconds). That effective start drives the singleflight/cache key, FFmpeg parameters, generated asset URLs, and subsequent segment lookup.

When the web client knows the movie duration, it first clamps the requested absolute playback target to that duration. For HLS it then applies the 10-second resume rewind to the clamped target and uses the result consistently for the manifest URL, session-window key, local playback offset, and absolute-time mapping. Direct playback initialization uses the same clamped absolute target.

Igloo exposes the rebased session as a VOD playlist. The files on disk start at `segment_0.m4s`, but the UI keeps absolute movie time. When a seek requires a different offset, the client asks for a manifest with a new `start` value and Igloo creates a new session.

A rebased session's playlist covers only the remaining time from the start offset to the end of the movie. While FFmpeg is still encoding, Igloo generates the VOD playlist from that remaining duration; after FFmpeg exits successfully, the finalized FFmpeg playlist with accurate segment durations is served with only its asset URLs rewritten. The client is responsible for mapping session-local playback time back to absolute movie time in the UI.

This is more complex than exposing FFmpeg's event playlist directly, but it gives browser players the behavior users expect from a movie: visible duration, seeking, resume, and a stable VOD presentation.

## Watch Rooms

Watch room HLS uses the same FFmpeg session machinery with room-specific cache keys:

```text
room:<room_id>
```

A room stores its audio track when it is created, so the value is validated up front rather than at first playback. Room creation rejects an `audio_track` beyond the movie's audio stream count, a non-zero `audio_track` on a movie without audio, and a non-zero `audio_track` combined with direct playback, which would serve the container's first track to every member regardless of the stored value.

Room sessions are isolated from personal playback sessions so a watch room cannot collide with a user's individual HLS session for the same movie. Watch rooms warm up HLS from the beginning so participants can join a prepared stream. When a room is deleted, Igloo marks the room session deleted, removes the cached session, kills the FFmpeg process if it is still running, and removes the temp directory.

## Subtitle Conversion

Subtitle WebVTT endpoints use FFmpeg only for text subtitle streams.

The endpoint:

```text
/api/movies/{id}/subtitles/{trackIndex}/web.vtt
```

uses `trackIndex` as the 0-based index into the movie's stored subtitle rows. It then maps that row back to the absolute ffprobe stream index and runs FFmpeg:

```text
ffmpeg -v error -i <source> -map 0:<stream_index> -c:s webvtt -f webvtt pipe:1
```

The output is returned directly from stdout and cached for one hour by movie ID and stream index. The request has a 60-second timeout so a difficult subtitle track cannot tie up a request indefinitely.

Bitmap subtitle codecs are rejected before FFmpeg runs:

- `hdmv_pgs_subtitle`
- `dvd_subtitle`
- `dvb_subtitle`

These codecs are image-based and cannot be reliably converted to WebVTT text. Rejecting them explicitly gives clients a clear unsupported-media response instead of a confusing conversion failure.

After conversion, Igloo replaces escaped `\h` sequences with spaces. This handles subtitle text that uses hard-space style escapes not wanted in WebVTT output.

## Operational Notes

For binary deployments:

- `TRANSCODE_DIR` seeds the Settings transcode directory on first launch; after that, edit it from Settings.
- HLS temp output is written below the Settings transcode directory.
- `HLS_MAX_CPU_TRANSCODES` is read at startup and limits concurrent HLS transcode sessions; copy-video (remux) sessions are not counted. It is not stored in Settings.
- `HLS_MAX_SESSIONS_PER_USER` is read at startup and limits cached plus in-flight personal HLS sessions per user; remux and transcode sessions are both counted. The default is 3. It is not stored in Settings.
- Configured media directories should be readable by the Igloo process. Igloo does not need write access to media libraries.

For local development:

- `make dev` uses the `externalbin` build tag and requires `ffmpeg` and `ffprobe` on `PATH`.
- Backend tests should be run with the `externalbin sqlite_fts5` build tags and require `ffmpeg` and `ffprobe` on `PATH`.
- `make build` uses embedded release payloads for the current native platform.
- `HARDWARE_ACCELERATION_DEVICE` can be set in `.env` for local testing.
- Apple VideoToolbox only applies to macOS builds with the Apple-capable FFmpeg binary.
- Linux hardware acceleration requires the host drivers, devices, and FFmpeg build support to be present.

For failures:

- ffprobe failure during scanning means the item cannot be indexed reliably.
- FFmpeg HLS startup failure is returned from the manifest request.
- FFmpeg runtime failure is logged with the stderr tail.
- Segment requests fail with "segment not ready", "segment does not exist", or "transcoding stopped" depending on session state.
- Subtitle extraction failures are logged server-side and returned to clients as a generic extraction failure.

## Maintenance Rules

When changing FFmpeg or ffprobe behavior:

- Check the embedded payload version with `ffmpeg -version` and `ffprobe -version` after refreshing binaries. Prefer the current stable Jellyfin FFmpeg release line for release payloads; do not switch to a generic upstream FFmpeg build or Jellyfin prerelease branch without a specific reason.
- Keep argument construction covered by the tests in `server/cmd/internal/ffmpeg/` (`ffmpeg_hls_args_test.go`, `ffmpeg_hls_args_additional_test.go`, `ffmpeg_hls_hardware_args_test.go`, `ffmpeg_hls_run_test.go`).
- Keep remux validation covered by `remux_validator` tests when changing fMP4 safety behavior.
- Keep HLS handler and playlist tests updated when changing playlist shape, filenames, query parameters, readiness rules, or resume behavior.
- Update `docs/openapi.json` when adding or changing HLS, subtitle, or playback settings endpoints.
- Update `.env.example`, settings validation, README hardware notes, and this document when adding a hardware acceleration device.
- Update `hls_profiles.go`, playback settings responses, OpenAPI schemas, frontend profile lists, and this document when adding or changing an HLS profile.
- Do not add new FFmpeg command-line options only in handlers. Keep FFmpeg argument construction centralized in the internal FFmpeg wrapper so tests can validate the full command.
- Treat browser compatibility as a product requirement. Prefer explicit fallback to a known playable profile over exposing a stream that might work on one browser and fail on another.

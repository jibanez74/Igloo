# FFmpeg and ffprobe in Igloo

This document explains how Igloo uses FFmpeg and ffprobe, why the current design exists, and what to keep in mind when changing media playback, scanning, subtitles, or deployment behavior.

Igloo uses these tools for three separate jobs:

- `ffprobe` reads media metadata during library scans.
- `ffmpeg` creates HLS output for browser playback when direct file playback is not enough.
- `ffmpeg` converts supported text subtitle streams to WebVTT.

Direct file streaming is separate from this flow. When the client can play a source file directly, Igloo can serve the original media without starting FFmpeg. FFmpeg is used when Igloo needs compatible HLS output, audio conversion, video transcoding, HDR tone mapping, or subtitle conversion.

## Binary Strategy

Release builds use embedded FFmpeg and ffprobe binaries. Platform-specific files under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/` use `//go:embed` to include the binary payload at compile time. At startup, Igloo extracts each binary into an operating-system temp directory such as `igloo-ffmpeg-*` or `igloo-ffprobe-*`, marks it executable, and keeps a singleton wrapper pointing at the extracted path.

Development and CI can use the `externalbin` build tag instead. With that tag, the wrappers do not extract embedded binaries. They resolve tools in this order:

- `IGLOO_FFMPEG_PATH` or `IGLOO_FFPROBE_PATH`
- `ffmpeg` or `ffprobe` on `PATH`

The split exists for practical reasons:

- Release packages are self-contained on supported platforms.
- Development and tests do not require large ignored payload files.
- Hardware acceleration still depends on the host runtime, drivers, and FFmpeg build support.
- The wrappers give the application a stable internal interface even though the binary source differs by build mode.

Both wrappers are singletons. `ffmpeg.New()` and `ffprobe.New()` return the same instance after first initialization. On shutdown, `ffmpeg.Cleanup()` and `ffprobe.Cleanup()` remove extracted temp directories in embedded mode and reset the singleton state. In `externalbin` mode there is no extracted directory, so cleanup only resets the wrapper instance.

## Metadata Scanning

Movie and music scans treat ffprobe as required infrastructure.

For movies, Igloo calls `app.Ffprobe.GetMetadata(path)` while processing each movie file. The scanner uses ffprobe output for:

- duration and runtime
- file size when ffprobe reports it
- container and stream metadata
- video, audio, and subtitle stream rows
- chapter information
- video dimensions, codec names, profiles, bit depth, pixel formats, frame rates, and color metadata
- audio codecs, language tags, channel layout, sample rate, and bitrate
- subtitle codecs, language tags, stream indices, and titles

For music, Igloo uses ffprobe to populate track metadata such as title, sort title, artist, album, genre, track number, disc number, release date, duration, bitrate, size, composer, and copyright.

The scanner stores stream data in SQLite so playback does not need to run ffprobe on every HLS request. That is intentional. HLS session creation reads movie, video stream, and audio stream rows from the database and starts FFmpeg from that stored metadata. This keeps playback startup predictable and avoids probing the same file repeatedly while users are trying to watch something.

`ffprobe.GetMetadata` runs:

```bash
ffprobe -v quiet -print_format json -show_streams -show_format -show_chapters <file>
```

The quiet JSON output keeps parsing deterministic and avoids mixing log text with structured data. Igloo rejects results with no streams because a scanned media item without streams cannot be played or indexed reliably.

## HLS Playback

Browser HLS playback is built around on-demand FFmpeg sessions.

When a client requests:

```text
/api/movies/{id}/hls/{profile}/playlist.m3u8
```

Igloo loads the movie and stream metadata from the database, creates a temp directory, starts FFmpeg in the background, caches the session, and returns a VOD-style playlist to the browser. Segment requests then read files from the session temp directory as FFmpeg produces them.

HLS sessions are keyed by movie ID, requested profile, audio track, and start time. If the same request arrives again, Igloo refreshes the cached session TTL and reuses the process. If the requested start time changes, the old session is evicted and a new FFmpeg process starts at the new offset. Concurrent creation is deduplicated with singleflight so multiple near-simultaneous manifest requests do not start duplicate transcodes for the same session.

FFmpeg runs with `context.Background()` after session creation. This is deliberate: an HLS process must outlive the HTTP request that created it, because the browser will request the manifest and segments as separate requests. The session cache owns the lifecycle. Expiration, eviction, room cleanup, or server shutdown stops the process and removes the temp directory.

HLS temp directories are created under `TRANSCODE_DIR`, which defaults to `$IGLOO_DATA_DIR/transcode`. This keeps heavy temporary media output in Igloo's configured runtime data area instead of the operating-system temp directory.

## HLS Output Format

Igloo writes fragmented MP4 HLS, not MPEG-TS HLS.

The FFmpeg HLS command uses:

- `-f hls`
- `-hls_segment_type fmp4`
- `-hls_fmp4_init_filename init.mp4`
- `-hls_segment_filename segment_%d.m4s`
- `-hls_playlist_type event`
- `-hls_list_size 0`
- `-hls_time 4`

The generated files match the HTTP handlers:

- `init.mp4`
- `segment_0.m4s`
- `segment_1.m4s`
- `playlist.m3u8`

fMP4 HLS is used because modern browser players handle it well and it works naturally with copied H.264 video, transcoded H.264 video, and AAC audio. A short 4-second target segment gives acceptable startup and seek behavior while keeping the number of segment files manageable.

FFmpeg writes an event playlist while encoding. Igloo exposes a VOD-style playlist to clients. During encoding, Igloo generates a complete VOD playlist from the known movie duration so hls.js sees a seekable on-demand asset instead of a live/event stream. After FFmpeg exits successfully, Igloo finalizes the FFmpeg playlist by switching it to VOD and appending `#EXT-X-ENDLIST` when needed.

The manifest handler rewrites playlist asset URLs so each `init.mp4` and `segment_N.m4s` URL includes the selected audio track and session query parameters. This keeps HLS asset requests tied to the same session configuration that created the manifest.

HLS responses use `Cache-Control: no-store`. Transcode output is session-scoped, temporary, and can vary by profile, audio track, start time, and playback context. Browser or proxy caching would make stale segment and playlist behavior harder to reason about.

## Remux, Transcode, and Fallback

Igloo supports a special HLS profile named `remux`. Remux mode copies the video stream with `-c:v copy` and only transcodes audio when the selected audio codec is not already AAC.

Remux exists because it preserves source video quality and avoids expensive video encoding when the source video is browser-compatible. It is much cheaper than transcoding and is the best path for compatible H.264 sources.

Remux is only attempted for browser-compatible H.264 codec names:

- `h264`
- `h.264`
- `avc`
- `avc1`

If the source video is not browser-compatible H.264, Igloo immediately falls back to the best-fit transcode profile. Igloo also falls back for H.264 streams that are not a safe browser remux target, including 10-bit, 4:2:2, or 4:4:4 sources identified from stored codec profile, bit depth, or pixel format metadata. This avoids serving copied video that browsers are unlikely to play through HLS.

Even H.264 remux can be unsafe. Some copied fMP4 fragments can start at samples that are not independently decodable by browser players. To avoid that, Igloo preflights remux output before committing to it:

- wait for `init.mp4`
- wait for the first 4 complete segments
- inspect the generated fMP4 fragments
- verify sync samples in the video track start with IDR frames
- cache the safe or unsafe verdict for 24 hours

If preflight times out or FFmpeg exits before enough output is available, Igloo falls back to transcoding without caching an unsafe verdict, because that kind of failure may be transient. If validation proves the fragments are unsafe, Igloo caches the unsafe verdict and falls back immediately for later sessions using the same movie, stream index, file size, and update timestamp.

The fallback profile is chosen with `BestFitHLSFallbackProfile`. Igloo picks the highest configured transcode profile whose target height fits within the source height. If the source is smaller than every configured profile, it falls back to `720p_3mbps` so playback still has a reliable transcode path.

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

CPU transcode uses:

```text
-c:v libx264 -preset veryfast -sc_threshold:v:0 0
```

The `veryfast` preset is a practical default for self-hosted playback: it prioritizes real-time performance over maximum compression efficiency. Scene-cut insertion is disabled for CPU transcodes because Igloo aligns keyframes on the HLS segment cadence. Predictable keyframes make HLS segmentation and seeking more reliable.

When the source frame rate is known and a hardware encoder is active, Igloo uses a fixed 4-second GOP:

```text
-g:v:0 <segment_time*fps> -keyint_min:v:0 <segment_time*fps>
```

Other video transcode paths use forced keyframe expressions:

```text
-force_key_frames:0 expr:gte(t,n_forced*4)
```

Both paths align keyframes with the 4-second HLS segment target. Without predictable keyframes, HLS segments can drift, seek behavior gets worse, and browsers may wait longer for independently decodable frames.

FFmpeg also runs with:

- `-fflags +genpts` to generate timestamps when sources have missing or awkward presentation timestamps.
- `-analyzeduration 5000000` and `-probesize 5000000` to give FFmpeg enough input data to identify streams without making startup unbounded.
- `-threads max(1, runtime.NumCPU()/2)` to avoid letting one transcode consume every CPU core on a home server.
- `-avoid_negative_ts make_zero` to normalize output timestamps.
- `-max_muxing_queue_size 1024` to tolerate sources with stream timing that would otherwise overflow FFmpeg's muxing queue.

## Audio Handling

HLS sessions map exactly one video stream and one audio stream:

```text
-map 0:<video_stream_index>
-map 0:<audio_stream_index>
```

The stream indices are absolute ffprobe stream indices stored during scanning. Igloo does not rely on FFmpeg's relative stream numbering at playback time.

If the selected audio codec is AAC, Igloo copies it:

```text
-c:a copy
```

Otherwise, Igloo converts audio to stereo AAC at `256k`:

```text
-c:a aac -ac 2 -b:a 256k
```

AAC is the safest baseline for browser HLS playback. Downmixing to stereo avoids playback failures on clients that do not support the source channel layout.

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
| `nvidia` | `-hwaccel cuda -hwaccel_output_format cuda` when CUDA filters are available; otherwise software decode | `h264_nvenc` | Linux with NVIDIA driver/runtime support |
| `intel` | `-hwaccel qsv` | `h264_qsv` | Linux with Intel QSV support |

NVIDIA adds:

```text
-rc vbr -preset p4
```

Intel adds:

```text
-look_ahead 1
```

At startup, Igloo probes FFmpeg for encoders, filters, hardware acceleration methods, and key filter options. CPU, unknown devices, missing hardware encoders, and failed NVENC runtime probes fall back to `libx264`. This is intentional: an invalid or unavailable hardware mode should not create a new unsupported encoder path inside the argument builder. The settings API validates known device names, but the HLS builder still has a safe CPU fallback.

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
format=yuv420p
```

For NVIDIA HDR tone mapping, Igloo uses `tonemap_cuda` only when the probed FFmpeg build exposes the CUDA tone-map filter and the options Igloo needs. If not, NVIDIA falls back to software `zscale`/`tonemap` while still using `h264_nvenc` when the encoder is usable. Intel HDR tone mapping also uses the software filter chain with the hardware encoder. The software filter chain needs software frames; forcing hardware decode there would complicate or break the filter pipeline. Keeping hardware encode still reduces CPU load on the final encode step.

The Hable tone curve is a practical default that gives reasonable SDR output for HDR movies without exposing tone-map tuning to users yet.

## Segment Serving and Readiness

FFmpeg writes segments sequentially while the browser is already requesting them. Igloo deliberately does not serve a segment as soon as the file appears.

A segment is treated as complete when:

- the next segment exists, which means FFmpeg has moved on, or
- FFmpeg has exited and the segment file exists.

`init.mp4` is treated as ready only after the first media segment exists. This avoids giving hls.js an init segment before any media is available.

This design prevents browsers from reading partially written `.m4s` files, which can cause decode errors, retry loops, or broken playback state. Segment requests wait up to `HLS_SEGMENT_WAIT` and poll every `HLS_SEGMENT_POLL`. If FFmpeg exits with an error before a requested segment exists, Igloo returns a transcode failure instead of hanging.

FFmpeg stderr is not streamed to clients. The HLS runner keeps the last 20 stderr lines and passes them to the session exit handler for logging. That gives enough context for server-side troubleshooting without storing unbounded FFmpeg output.

## Seeking and Resume Behavior

The HLS manifest accepts a `start` query parameter. When `start` is greater than zero, FFmpeg starts from that source offset with `-ss`.

Igloo exposes the rebased session as a VOD playlist. The files on disk start at `segment_0.m4s`, but the UI keeps absolute movie time. When a seek requires a different offset, the client asks for a manifest with a new `start` value and Igloo creates a new session.

Final playlists from completed rebased sessions can include accurate FFmpeg segment durations for the generated portion. Igloo fills earlier timeline space with placeholder durations so hls.js has a coherent total-duration timeline.

This is more complex than exposing FFmpeg's event playlist directly, but it gives browser players the behavior users expect from a movie: visible duration, seeking, resume, and a stable VOD presentation.

## Watch Rooms

Watch room HLS uses the same FFmpeg session machinery with room-specific cache keys:

```text
room:<room_id>
```

Room sessions are isolated from personal playback sessions so a watch room cannot collide with a user's individual HLS session for the same movie. Watch rooms warm up HLS from the beginning so participants can join a prepared stream. When a room is deleted, Igloo marks the room session deleted, removes the cached session, kills the FFmpeg process if it is still running, and removes the temp directory.

## Subtitle Conversion

Subtitle WebVTT endpoints use FFmpeg only for text subtitle streams.

The endpoint:

```text
/api/movies/{id}/subtitles/{trackIndex}/web.vtt
```

uses `trackIndex` as the 0-based index into the movie's stored subtitle rows. It then maps that row back to the absolute ffprobe stream index and runs FFmpeg:

```text
ffmpeg -v error -y -i <source> -map 0:<stream_index> -c:s webvtt -f webvtt pipe:1
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

- `IGLOO_DATA_DIR` defaults to `./data`.
- `TRANSCODE_DIR` defaults to `$IGLOO_DATA_DIR/transcode`.
- HLS temp output is written below `TRANSCODE_DIR`.
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

- Keep argument construction covered by tests in `server/cmd/internal/ffmpeg/ffmpeg_hls_test.go`.
- Keep remux validation covered by `remux_validator` tests when changing fMP4 safety behavior.
- Keep HLS handler and playlist tests updated when changing playlist shape, filenames, query parameters, readiness rules, or resume behavior.
- Update `docs/openapi.json` when adding or changing HLS, subtitle, or playback settings endpoints.
- Update `.env.example`, settings validation, README hardware notes, and this document when adding a hardware acceleration device.
- Update `hls_profiles.go`, playback settings responses, OpenAPI schemas, frontend profile lists, and this document when adding or changing an HLS profile.
- Do not add new FFmpeg command-line options only in handlers. Keep FFmpeg argument construction centralized in the internal FFmpeg wrapper so tests can validate the full command.
- Treat browser compatibility as a product requirement. Prefer explicit fallback to a known playable profile over exposing a stream that might work on one browser and fail on another.

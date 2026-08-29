# FFmpeg and ffprobe in Igloo

This document explains how Igloo uses FFmpeg and ffprobe, why the current design exists, and what to keep in mind when changing media playback, scanning, subtitles, or deployment behavior.

Igloo uses these tools for three separate jobs:

- `ffprobe` reads media metadata during library scans.
- `ffmpeg` creates HLS output for browser playback when direct file playback is not enough.
- `ffmpeg` converts supported text subtitle streams to WebVTT.

Direct file streaming is separate from this flow. When the client can play a source file directly, Igloo can serve the original media without starting FFmpeg. FFmpeg is used when Igloo needs compatible HLS output, audio conversion, video transcoding, HDR tone mapping, or subtitle conversion.

## Binary Strategy

Release builds use embedded FFmpeg and ffprobe binaries. Platform-specific files under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/` use `//go:embed` to include the binary payload at compile time. At startup, Igloo extracts each binary into an operating-system temp directory such as `igloo-ffmpeg-*` or `igloo-ffprobe-*`, marks it executable, and keeps a singleton wrapper pointing at the extracted path.

Igloo uses Jellyfin FFmpeg builds for release payloads, never generic upstream builds — the Linux x64 payloads in this repository report `7.1.4-Jellyfin`. Follow the current stable `jellyfin-ffmpeg` line; do not move to a prerelease line or an upstream build without a specific reason.

When refreshing payloads, update both `ffmpeg_<platform>` and `ffprobe_<platform>` from the same Jellyfin release. Linux x64 uses `ffmpeg_linux_amd64` / `ffprobe_linux_amd64`; macOS ARM64 expects `ffmpeg_darwin_arm64` / `ffprobe_darwin_arm64`. `make build` checks that the current platform's payloads exist before compiling.

Development and CI use the `externalbin` build tag instead, which skips extraction and resolves `IGLOO_FFMPEG_PATH` / `IGLOO_FFPROBE_PATH` first, then `ffmpeg` / `ffprobe` on `PATH`. The split keeps release packages self-contained while keeping large payload files out of development checkouts; either way the wrappers present the same internal interface, and hardware acceleration always depends on the host runtime regardless of build mode.

Both wrappers are singletons. `ffmpeg.New()` and `ffprobe.New()` return the same instance after first initialization. Each wrapper verifies the resolved binary with `-version` before accepting it, so a corrupt, wrong-architecture, or non-executable binary fails during startup instead of during the first scan or transcode. On shutdown, `ffmpeg.Cleanup()` and `ffprobe.Cleanup()` remove extracted temp directories in embedded mode and reset the singleton state. In `externalbin` mode there is no extracted directory, so cleanup only resets the wrapper instance.

## Metadata Scanning

Movie and music scans treat ffprobe as required infrastructure.

For movies, the scanner (`server/cmd/internal/scanner/movie`) calls `GetMetadata(ctx, path)` while processing each file and persists duration, container, chapters, and a row per video, audio, and subtitle stream — dimensions, codec names and profiles, bit depth, pixel format, frame rates, color metadata, language tags, channel layout, and dispositions. The database schema is the authoritative list; two details are not obvious from it:

- **Rotation** comes from display-matrix side data, and the absence of a matrix is distinct from a zero one: an explicit 0-degree matrix persists as `0`, a stream with no matrix as `NULL`.
- **Stream tag keys** are normalized like format tags (lowercased, separators stripped, `lang` accepted as a `language` alias), so Matroska muxers writing `TITLE`/`LANGUAGE` still produce labelled, preference-matchable streams.

For music, `GetAudioMetadata(ctx, path)` populates track metadata (title, artist, album, genre, track and disc numbers, release date, duration, bitrate, composer, copyright); the library scan supplies each file's size from the filesystem.

The scanner stores stream data in SQLite so playback does not need to run ffprobe on every HLS request. That is intentional. HLS session creation reads movie, video stream, and audio stream rows from the database and starts FFmpeg from that stored metadata. This keeps playback startup predictable and avoids probing the same file repeatedly while users are trying to watch something.

Movie scans run:

```bash
ffprobe -v quiet -print_format json -show_streams -show_format -show_chapters <file>
```

Both metadata calls take the caller's context and cap each probe at 60 seconds on top of it. The scan context is the one canceled by shutdown, so stopping the server kills an in-flight ffprobe rather than leaving `app.Wait.Wait()` to sit out the timeout — which matters on slow or network-mounted media. The two failures are reported differently: a canceled caller yields `ffprobe canceled for <file>`, while a probe that outlives its own deadline yields `ffprobe timed out for <file> after 1m0s`, so only the latter indicates a file that is genuinely slow to read.

Music scans limit `-show_entries` to the fields the music scanner needs. The quiet JSON output keeps parsing deterministic and avoids mixing log text with structured data. Igloo rejects results with no streams, because a scanned item without streams cannot be played or indexed reliably.

## HLS Playback

Browser HLS playback is built around on-demand FFmpeg sessions.

When a client requests a personal HLS playlist:

```text
/api/movies/{id}/hls/{profile}/playlist.m3u8?playback_session=<uuid>&start=<seconds>&audio_track=<index>
```

`audio_track` is omitted for video-only movies. Igloo loads the movie duration, normalizes the requested start, reserves personal-session capacity, loads the remaining stream metadata, creates a temp directory, starts FFmpeg in the background, converts the reservation into a cached session, and returns a VOD-style playlist to the browser. Segment requests then read files from the session temp directory as FFmpeg produces them.

Personal HLS sessions are keyed by authenticated owner user ID, movie ID, requested profile, audio track, audio mode, playback session ID, and effective normalized start time. If the same request from the same owner arrives again, Igloo refreshes the cached session TTL and reuses the process. The owner is also part of the singleflight key, so concurrent identical requests from one user deduplicate while identical URL tuples from different users create distinct FFmpeg sessions and cache entries. Before FFmpeg starts, Igloo evicts expired entries and removes only superseded windows for the same movie, user, and `playback_session` UUID; sessions from other users or playback UUIDs remain isolated. The owner check on a retrieved session remains as defense-in-depth. Different clients can therefore play the same movie concurrently unless the per-user cap requires an LRU replacement.

Cached personal sessions and in-flight creations share the `HLS_MAX_SESSIONS_PER_USER` cap (default 3). Admission reserves capacity before FFmpeg starts. At the cap, the owner's cached personal sessions are evicted in least-recently-used order until the new reservation fits; rooms and other users' sessions are never candidates. If every slot is already held by an in-flight reservation, the manifest request returns `503 Service Unavailable` with `Retry-After`. Remux and transcode creations both participate in this cap. A successful creation atomically exchanges its reservation for the cache entry, while every failure path releases the reservation. Concurrent creation of the same effective key is still deduplicated with singleflight. Clients can also tear a playback session's HLS sessions down explicitly with `POST /api/movies/{id}/hls/session/stop`; the stop endpoint stays scoped to its own playback session ID so a late stop from a closing tab cannot remove a session the user just created after reopening.

Personal sessions use a 5-minute idle TTL that every manifest and segment request refreshes. Because hls.js stops fetching once its buffer is full and a paused tab fetches nothing, the web player refetches the manifest every 2 minutes while HLS playback is ready and the video player is rendered. A fatal playback error removes the player and stops the timer immediately; a successful retry renders the player and enables it again. A client that skips the keepalive (or wakes from OS sleep after eviction) recovers transparently because a manifest request recreates the session at the same start offset. Watch-room sessions keep a 30-minute TTL — rooms have no per-client keepalive and always warm from the beginning, so evicting an idle room would restart playback for every participant. The cache sweep runs every minute, so an abandoned personal session is fully reclaimed (FFmpeg killed, temp dir removed, transcode permit released) within about six minutes even without an explicit stop.

HLS requests additionally accept an optional `reload` query parameter. It is an opaque client-supplied value that is echoed into the rewritten playlist asset URLs; it is not part of the session cache key.

Authentication is checked on every personal and watch-room manifest and asset request. Watch-room requests also authorize current membership each time; successful membership lookups are cached for 30 seconds, while denials and query failures are never cached. Rewritten playlists never embed credentials, so native clients must send their cookie or bearer token again for each `init.mp4` and `segment_N.m4s` fetch. Personal asset URLs propagate the selected audio track, explicit audio profile, normalized start, playback-session UUID, and reload value so every request resolves the session that created the playlist; watch-room asset URLs propagate the room's selected audio track.

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

`#EXT-X-INDEPENDENT-SEGMENTS` is emitted only where the guarantee is proven, which is transcode sessions whose encoder turns `-force_key_frames` into real IDR frames. `ffmpeg.HLSSegmentsAreIndependent` is the single predicate: it decides both whether FFmpeg receives `-hls_flags independent_segments` (which controls the tag in FFmpeg's own playlist, never segmentation) and whether the synthesized transcode playlist writes the tag, so a session's two playlist flavors always agree. libx264 and VideoToolbox always force IDRs; NVENC and QSV only do with `-forced-idr`/`-forced_idr`, so a build that does not expose the option loses the tag along with the guarantee.

Copy-video sessions never carry the tag. Their segments split on whatever keyframes the source encode left behind, and the remux validator only inspects the first 4 fragments at the session's start offset — a source whose GOP structure changes later in the file is never ruled out, so claiming whole-playlist independence would let a native HLS player seek straight into a segment that still references the previous GOP. hls.js ignores the tag in media playlists either way, so the practical beneficiary is native HLS playback (Safari) plus spec conformance. `#EXT-X-START` is deliberately not emitted: every session is rebased to zero and the web client passes an explicit `startPosition` to hls.js, which would override the tag anyway.

FFmpeg writes an event playlist while encoding. Which playlist a client receives depends on whether the session copies video, because only one of the two can be described arithmetically:

- **Transcode sessions** get a playlist synthesized from the known movie duration: `ceil(duration / 4)` entries each declared as 4 seconds, with a target duration of 8. This is exact, because `-force_key_frames` pins every boundary to a 4-second mark (measured drift over 300 s of output: 8 ms). The whole movie is listed from the first request, so the asset is seekable end to end immediately and hls.js sees an on-demand asset rather than a live stream.
- **Copy-video (`remux`) sessions** are served FFmpeg's own playlist with only the asset URLs rewritten. Copied segments split only at source keyframes, so their durations are whatever the source encode dictates and vary widely both within and between files. Synthesizing one would advertise durations FFmpeg never produced and segments that never exist, and the surplus entries `404`ed once the session finished — breaking playback near the end of the film.

While a copy-video session is still encoding, its playlist is `#EXT-X-PLAYLIST-TYPE:EVENT` with no `#EXT-X-ENDLIST`, so players reload it and pick up new segments as FFmpeg publishes them. A manifest request for one that has not published its first segment yet waits up to 30 seconds and then returns `503` with a short `Retry-After` rather than falling back to a synthesized playlist. Together with the possible 15-second transcode-capacity wait, a cold personal manifest request has a maximum 45-second server wait budget.

When FFmpeg exits, Igloo finalizes whatever playlist it left behind by switching it to VOD and appending `#EXT-X-ENDLIST` — on failed exits as well as clean ones, because the live playlist file outlives the process that was appending to it, and terminating it is what lets a client play up to the failure point and stop rather than reload an unterminated playlist forever.

Both flavors check exit status before answering: for copy-video ahead of reading the live playlist file, since that file outlives its writer; for transcodes ahead of synthesizing, since a synthesized playlist describes output that only exists while FFmpeg is still running. A finalized playlist with playable segments is served through the failure point. A finalized playlist without playable segments is an empty session; an exit error without a publishable playlist is an FFmpeg session failure. Neither may be answered with a complete playlist — that hides the failure until the client has waited out every segment request in turn.

The practical cost is that a copy-video session is seekable only across what FFmpeg has produced. During steady playback the encoder runs several times faster than realtime, and the web client rebases the session for any seek more than 120 seconds ahead, so this is narrower than it sounds.

The manifest handler rewrites playlist asset URLs so each `init.mp4` and `segment_N.m4s` URL includes the selected audio track and session query parameters. The rewritten `start` is the effective normalized start used by the cache key and FFmpeg, not an out-of-range value from the original request. This keeps HLS asset requests tied to the same session configuration that created the manifest.

HLS responses use `Cache-Control: no-store`. Transcode output is session-scoped, temporary, and can vary by profile, audio track, start time, and playback context. Browser or proxy caching would make stale segment and playlist behavior harder to reason about.

## Remux, Transcode, and Fallback

Igloo supports a special HLS profile named `remux`. Remux mode copies the video stream with `-c:v copy` and only transcodes audio when the selected audio codec is not already AAC.

Remux exists because it preserves source video quality and avoids expensive video encoding when the source video is browser-compatible. It is much cheaper than transcoding and is the best path for compatible H.264 sources. It is also reached without the user selecting it: choosing any audio track other than the container's first resolves the mode to `remux` (see Audio Handling).

Remux is only attempted for browser-compatible H.264 codec names:

- `h264`
- `h.264`
- `avc`
- `avc1`

If the source video is not browser-compatible H.264, Igloo immediately falls back to the best-fit transcode profile. It also falls back for H.264 that is not a safe browser remux target, decided by `isBrowserSafeH264RemuxCandidate` from the stored codec profile, bit depth, pixel format, and field order:

- **Pixel format** is an allowlist of the 8-bit 4:2:0 formats browsers decode — `yuv420p`, `yuvj420p`, `nv12`, `nv21` — so 10-bit, 4:2:2, 4:4:4, and anything unrecognised falls back rather than being assumed safe.
- **Interlaced** sources (`field_order` of `tt`/`bb`/`tb`/`bt`) fall back because browsers do not deinterlace, so a copied interlaced stream displays combed; the transcode path applies `yadif` instead. Rows scanned before `field_order` was persisted are `NULL` and treated as progressive.

These two lists are the single definition of "browser-safe" for the whole system. The web client's direct-play gate applies the same rules against its own copy of both lists — **the copies must stay in sync**.

Even H.264 remux can be unsafe. Some copied fMP4 fragments can start at samples that are not independently decodable by browser players. To avoid that, Igloo preflights remux output before committing to it:

- wait for `init.mp4`
- wait for the first 4 complete segments
- inspect the generated fMP4 fragments
- verify sync samples in the video track start with IDR frames
- persist the safe or unsafe verdict in the database (`remux_safety_verdicts`), keyed by movie and stream with a fingerprint of the file (size, update timestamp), the stream properties the safety gate reads, and the producer that generated the validated output

This is a sample, not a proof: the 4 segments are the session's first 4, so a preflight run for a seek covers that offset rather than the head of the file, and a GOP structure that changes later is not ruled out — which is why copy-video playlists never advertise `#EXT-X-INDEPENDENT-SEGMENTS`.

Persisted verdicts survive server restarts, so the preflight cost is paid once per file rather than once per process. A verdict is recomputed only when its fingerprint changes — the file was replaced or rescanned with a new size or timestamp, its stream properties changed, or the producer changed. The producer terms matter because a verdict validates FFmpeg-generated fMP4 output, not just the source: the fingerprint carries the FFmpeg version parsed from the startup `-version` banner (so an upgraded embedded payload or a swapped `IGLOO_FFMPEG_PATH` binary invalidates it) plus `remuxVerdictProducerRevision`, a constant to bump whenever the remux arguments or `ValidateRemuxSafety` change. Either kind of change costs one re-preflight per file. If preflight times out or FFmpeg exits before enough output is available, Igloo falls back to transcoding without persisting an unsafe verdict, because that kind of failure may be transient. If validation proves the fragments are unsafe, Igloo persists the unsafe verdict and falls back immediately for later sessions on the same fingerprint.

The fallback profile is chosen with `BestFitHLSFallbackProfile`. Igloo picks the highest configured transcode profile whose target height fits within the source height. If the source is smaller than every configured profile, it falls back to `720p_3mbps` so playback still has a reliable transcode path.

## Direct Play Eligibility and Fallback

Direct play serves the original file over HTTP range requests with no FFmpeg process. Whether the web client offers it is decided from the scanned metadata plus one browser probe (`web/src/lib/playback.ts`, `getAvailableModes`):

- **Container.** Only MP4 (`mp4`/`m4v`) is eligible. The container→MIME mapping is pinned in `helpers.VideoMimeTypes` — never derived from the host's MIME tables — and MKV must never be added: Chrome and Firefox fail Matroska in a `<video>` element silently at 0ms with no `MediaError`.
- **Video.** H.264 codec names only, and the stream must pass the same browser-safety rules as the server's remux gate (see Remux, Transcode, and Fallback) — the client keeps its own copy of both lists.
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

### Interlacing, Rotation, and VFR

Interlaced sources (`isInterlacedStream`) prepend a software `yadif` at the head of the transcode filter chain, in default `send_frame` mode so the frame rate and the GOP math above are unchanged. Deinterlacing must happen at native resolution before any scaling, and decoded frames are in system memory at the chain head on every path — no chain sets `-hwaccel_output_format`, and the CUDA/QSV chains `hwupload` from system memory — so one prepend covers the CPU, NVIDIA, and Intel chains. The exception is the Apple HDR `scale_vt` chain, which consumes hardware frames: interlaced HDR sources (vanishingly rare, since interlacing is legacy broadcast SDR) route to the software tone-map chain instead. Copy-video never filters; the remux gate keeps interlaced sources off the copy paths entirely.

Rotation needs no filter work: FFmpeg's CLI applies display-matrix rotation automatically during transcode (verified against a real rotated source in `ffmpeg_integration_externalbin_test.go` — the output is rotated and the matrix consumed), and copy/direct paths pass the matrix through untouched, which browsers honor for MP4. Igloo persists the rotation only for visibility and logs it at session start.

Variable frame rate is detected (`isVFRStream` compares the stored average and nominal frame rates) and logged at session start as `vfr_detected`, but no `fps` filter is applied: forcing a rate can introduce judder on healthy content, and `-force_key_frames` already keeps segmentation correct on VFR sources.

Igloo does not pass `-threads` to FFmpeg. libx264 and the hardware encoders choose their own per-process thread behavior. Encoding pressure on a home server is bounded by the HLS transcode limiter: `HLS_MAX_CPU_TRANSCODES` sets the maximum number of concurrent HLS sessions that encode video or audio, and the default is `max(1, runtime.NumCPU()/4)`. A copy-video remux bypasses this limiter only when its selected audio is also copied or the movie has no audio. Legacy audio conversion and explicit AC-3/E-AC-3 output require a permit even when video is copied. Every personal session, including a true copy-only remux, still requires a per-user personal-session reservation.

Admission when permits are exhausted runs in three steps. First, personal playback may reclaim the owner's least-recently-used session that owns a transcode permit, but only when it has been idle for at least 30 seconds and FFmpeg is still running; reclaim skips completed sessions, rooms, other users' sessions, true copy-only sessions, and fresh active sessions, continuing through LRU candidates until it finds an eligible running encode. This includes copy-video sessions that encode audio. Second — whether or not reclaim found a victim — Igloo retries the start, and that retry parks on the permit channel for up to `hlsTranscodeAcquireWait` (15 s), releasing early the moment a permit frees or the request is cancelled. Third, a request that outlasts the budget gets the normal `503` plus `Retry-After`.

The wait is what guarantees progress. Reclaim only covers the abandoned-client case; when every permit belongs to a stream that is genuinely playing, nothing goes idle and an instant refusal starves the queued stream forever. Parking is a send on the permit channel rather than a poll, so the runtime hands a freed slot straight to the longest-waiting request with no idle-slot gap. A background room warm-up passes a zero budget and never parks.

The client contract: the park is invisible on the wire — a queued manifest request simply takes longer to answer — so the player cannot distinguish it from a slow cold start on its own. `useHlsCapacityRetry` therefore keeps its "Waiting for server capacity…" notice up from the first capacity `503` until a manifest actually arrives (`onManifestLoaded`), rather than clearing it when it fires each retry. Anything that lengthens `hlsTranscodeAcquireWait` lengthens that silent stretch, and the client's total patience is approximately the initial request plus the retry budget: 7 × wait + 6 × `Retry-After` — currently 7 × 15 s + 6 × 5 s = 135 s before the stream is reported dead.

## Audio Handling

HLS sessions always map one video stream and map one audio stream when the movie has audio:

```text
-map 0:<video_stream_index>
-map 0:<audio_stream_index>
```

For video-only movies, Igloo omits the audio map and audio codec options. The stream indices are absolute ffprobe stream indices stored during scanning. Igloo does not rely on FFmpeg's relative stream numbering at playback time.

If the selected audio codec is AAC with a scanned `codec_profile` confirmed as `LC` (`isCopySafeAACStream` in `server/cmd/api/hls_session.go`), Igloo copies it:

```text
-c:a copy
```

Otherwise — non-AAC codecs, HE-AAC/xHE-AAC profiles, or AAC whose profile was never scanned — Igloo converts audio to stereo AAC at `320k`:

```text
-c:a aac -ac 2 -b:a 320k
```

AAC-LC is the safest baseline for browser HLS playback; browser support for SBR/PS profiles inside fMP4 HLS is spotty, and an unknown profile cannot prove safety. Downmixing to stereo avoids playback failures on clients that do not support the source channel layout.

### Explicit Audio Profiles (AC-3 / E-AC-3)

The legacy behavior above removes surround channels from DTS, TrueHD, and other incompatible sources. For clients whose playback stack handles Dolby formats (the TV client feeding a Sonos system), the personal movie HLS routes accept an explicit audio profile:

```text
?audio_codec=<ac3|eac3>&audio_channels=<2|6>
```

The two parameters form one request: both absent is legacy mode exactly as documented above, both present is explicit mode, and one alone is HTTP 400 — an invalid pair is never silently normalized to legacy stereo, because legacy mode may copy a multichannel AAC-LC track and explicit AAC stereo would change existing playback. `aac` is not an accepted explicit value; AAC output exists only through legacy behavior. Watch-room HLS has no audio-profile contract and always runs in legacy mode.

Explicit requests always encode — the AAC-LC copy gate never applies, so `audio_codec=eac3` cannot return AAC because the source happened to be copy-safe. The server owns every encoding constant (`helpers/hls_audio_profiles.go`): raw query values never reach the FFmpeg command line, only a resolved typed profile validated against those tables. Output is always 48 kHz, with bitrate selected from the codec and the effective channel count:

| Output codec | 1 channel | 2 channels | 3-4 channels | 5-6 channels |
| --- | ---: | ---: | ---: | ---: |
| AC-3 | 192k | 384k | 448k | 640k |
| E-AC-3 | 192k | 384k | 512k | 768k |

`audio_channels` is a ceiling resolved against the selected `audio_track`'s stored `channels`/`channel_layout` row, regardless of source codec: mono and stereo are never upmixed, a source within the ceiling keeps its channel count and stored layout, 7.1 downmixes to standard 5.1 under a maximum of 6, and anything above 2 downmixes to standard stereo under a maximum of 2. Conversion happens through `-ac`, which rematrixes via libswresample so center, surround, and LFE content participate in downmixes. A selected audio row with no stored channel count returns a typed HTTP 422 before any session resources are allocated; a probed FFmpeg build without the resolved encoder returns non-retryable HTTP 500 before the temp directory is created or a transcode permit acquired. AAC remains required for legacy playback, but AC-3/E-AC-3 may be missing from a swapped external binary without preventing startup. An explicit audio profile is invalid for video-only media and returns HTTP 400 rather than being ignored.

The normalized pair joins the personal session cache key (`legacy` vs `explicit:<codec>:<max>`), so legacy and explicit requests — and different codecs or ceilings — never share segments, and it is propagated onto every rewritten `init.mp4`/`segment_N.m4s` URL so asset requests compute the same key. The requested profile survives a remux-safety fallback to a video transcode. The manifest response describes the session's actual audio in `X-Igloo-Effective-Audio-Codec`, `X-Igloo-Effective-Audio-Channels`, and `X-Igloo-Effective-Audio-Bitrate` (source values for copied legacy AAC, `aac`/2/`320k` for the legacy transcode, the resolved encode for explicit mode; omitted for video-only sessions). A copied legacy track with no stored channel count omits `X-Igloo-Effective-Audio-Channels` instead of reporting zero. These are diagnostic; the media stream stays the playback authority, and the media playlist gains no `CODECS` attribute or audio rendition group.

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
-forced-idr 1
```

Intel adds:

```text
-preset veryfast
-look_ahead 1
-forced_idr 1
```

Igloo only sends these encoder options when the probed FFmpeg build lists them for the encoder in question. The forced-IDR options are load-bearing rather than cosmetic: both encoders default them to false, and with the default FFmpeg asks for a plain intra frame at each `-force_key_frames` boundary instead of an IDR, so later frames may still reference across the segment boundary. When the option is missing from the build, the session also drops `#EXT-X-INDEPENDENT-SEGMENTS` rather than claim a guarantee it cannot make. Note the spelling differs by encoder: `-forced-idr` for `h264_nvenc`, `-forced_idr` for `h264_qsv`.

At startup, after the `-version` executability check, Igloo probes FFmpeg for encoders, filters, hardware acceleration methods, key filter options, encoder options, and selected runtime filter chains. Anything unproven falls back to `libx264` — CPU, unknown devices, missing hardware encoders, failed NVENC or QSV runtime probes, missing Apple VideoToolbox support. This is intentional: an invalid or unavailable hardware mode must not create a new unsupported encoder path inside the argument builder. The settings API validates known device names, but the HLS builder keeps its own CPU fallback regardless.

Hardware acceleration always depends on host drivers, device access, and matching FFmpeg build support; Apple VideoToolbox additionally requires a macOS build.

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

FFmpeg writes segments sequentially while the browser is already requesting them. Igloo deliberately does not serve a segment before it is complete.

On an FFmpeg build whose hls muxer supports it — the embedded Jellyfin build does, and the capability probe below is what decides — every session runs with `-hls_flags temp_file` (merged into the same single `-hls_flags` value as the conditional `independent_segments` — FFmpeg reads only one occurrence): the muxer writes each segment to `segment_N.m4s.tmp` and renames it on close, so a segment whose **final name** exists non-empty is complete (`segmentReady`, `server/cmd/api/hls_handler.go`). `.tmp` names are rejected by the segment-filename validator, so a partially-written file is unreachable.

`init.mp4` is the exception: the hls muxer opens it under its final name directly, with no rename (verified by strace against the embedded build), so existence does not prove it was closed. It is ready once it is non-empty **and** FFmpeg has moved past it — evidenced by `segment_0.m4s` under either its temp or final name, which the muxer only opens after closing the init file — or once FFmpeg has exited, since nothing can be appended to a dead session's output. **The exit half is load-bearing**: without it a session that dies between writing `init.mp4` and opening `segment_0` waits out the full deadline for a file that is already on disk and final.

The `temp_file` behavior is capability-probed at startup (`ffmpeg -h muxer=hls`). A swapped `IGLOO_FFMPEG_PATH` binary whose hls muxer lacks the flag falls back to the legacy successor-file heuristic (`segmentComplete`): a file is complete when the file FFmpeg writes after it exists, or FFmpeg has exited and the file itself exists — at the cost of one extra encoded segment of startup latency.

This design prevents browsers from reading partially written `.m4s` files, which cause decode errors, retry loops, or broken playback state. Segment requests wait up to `hlsSegmentWait` and poll every `hlsSegmentPoll`. If FFmpeg exits with an error before a requested segment exists, Igloo returns a transcode failure instead of hanging. A segment that is merely not encoded yet when the wait expires returns `503` with a `Retry-After`; the web client grants those a bounded number of fresh load attempts before reporting. Its fragment timeout must stay above `hlsSegmentWait` — set equal, the two race and neither outcome is recoverable.

The poll interval is deliberately short. A segment that lands just after a check waits a full interval before it is served, and that wait sits directly on the startup and post-seek path; the readiness check itself is one or two stats against page-cached directory entries, so polling tightly costs far less than the latency it removes. The wait also ends as soon as the request context is cancelled — a seek abandons the in-flight segment request, and without that the goroutine would keep polling for the full `hlsSegmentWait`, so scrubbing would accumulate them.

Segments and whole media files are served through the kernel's `sendfile(2)` path. The session middleware wraps the response writer in a type that does not implement `io.ReaderFrom`, which would silently force every byte through a userspace copy, so `restoreSendfile` re-exposes the capability for the whole router. Once an HLS file is ready, the handler opens and stats it before calling `http.ServeContent`; the open descriptor pins the file across concurrent session cleanup, while an open/stat race is logged with its internal details and returned as a controlled, sanitized JSON 500.

Initialization files and media segments for both personal and watch-room HLS sessions support HTTP conditionals and byte ranges. Ready assets publish `Accept-Ranges` and `Last-Modified`; an applicable `If-Modified-Since` returns `304`. A complete asset response uses `200 OK`; one satisfiable range uses `206 Partial Content` with `Content-Range`; multiple satisfiable ranges use a `multipart/byteranges` 206 body. Malformed and unsatisfiable ranges return Go's plain-text `416 Requested Range Not Satisfiable` body rather than the JSON envelope, with `Content-Range` present only when the asset size applies.

Status classification is deliberate for native clients. Missing movies or sessions return 404, invalid audio selections return 400, and unusable stored media metadata returns 422. A missing requested Dolby encoder and unexpected database, filesystem, or FFmpeg failures return sanitized 500 responses while their detailed causes remain in server logs. Only capacity, transcode-storage pressure, playlist-not-ready, and segment-not-ready failures return 503; every HLS 503 includes `Retry-After`.

FFmpeg stderr is not streamed to clients. The HLS runner keeps the last 20 stderr lines and passes them to the session exit handler for logging. That gives enough context for server-side troubleshooting without storing unbounded FFmpeg output.

## Seeking and Resume Behavior

The HLS manifest accepts a `start` query parameter. When `start` is greater than zero, FFmpeg starts from that source offset with `-ss`, placed before `-i`. Input seeking lands on the source keyframe at or before the requested time. When re-encoding, FFmpeg then discards frames up to the requested offset, so a transcode starts exactly where it was asked to; stream copy cannot discard frames, so a copy-video session really begins at that earlier keyframe — by up to a full GOP. The output is rebased to start at zero either way by `-avoid_negative_ts make_zero`.

For copy-video sessions Igloo resolves where the media actually begins and publishes it as the `X-Igloo-Actual-Start` response header on the manifest. The client maps session time back to absolute movie time from that value, so the displayed clock and saved watch progress follow the picture rather than the request.

The primary source is a **persisted keyframe index** (`keyframe_indexes` table), extracted once per file from the container's own seek tables — Matroska Cues for mkv/webm, the stts/ctts/stss sample tables (with single-edit `elst` handling) for mp4/m4v/mov — by the `keyframeindex` package. FFmpeg's `-ss` input seek consults these same structures, so the index answer matches where FFmpeg actually lands by construction. Extraction reads only index data (a few bounded reads, never the media), runs in the background on the first copy-video session of a file (including sessions starting at 0, as a prefetch), and is keyed like `remux_safety_verdicts`: movie + stream with a file-identity fingerprint, so a rescan or file change invalidates it. With a persisted index a seek is answered synchronously by binary search, so the header arrives on the first manifest response and is exact regardless of GOP length.

Files without a usable index — avi, or containers whose seek tables are missing or unsupported — fall back to the previous behavior: a bounded ffprobe keyframe lookup (30-second lookback) that runs alongside FFmpeg so it adds no startup latency. Its single-point answer is never persisted. The whole resolution is advisory: on failure the header is omitted and the client falls back to the requested start.

The manifest also carries `X-Igloo-Effective-Profile`, naming the profile FFmpeg actually ran. A `remux` request that fails the safety gate is still served from the `/hls/remux/` path, so without this the client cannot tell a stream copy from the full transcode it was silently given.

Subtitles are rebased to match. The WebVTT endpoint stores cues with absolute source timestamps, and its `start` query parameter shifts them onto the requesting session's timeline when it is served. The cache stays keyed on movie and stream index alone, so shifting costs no extra extraction and no extra cache entries. A raw API start offset at or past the movie's duration — stale saved progress after a re-scan, or rounding at the very end — is clamped to five seconds before the end instead of failing (or to zero when the movie is shorter than five seconds). That effective start drives the singleflight/cache key, FFmpeg parameters, generated asset URLs, and subsequent segment lookup.

When the web client knows the movie duration, it first clamps the requested absolute playback target to that duration. For HLS it then applies the 10-second resume rewind to the clamped target and uses the result consistently for the manifest URL, session-window key, local playback offset, and absolute-time mapping. Direct playback initialization uses the same clamped absolute target.

Igloo exposes the rebased session as a VOD playlist. The files on disk start at `segment_0.m4s`, but the UI keeps absolute movie time. When a seek requires a different offset, the client asks for a manifest with a new `start` value and Igloo creates a new session.

A rebased session's playlist covers only the remaining time from the start offset to the end of the movie. While FFmpeg is still encoding, Igloo generates the VOD playlist from that remaining duration; once it exits, the finalized playlist with accurate segment durations is served with only its asset URLs rewritten. The client maps session-local playback time back to absolute movie time in the UI.

## Watch Rooms

Watch room HLS uses the same FFmpeg session machinery with room-specific cache keys:

```text
room:<room_id>
```

A room stores its audio track when it is created, so the value is validated up front rather than at first playback. Room creation rejects an `audio_track` beyond the movie's audio stream count, a non-zero `audio_track` on a movie without audio, and a non-zero `audio_track` combined with direct playback, which would serve the container's first track to every member regardless of the stored value.

Room sessions are isolated from personal playback sessions so a watch room cannot collide with a user's individual HLS session for the same movie. Watch rooms warm up HLS from the beginning so participants can join a prepared stream. When a room is explicitly deleted, Igloo deletes the database row and immediately advances the authorization-cache generation and removes every cached member authorization for that room. Only then does it mark the HLS room session deleted, remove the cached session, kill FFmpeg if it is still running, remove the temp directory, and close room WebSockets. A membership lookup that began before deletion may finish its already-authorized request, but its older generation cannot publish a late cache fill that restores direct-play or HLS access after deletion.

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

The endpoint accepts an optional `start` query parameter giving the HLS session offset the cues will be played against. Cues are extracted and cached with absolute source timestamps; `start` shifts them at serve time so they line up with a rebased session's media timeline. Cues ending before the session start are dropped and a cue straddling it is clamped to zero. Direct play and sessions starting at zero omit the parameter and get the absolute cues unchanged.

Bitmap subtitle codecs are rejected before FFmpeg runs:

- `hdmv_pgs_subtitle`
- `dvd_subtitle`
- `dvb_subtitle`

These codecs are image-based and cannot be reliably converted to WebVTT text. Rejecting them explicitly gives clients a clear unsupported-media response instead of a confusing conversion failure.

After conversion, Igloo replaces escaped `\h` sequences with spaces. This handles subtitle text that uses hard-space style escapes not wanted in WebVTT output.

## Operational Notes

For binary deployments:

- `TRANSCODE_DIR` seeds the Settings transcode directory on first launch; after that, edit it from Settings.
- HLS temp output is written below the Settings transcode directory. A session generates the whole remaining movie ahead of the playhead, so Igloo refuses to start one when that filesystem has less than 2 GB free, returning `503` rather than failing mid-stream. A filesystem it cannot measure is not treated as full.
- `HLS_MAX_CPU_TRANSCODES` is read at startup and limits concurrent HLS sessions that encode video or audio. Copy-video remux sessions count when they encode audio and bypass the limit only when audio is copied or absent. It is not stored in Settings.
- `HLS_MAX_SESSIONS_PER_USER` is read at startup and limits cached plus in-flight personal HLS sessions per user; remux and transcode sessions are both counted. The default is 3. It is not stored in Settings.
- Configured media directories should be readable by the Igloo process. Igloo does not need write access to media libraries.

Build tags, `make` targets, and the environment variables above are documented in `CLAUDE.md`, `README.md`, and `.env.example`; the only FFmpeg-specific note is that every `externalbin` build — `make dev` and all backend tests — needs each binary on `PATH` or supplied through its `IGLOO_FFMPEG_PATH` / `IGLOO_FFPROBE_PATH` override.

For failures:

- ffprobe failure during scanning means the item cannot be indexed reliably.
- FFmpeg HLS startup failure is returned from the manifest request.
- FFmpeg runtime failure is logged with the stderr tail.
- Segment requests fail with "segment not ready", "segment does not exist", or "transcoding stopped" depending on session state.
- Subtitle extraction failures are logged server-side and returned to clients as a generic extraction failure.

## Maintenance Rules

When changing FFmpeg or ffprobe behavior:

- Check the embedded payload version with `ffmpeg -version` and `ffprobe -version` after refreshing binaries. Prefer the current stable Jellyfin FFmpeg release line for release payloads; do not switch to a generic upstream FFmpeg build or Jellyfin prerelease branch without a specific reason.
- Keep argument construction covered by the tests in `server/cmd/internal/ffmpeg/` (`ffmpeg_hls_args_test.go`, `ffmpeg_hls_hardware_args_test.go`, `ffmpeg_hls_run_test.go`).
- Keep remux validation covered by `remux_validator` tests when changing fMP4 safety behavior.
- Keep HLS handler and playlist tests updated when changing playlist shape, filenames, query parameters, readiness rules, or resume behavior.
- Update `docs/openapi.json` when adding or changing HLS, subtitle, or playback settings endpoints.
- Update `.env.example`, settings validation, README hardware notes, and this document when adding a hardware acceleration device.
- Update `hls_profiles.go`, playback settings responses, OpenAPI schemas, frontend profile lists, and this document when adding or changing an HLS profile.
- Do not add new FFmpeg command-line options only in handlers. Keep FFmpeg argument construction centralized in the internal FFmpeg wrapper so tests can validate the full command.
- Treat browser compatibility as a product requirement. Prefer explicit fallback to a known playable profile over exposing a stream that might work on one browser and fail on another.

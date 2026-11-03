# FFmpeg and ffprobe in Igloo

This is the playback reference for clients and server contributors. Igloo uses ffprobe to persist technical metadata, FFmpeg to produce HLS and subtitle output, and HTTP file serving for direct playback. HLS transcoding and decoding remain works in progress; a configured encoder or a passing unit test does not establish playback support on every device.

The [OpenAPI contract](openapi.json) owns endpoint schemas, authentication, parameters, and response types. This guide explains how those endpoints work together. [Library scanning](library-scanning.md) covers catalog enrichment, provider matching, file management, the watcher, and artwork. Sections explicitly labeled **web client** describe the included React player's policies, not requirements imposed on other clients.

- [Binary strategy](#binary-strategy)
- [Probed metadata and cache invalidation](#probed-metadata-and-cache-invalidation)
- [Media identity and requests](#media-identity-and-requests)
- [Direct playback](#direct-playback)
- [Profiles and remux fallback](#profiles-and-remux-fallback)
- [Audio selection and encoding](#audio-selection-and-encoding)
- [Session lifecycle](#session-lifecycle)
- [HLS output and video processing](#hls-output-and-video-processing)
- [Hardware acceleration and HDR](#hardware-acceleration-and-hdr)
- [Timeline, seeking, and resume](#timeline-seeking-and-resume)
- [Readiness, HTTP responses, and recovery](#readiness-http-responses-and-recovery)
- [Subtitles](#subtitles)
- [Operational settings and budgets](#operational-settings-and-budgets)
- [Verification and maintenance](#verification-and-maintenance)

## Binary strategy

Production builds embed zstd-compressed Jellyfin FFmpeg and ffprobe payloads for Linux x64 or macOS ARM64. Refresh the pair from the same Jellyfin release; arbitrary system builds are not equivalent substitutes. The platform files are `server/cmd/internal/ffmpeg/ffmpeg_<platform>` and `server/cmd/internal/ffprobe/ffprobe_<platform>`, with `linux_amd64` or `darwin_arm64` as the suffix. `make build` checks the native payloads and regenerates their `.zst` files when needed. See [building from source](../README.md#build-a-production-binary).

During this audit, both Linux executables ran `-version` and reported `8.1.3-Jellyfin`. Both macOS files were identified as ARM64 Mach-O executables and contained `8.1.3-Jellyfin` strings; that was static inspection, not a macOS runtime check. These observations identify the available local payloads, not their download archive provenance or the latest upstream release.

[`mediabin`](../server/cmd/internal/mediabin/mediabin.go) extracts embedded binaries into `os.UserCacheDir()/igloo/bin/<binary>-<hash>/`, using the first 16 hexadecimal characters of the compressed payload's SHA-256. Reuse checks the executable against its `.sha256` marker. A newly extracted cache entry prunes older entries for that binary. Cache failure falls back to a randomized operating-system temporary directory. Cleanup removes fallback directories and resets the wrapper singletons; cached executables remain.

Development and tests use `externalbin`, resolving an explicit override or a command on `PATH`; see the [settings table](#operational-settings-and-budgets). An invalid override fails validation without falling back. Embedded builds ignore overrides. Make's development/test prerequisites independently require `ffmpeg` and `ffprobe` on `PATH` even when overrides are supplied.

Both wrappers verify executability with `-version` before accepting their singleton. FFmpeg then inventories encoders, filters, hardware methods, selected CLI/filter/encoder options, and the HLS muxer's `temp_file` flag. Selected hardware paths also receive runtime probes, as described under [hardware acceleration](#hardware-acceleration-and-hdr). Startup does not prove every codec, decoder, filter, or source combination usable.

## Probed metadata and cache invalidation

The [ffprobe metadata reader](../server/cmd/internal/ffprobe/ffprobe_metadata.go) runs the following for movies and TV files:

```text
ffprobe -v quiet -print_format json -show_streams -show_format -show_chapters <file>
```

Music uses `GetAudioMetadata` with a restricted `-show_entries` list. Both calls inherit scan cancellation and have the probe budget in the [settings table](#operational-settings-and-budgets). Caller cancellation and probe timeout are reported separately. Results without streams are rejected; music additionally requires an audio stream and uses the first one. Embedded artwork alongside audio is accepted.

Movie and TV imports require accepted moving video. [`ClassifyStreams`](../server/cmd/internal/scanner/streams.go) excludes `attached_pic` streams and PNG/GIF/BMP video; moving MJPEG is accepted. Persisted indices remain the **absolute ffprobe stream indices**, even when audio or excluded artwork precedes video. The [schema](../server/sqlc/schema.sql) defines the complete stored fields: container and duration, chapters, video dimensions/profile/level/pixel format/bit depth/frame rates/color/field order/rotation, audio channels/layout/sample rate, and track language/title/dispositions.

Rotation comes from display-matrix side data: an explicit zero is distinct from a missing matrix (`NULL`). Subtitle `is_forced` comes from the forced disposition or from a title that names the track as forced: “forced” as a whole word, case-insensitive (“Forced”, “English (Forced)”, “ENG.FORCED”), unless the word just before it is “non”, “not”, or “no”. Many releases mark forced tracks only in the title, and clients' default subtitle rules read `is_forced`. Format and stream tag aliases are selected deterministically: canonical spelling, case-insensitive spelling, then equivalent keys with separators removed, with lexical tie-breaking. Distinct aliases retain precedence (`language` before `lang`, for example); empty and non-string values are ignored. Music language uses the first audio stream, falling back to the format tag when absent or `und`; explicit `zxx` remains meaningful.

HLS planning reads persisted stream metadata rather than probing the whole file on every request. Playback-specific probes still exist for [keyframes](#timeline-seeking-and-resume) and [cached subtitle conversion](#webvtt). Technical-details responses expose filenames and technical fields without absolute library paths. Chapters are normalized against file duration.

Committed technical file changes invalidate runtime playback data: direct-stream lookups, stored-stream caches, HLS sessions, WebVTT output, and subtitle track copies; technical replacement also removes persisted keyframe indexes and remux verdicts. Invalidation is file-scoped for combined episodes. WebVTT generations are captured before source metadata is read; invalidation prevents older extractions from repopulating the cache or sharing work with newer requests. Unchanged files retain caches, and metadata-only provider retries do not rebuild technical rows or invalidate playback. Persisted keyframe/remux entries are additionally checked against fingerprints containing file identity, size, update timestamp, and stream index; remux fingerprints also include relevant stream properties and producer identity. See [file-change detection](library-scanning.md#file-change-detection) for what constitutes a technical change.

## Media identity and requests

Use `/api/movies/{id}` for a movie or `/api/shows/episodes/{id}` for an episode. Both expose technical details, direct streaming, personal HLS, subtitles, and watch progress through the same playback implementation. Trailer pre-roll and watch rooms are movie-only.

An episode ID identifies the logical episode in URLs, personal HLS keys, and per-user progress. Its playback source is the linked `show_files` row with the lowest file ID when duplicate copies exist. A combined file can back multiple episodes: each episode addresses the **whole file**, with no inferred episode boundaries or offsets. The episodes retain independent progress. Keyframe indexes, remux verdicts, and subtitle caches share work by physical file; media kind separates movie IDs from show-file IDs. The episode detail's `next_episode` skips metadata-only episodes and episodes whose selected playback file is the file just played, follows listing order with specials last, and includes the requesting user's progress.

A personal HLS request, for a file with audio, looks like:

```text
GET /api/movies/42/hls/remux/playlist.m3u8?playback_session=550e8400-e29b-41d4-a716-446655440000&start=120&audio_track=0
```

`playback_session` must be a UUID. `start` is required, in nonnegative whole seconds. `audio_track` is required when audio exists and must be absent for video-only media. [Audio](#audio-selection-and-encoding) defines track ordinals and the optional explicit profile pair. `reload` is an opaque value echoed into asset URLs, not a cache-key component. `keepalive=1` has the refresh-only meaning defined under [session lifecycle](#session-lifecycle).

Authenticate every request with a session cookie or device bearer token, including manifests, initialization files, segments, and subtitles. Playlist URLs contain no credentials. Follow the rewritten asset URLs exactly: they carry the normalized session start, UUID, selected audio, explicit audio profile when present, and reload value. An effective profile or actual-start header does **not** change the URL key.

Watch-room URLs use `/api/watch-rooms/{id}/hls/playlist.m3u8`, `/hls/{filename}`, or `/stream`. The room owns its mode and selected tracks. Membership is checked on requests, with successful results cached for the duration in the [settings table](#operational-settings-and-budgets); an unknown room and a nonmember both receive `403`. Room asset lookup uses the room ID and ignores query parameters. Audio/subtitle pins retain stream index and optional language: drift after a rescan returns `409` on the applicable manifest/direct-stream checks and requires recreating the room with current tracks. Asset requests do not perform those pin checks. See OpenAPI for the precise per-route rules.

## Direct playback

The original-file `/stream` endpoints do not start FFmpeg, select tracks, enforce browser codec eligibility, or transcode. Clients decide whether their player can use the original file. Range and conditional behavior is documented under [HTTP responses](#http-responses).

**Web client:** [`getAvailableModes`](../web/src/lib/playback.ts) permits direct playback only after these checks:

- MP4 MIME type (`video/mp4`, including scanned `mp4`/`m4v`), using the project's deterministic container-to-MIME mapping.
- An accepted H.264 video stream passing the [video safety gate](#profiles-and-remux-fallback).
- First audio codec in `aac`, `mp3`, `opus`, `vorbis`, or `flac`, when audio exists. This allowlist is a policy, not a universal browser-support guarantee.
- Unambiguous audio defaults: with multiple streams, no default flags or exactly one on the first stream. A non-first default or multiple defaults prevents direct playback.
- A nonempty `canPlayType` result for a codec string derived from the stored metadata. This probe may narrow, but never expand, the static eligibility rules.

The web player waits for technical details before requesting media. Failed details show a retryable UI error. A decode error, unsupported-source error, or metadata-load timeout triggers one switch to remux per stream window, preserving position and selected tracks. Selecting non-first audio also requires HLS; see [audio selection](#audio-selection-and-encoding).

Watch-room creation enforces the container, video, first-track selection, and disposition rules, but currently **does not check the first audio codec**. It cannot run members' browser probes either. A room accepted by the server therefore does not prove direct-play eligibility in the web client. This implementation gap is recorded in the [audit findings](ffmpeg-audit.md#unresolved-findings).

## Profiles and remux fallback

[`hls_profiles.go`](../server/cmd/internal/helpers/hls_profiles.go) owns the profile ladder. Each request selects one rendition; the server does not provide an adaptive master playlist.

| Profile | Video bitrate / maxrate | Buffer | Target height |
| --- | ---: | ---: | ---: |
| `2160p_16mbps` | `16M` | `32M` | 2160 |
| `1080p_8mbps` | `8M` | `16M` | 1080 |
| `1080p_6mbps` | `6M` | `12M` | 1080 |
| `1080p_4mbps` | `4M` | `8M` | 1080 |
| `720p_3mbps` | `3M` | `6M` | 720 |
| `remux` | Source video copied | — | Source height |

The remux video gate accepts `h264`, `h.264`, `avc`, or `avc1` and rejects any of:

- Stored bit depth above 8.
- A nonempty pixel format outside `yuv420p`, `yuvj420p`, `nv12`, `nv21`.
- A codec profile containing `10`, `4:2:2`, `422`, `4:4:4`, or `444`.
- Interlaced field order `tt`, `bb`, `tb`, or `bt`.

Absent pixel format, bit depth, or field order does not itself reject the source. Unknown field order is treated as progressive. The server and web client keep separate copies of this gate; changes must keep them aligned. Audio processing is independent of video copy/transcode and is defined [below](#audio-selection-and-encoding).

Eligible remux output undergoes fMP4 preflight unless a matching persisted verdict exists. The validator waits for initialization and the sample of complete fragments specified in the [settings table](#operational-settings-and-budgets), then checks that video sync samples begin with IDR frames. A clean early exit with at least one complete fragment can validate a shorter sample: a safe short sample applies only to that session; an unsafe result is persisted. A full sample can be validated even if FFmpeg subsequently fails. A timeout, empty clean exit, or premature failed exit falls back without storing an unsafe verdict.

Verdicts are keyed by file and stream, with the fingerprint described under [cache invalidation](#probed-metadata-and-cache-invalidation). Producer identity includes the parsed FFmpeg version and `remuxVerdictProducerRevision`, which must change when relevant arguments or validation change. The sampled fragments start at the session's offset. A safe persisted sample does not prove all later GOPs safe; copy-video playlists never advertise independent segments.

When the gate or preflight rejects remux, fallback selection proceeds as follows:

1. Pick the tallest configured height no greater than the source height; use the lowest profile if the source is shorter than the ladder.
2. At that height, select the highest bitrate no greater than the stored video bitrate. Unknown bitrate uses the richest profile; a bitrate below all entries uses the cheapest at that height. Missing video bitrate falls back to `file size × 8 / duration`, an estimate that includes audio and container overhead.
3. Apply the effective device height cap and configured upload-bandwidth headroom from the [settings table](#operational-settings-and-budgets). If nothing fits, use the lowest profile; this is a minimum, not a guarantee of fitting a very slow link.

Explicit transcode requests receive the height cap only. Fallback keeps the original `/hls/remux/` URL and requested audio profile; `X-Igloo-Effective-Profile` identifies the profile actually run. A capacity retry reuses the planned fallback without repeating preflight within that request.

## Audio selection and encoding

`audio_track` is the zero-based **ordinal** in the audio rows ordered by `stream_index`, not an absolute ffprobe index. Subtitle `trackIndex` follows the corresponding subtitle ordering. FFmpeg receives the resolved absolute indices:

```text
-map 0:<video_stream_index> -map 0:<audio_stream_index>
```

Video-only sources omit the audio map and all audio codec options. The web client resolves any non-first audio selection in direct mode to remux, whether that selection came from a preference, saved settings, or a URL. A transcode selection keeps its mode. Raw file serving itself cannot enforce a player's track choice.

### Default audio mode

With neither explicit audio parameter, a selected AAC stream whose scanned profile is confirmed as LC is copied (`-c:a copy`), retaining its channels. Every other codec, non-LC AAC profile, and unknown AAC profile is encoded with `-c:a aac -ac 2 -b:a 320k`. No output sample rate is explicitly set in this mode. Copied multichannel AAC still requires a capable client/output device.

This is active default behavior, including watch-room HLS. Its internal key/log token `legacy` does not mean deprecated.

### Explicit audio profiles

Personal movie and episode HLS accept, together:

```text
audio_codec=ac3&audio_channels=6
```

Allowed codecs are `ac3` and `eac3`; allowed channel ceilings are 2 and 6. Supplying only one parameter, an invalid value, or a pair on video-only media returns `400`. `aac` is not an explicit option. Explicit mode always encodes, even when the input is AAC-LC or already the requested Dolby codec.

The server resolves output channels to `min(source channels, requested ceiling)` and encodes at 48 kHz. A missing/nonpositive stored channel count returns `422`. A missing requested encoder in the startup capability inventory returns `500` before allocating the output directory or a video-transcode permit. Encoder presence is not a per-source runtime test.

| Codec | Mono | Stereo | 3–4 channels | 5–6 channels |
| --- | ---: | ---: | ---: | ---: |
| AC-3 | 192k | 384k | 448k | 640k |
| E-AC-3 | 192k | 384k | 512k | 768k |

The resolver retains or derives a layout name and supplies it to `aresample` as `out_chlayout` and to the encoder as `-ch_layout:a`. Explicit encoding sets `lfe_mix_level=0.70710678` and `rematrix_maxval=1`: stereo downmixes include center, surround, and LFE content, with a normalized mixing matrix to avoid clipping. Sources within the requested ceiling retain their channel count, and their layout when AC-3/E-AC-3 support it (`mono`, `stereo`, `2.1`, `3.0`, `3.0(back)`, `3.1`, `4.0`, `quad`, `quad(side)`, `4.1`, `5.0`, `5.0(side)`, `5.1`, `5.1(side)`); another layout (`6.0`, `hexagonal`), a missing one, or one whose channel count disagrees with the stored count is remixed to the standard layout for the count. Sources above the ceiling are mixed to stereo or 5.1. The encoder is never given `-ac`: with a channel count alone it picks its own layout for that count, which turned `2.1`, `3.1`, and `4.1` sources into layouts without LFE. Encoders can normalize layout labels (for example, `5.1` to `5.1(side)`), so exact source layout labels are not guaranteed. Default AAC behavior is unchanged.

The normalized codec/ceiling pair participates in the personal session key and every rewritten asset URL, including after video fallback. Successful manifest headers describe the session's audio: `X-Igloo-Effective-Audio-Codec`, `-Channels`, and `-Bitrate`. Copied AAC uses stored source values; unavailable channel count or bitrate is omitted. Video-only sessions omit the audio headers. These are diagnostics; inspect the media for decoding, and do not expect a `CODECS` attribute or an audio rendition group in the media playlist.

## Session lifecycle

[`hls_session.go`](../server/cmd/api/hls_session.go) owns HLS processes and output directories independently of the manifest request. A personal cache key includes authenticated owner, media kind/ID, requested profile, audio ordinal/mode, playback UUID, and effective normalized start. Concurrent identical creations share work only within that owner. If the initiating request is canceled, a live follower retries creation under its own context, with a bounded attempt count in the [settings table](#operational-settings-and-budgets).

Admission reserves a personal slot before starting FFmpeg; cached and in-flight sessions both count, including remux. At the cap, the owner's cached personal sessions are evicted in LRU order. If reservations occupy every slot, admission returns the capacity response described under [HTTP responses](#http-responses). A successful creation exchanges its reservation for a cache entry; failure releases it. Rooms and other owners are never personal-cap eviction candidates.

After admission, creation retires superseded windows for the same owner, media, and playback UUID across profile, audio, and start changes. This happens **before planning**: a replacement that then fails can already have ended the previous window. FFmpeg is stopped synchronously to release its permit; output deletion normally runs in the background, but stays synchronous under storage pressure.

### Capacity and storage

Video encoders determine the limiter pool: `libx264` uses CPU capacity; hardware encoders use hardware capacity. Copy-video sessions hold neither permit, even when converting audio. A hardware encoder using software filters still counts against the hardware pool and can impose substantial CPU load. Permits are released when FFmpeg exits, while completed output can remain cached.

A personal request denied an encode permit can reclaim its owner's idle, still-running session in the same pool, then wait for a permit using the budget in the [settings table](#operational-settings-and-budgets). Rooms, other users, completed encodes, copy-video sessions, and fresh sessions are excluded from this reclaim. Room creation/warm-up never waits for a permit; failed warm-up removes the newly created room and returns the relevant error.

New HLS output requires the free-space floor in the settings table. Failure to measure free space is not treated as a full disk. This is an admission check, not a reservation or a continuous guarantee: a session produces the remaining file, so output can still exhaust storage later. Each session has an `igloo-hls-*` directory under the stored transcode directory; startup removes abandoned HLS directories.

### Refresh, failure, and stop

Manifest and asset requests refresh the idle TTL. A personal manifest with `keepalive=1` refreshes an existing healthy session and returns its playlist, but never creates one; a missing or failed session returns `404`. An ordinary manifest replaces a failed FFmpeg session. Assets never create sessions and can still serve completed output from a failed process. Client retry budgets must bound repeated failures at the same source position.

`POST /api/movies/{id}/hls/session/stop?playback_session=<uuid>` (or the episode equivalent) stops that owner's cached sessions for this media and UUID. It returns success even when none exist. **An in-flight creation can publish after stop and remain until idle expiry**; stop is not a cancellation barrier. Allocate a fresh UUID for each independent playback instance so a late stop cannot target a newly opened player.

Room sessions use `room:<id>`, always start at zero, and warm up at room creation. Any member's requests refresh them; a later manifest recreates an expired or failed session from zero. Room deletion invalidates authorization caches, blocks late session publication with a tombstone, tears down output, and closes room WebSockets. Scanner deletion of the movie performs equivalent teardown after commit. Ordinary technical rescans invalidate output without deleting rooms. Deleting a user stops that user's personal sessions.

**Web client:** each player mount generates a fresh UUID without persistent storage. While HLS is ready and the player is rendered, a timer uses the keepalive interval in the settings table. Personal pings are refresh-only; room pings are ordinary manifests. The timer pauses during capacity recovery, skips overlapping pings, discards response bodies, and aborts on unmount. Room recovery reloads the stream and reapplies the latest room position/play state after loading.

### Shutdown and process ownership

Shutdown first prevents HLS starts/publication, cancels subtitle work, empties the HLS cache, and stops sessions concurrently. Waiting requests see the lost-session or unavailable responses described [below](#http-responses). HTTP connections then receive the grace period in the settings table, after which remaining streams are closed. Main waits for teardown rather than exiting when the listener closes.

FFmpeg children have separate process groups. Linux adds a parent-death `SIGKILL`; macOS has no equivalent here, so an abrupt server death can leave FFmpeg running until it finishes or fails. Embedded-binary cleanup follows the [binary strategy](#binary-strategy).

## HLS output and video processing

[`ffmpeg_hls.go`](../server/cmd/internal/ffmpeg/ffmpeg_hls.go) centralizes arguments. HLS uses fragmented MP4 with `init.mp4`, `segment_N.m4s` starting at zero, and `playlist.m3u8`. The muxer receives `-f hls -hls_segment_type fmp4 -hls_playlist_type event -hls_list_size 0`, `movflags=+frag_discont` as segment options, absolute output paths, and the segment target in the [settings table](#operational-settings-and-budgets). Applicable `temp_file` and `independent_segments` flags are combined in one `-hls_flags` value.

| Session state | Served playlist |
| --- | --- |
| Running video transcode | Synthesized VOD playlist for the remaining duration, allowing requests anywhere in the window before all output exists |
| Running copy-video | FFmpeg's EVENT playlist with rewritten asset URLs and `#EXT-X-START:TIME-OFFSET=0`; only produced segments are listed |
| Exited process with playable output | Finalized FFmpeg playlist, converted to VOD with `#EXT-X-ENDLIST`, even after a failed exit |

The synthesized count uses the last video frame predicted by `HLSSegmentCount`, not simply `ceil(duration / segment time)`. It uses the lower available nominal/average rate, or the fallback rate in the settings table. Segment declarations use the target duration except for the final remainder. This prediction can differ from unusual source timing; the [past-end response](#http-responses) distinguishes a nonexistent trailing segment from a lost session. A finalized playlist without playable segments is not served as a successful complete asset.

Only transcodes whose encoder is considered capable of forced IDRs advertise `#EXT-X-INDEPENDENT-SEGMENTS`. `HLSSegmentsAreIndependent` controls both FFmpeg and synthesized playlists. CPU and VideoToolbox paths qualify; NVENC/QSV require their probed forced-IDR options. Copy-video never qualifies, for the sampling limitation described under [remux fallback](#profiles-and-remux-fallback).

Video transcodes use H.264 High profile, the selected bitrate/maxrate/buffer, aspect-preserving scaling to the target height with width `-2`, and BT.709 output color tags. CPU encoding uses `libx264 -preset veryfast -sc_threshold:v:0 0`. Known frame rate supplies `-g:v:0` and `-keyint_min:v:0` as the ceiling of frame rate times segment target; every encoder also receives `-force_key_frames:0 expr:gte(t,n_forced*4)`. Frame timestamps still determine actual muxed boundaries.

Common arguments are `-nostdin -nostats -y -fflags +genpts`, bounded input analysis (`-analyzeduration 5000000 -probesize 5000000`), metadata/chapter removal, `-avoid_negative_ts make_zero`, and `-max_muxing_queue_size 1024`. Read-rate options are included only when probed available, with values in the settings table. No per-process `-threads` limit is set.

Interlaced video prepends software `yadif` in its default frame-preserving mode before scaling. Rotation relies on FFmpeg autorotation during transcode; copy paths leave source rotation metadata intact. A difference above the configured tolerance between nominal and average frame rates is logged as VFR; no `fps` filter is applied.

For SDR tagged `smpte170m`/`bt470bg`, or untagged sources narrower than 1280 and no taller than 576, the software scale converts the BT.601 matrix to BT.709. CUDA/QSV scaling paths perform that conversion before upload. This converts the matrix, not color primaries. Merely setting BT.709 metadata is not a pixel conversion.

## Hardware acceleration and HDR

The stored device setting and effective device can differ. [`capabilities.go`](../server/cmd/internal/ffmpeg/capabilities.go) resolves unsupported or unusable selections to CPU before encoding. `/api/settings/playback` reports the configured/effective devices, fallback reason, and height cap. Drivers, device access, and source decoding still affect runtime success.

| Device | Decode request | Video encoder and checks | Scaling/filter path |
| --- | --- | --- | --- |
| `cpu` | Software | `libx264` | Software |
| `apple` | `-hwaccel videotoolbox` | `h264_videotoolbox` must be listed; no general encode runtime probe | Software SDR; progressive HDR can use runtime-probed `scale_vt` |
| `nvidia` | `-hwaccel cuda` when listed | `h264_nvenc` must pass a runtime encode probe | Software unless CUDA scale/tone-map chains also pass their probes |
| `intel` | Software; no QSV decode flag | `h264_qsv` must pass a runtime encode probe | Software, or runtime-probed `scale_qsv` for SDR |

NVENC receives `-rc vbr -preset p4` and, when exposed, `-forced-idr 1`. QSV receives `-preset veryfast`, `-look_ahead 1`, and `-forced_idr 1` only for options exposed by its encoder help. Device/filter probes use synthetic frames, not an exhaustive source-codec test. No path sets `-hwaccel_output_format`; filter chains operate on system-memory frames or explicitly upload them.

The software SDR chain is `scale=-2:<height>,format=<pixel format>,setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709`. The pixel format is `nv12` for Intel and `yuv420p` otherwise. CUDA SDR uses `format=nv12,setparams=...,hwupload,scale_cuda=w=-2:h=<height>:format=yuv420p`; QSV SDR uses `format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=<height>:format=nv12`. Hardware upload chains initialize a named filter device. The output `-pix_fmt yuv420p` is omitted for QSV and CUDA/VideoToolbox hardware-frame chains, which choose format in the filter graph.

HDR detection uses stored `color_transfer`: `smpte2084` (PQ) or `arib-std-b67` (HLG). Copy-video does not tone-map. Transcoding HDR to the SDR profiles normally uses:

```text
zscale=w=-2:h=<height>:t=linear:npl=100,format=gbrpf32le,
zscale=p=bt709,tonemap=tonemap=hable:desat=0,
zscale=t=bt709:m=bt709:r=tv,format=<pixel format>,
setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709
```

With usable CUDA filters/options and successful scale and tone-map probes, NVIDIA instead uses:

```text
format=p010le,hwupload,scale_cuda=w=-2:h=<height>:format=p010,
tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0
```

Progressive Apple HDR can use the runtime-probed chain:

```text
format=p010le,hwupload,
scale_vt=w=-2:h=<height>:color_matrix=bt709:color_primaries=bt709:color_transfer=bt709
```

Interlaced Apple HDR uses software tone mapping because the probe covers only the progressive chain. Intel HDR also stays on software filters. A failed hardware filter probe can retain hardware encoding with software filtering; it does not necessarily select CPU encoding. Tone-map tuning is not user-configurable.

## Timeline, seeking, and resume

Keep three values separate:

| Value | Meaning and use |
| --- | --- |
| Requested `start` | Nonnegative integer source offset supplied by the client |
| Effective normalized start | Server clamps a request at/past duration to the tail offset in the settings table, or zero for shorter media; used for the session key, FFmpeg input seek, and rewritten asset URLs |
| Advisory actual start | `X-Igloo-Actual-Start`, when known; used to map local media time and subtitle timing back to source time, not to replace the session URL key |

For positive starts, `-ss` precedes `-i`. Transcode planning reports the effective start as actual; copy-video may begin at an earlier source keyframe. Copy-video sessions add `-noaccurate_seek`, so audio they encode starts at that same keyframe instead of being trimmed to the target: an accurately seeked audio stream began up to a GOP after the copied video, and when that gap outlasted the first segment (Matroska Cues that skip keyframes make it long), the fMP4 muxer wrote `init.mp4` before any E-AC-3 packet was parsed, without `dec3` or `mvex`, which Media3 cannot play. Timestamps, and so A/V sync, are unchanged. A source whose own audio track starts more than a segment after its video still produces such an `init.mp4` for E-AC-3/AC-3 output, at any start. Output timestamps are normalized toward zero. Do not interpret the header as a measurement of the first decoded audio/video sample under every source timing condition.

Copy-video start resolution first uses a fingerprint-validated persisted keyframe index. The [`keyframeindex`](../server/cmd/internal/keyframeindex) package reads Matroska Cues or MP4 sample tables, including supported single-edit handling. A cache miss starts shared background extraction, even at zero to prepare later seeks. Unsupported/missing indexes fall back to a bounded ffprobe keyframe lookup with the lookback and budgets in the [settings table](#operational-settings-and-budgets). Resolution runs alongside FFmpeg. Its failure does not fail playback; a positive copy-video start can have no actual-start header until resolution succeeds, or never receive one. Index-derived results are advisory, not a guarantee for corrupt or unusual seek tables.

A client maps absolute position as session-local time plus the actual start when available, otherwise the effective normalized start. Supply that same offset as the subtitle endpoint's optional numeric `start`; subtitle offsets can be fractional even though HLS request starts are whole seconds. Direct playback normally omits subtitle `start`. [Subtitle handling](#subtitles) defines the different cue/packet trimming behavior.

**Web client:** resume clamps its absolute target to known duration and applies the HLS rewind in the settings table before creating the window. Direct initialization uses the clamped target without HLS rebasing. Actual-start correction can rebuild the player while preserving play/pause intent. These are player policies; the server neither reads saved progress to choose HLS `start` nor applies the web rewind itself.

The web player rebases when seeking before its current window or sufficiently far ahead of the settled playhead, using the threshold and settling delay in the settings table. Element seek events cover native/fullscreen/PiP controls as well as page controls; small stall nudges do not outrank a pending user seek. The latest input wins across overlapping seek timers. Each rebase requests a new manifest window while the UI continues to show absolute source time. Watch rooms do not use personal rebasing.

Movie trailer pre-roll is another **web-client** policy: the account preference and `/preroll` queue select YouTube embeds, without server video download/proxying. The movie player mounts during the final trailer's warm-up interval, on skip, or when no usable queue exists. Episodes, rooms, resumed playback, and `autoplay=true` skip pre-roll; the decision is made once at page entry. Progress is not saved during trailers.

## Readiness, HTTP responses, and recovery

### Server readiness

With a successful HLS muxer `temp_file` capability probe, segments are written as `.tmp` and renamed on completion. Only nonempty final names are served; temporary names are invalid asset requests. Initialization has no temporary rename: nonempty `init.mp4` is ready when the first segment's temporary/final name exists or the process has exited. Without `temp_file`, readiness uses existence of the successor file or process exit.

A running transcode manifest waits for ready initialization; a running copy-video manifest waits for a published segment. Manifest, preflight, capacity, segment wait, and polling budgets are collected in the [settings table](#operational-settings-and-budgets). A cold personal remux request can traverse all three startup budgets; cancellation of a shared creator can add further attempts. Configure client timeouts accordingly. Request cancellation ends readiness polling.

Ready files are opened and statted before `http.ServeContent`, pinning the descriptor across cleanup. Unexpected open/stat failures are logged internally and sanitized for clients; a known stopped session is classified as lost instead. The router restores `io.ReaderFrom` through session middleware so file serving can use the transport's optimized copy path where supported.

### HTTP responses

This table explains playback decisions; [OpenAPI](openapi.json) remains the exhaustive per-endpoint contract.

| Response | Meaning and client responsibility |
| --- | --- |
| Manifest `200` | HLS media playlist (`application/vnd.apple.mpegurl`); follow its URLs and inspect diagnostic headers as needed |
| Asset/direct `200` or `206` | Complete file or byte range; multiple ranges use `multipart/byteranges`; HLS assets use `video/mp4`, direct files their media MIME type |
| `304` | Bodyless conditional response, not a JSON envelope |
| `412` | Bodyless failed `If-Match` or applicable `If-Unmodified-Since` precondition |
| `416` | Go `ServeContent` plain-text malformed/unsatisfiable range error; `Content-Range` is present when applicable |
| `400` | Invalid parameters or track selection; fix the request rather than blindly retrying |
| `401` / room `403` | Authentication / room authorization failure |
| Room `409` | Pinned track drift on routes that check it; recreate the room with current tracks |
| Manifest `404` | Missing media, no playable output, stopped/replaced session during a wait, or refresh-only session loss; inspect context before retrying |
| Asset `404` without marker | Missing/lost session; request a manifest for the desired position |
| Asset `404` with `X-Igloo-Segment: past-end` | A cleanly exited process did not write that file; treat it as end of available media, not session loss |
| `422` | Unusable stored media/audio metadata; repeated unchanged requests will not repair it |
| HLS `500` | Unexpected preparation/I/O/FFmpeg error or unavailable explicit audio encoder; detailed causes stay in logs. A missing asset after a failed encode can be recovered by requesting an ordinary manifest, which replaces the session |
| HLS `503` | Capacity/storage/shutdown or output not ready; honor `Retry-After` and bound retries |
| Subtitle `415` | Codec/container unsupported by that subtitle endpoint |
| Subtitle-stream `503` | Stream slots exhausted or shutdown; honor its `Retry-After` |

Application errors before streaming normally use the JSON error envelope. Failures after response bytes have started cannot switch to a JSON status; [Matroska streams](#matroska-streams) deliberately terminate incomplete responses on FFmpeg failure.

Direct files emit a strong ETag derived from size and nanosecond mtime plus Last-Modified. Ready HLS assets emit Last-Modified but **no ETag**. Matching `If-None-Match` (including `*`) or applicable `If-Modified-Since` can return `304`; `If-Match` with a specific tag cannot match an HLS asset. `If-Range` validates a range or causes a full `200` when it does not match; HLS supports date validators, while direct files also support their ETag. Do not assume every non-200 response has JSON.

HLS manifest, asset, and stop handlers set `Cache-Control: no-store`, including handler errors. Authentication/session middleware can answer before that header is set, and Go's range-error response strips it. Successful subtitle streams also use `no-store`. Retry delays by cause are listed once in the [settings table](#operational-settings-and-budgets).

### Web-client recovery policies

The web player prefers hls.js when classic MediaSource supports its fMP4 baseline; native HLS is the fallback. These implementations expose different errors, so the following policies are not universal server guarantees:

- Fatal manifest `503` enters capacity retry, retaining the waiting notice until a manifest loads. Keepalive pauses during this state.
- hls.js does not internally retry `503`, or segment `500`; application recovery owns them. Manifest/level `500` and other server/proxy failures retain hls.js retry behavior.
- Plain segment `404`, fatal segment `500`, or refresh-only `404` requests a personal rebase at the playhead. An instance with no buffered media reports its intended start rather than a reset element time. A marked past-end `404` stops loading and signals end of stream; a subsequent seek rearms loading.
- Segment-not-ready `503` and no-answer/status-zero failures have separate bounded load retries. Buffered fragments reset those consecutive-failure budgets and one-shot media-error recovery.
- Lost-session retries belong to the media item and survive window replacement; they reset only after the incident interval. Opening a different media item remounts the player.
- Native HLS gets manifest preflight handling but lacks the hls.js segment-status handlers. A native `MEDIA_ERR_NETWORK` invokes bounded lost-session recovery; a real network outage can exhaust it.

The exact web budgets and timeouts are in the [settings table](#operational-settings-and-budgets). Room reload/re-synchronization is described under [session lifecycle](#session-lifecycle).

## Subtitles

Movie and episode subtitle endpoints use the ordinal and authentication rules under [media requests](#media-identity-and-requests) and [audio selection](#audio-selection-and-encoding). They are independent of HLS sessions; clients choose the offset using the [timeline rules](#timeline-seeking-and-resume). Igloo does not burn these subtitles into HLS video.

### WebVTT

`/subtitles/{trackIndex}/web.vtt` converts the selected stream and returns `text/vtt`:

```text
ffmpeg -v error -i <source> -map 0:<absolute stream index> -c:s webvtt -f webvtt pipe:1
```

The early bitmap rejection list is PGS (`hdmv_pgs_subtitle`), DVD (`dvd_subtitle`), DVB (`dvb_subtitle`), and XSUB (`xsub`), returning `415`. This is not exhaustive classification of every unsupported subtitle codec: other unconvertible codecs can reach FFmpeg and fail with `500`. Escaped `\h` sequences in successful output become spaces.

Full, unshifted output is cached by media kind, physical file ID, and absolute stream index. Concurrent requests share an extraction detached from the first request, so a departing caller stops waiting without canceling others' work. Shutdown or the extraction timeout cancels it. Cache hits renew its idle lifetime. Invalidation advances a generation captured before source metadata resolution; extraction deduplication includes that generation, and cache publication and renewal are synchronized with invalidation. Older requests can finish with their original cues but cannot repopulate or renew the cache. Track copies are invalidated before the WebVTT generation advances, so new work cannot convert an invalidated copy. Eviction remains specific to the physical file and media kind; see the [settings table](#operational-settings-and-budgets).

At serve time, positive `start` subtracts the session offset, drops cues ending before it, and clamps a cue spanning it to zero. A cached Matroska track can supply conversion input without starting or waiting for a copy: the server probes the source container's start time, preserves cached timestamps with `-copyts`, and subtracts container start using `-output_ts_offset`. Unknown start time or failed conversion falls back to reading the source file. Do not assume all containers start at zero.

### Matroska streams

`/subtitles/{trackIndex}/stream.mkv` supplies one track for a client that can decode and render it. [`SubtitleMatroskaPlanFor`](../server/cmd/internal/helpers/subtitles.go) chooses:

| Stored codec/container | Operation |
| --- | --- |
| `subrip`, `ass`, `ssa`, `hdmv_pgs_subtitle`, `dvb_subtitle` | Copy |
| `dvd_subtitle` with stored container `mkv` | Copy, retaining palette codec data |
| `dvd_subtitle` elsewhere; `xsub`, `dvb_teletext`, `arib_caption`, empty codec | Reject with `415` |
| Other codecs | Attempt SubRip conversion; conversion can still fail |

The argument shape is:

```text
ffmpeg -v error -nostdin [-seek_timestamp 1 -ss <start>] -i <source>
  -copyts -map 0:<stream index> -c:s copy|srt -map_chapters -1 -map_metadata:g -1
  [-ss <start> [-output_ts_offset -<start>]] -flush_packets 1 -f matroska pipe:1
```

Input seek skips work when reading the media file. Output seek drops packets starting before `start` and rebases copied packets; converted tracks additionally need the negative output offset. Cached-track input omits input seek because its first timestamp can be the first cue's timestamp. Unlike WebVTT, a cue beginning before the offset is dropped even if it spans it. Zero/absent start keeps the source packet timeline.

The response is non-seekable `video/x-matroska`, without Range support or a known content length (chunked under HTTP/1.1). A client needing earlier packets must reopen it. Before sending the header, the server parses through Tracks and inserts an empty timestamp-zero Cluster so a sparse track can become ready before its first cue. The relay then writes complete elements and periodic Void keepalives at element boundaries. The keepalive interval also bounds how long a cancelled client holds the stream: a player that cancels a load without closing its socket (Media3 sets a flag and closes once its blocked read returns) keeps FFmpeg and a stream slot until the next bytes arrive. Clients must avoid letting a sparse subtitle input stall video loading in their merged player; no external client's implementation is assumed here.

The stream has a request-owned FFmpeg process. Disconnect/shutdown cancels it; successful EOF waits for process exit. Failure before the primed header returns sanitized `500`; failure afterwards aborts the response rather than reporting clean completion. Stream slots, waiting, retry, and silence intervals appear in the [settings table](#operational-settings-and-budgets).

### Subtitle track cache

After a source-backed Matroska stream stays open through the cache gate, the server schedules a full-file track extraction. One extraction runs at a time server-wide and identical requests share it; a large source can delay other tracks. WebVTT alone never schedules copies.

Copies preserve source timestamps and use the Matroska codec plan above. They live at:

```text
<transcode dir>/igloo-subtitle-cache/<movie|episode>/<file id>/<stream index>-<source size>-<source mtime ns>.mkv
```

Publication renames a temporary file only after clean exit, an unchanged source size/mtime, unchanged transcode root, and unchanged invalidation generation. Copies reaching the byte cap are discarded even if FFmpeg exits successfully. Low free space skips copying. The settings table owns the gate, concurrency, size, timeout, and retention values.

Serving a copy updates its mtime for retention. Stale source fingerprints are removed on lookup; an unreadable or prematurely broken copy is deleted and future work uses the source. A client disconnect alone does not invalidate a copy. Startup/periodic sweeps remove unused copies, expired temporary files, and empty directories. Technical invalidation blocks serving/publishing affected copies while removal runs; moving the transcode directory removes the old subtitle-cache subtree. HLS startup cleanup does not remove this cache.

## Operational settings and budgets

This is the single reference for operational defaults and fixed budgets in this guide. Environment settings read at startup are distinct from values that seed the first Settings row. After that row exists, change seeded values in Settings. Profile encoding values live in the [video](#profiles-and-remux-fallback) and [audio](#audio-selection-and-encoding) tables instead.

| Setting or budget | Current value/default | Scope/source |
| --- | --- | --- |
| `IGLOO_FFMPEG_PATH`, `IGLOO_FFPROBE_PATH` | Unset: command on `PATH` | `externalbin` only; `mediabin` |
| `TRANSCODE_DIR` | `./transcode` | Seeds Settings; writable HLS/subtitle workspace |
| `HARDWARE_ACCELERATION_DEVICE` | `cpu`; choices `cpu`, `apple`, `nvidia`, `intel` | Seeds Settings; effective device depends on probes |
| `HLS_MAX_CPU_TRANSCODES` | `max(1, runtime.NumCPU()/4)` | Startup environment; `hls_limiter.go` |
| `HLS_MAX_HW_TRANSCODES` | 3 | Startup environment; hardware video encodes |
| `HLS_MAX_SESSIONS_PER_USER` | 3 | Startup environment; personal cached + in-flight sessions |
| Invalid limiter environment value | Fall back to default unless a positive integer | `configuredPositiveInt` |
| Effective transcode height cap | CPU 1080; hardware 2160 | `HLSMaxTranscodeHeight` |
| Fallback upload headroom | 0.8 × configured upload Mbps; unset means no bandwidth cap | `hlsBandwidthHeadroomFactor` |
| Personal / room idle TTL; sweep | 5 min / 30 min; 1 min | `hls_session.go` |
| Idle same-owner encode reclaim threshold | 30 s | `hlsIdlePermitReclaimThreshold` |
| Personal permit wait / room permit wait | 15 s / none | `hlsTranscodeAcquireWait` |
| Shared-creation attempts after leader cancellation | 3 | `hlsSessionFlightAttempts` |
| Free-space floor for HLS starts and subtitle copies | 2 GiB | `hlsMinFreeTranscodeBytes` |
| Remux preflight | 4 fragments; up to 30 s; 250 ms polling | Remux/session handler constants |
| Manifest readiness / asset readiness | 30 s / 120 s; 25 ms polling | `hls_handler.go` |
| HLS `Retry-After`: capacity/storage/shutdown / output not ready | 5 s / 1 s | Limiter and handler constants |
| Segment target / synthesized target duration | 4 s / 8 s | HLS constants and playlist builder |
| Unknown playlist frame rate | 24 fps | `hlsFallbackFrameRate` |
| Read-rate speed / initial burst, when supported | 4× / 60 s | `ffmpeg_hls.go` |
| VFR relative tolerance | 0.5% | `hlsVFRRelativeTolerance` |
| Start-at/past-duration clamp | 5 s before end, floored to whole seconds; minimum zero | `normalizedHLSStartSec` |
| Keyframe extraction / fallback probe budget; fallback lookback | 15 s each; 30 s lookback | `hls_keyframes.go`, ffprobe keyframe reader |
| Metadata probe / capability probe | 60 s / 5 s per probe | ffprobe metadata / FFmpeg capabilities |
| Successful room-membership cache | 30 s | `watch_room_auth.go` |
| HTTP shutdown grace | 10 s | Server shutdown |
| HLS stderr tail | Last 20 lines | `ffmpeg_hls.go` |
| WebVTT extraction / source-start probe | 60 s / 10 s within extraction | `subtitle_handler.go` |
| WebVTT cache idle TTL / sweep | 1 h / 10 min | Subtitle cache constants |
| Subtitle stream slots / acquisition wait / `Retry-After` | 8 / 3 s / 2 s | `subtitle_stream_handler.go` |
| Subtitle stream silence keepalive | 3 s at element boundaries | `subtitleStreamKeepalive` |
| Track-copy gate / concurrency / timeout / byte cap | 30 s / 1 / 3 h / 512 MiB | `subtitle_track_cache.go` |
| Track-copy retention / sweep | 30 days since use / startup then daily | Subtitle track cache |
| Web keepalive | 2 min | `HLS_SESSION_KEEPALIVE_INTERVAL_MS` |
| Web capacity retries / missing-header fallback delay | 6 / 5 s | `HLS_CAPACITY_RETRY_*` |
| Web segment-not-ready retries | 3 fresh attempts after initial failure | `HLS_SEGMENT_NOT_READY_MAX_RETRIES` |
| Web no-answer retry delays | 2, 4, 8 s | `HLS_NETWORK_RECOVERY_DELAYS_MS` |
| Web lost-session budget / minimum spacing / incident interval | 3 / 2 s / 60 s | `HLS_SESSION_LOST_*` |
| Web HLS load / fragment timeout | 120 s / 150 s | `HLS_JS_*LOAD_TIMEOUT_MS` |
| Web HLS rewind / forward rebase threshold / seek settling / nudge tolerance | 10 s / 120 s / 250 ms / 1 s | Web playback constants |
| Web direct metadata timeout | 10 s | `DIRECT_PLAY_STALL_TIMEOUT_MS` |
| Web final-trailer warm-up | 20 s | `PREROLL_MOVIE_WARMUP_SEC` |

Media libraries need read access; application cache/transcode directories need write access. A source with missing or corrupt seek tables can make input seeking slow even with these request budgets. Budgets limit waiting, not source I/O throughput. See [README configuration](../README.md#configuration) and [.env.example](../.env.example) for broader application configuration.

## Verification and maintenance

Use the root Make targets with Jellyfin binaries. For this Linux checkout:

```bash
export IGLOO_FFMPEG_PATH="$PWD/server/cmd/internal/ffmpeg/ffmpeg_linux_amd64"
export IGLOO_FFPROBE_PATH="$PWD/server/cmd/internal/ffprobe/ffprobe_linux_amd64"
make test-server
make test-web
make test-openapi
```

These tests provide complementary evidence:

| Area | Existing coverage |
| --- | --- |
| Binary extraction/resolution and probes | `mediabin_test.go`, FFmpeg/ffprobe lifecycle tests, `capabilities_test.go`, `capabilities_probe_test.go` |
| Probed metadata and scanner persistence | `ffprobe_metadata_test.go`, scanner stream/fingerprint/probe integration tests |
| Direct/range/conditional serving | API `openapi_media_contract_test.go`, `stream_file_test.go`, `movie_stream_handler_test.go`, `watch_room_handler_test.go` |
| HLS keys, admission, eviction, shutdown, HTTP | API `hls_session*_test.go`, `hls_limiter_test.go`, `hls_shutdown_test.go`, `hls_handler_test.go`, `hls_room_test.go` |
| Remux safety | FFmpeg `remux_validator_test.go`, `remux_parser_test.go`, `remux_fuzz_test.go`; API `hls_remux_safety_test.go` |
| Profiles, audio, hardware arguments, real CPU output | Helpers profile tests; `ffmpeg_hls_args_test.go`, `ffmpeg_hls_hardware_args_test.go`, `ffmpeg_integration_externalbin_test.go` |
| Seeking and playlists | API `hls_keyframes*_test.go`, `hls_playlist_test.go`; `keyframeindex` tests; FFmpeg segment-count tests |
| Subtitle timing, caching, transport | `ffmpeg_subtitles_test.go`, `ffmpeg_subtitle_streams_externalbin_test.go`; API subtitle tests including real-FFmpeg HTTP tests; `matroska_stream_test.go` |
| Web policies | Playback/direct-probe utilities, player/page, seek/recovery, capacity, keepalive, and watch-room unit tests under `web/src` |

Real-FFmpeg tests cover generated fixtures, including CPU HLS, remux, explicit Dolby audio codec/layout/channel/sample-rate checks and decoded signal contribution (including nonzero starts and isolated center/surround/LFE mixes, and LFE kept in 2.1, 3.1, and 4.1), deinterlacing, rotation, BT.601 matrix conversion, and copied/converted subtitle timestamps. Hardware argument tests use constructed capabilities; they do not establish NVIDIA, Intel, or Apple playback correctness. External-client behavior requires testing that client. See the [audit record](ffmpeg-audit.md) for this run's results and unresolved findings.

Keep argument construction centralized in the FFmpeg wrapper. When media behavior changes, update this guide and relevant tests; update OpenAPI for contract changes and run route coverage after route changes. Profile/device changes also require reviewing playback-settings responses, client choices, and configuration docs. Follow [repository guidelines](../AGENTS.md) and the scoped server/web guidelines for required checks.

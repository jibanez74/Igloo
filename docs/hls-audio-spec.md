# Igloo HLS Configurable Audio Profiles

**Status:** Draft  
**Component:** Igloo Go server  
**Feature area:** Movie HLS audio transcoding  
**Repository baseline:** `jibanez74/Igloo`, `main` at `1c41abd7b87f81f3274b27a3b070f057931d9ee7`  

## 1. Purpose

Igloo currently has two HLS audio behaviors:

- A selected AAC-LC source track is copied into the fragmented MP4 HLS output, including multichannel AAC.
- Any other selected audio track is transcoded to two-channel AAC at 320 kbps.

This is the compatibility contract for the web client and must remain unchanged. However, it removes surround channels from DTS, DTS-HD, TrueHD, and other incompatible source tracks. It can also reduce an existing AAC 5.1 track to AAC 2.0 when the source is not accepted by the current strict AAC-LC copy gate.

The target TV setup cannot reliably deliver multichannel PCM: even when the TV client decodes DTS or AAC 5.1, the television reports or emits two-channel PCM. The Media3 FFmpeg extension has therefore been removed from the TV-client design. Instead, the server will transcode the selected source track to AC-3 or E-AC-3 so the TV and connected Sonos Arc can use a supported Dolby multichannel format.

This feature adds an explicit audio-output profile to the existing personal movie HLS URLs. A client may request the output audio codec and maximum channel count. The server validates the request, chooses the bitrate, resolves the effective layout from the selected source track, and passes only typed values to the existing FFmpeg HLS command builder.

The main target cases are:

```text
DTS-HD MA 5.1 -> E-AC-3 5.1 at 768 kbps
```

or:

```text
DTS-HD MA 5.1 -> AC-3 5.1 at 640 kbps
```

and:

```text
AAC 5.1 -> E-AC-3 5.1 at 768 kbps
```

or:

```text
AAC 5.1 -> AC-3 5.1 at 640 kbps
```

The new explicit TV profile does not expose AAC output. AAC remains part of the untouched legacy web behavior only.

## 2. Goals

- Preserve surround channels when a client explicitly requests a supported multichannel HLS profile.
- Treat multichannel preservation as independent of the source codec, including AAC 5.1 and DTS-HD MA 5.1 sources.
- Keep current web and older-client behavior unchanged when the new query parameters are omitted.
- Support AC-3 and E-AC-3 explicit output through an allow-listed API.
- Keep encoding bitrate selection under server control.
- Resolve audio behavior from the selected `audio_track`, not the first audio stream in the file.
- Preserve the current video profile, remux-safety fallback, seeking, subtitle, playlist, segment-readiness, limiter, and cleanup behavior.
- Keep HLS sessions with different audio requests isolated.
- Expose the effective audio result for client diagnostics.
- Avoid any dependency on the Media3 FFmpeg extension or multichannel PCM output in the TV client.

## 3. Non-goals

- Direct-play eligibility or direct-streaming changes.
- A public passthrough or `copy` value. The existing internal AAC-LC copy optimization remains limited to legacy mode.
- Arbitrary codec, bitrate, sample-rate, channel-layout, filter, or FFmpeg arguments supplied by a client.
- Automatic device capability detection by the server.
- Explicit AAC output or a server-controlled multichannel AAC-to-PCM playback path. AAC remains available only through legacy behavior.
- Reintroducing the Media3 FFmpeg extension into the TV client.
- Upmixing mono or stereo sources to surround sound.
- Preserving Dolby Atmos, DTS:X, or other object-based metadata during transcoding.
- Producing more than six output channels in the first version.
- Adding HLS master playlists or separate audio renditions.
- Changing the current fragmented MP4 HLS output, segment naming, or media-playlist architecture.
- Extending watch-room creation or persistence with an audio-output profile in the first version.

## 4. Existing implementation constraints

The implementation must extend the current architecture rather than create a parallel HLS path.

| Concern | Current Igloo location |
| --- | --- |
| Personal HLS route and query parsing | `server/cmd/api/hls_handler.go` |
| Session creation, audio-track resolution, cache keys, and logging | `server/cmd/api/hls_session.go` |
| Playlist asset-query propagation | `server/cmd/api/hls_playlist.go` |
| FFmpeg HLS argument construction | `server/cmd/internal/ffmpeg/ffmpeg_hls.go` |
| FFmpeg encoder capability detection | `server/cmd/internal/ffmpeg/capabilities.go` |
| Video HLS profile definitions | `server/cmd/internal/helpers/hls_profiles.go` |
| API contract | `docs/openapi.json` |

The existing personal routes remain unchanged:

```text
GET /api/movies/{id}/hls/{profile}/playlist.m3u8
GET /api/movies/{id}/hls/{profile}/{filename}
POST /api/movies/{id}/hls/session/stop
```

The existing `profile` path value continues to describe the video profile. Audio settings remain query parameters and must not be added to the video profile identifiers.

## 5. Public API

The personal movie HLS manifest and asset routes accept the following new query parameters.

| Parameter | Allowed values | Meaning |
| --- | --- | --- |
| `audio_codec` | `ac3`, `eac3` | Requested Dolby output codec. Explicit requests are encoded in the first version. |
| `audio_channels` | `2`, `6` | Maximum output channels. `2` means no more than stereo; `6` means preserve up to 5.1. |

The names follow Igloo's existing snake-case HLS query convention, including `audio_track`, `playback_session`, and `start`.

There is no `audio_bitrate` query parameter. Bitrates are controlled by the server profile table in Section 7.

### 5.1 Pairing requirement

The two parameters form one request and must either both be present or both be absent.

- Both absent: use legacy audio behavior exactly as it works today.
- Both present: use the explicit audio profile described in this specification.
- Only one present: return HTTP 400.

This distinction is required for backward compatibility. Omitting the parameters cannot be normalized to explicit AAC stereo because current Igloo may copy a confirmed AAC-LC source track, including a 5.1 or 7.1 track. Treating omission as explicit AAC stereo would change existing playback.

### 5.2 Example manifest requests

Existing legacy behavior:

```text
/api/movies/42/hls/1080p_8mbps/playlist.m3u8?audio_track=0&playback_session=<uuid>&start=0
```

Explicit AC-3 with up to 5.1 channels:

```text
/api/movies/42/hls/1080p_8mbps/playlist.m3u8?audio_track=0&audio_codec=ac3&audio_channels=6&playback_session=<uuid>&start=0
```

Explicit E-AC-3 with up to 5.1 channels:

```text
/api/movies/42/hls/remux/playlist.m3u8?audio_track=0&audio_codec=eac3&audio_channels=6&playback_session=<uuid>&start=0
```

An explicit audio profile affects audio only. A requested `remux` video profile may still pass through the existing remux-safety gate and fall back to a video transcode. The requested audio profile remains in force across that fallback.

### 5.3 Asset URL propagation

When the manifest request includes an explicit audio profile, every rewritten `init.mp4` and `segment_N.m4s` URL must include the same normalized `audio_codec` and `audio_channels` values.

`hlsAssetQueryParams` and `buildHLSAssetQuerySuffix` must preserve them alongside:

- `audio_track`
- `start`
- `playback_session`
- `reload`

Legacy manifest requests must continue omitting the new parameters from asset URLs.

### 5.4 Client responsibilities

- The web client must continue constructing HLS URLs exactly as it does at the repository baseline and must not send `audio_codec` or `audio_channels`.
- Older clients that know nothing about this feature remain in legacy mode automatically.
- The TV client may add both parameters after choosing AC-3 or E-AC-3 from its own device/profile configuration.
- The TV client must never send only one parameter and must not request `audio_codec=aac`.
- Subtitle-track selection, `audio_track`, `playback_session`, `start`, and all other existing query behavior remain independent of the new pair.

## 6. Legacy and explicit behavior

### 6.1 Legacy mode

When both new parameters are omitted, no audio behavior may change:

- `isCopySafeAACStream` remains the gate for copying a confirmed AAC-LC source track.
- A copied AAC-LC track retains its source channel count and bitrate.
- Every non-copy-safe audio track is encoded using the current arguments:

```text
-c:a aac -ac 2 -b:a 320k
```

- Video-only movies continue producing HLS without an audio stream.
- Existing watch-room HLS sessions continue using this legacy mode.

### 6.2 Explicit mode

When both new parameters are present:

- The effective output codec must match the requested codec.
- An AAC 5.1 source requested as `ac3` or `eac3` with a maximum of six channels must retain six channels while being encoded to the requested codec.
- A DTS-HD MA 5.1 source requested as `ac3` or `eac3` with a maximum of six channels must retain six channels while being encoded to the requested codec.
- Explicit requests always encode in the first version. The legacy AAC copy decision must not leak into explicit mode.
- The output channel count and layout are resolved according to Section 8.
- The bitrate is resolved according to Section 7.
- The profile remains effective if a requested video remux falls back to a video transcode.

The URL is deterministic about codec and maximum channels. For example, `audio_codec=eac3` must not return AAC because the source happened to be copy-safe.

Copy optimization for matching AC-3 or E-AC-3 sources may be added later only after a dedicated fMP4 copy-safety predicate and integration coverage are introduced. It is not part of this version.

## 7. Server-owned audio profiles

The server must define the public codecs and encoding constants in one typed profile table. A suitable location is a new `server/cmd/internal/helpers/hls_audio_profiles.go`, next to the existing video HLS profiles.

### 7.1 Codec mapping

| Public value | FFmpeg encoder | Required encoded profile | Diagnostic/HLS codec name |
| --- | --- | --- | --- |
| `ac3` | `ac3` | AC-3 | `ac3` |
| `eac3` | `eac3` | E-AC-3 | `eac3` |

Raw query values must never be used as encoder names. The encoder string must come from this table after validation.

### 7.2 Bitrate mapping

When encoding is required, bitrate is selected from the codec and effective channel count.

| Output codec | 1 channel | 2 channels | 3-4 channels | 5-6 channels |
| --- | ---: | ---: | ---: | ---: |
| AC-3 | 192 kbps | 384 kbps | 448 kbps | 640 kbps |
| E-AC-3 | 192 kbps | 384 kbps | 512 kbps | 768 kbps |

The values must be represented in the form needed by both logging and FFmpeg argument construction without duplicating conversions in handlers.

The legacy AAC fallback remains 320 kbps and must not be redirected through a new profile value that changes its output.

### 7.3 Sample rate

- Explicit AC-3 and E-AC-3 output must use 48 kHz.
- Legacy audio behavior must not gain a new `-ar` argument as a side effect of this feature.

## 8. Channel-count and layout resolution

`audio_channels` is a maximum. It does not instruct Igloo to create channels that are not present in the source.

The server must inspect `database.AudioStream.Channels` and `ChannelLayout` for the selected `audio_track` before starting FFmpeg.

### 8.1 Required outcomes

| Source | Request maximum | Effective output |
| --- | ---: | --- |
| Mono | 2 or 6 | Mono |
| Stereo | 2 or 6 | Stereo |
| 3.0, 4.0, 4.1, or 5.0 | 6 | Preserve the source channel count and compatible layout |
| 5.1 | 6 | 5.1 |
| 7.1 | 6 | Downmix to 5.1 |
| Any source above 2 channels | 2 | Downmix to stereo |

The implementation must never upmix mono or stereo to 5.1.

These rules apply regardless of source codec. In particular, AAC 5.1 and DTS-HD MA 5.1 sources requested with a maximum of six channels have an effective channel count of six. Their source codecs must never cause the channel resolver to select stereo.

### 8.2 Layout rules

- When the source channel count is within the requested maximum and its stored layout is valid for the selected encoder, preserve that layout.
- When downmixing to two channels, produce standard stereo.
- When downmixing more than six channels, produce standard 5.1.
- Do not implement conversion by selecting only the first N source channels.
- Use FFmpeg's channel-layout conversion or an explicit audio filter so center, surround, and LFE content participate in the downmix.
- The chosen arguments must be covered by command-construction tests and at least one real multichannel integration fixture.

If the selected audio row has `Channels <= 0`, the server cannot resolve a safe explicit profile. It must return a typed media-profile error before creating the temp directory or starting FFmpeg.

### 8.3 TV and Sonos output boundary

This feature deliberately avoids the unreliable multichannel PCM path observed in the target installation. The required TV-client path is:

```text
AAC 5.1 or DTS-HD MA 5.1 source -> server AC-3/E-AC-3 5.1 HLS -> TV playback -> Sonos Arc
```

The TV client must request AC-3 or E-AC-3 and use the platform playback stack without the Media3 FFmpeg extension. Successful playback should appear in the Sonos app as `Dolby Digital 5.1` for AC-3 or `Dolby Digital Plus 5.1` for E-AC-3, subject to the television's passthrough/output configuration. A PCM 2.0 result is not an acceptable verification result for a six-channel explicit request.

The server guarantees the requested encoded codec and resolved channel count in HLS. It cannot guarantee the television's HDMI mode, passthrough configuration, or final format reported by Sonos; those remain device-integration concerns.

## 9. Validation and errors

All validation must occur before user-controlled values reach the FFmpeg wrapper.

### 9.1 HTTP 400

Return HTTP 400 through Igloo's existing `helpers.ErrorJSON` envelope when:

- Only one of `audio_codec` or `audio_channels` is present.
- `audio_codec` is not exactly `ac3` or `eac3` after normal whitespace handling.
- `audio_channels` is not an integer.
- `audio_channels` is not `2` or `6`.

The server must not silently replace an invalid request with legacy AAC stereo.

### 9.2 HTTP 422

Return HTTP 422 when an explicit request is syntactically valid but the selected audio stream does not contain enough channel metadata to resolve it safely.

This should use a typed error recognized by `writeHLSSessionError`, not string matching.

### 9.3 HTTP 503

The existing FFmpeg capability probe already records encoder availability. Before creating a temp directory or acquiring a transcode permit, explicit mode must call `Capabilities.SupportsEncoder` for the resolved encoder.

If the encoder is unavailable, return HTTP 503 with `Retry-After` only if that matches the existing server-capability error convention. This is a server installation problem, not an invalid query.

AAC remains required for legacy playback. AC-3 and E-AC-3 may be unavailable on a swapped external FFmpeg binary without preventing the server from starting.

## 10. Internal models

The API-layer request must be separate from the resolved FFmpeg configuration. Names may be adjusted to project conventions, but the distinction is required.

```go
type HLSAudioCodec string

const (
	HLSAudioCodecAC3  HLSAudioCodec = "ac3"
	HLSAudioCodecEAC3 HLSAudioCodec = "eac3"
)

// A nil *HLSAudioProfileRequest means legacy mode.
type HLSAudioProfileRequest struct {
	Codec       HLSAudioCodec
	MaxChannels int
}

type HLSResolvedAudioProfile struct {
	Codec         HLSAudioCodec
	Encoder       string
	Channels      int
	ChannelLayout string
	Bitrate       string
	SampleRate    int
	Copy          bool
}
```

`hlsRequestParams` should carry `*HLSAudioProfileRequest`. The same typed request must flow through:

```text
parseHLSParams
HLSManifest
GetOrCreateHLSSession
createHLSSession
hlsSessionStartParams
startHLSSession
ffmpeg.HLSParams
```

The HTTP handler must not build audio FFmpeg arguments or reproduce the bitrate table.

## 11. FFmpeg integration

`server/cmd/internal/ffmpeg/ffmpeg_hls.go` remains the only owner of HLS FFmpeg command construction.

The existing `HLSParams` should receive a resolved audio profile rather than raw query strings. The implementation may replace `CopyAudio bool` with a typed audio configuration or add typed fields while preserving legacy behavior.

Representative arguments are:

```text
Legacy AAC stereo fallback: -c:a aac  -ac 2 -b:a 320k
AC-3 5.1:    -c:a ac3  -ac 6 -b:a 640k -ar 48000
E-AC-3 5.1:  -c:a eac3 -ac 6 -b:a 768k -ar 48000
```

If an explicit layout or audio filter is needed, it must be appended from the resolved typed profile. It must not be assembled from raw query values.

The existing mapping must remain explicit:

```text
-map 0:<primary-video-stream-index>
-map 0:<selected-audio-stream-index>
```

The selected audio ordinal must continue being translated to `database.AudioStream.StreamIndex` before it reaches FFmpeg.

No part of this feature may change:

- Video encoder selection or hardware fallback.
- HDR tone mapping.
- Deinterlacing.
- Keyframe placement.
- `-avoid_negative_ts make_zero`.
- fMP4 HLS muxer arguments.
- `temp_file` or `independent_segments` capability behavior.

## 12. Session identity and lifecycle

The normalized requested audio mode must participate in personal HLS session identity.

`HLSSessionKey` must distinguish at least:

```text
legacy
explicit:ac3:2
explicit:ac3:6
explicit:eac3:2
explicit:eac3:6
```

This value must be added to the key alongside the existing movie ID, video profile, audio track, playback-session UUID, and normalized start time.

Required behavior:

- A legacy request cannot reuse any explicit request.
- AC-3 and E-AC-3 requests cannot reuse each other's segments.
- Stereo and six-channel-maximum requests cannot reuse each other's segments.
- A manifest request and all of its asset requests must calculate the same key.
- The existing `singleflight` behavior must deduplicate only identical normalized requests.
- Switching an audio profile within the same `playback_session` must follow the existing superseded-session cleanup behavior.
- `POST /api/movies/{id}/hls/session/stop` must continue stopping all matching personal sessions for the playback-session UUID regardless of audio profile.

The effective channel count does not need to be part of the initial lookup key because it is deterministically resolved from the movie and selected stored audio row. It must still be stored on the session for diagnostics and headers.

## 13. HLS session state and response headers

`HLSSession` must store the effective audio result for the running session when audio is present.

Suggested fields are:

```go
RequestedAudioProfile *HLSAudioProfileRequest
EffectiveAudioProfile *HLSResolvedAudioProfile
```

`writeHLSPlaylistHeaders` must continue publishing:

```text
X-Igloo-Effective-Profile
X-Igloo-Actual-Start
```

and add the following when the session contains audio:

```text
X-Igloo-Effective-Audio-Codec
X-Igloo-Effective-Audio-Channels
X-Igloo-Effective-Audio-Bitrate
```

For copied legacy AAC, these headers describe the stored source values. For encoded legacy AAC, they describe AAC stereo at 320 kbps. For explicit mode, they describe the resolved encoded AC-3 or E-AC-3 profile. Video-only sessions omit them.

These headers are diagnostic output; clients must still use the media stream as the playback authority.

The HLS endpoints currently return media playlists rather than master playlists. This feature must not add `CODECS` attributes or audio rendition groups to the media playlist.

## 14. Watch rooms

The first version applies the new public query parameters only to personal movie HLS routes.

Watch-room HLS currently uses:

- A room-owned `audio_track`.
- A room-only cache key of `room:<room_id>`.
- A manifest route without an audio-profile query contract.
- Warm-up immediately after room creation.

Changing the watch-room audio output therefore requires a room-level persisted setting and creation API change, not merely two query parameters. That work is outside this specification.

The shared session and FFmpeg changes must keep watch rooms in legacy audio mode and must not alter existing watch-room output, warm-up, cache identity, or cleanup.

## 15. Observability

Extend the existing `hls session starting` structured log with:

- `requested_audio_mode`: `legacy` or `explicit`.
- `requested_audio_codec` when explicit.
- `requested_audio_max_channels` when explicit.
- `source_audio_channels`.
- `source_audio_channel_layout`.
- `effective_audio_codec`.
- `effective_audio_channels`.
- `effective_audio_channel_layout`.
- `effective_audio_bitrate`.
- `effective_audio_sample_rate`.
- `audio_downmix`: boolean.

Keep the existing `audio_track`, absolute `audio_stream_index`, source `audio_codec`, `audio_codec_profile`, and `copy_audio` fields.

Session-finished, stopped, and failed logs should include the effective audio codec and channels so a runtime FFmpeg failure can be tied to the profile that produced it.

## 16. API contract and generated types

Update `docs/openapi.json` manually to add:

- `HLSAudioCodecQuery`.
- `HLSAudioChannelsQuery`.
- Both parameters on the personal manifest endpoint.
- Both parameters on the personal HLS asset endpoint.
- The new effective-audio response headers on the HLS playlist response.
- Descriptions stating that the two query parameters must appear together and that omission preserves legacy behavior.

The asset endpoint must document the parameters because the generated playlist includes them on `init.mp4` and segment URLs.

After editing the OpenAPI contract, run the existing generation and contract checks so `web/src/types/openapi.gen.ts` remains synchronized.

## 17. Testing requirements

### 17.1 `server/cmd/api/hls_handler_test.go`

- Both parameters omitted parses as legacy mode.
- AC-3 and E-AC-3 parse with channel maximum 2 and 6.
- AAC is rejected as an explicit codec.
- Only one of the pair returns HTTP 400.
- Unknown codec, empty explicit value, non-numeric channels, and unsupported channel count return HTTP 400.
- Manifest creation passes the typed request to session creation.
- Effective audio headers are written for copy, legacy transcode, and explicit transcode sessions.
- Video-only sessions omit effective audio headers.

### 17.2 `server/cmd/api/hls_playlist_test.go`

- Explicit audio parameters appear on `init.mp4` and every `segment_N.m4s` URL.
- Legacy URLs do not gain the new parameters.
- Existing `audio_track`, `start`, `playback_session`, and `reload` propagation remains unchanged.
- URL encoding remains deterministic through `url.Values`.

### 17.3 `server/cmd/api/hls_session_test.go`

- Legacy AAC-LC remains copy-safe and preserves multichannel audio.
- Legacy incompatible audio remains AAC stereo at 320 kbps.
- Explicit AC-3 or E-AC-3 requests never copy an AAC source, but preserve its six-channel layout when the maximum is six.
- Explicit AC-3 or E-AC-3 requests preserve six channels from DTS-HD MA 5.1.
- Explicit requests always encode, including when the source codec already matches.
- The selected audio row, not the first row, drives channel resolution.
- Mono and stereo are never upmixed.
- 3-6 channels are preserved under a maximum of 6 when the layout is supported.
- 5.1 is downmixed to stereo under a maximum of 2.
- 7.1 is downmixed to 5.1 under a maximum of 6.
- Missing channel metadata returns the typed HTTP 422 error.
- A remux-safety fallback retains the explicit audio profile.
- Encoder capability rejection occurs before temp-directory creation and limiter acquisition.
- Watch-room creation continues using legacy audio mode.

### 17.4 `server/cmd/api/hls_session_cache_test.go`

- Legacy, AC-3, and E-AC-3 modes produce different cache keys.
- Maximum channel values 2 and 6 produce different cache keys.
- Identical normalized requests still share the existing `singleflight` session.
- Switching audio profile cleans up the prior session only within the same owner, movie, and playback-session UUID.

### 17.5 `server/cmd/internal/ffmpeg/ffmpeg_hls_args_test.go`

- Legacy safe AAC produces `-c:a copy` exactly as before.
- Legacy incompatible audio produces `-c:a aac -ac 2 -b:a 320k` exactly as before.
- Explicit AAC 5.1 input produces the requested AC-3 or E-AC-3 codec with six channels.
- Explicit DTS-HD MA 5.1 input produces the requested AC-3 or E-AC-3 codec with six channels.
- Explicit AC-3 5.1 produces AC-3, six channels, 640 kbps, and 48 kHz.
- Explicit E-AC-3 5.1 produces E-AC-3, six channels, 768 kbps, and 48 kHz.
- Explicit downmix arguments use the resolved safe layout.
- Raw query values cannot reach the argument builder.
- Existing video, seek, keyframe, fMP4, and HLS flags remain unchanged.

### 17.6 Capability and integration tests

- Capability parsing continues recognizing legacy `aac` and recognizes explicit `ac3` and `eac3` from `ffmpeg -encoders`.
- Requests for missing AC-3 or E-AC-3 encoders fail with the typed server-capability error.
- The configured embedded Jellyfin FFmpeg build successfully produces fMP4 HLS with each explicit codec.
- ffprobe verifies the init segment and media fragments use the expected codec and channel count.

Minimum integration fixtures:

| Source audio | Request | Expected output |
| --- | --- | --- |
| DTS-HD MA 5.1 | `eac3`, `6` | E-AC-3 5.1 at 768 kbps |
| DTS-HD MA 5.1 | `ac3`, `6` | AC-3 5.1 at 640 kbps |
| DTS-HD MA 5.1 | legacy | AAC stereo at 320 kbps |
| TrueHD 7.1 | `eac3`, `6` | E-AC-3 5.1 at 768 kbps; no Atmos guarantee |
| AAC-LC 5.1 | legacy | Copied AAC-LC 5.1 |
| AAC 5.1 | `ac3`, `6` | AC-3 5.1 at 640 kbps |
| AAC 5.1 | `eac3`, `6` | E-AC-3 5.1 at 768 kbps |
| AAC stereo | `eac3`, `6` | E-AC-3 stereo at 384 kbps; no upmix |
| Mono | `ac3`, `6` | AC-3 mono at 192 kbps; no upmix |

Manual release verification must cover:

- Existing web HLS playback with no new parameters.
- The web client emits no `audio_codec` or `audio_channels` parameters and behaves exactly as at the repository baseline.
- Igloo TV playback requesting AC-3 5.1 from both AAC 5.1 and DTS-HD MA 5.1 sources.
- Igloo TV playback requesting E-AC-3 5.1 from both AAC 5.1 and DTS-HD MA 5.1 sources.
- Playback through the target Android TV device and connected surround system without the Media3 FFmpeg extension. Sonos should report Dolby Digital 5.1 or Dolby Digital Plus 5.1 for the corresponding explicit request.
- No six-channel request may be reported by the Sonos system as 2.0 because of a server-side downmix.
- Audio-track switching and subtitle selection with each explicit codec.
- Seeking, session replacement, and stop-session behavior.

## 18. Acceptance criteria

The feature is complete when:

- Requests without `audio_codec` and `audio_channels` behave exactly as they did at the repository baseline.
- The web client does not add either new parameter; confirmed AAC-LC remains copy-safe and all other selected audio retains the existing AAC stereo 320 kbps fallback.
- The TV client can request AC-3 or E-AC-3 through the existing personal HLS URL.
- A DTS-HD MA 5.1 source produces AC-3 5.1 at 640 kbps or E-AC-3 5.1 at 768 kbps when requested.
- An AAC 5.1 source requested as AC-3 or E-AC-3 with a maximum of six channels remains 5.1 after transcoding.
- The TV feature does not require or reintroduce the Media3 FFmpeg extension and does not depend on multichannel PCM output.
- Mono and stereo sources are never upmixed.
- Sources above the requested maximum are downmixed using a defined layout conversion.
- Invalid or partial profiles return HTTP 400 and never fall back silently.
- Insufficient source channel metadata returns the typed media-profile error.
- Missing encoders produce a server-capability error before session resources are allocated.
- Audio profile values participate in manifest, asset, cache, and `singleflight` identity.
- Requested audio settings survive video remux fallback.
- Existing audio-track, subtitle, seeking, playlist, segment-readiness, watch-room, and stop-session behavior remains intact.
- Effective audio headers and structured logs describe the actual output.
- OpenAPI, generated web types, server tests, and FFmpeg integration tests pass.

## 19. Implementation sequence

1. Add typed audio codec, request, and resolved-profile definitions plus the centralized bitrate table.
2. Extend `hlsRequestParams` and `parseHLSParams` with pair validation and legacy-mode preservation.
3. Add the normalized requested audio mode to personal session keys and function signatures.
4. Resolve the effective channel count, layout, bitrate, and sample rate from the selected audio row while preserving the separate legacy copy decision.
5. Check encoder availability before temp-directory creation and limiter acquisition.
6. Pass the resolved profile into `ffmpeg.HLSParams` and update centralized audio argument construction.
7. Propagate explicit audio parameters through `hlsAssetQueryParams` to init and segment URLs.
8. Store effective audio state on `HLSSession`, add response headers, and extend structured logs.
9. Update `docs/openapi.json`, then run `make generate-openapi` and `make test-openapi`.
10. Add unit and fMP4 integration coverage, then run `make test-server` and the relevant full-project checks.
11. Verify legacy web playback and explicit TV surround playback on real hardware.

## 20. Technical references

- [Igloo HLS handler](https://github.com/jibanez74/Igloo/blob/main/server/cmd/api/hls_handler.go)
- [Igloo HLS session management](https://github.com/jibanez74/Igloo/blob/main/server/cmd/api/hls_session.go)
- [Igloo FFmpeg HLS builder](https://github.com/jibanez74/Igloo/blob/main/server/cmd/internal/ffmpeg/ffmpeg_hls.go)
- [Igloo FFmpeg capability probing](https://github.com/jibanez74/Igloo/blob/main/server/cmd/internal/ffmpeg/capabilities.go)
- [Igloo FFmpeg documentation](https://github.com/jibanez74/Igloo/blob/main/docs/ffmpeg.md)
- [FFmpeg codec documentation](https://ffmpeg.org/ffmpeg-codecs.html)
- [FFmpeg stream-specifier documentation](https://ffmpeg.org/ffmpeg-all.html#Stream-specifiers-1)
- [Android Media3 supported formats](https://developer.android.com/media/media3/exoplayer/supported-formats)
- [Android TV audio capabilities](https://developer.android.com/training/tv/playback/audio-capabilities)
- [Sonos supported home-theater audio formats](https://support.sonos.com/en/article/supported-home-theater-audio-formats)

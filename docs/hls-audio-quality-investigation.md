# HLS Audio Quality Investigation

Last reviewed: 2026-06-22

## Summary

Igloo can improve HLS audio quality, but the current workflow is deliberately
compatibility-first. The largest quality loss is not the HLS container itself.
The largest loss is that non-AAC surround source tracks are downmixed and
re-encoded to stereo AAC at `320k` for HLS playback.

The recommended default remains stereo AAC at `320k` because it is the safest
baseline for broad browser playback. Improvements should be added as explicit
policies or clearer UI reporting, not as an unconditional change to passthrough
or surround output.

## Scope

This report covers Igloo's browser HLS movie playback path. Direct streaming is
separate: when the browser can play the original source file directly, Igloo can
serve that source without creating HLS output.

This report is not a playback benchmark. It does not claim that a proposed
surround or passthrough mode works until a future sample-file matrix verifies it
across browsers, devices, and receiver paths.

## Current Igloo Workflow

During library scans, `ffprobe` stores the source audio metadata that matters for
playback decisions and UI display:

- codec
- bitrate
- sample rate
- channel count
- channel layout
- language
- title

The web client selects one `audio_track`. HLS playback URLs include that
selection as a query parameter, and HLS sessions are keyed by movie, requested
profile, selected audio track, playback session, and start time.

Each HLS session maps one video stream and, when the movie has audio, one audio
stream:

```text
-map 0:<video_stream_index>
-map 0:<audio_stream_index>
```

Those stream indices are the absolute `ffprobe` stream indices stored during the
scan. Igloo does not re-probe the file at HLS request time.

For the selected audio stream:

- AAC source audio is copied with `-c:a copy`.
- Every non-AAC source codec is transcoded with `-c:a aac -ac 2 -b:a 320k`.

Igloo writes fragmented MP4 HLS:

```text
-f hls
-hls_segment_type fmp4
-hls_fmp4_init_filename init.mp4
-hls_segment_filename segment_%d.m4s
-hls_playlist_type event
-hls_list_size 0
-hls_time 4
```

The served HLS output is a single muxed audio/video media playlist. Igloo does
not currently generate a master playlist with alternate audio renditions.

## Quality Impact

The current output policy has different effects depending on the selected source
track.

| Selected source audio | Current HLS audio output | Quality impact |
| --- | --- | --- |
| AAC | Copied | Igloo does not re-encode the track. Source quality is preserved by the server, though browser/device output may still vary. |
| AC3, EAC3, DTS, TrueHD, FLAC, Opus, or other non-AAC stereo | Stereo AAC `320k` | The track is decoded and re-encoded. Lossless sources become lossy; lossy sources take a generation loss. |
| AC3, EAC3, DTS, TrueHD, FLAC, Opus, or other non-AAC surround | Stereo AAC `320k` | The track is downmixed to stereo and re-encoded. This loses surround channels and may lose quality from lossy transcoding. |

The `320k` stereo AAC bitrate may reduce encoding artifacts on some material,
but it does not solve the main quality problem for surround tracks. Once
`-ac 2` is applied, channel layout has already been reduced to stereo.
Preserving the channel layout is the meaningful improvement for users who care
about surround playback.

The current UI labels audio tracks from scanned source metadata. For example, a
source track can appear as `5.1 surround`, but HLS playback may still output
stereo AAC if that source track is not AAC. That can set an incorrect user
expectation even when the backend is behaving as designed.

## Compatibility Constraints

Fragmented MP4 HLS is a valid HLS output shape. FFmpeg supports
`-hls_segment_type fmp4`, and RFC 8216 defines fragmented MP4 HLS through media
initialization sections. hls.js also supports fMP4 HLS.

AAC in MP4/fMP4 remains the safest browser-oriented baseline for Igloo's current
web playback target. Browser support for audio codecs and containers is not just
"can this codec exist in HLS"; it also depends on the browser, operating system
media stack, native HLS support, Media Source Extensions behavior, and the audio
endpoint.

AC3, EAC3, DTS, TrueHD, FLAC, Opus, and multichannel AAC should be treated as
advanced output modes, not universal defaults. Some combinations can work on
some devices, especially through native HLS or platform decoders, but Igloo
should not assume they work reliably across Chrome, Firefox, Safari, hls.js, TV
browsers, HDMI receivers, soundbars, and simple laptop speakers.

## Recommendation

Keep stereo AAC at `320k` as the default HLS compatibility mode.

This default is predictable, easy to reason about, and aligned with Igloo's
self-hosted target: most users should be able to press play without having to
understand codec passthrough, receiver support, or per-browser decoder behavior.

The best near-term improvement is to make the output policy visible:

- Show or explain that HLS may convert non-AAC surround tracks to stereo AAC.
- Distinguish source track metadata from expected HLS output where playback
  settings are shown.
- Avoid presenting a source `5.1 surround` label as a guarantee that the HLS
  output will be surround.

The best medium-term improvement is an opt-in audio output policy, for example:

| Policy | Behavior | Risk |
| --- | --- | --- |
| Compatibility stereo AAC | Current behavior: copy AAC, transcode non-AAC to stereo AAC `320k` | Lowest risk; should remain the default. |
| Higher-bitrate stereo AAC | Keep stereo downmixing, but encode non-AAC at a higher stereo bitrate | Low to medium risk; helps only marginally for channel quality. |
| Tested surround mode | Preserve multichannel layout for selected codecs/output formats that pass a test matrix | Medium risk; requires explicit compatibility testing and fallback behavior. |

Do not make AC3/EAC3 passthrough, DTS passthrough, TrueHD passthrough, FLAC
passthrough, Opus passthrough, or multichannel AAC the unconditional default
until Igloo has device-specific validation and clear fallback behavior.

## Future Work

### UI Reporting

A low-risk follow-up is to adjust playback labels and descriptions. The UI can
continue showing the source track label, but HLS modes should also communicate
that non-AAC surround tracks may be played as stereo AAC.

This is a reporting change only. It does not require API, schema, or FFmpeg
changes unless the UI needs the server to expose resolved output policy.

### Audio Output Policy

A future setting could choose the HLS audio output policy. Plan this as a real
public interface change:

- settings schema and validation
- API response/request fields
- frontend playback settings
- URL or session key impact if policy affects HLS output
- OpenAPI updates
- tests for session cache keys and FFmpeg argument generation

If the policy changes output media, it must be part of the session identity so
clients cannot accidentally reuse a session created with a different audio
policy.

### Alternate Audio Renditions

Alternate audio renditions are useful for language, commentary, stereo/surround,
or codec-specific audio choices, but they are not a quick fix for the current
audio quality issue.

Supporting alternate renditions would require a master playlist, one or more
audio media playlists, correct `EXT-X-MEDIA` groups, bandwidth and codec
signaling, player UI integration, and compatibility testing. Igloo currently
loads `hls.js/light`, so a future implementation should also verify whether the
current player bundle supports the needed alternate-audio behavior or whether it
must switch to another hls.js build.

Treat alternate audio as a later architecture change.

## Test Matrix For Any Behavior Change

Before changing default HLS audio behavior, test representative source files:

- AAC stereo
- AAC 5.1
- AC3 5.1
- EAC3 5.1
- DTS 5.1
- TrueHD or lossless surround
- FLAC stereo
- FLAC 5.1
- Opus stereo
- Opus 5.1

Test playback paths:

- Chrome desktop with hls.js
- Firefox desktop with hls.js
- Safari desktop native HLS
- iOS/iPadOS Safari native HLS
- Chromium-based browser on Linux
- simple stereo speakers/headphones
- HDMI or optical output to a receiver/soundbar when available
- TV browser or casting path if Igloo later targets those explicitly

For each case, record:

- manifest and segment shape
- browser/player errors
- whether audio starts reliably
- whether seeking remains reliable
- observed channel count at the endpoint when measurable
- fallback behavior when a codec or channel layout is unsupported

## Source Links

- [FFmpeg formats documentation](https://ffmpeg.org/ffmpeg-formats.html)
- [FFmpeg codecs documentation](https://ffmpeg.org/ffmpeg-codecs.html)
- [hls.js project documentation](https://github.com/video-dev/hls.js/)
- [hls.js API documentation](https://hlsjs.video-dev.org/api-docs/)
- [RFC 8216: HTTP Live Streaming](https://datatracker.ietf.org/doc/html/rfc8216)
- [Apple HLS authoring specification for Apple devices](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices)
- [Apple: Preparing Audio for HTTP Live Streaming](https://developer.apple.com/documentation/http-live-streaming/preparing-audio-for-http-live-streaming)
- [MDN Web audio codec guide](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Audio_codecs)
- [MDN media container formats guide](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Containers)

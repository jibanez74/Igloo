# FFmpeg reference audit — 2026-11-02

This documentation-only audit checked the working-tree implementation, manually maintained OpenAPI, and existing tests. The playback reference was reorganized by topic, catalog material moved to [library scanning](library-scanning.md), and README links updated. The pre-existing artwork-source correction was preserved. No API, schema, generated type, test, or runtime code was changed. The untracked `docs/known-issues.md` was read and left unchanged.

## Unresolved findings

These findings are separate from intended playback requirements. OpenAPI remains authoritative; discrepancies need a follow-up implementation/contract decision.

| Finding | Evidence and source location | Client impact |
| --- | --- | --- |
| Explicit audio does not enforce the resolved channel layout or preserve every input channel in a downmix | [Resolver](../server/cmd/internal/helpers/hls_audio_profiles.go), `ResolveHLSAudioProfile` (lines 138–155), derives `ChannelLayout`; [argument builder](../server/cmd/internal/ffmpeg/ffmpeg_hls.go), `buildHLSArgs` (lines 340–350), never passes it. [OpenAPI](openapi.json), `HLSAudioChannelsQuery` (line 6689), promises “full channel-layout conversion.” Local Jellyfin checks below demonstrate the narrower behavior. Existing `TestExternalFFmpegExplicitAudioProfiles` checks codec and channel count using silent fixtures, not channel routing. | A client cannot infer exact speaker layout or retained LFE content from the requested ceiling, diagnostic channel count, or logged layout. The old guide's stronger layout/LFE assertions were removed; OpenAPI was not changed. |
| Direct watch-room admission omits the web client's audio-codec gate | [Room creation](../server/cmd/api/watch_room_handler.go), direct-mode validation (lines 319–362), checks track selection/defaults, container and video, but not the first audio codec. [Web eligibility](../web/src/lib/playback.ts), `getAvailableModes` (lines 297–310), also checks the audio allowlist. Confirmed by code inspection; not an OpenAPI schema violation. | A client can create a direct room whose audio the included web player would reject. Server acceptance is not proof of browser playback support. |
| WebVTT's image-codec rejection is narrower than its contract wording | [OpenAPI](openapi.json), movie/episode WebVTT descriptions (lines 2602 and 3776), say image-based tracks return `415`. [WebVTT handler](../server/cmd/api/subtitle_handler.go), line 135, calls [IsBitmapSubtitleCodec](../server/cmd/internal/helpers/subtitles.go), whose list contains only PGS/DVD/DVB. `xsub` is recognized as unconvertible only by the separate Matroska plan. Confirmed control-flow mismatch; no XSUB media reproduction was run. | Unsupported bitmap/data tracks outside the early rejection list can reach FFmpeg and return `500`, rather than the advertised unsupported-media response. The guide now states the exact list. |
| An in-flight WebVTT extraction can repopulate an invalidated cache | [Invalidation](../server/cmd/api/subtitle_handler.go), `invalidateSubtitleVTTCache` (line 43), deletes existing keys; detached extraction later performs an unconditional `SubtitleVTTCache.Set` (line 208). The key contains no source fingerprint or invalidation generation. This is a race inferred from code, not a locally reproduced concurrent-rescan failure. OpenAPI's same WebVTT descriptions say a rescan drops the cache. | An extraction started before a technical rescan can publish old cues afterward; renewed cache hits can prolong them. The on-disk subtitle track cache has generation protection, but that does not protect this separate in-memory WebVTT cache. |

The audio check used the local `8.1.3-Jellyfin` Linux executable with the builder's explicit audio flags. Encoding a `5.1` lavfi source to either AC-3 or E-AC-3 with `-ac 6` produced `5.1(side)` according to ffprobe. Separate 48 kHz, 0.25-second `5.1` sources used `aevalsrc` with a 100 Hz sine in either the front-left or LFE channel, all other channels zero. Encoding with `-ac 2 -b:a 384k -ar 48000` and decoding to float PCM produced a front-channel peak near 1.0 and an LFE-only peak around 0.00000423 for both codecs. This checks FFmpeg audio behavior, not full HLS playback or listening quality.

## Review of existing known-issues reports

| Existing report | Audit status |
| --- | --- |
| Forced subtitles identified only by their title are not automatically forced | Confirmed implementation limitation: [scanner conversion](../server/cmd/internal/scanner/streams.go), line 151, uses only the forced disposition. The reported title-specific TV experience was not exercised. |
| One large track extraction delays other copies | Confirmed scheduling limitation: [track cache](../server/cmd/api/subtitle_track_cache.go), `cacheSubtitleTrack`, serializes work through `SubtitleTrackExtractSlot`. The reported NAS throughput and duration were not remeasured. |
| Cancellation may keep a TV subtitle request alive until the next keepalive | The server's silence interval and request-context cancellation are confirmed in [subtitle streaming](../server/cmd/api/subtitle_stream_handler.go). The Media3 socket/cancellation timing report was not reproduced; server behavior alone does not prove it. |
| DTS-to-E-AC-3 HLS resume creates unusable initialization output | Not reproduced in this audit. The existing real-FFmpeg audio-profile tests start at zero; passing them does not establish this nonzero-start case. Keep the reported missing-`mvex`/Media3 failure as an open report, not intended behavior. |
| A particular Oppenheimer file has invalid Cues and slow seeks | The named source was not inspected. Bounded keyframe lookup and missing-index fallback are confirmed, but do not validate that file or its reported seek throughput. |
| QA deployment lacks subtitle routes | Not checked against the remote deployment. The routes exist locally and subtitle HTTP tests passed. |
| TV double subtitle opens, chapter focus, and memory budget | External-client reports not verified from this repository or on a TV device. No assertions about those client implementations were retained as server guarantees. |
| Web tests time out under heavy load | The full web suite passed in this run. The reported contention scenario was not recreated. |

Other confirmed limitations are documented in their reference sections: [stop does not cancel in-flight creation](ffmpeg.md#refresh-failure-and-stop), [remux preflight samples only part of a file](ffmpeg.md#profiles-and-remux-fallback), and [actual-start resolution is advisory](ffmpeg.md#timeline-seeking-and-resume). They were not treated as resolved defects or stronger guarantees.

## Verification

| Check | Result |
| --- | --- |
| `make test-server` | Passed with `IGLOO_FFMPEG_PATH` and `IGLOO_FFPROBE_PATH` pointing to the local Jellyfin Linux payloads; Make supplied `externalbin sqlite_fts5` and CGO |
| `make test-web` | Passed: 164 files, 1,573 tests |
| `make test-openapi` | Passed Redocly lint and registered-route coverage |
| Binary version checks | Linux FFmpeg and ffprobe executed and reported `8.1.3-Jellyfin`; macOS ARM64 payloads were only inspected as Mach-O files with matching version strings |
| Documentation checks | Relative links/anchors, request examples, referenced constants, relocation, whitespace/diff scope, and preservation of the known-issues file checked |

The backend run included real CPU media fixtures, subtitle HTTP/timestamp tests, range/conditional contract tests (including `412`), session/admission/shutdown tests, remux validation, scanner/artwork tests, and hardware argument-selection tests. Opt-in movie/show benchmarks and `TestSampleLibraryReadOnly` were skipped; process-helper entrypoints were skipped in the parent process and are exercised by their parent tests. No explicit-audio fixture cases were skipped.

No macOS execution, GPU playback, live external-client playback, live provider integration suites, or Playwright browser playback was run. The requested unit/integration checks passing does not establish those paths. No archive-origin checksums or unsupported performance/compatibility claims were retained.

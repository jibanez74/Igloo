# FFmpeg and OpenAPI verification — 2026-11-02

This follow-up checks the API contract and fixes the three runtime mismatches identified by the earlier documentation audit. No routes, request fields, dependencies, schema migrations, or default AAC behavior were added or changed.

## Contract review

The inventory covers 166 operations across 134 paths and all 300 shared schemas in [OpenAPI](openapi.json). Review included route registration, cookie/bearer and admin/ownership checks, request validation and defaults, serialized payloads and nullability, response statuses, headers, direct-file conditionals/ranges, HLS, subtitle transport, and WebSocket upgrades/events. Static inspection of handler calls found no additional undocumented handler status codes. The existing unfiltered API suite requires a validated successful exchange for every JSON-success operation, including populated collection fixtures; media tests separately validate non-JSON responses, range/conditional variants, and HEAD behavior.

New `TestOpenAPIAllRoutesMiddlewareErrors` checks missing/invalid credentials and failed cookie/bearer database access across the operation inventory. Public session-backed endpoints are also checked with a failing session load. Existing real-HTTP HEAD tests retain body-suppression coverage. The contract now describes the shared JSON decoder's existing 1 MiB limit, single-value/unknown-field rules, and 15-second read deadline; malformed and oversized JSON bodies remain `400`. Movie deletion retains its documented optional-body exception.

`GET /api/openapi.json` remains public and outside session middleware. The router test verifies exact embedded bytes, `application/json`, `no-cache`, `nosniff`, no session cookie or `Vary: Cookie`, and success with absent, invalid, combined, and malformed credentials while the database is available or closed. The embedded copy and frontend schemas are refreshed through Make targets.

## Resolved findings

| Finding | Correction and evidence |
| --- | --- |
| Explicit Dolby audio omitted the resolved layout and discarded LFE in stereo downmixes | The centralized [argument builder](../server/cmd/internal/ffmpeg/ffmpeg_hls.go) passes `aresample=out_chlayout=<resolved layout>:lfe_mix_level=0.70710678:rematrix_maxval=1`. Codec, channel ceiling, bitrate, sample rate, selected-track mapping, video fallback, and session identity remain unchanged. Documentation distinguishes channel preservation/mixing from encoder layout-label normalization. |
| XSUB reached WebVTT extraction instead of returning `415` | The shared [subtitle classification](../server/cmd/internal/helpers/subtitles.go) includes XSUB, removing its duplicate classification from the Matroska-only rejection list. Both movie and episode routes reject PGS/DVD/DVB/XSUB before FFmpeg, with contract-validated JSON errors. |
| Old WebVTT work could repopulate an invalidated cache | [WebVTT handling](../server/cmd/api/subtitle_handler.go) captures a generation before source metadata resolution, includes it in extraction deduplication, and synchronizes publication and hit renewal with invalidation. Track copies are blocked before advancing the WebVTT generation. A global counter follows the existing cache pattern without an ever-growing per-file generation map; eviction itself stays file-specific. |

Regression tests reproduced missing LFE contribution, XSUB extraction, and post-invalidation requests joining stale extraction before the fixes. They pass after the fixes. Old requests may finish with their original cues, but cannot publish over a newer cache entry; new requests do not wait for an old generation. Tests cover invalidation during metadata resolution and extraction, stale renewal, file/kind separation, combined-episode cache sharing, rebasing, cached-copy conversion/fallback, request cancellation, extraction timeout, and shutdown.

The new real-media signal suite uses the local Linux `8.1.3-Jellyfin` pair. For **both AC-3 and E-AC-3**, it generates mono, stereo, 5.1, 5.1(side), DTS 5.1, and 7.1 input; starts HLS at one second; verifies codec, encoded layout, channel count and 48 kHz output; then decodes the output and checks signal energy in the expected channels. Isolated center, left/right surround, and LFE stereo mixes and separate 7.1 side/back contributions prevent silent-fixture false positives. Existing tests retain default AAC copy/fallback, video-only, track selection, remux fallback, bitrate and session-key coverage. This is generated-fixture evidence, not a listening-quality assessment or proof of every channel layout.

## Unresolved findings

| Finding | Evidence and client impact |
| --- | --- |
| Direct watch-room admission omits the web client's audio-codec gate | [Room creation](../server/cmd/api/watch_room_handler.go) checks track selection/defaults, container and video, but not the first audio codec. [Web eligibility](../web/src/lib/playback.ts) also checks an audio allowlist. Server acceptance does not establish browser playback support. This accurately documented limitation is outside this task. |

## Review of existing known-issues reports

| Existing report | Audit status |
| --- | --- |
| Forced subtitles identified only by their title are not automatically forced | Fixed after the audit: [scanner conversion](../server/cmd/internal/scanner/streams.go) also reads a title that names the track as forced (`titleMarksForced`). Existing rows change when their file is rescanned as changed or the database is recreated. |
| One large track extraction delays other copies | Confirmed scheduling limitation: [track cache](../server/cmd/api/subtitle_track_cache.go), `cacheSubtitleTrack`, serializes work through `SubtitleTrackExtractSlot`. The reported NAS throughput and duration were not remeasured. |
| Cancellation may keep a TV subtitle request alive until the next keepalive | The server's silence interval and request-context cancellation are confirmed in [subtitle streaming](../server/cmd/api/subtitle_stream_handler.go). The Media3 socket/cancellation timing report was not reproduced; server behavior alone does not prove it. |
| DTS-to-E-AC-3 HLS resume creates unusable initialization output | Not reproduced in this audit. New signal tests start DTS 5.1 playback at one second and successfully decode both Dolby HLS outputs. That generated fixture does not reproduce the reported source or establish Media3 behavior. Keep the reported missing-`mvex`/Media3 failure as an open report, not intended behavior. |
| A particular Oppenheimer file has invalid Cues and slow seeks | The named source was not inspected. Bounded keyframe lookup and missing-index fallback are confirmed, but do not validate that file or its reported seek throughput. |
| QA deployment lacks subtitle routes | Not checked against the remote deployment. The routes exist locally and subtitle HTTP tests passed. |
| TV double subtitle opens, chapter focus, and memory budget | External-client reports not verified from this repository or on a TV device. No assertions about those client implementations were retained as server guarantees. |
| Web tests time out under heavy load | The unrestricted run hit 41 failures in three route/transition files; all 44 tests in those files passed with one worker, and the full 1,573-test suite passed with four available CPUs. No tests or timeouts changed. |

Other confirmed limitations are documented in their reference sections: [stop does not cancel in-flight creation](ffmpeg.md#refresh-failure-and-stop), [remux preflight samples only part of a file](ffmpeg.md#profiles-and-remux-fallback), and [actual-start resolution is advisory](ffmpeg.md#timeline-seeking-and-resume). They were not treated as resolved defects or stronger guarantees.

## Verification

| Check | Result |
| --- | --- |
| `make test-openapi` | Passed Redocly lint and registered-route coverage |
| `make generate-openapi` and generated-type verification | Passed; generation is reproducible against the intended updated TypeScript file. The existing drift target compares against the Git index, so verification used a temporary index containing that file, leaving the user's staging area unchanged. |
| Focused media/contract regression tests | Passed with the local Jellyfin Linux binaries; no explicit Dolby signal cases skipped |
| Focused `go test -race` | Passed subtitle/cache, committed-invalidation, public OpenAPI serving, and full-inventory middleware-error tests with `externalbin sqlite_fts5` and CGO |
| `make build-web` | Passed TypeScript checking and production build |
| `taskset -c 0-3 make check` | Passed contract lint/route coverage, generated-type verification, server vet and both dead-code passes, full backend tests, web lint, all 164 web test files / 1,573 tests, and the production build |

The initial unrestricted `make check` passed contract checks, backend vet/dead-code analysis and tests, and web lint, then hit 41 web-test failures across three route/transition files. All 44 tests in those files passed with `--maxWorkers=1` and no code changes. The complete check was rerun with four available CPUs (`taskset -c 0-3 make check`) to bound test-runner contention. No assertions, timeouts, tests, or lint rules were weakened.

Backend opt-in movie/show benchmarks and `TestSampleLibraryReadOnly` were skipped. Process-helper entrypoints skip in the parent process and are exercised by their parent tests. The embedded document matches the source byte for byte; generated artifacts, local documentation links, and final diff whitespace were checked.

Browser verification ran the existing movie-player, episode-player, playback-settings, watch-room, HLS-transcode, and direct-play-media suites in Chromium: **17 passed, 11 skipped**. The skipped checks require a configured live instance, credentials, and matching media fixtures; the passing browser cases use the repository's mock API.

No macOS execution, GPU playback, TV/Media3 playback, or live provider integrations were performed. Linux CPU media fixtures and hardware argument-selection tests do not establish those paths. The direct-room audio eligibility limitation and source-specific external-client reports remain open.

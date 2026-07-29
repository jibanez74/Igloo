# Igloo Web HLS Playback Audit

Date: 2026-07-28. Branch `fix/hls-playback` at `31ea3b0b`.

Companion to `docs/web-direct-playback-audit.md`, which explicitly excluded everything
audited here (its §1, "What was not audited": HLS manifest and segment generation, FFmpeg
encoding, hardware acceleration, HLS session cleanup, FFmpeg process management, and
hls.js lifecycle/recovery internals).

---

## 1. Scope and methodology

### 1.1 What was audited

The HLS delivery path end to end: pipeline selection, FFmpeg argument construction,
manifest and segment generation, segment readiness and seeking, session and process
lifecycle, watch-room HLS, hardware-acceleration resolution, the hls.js integration, and
the HLS-specific parts of the player UI.

Direct playback appears only where it decides whether HLS is used, hands off to it, or
reports the resulting mode. The direct-playback audit is treated as context, not as
evidence; every HLS-relevant claim it makes was re-verified here (§6.4, §12.1).

### 1.2 Three architectural facts that reshape this report

These are not defects. They are stated once so later sections do not repeatedly report
absent features as broken ones.

1. **There is no ABR ladder and no master playlist.** Every session serves a single media
   playlist; the requested profile is a path segment (`/hls/{profile}/playlist.m3u8`).
   `#EXT-X-STREAM-INF`, `BANDWIDTH`, `AVERAGE-BANDWIDTH`, `CODECS`, `RESOLUTION`,
   `FRAME-RATE` and `CLOSED-CAPTIONS` do not exist anywhere in the codebase. Quality is
   chosen before playback and changing it creates a new session.
2. **Subtitles are never part of HLS output.** `buildHLSArgs`
   (`server/cmd/internal/ffmpeg/ffmpeg_hls.go:68`) emits no `-c:s`, no `-map 0:s`, no
   `subtitles=`/`ass=` filter and no burn-in, under any profile. Subtitles are a sidecar
   `<track>` element pointing at `/api/movies/{id}/subtitles/{i}/web.vtt`. There is no
   `#EXT-X-MEDIA:TYPE=SUBTITLES` rendition, no `FORCED` attribute, and no subtitle group.
   §10 therefore audits the sidecar path *as it interacts with HLS*.
3. **Audio renditions do not exist either.** Exactly one audio stream is muxed into the
   variant. There is no `#EXT-X-MEDIA:TYPE=AUDIO`, no `GROUP-ID`, no `CHANNELS`
   attribute. Changing audio track creates a new session rather than switching rendition.

A consequence worth stating up front: because the delivered playlist is a plain VOD media
playlist with `#EXT-X-ENDLIST`, **clients must not reload it** (RFC 8216 §6.2.1). Anything
wrong in the first manifest a client receives stays wrong for the whole session. This is
load-bearing for H1 and H2.

### 1.3 Method and evidence quality

Findings are graded:

- **Confirmed** — reproduced against this repository, with the artifact named.
- **Likely** — follows necessarily from code that was read, but the user-visible
  consequence was not itself reproduced.
- **Hypothesis** — plausible, not established; the settling experiment is named.

Empirical work used two arms, both isolated from the user's real data:

- **Offline arm.** The embedded release binary
  (`server/cmd/internal/ffmpeg/ffmpeg_linux_amd64`, `7.1.4-Jellyfin`) run directly against
  library files with the argv reconstructed from `buildHLSArgs`, bounded by `-t`.
- **Live arm.** A throwaway build (`go build -tags "externalbin sqlite_fts5"`) on port
  8099 against a **copy** of `db/igloo.db` in the scratchpad, with `transcode_dir` pointed
  at the scratchpad and a default admin seeded into the copy. The real database, the real
  `./transcode`, and the user's account were never written.

The reconstruction was validated rather than assumed: the argv captured from
`/proc/<pid>/cmdline` of a live session is byte-identical to the offline argv apart from
the `-t` bound (§7.1, artifact `C1-real-argv.txt`). Every offline measurement is therefore
a statement about production behaviour.

**Browsers were deliberately excluded from this audit's scope.** Findings whose
user-visible consequence depends on hls.js or MSE internals are graded **Likely** and
carry the exact browser experiment needed to settle them (§20.3).

Note on binaries: dev and test builds use the `externalbin` tag and resolve `ffmpeg` from
`PATH` (here Ubuntu 6.1.1); release builds embed Jellyfin 7.1.4. Offline work used the
**embedded** binary, so it reflects release behaviour. Both builds were checked for every
option the code emits and both support all of them (§8.1, artifact `A1-caps.txt`).

### 1.4 Library composition (constrains what is reachable)

Measured from the movie library:

| Property | Count | Consequence |
| --- | ---: | --- |
| HEVC Main 10, `smpte2084` | 146 | Always full transcode + software tone-map |
| HEVC Main 10, bt709/null | 94 | Always full transcode |
| H.264 High 8-bit `yuv420p` | 159 | The only remux-eligible sources |
| H.264 at ≥1440p | **0** | Remux is unreachable above 1080p |
| AAC streams with profile `LC` | 200 | Safe to copy |
| AAC streams with **no profile recorded** | 35 | Copied blind — could be HE-AAC (H15) |
| AAC streams with ≥6 channels | 104 | The live `-c:a copy` multichannel case |

The library is mounted over CIFS (Tailscale) at roughly 3.1 MB/s. Sustained read of a 4K
source at 40 Mbps needs ~5 MB/s just to keep 1× realtime, so `-readrate 4` is never the
binding constraint for 4K and the mount is. The mount is `soft`, so read timeouts surface
to FFmpeg as I/O errors rather than hangs.

---

## 2. Current HLS architecture

### 2.1 Lifecycle, personal playback

1. **Mode selection is client-side.** `getAvailableModes` (`web/src/lib/playback.ts:195`)
   filters `STREAM_MODES`; `resolvePlaybackSettings` (`:258`) picks the effective mode from
   user preferences, the bandwidth-derived `recommendedProfileId`, and deep-link search
   params. `resolveModeForAudioTrack` (`:231`) upgrades `direct` → `remux` whenever a
   non-first audio track is selected. `isHlsPlayback = resolvedMode !== "direct"`
   (`useMoviePlaybackData.ts:130`).
2. **URL construction.** `buildMovieStreamUrl` (`web/src/lib/movie-playback.ts:169`) emits
   `/api/movies/{id}/hls/{mode}/playlist.m3u8?playback_session=<uuid>&start=<sec>[&audio_track=<n>][&reload=<n>]`.
   The UUID comes from `getOrCreateMovieHlsPlaybackSessionId` (`:52`), persisted per movie
   in `sessionStorage`.
3. **Manifest handler.** `HLSManifest` (`server/cmd/api/hls_handler.go:52`) →
   `parseHLSParams` (`:258`) → `GetOrCreateHLSSession` (`hls_session.go:704`), singleflight
   per key, personal-session reservation, then `createHLSSession` (`:929`).
4. **Profile decision.** `createHLSSession` loads stream rows, picks `primaryVideoStream`,
   resolves the `audio_track` ordinal to an absolute ffprobe index, computes
   `requestedProfile` / `effectiveProfile` / `fallbackProfile`, and applies the remux gate
   (§3).
5. **Process start.** `startHLSSession` (`:550`) creates `igloo-hls-*` under the transcode
   dir, acquires a CPU permit for non-copy sessions, and calls `RunHLS`
   (`ffmpeg_hls.go:333`) on `context.Background()` so FFmpeg outlives the request.
6. **Playlist body.** `buildHLSPlaylistBody` (`hls_handler.go:144`) serves
   `rewritePlaylistURLs(FinalPlaylist)` once FFmpeg has exited cleanly, otherwise
   `generateVODPlaylist` (`hls_playlist.go:84`).
7. **Segments.** `HLSSegment` (`:102`) re-derives the cache key, checks ownership
   (`canAccessPersonalHLSSession`), refreshes the TTL, and calls `serveReadyHLSSegment`
   (`:156`), which polls `segmentComplete` (`:321`) every 25 ms up to 120 s.
8. **Client playback.** `VideoPlayer.tsx:169-250` dynamically imports `hls.js/light`,
   constructs `Hls`, calls `loadSource` then `attachMedia`.
9. **Teardown.** `pagehide` and effect cleanup in `play.tsx:289-324` call
   `stopMovieHlsPlaybackSession`; otherwise TTL expiry drives `OnEvicted` →
   `cleanupHLSSession` (`hls_session.go:480`).

### 2.2 Watch-room lifecycle (differences only)

Cache key `room:<id>` (`RoomHLSSessionKey`, `hls_session.go:163`), 30-minute TTL, warmed at
room creation from `startSec=0` with an empty `playbackSession`
(`GetOrCreateRoomHLSSession:822`). `watchRoomStreamUrl` (`web/src/lib/watch-room.ts:4`)
emits `/api/watch-rooms/{id}/hls/playlist.m3u8` — **no profile, no start, no
playback_session, no reload**. All participants share one FFmpeg process and one temp dir;
synchronisation is client-side over the WebSocket hub.

### 2.3 Component inventory

| Layer | Location |
| --- | --- |
| Routes | `server/cmd/api/routes.go:172-177` (personal), `:219-222` (room) |
| Handlers | `hls_handler.go`, `watch_room_media_handler.go` |
| Session manager | `hls_session.go` (cache, keys, admission, profile decision, lifecycle) |
| Playlist builder | `hls_playlist.go` |
| Remux safety | `hls_remux_safety.go`, `ffmpeg/remux_validator.go` |
| Limiter | `hls_limiter.go` |
| Arg builder / runner | `ffmpeg/ffmpeg_hls.go` |
| Capability probes | `ffmpeg/capabilities.go` |
| FFprobe parse / persist | `ffprobe/ffprobe_metadata.go`, `movies_scanner.go:917` |
| Player | `web/src/components/playback/VideoPlayer.tsx` (sole hls.js site) |
| Orchestration | `web/src/routes/_auth/movies/$id/play.tsx` |
| HLS hooks | `useHlsCapacityRetry.ts`, `useHlsSessionKeepalive.ts`, `useHlsSessionRecovery.ts` |
| Watch room | `WatchRoomPage.tsx`, `useWatchRoomConnection.ts` |

---

## 3. HLS pipeline-selection assessment

### 3.1 The available pipelines

| Pipeline | Reached when | Args |
| --- | --- | --- |
| Copy video + copy audio | `remux` requested, gate passes, audio codec is `aac` | `-c:v copy -c:a copy` |
| Copy video + transcode audio | `remux`, gate passes, audio ≠ `aac` | `-c:v copy -c:a aac -ac 2 -b:a 320k` |
| Transcode video + copy audio | any non-remux profile **and** audio codec is `aac` | encoder + `-c:a copy` |
| Full transcode | any non-remux profile and audio ≠ `aac` | encoder + `-c:a aac -ac 2 -b:a 320k` |
| Subtitle conversion | never in HLS (sidecar endpoint) | — |
| Subtitle burn-in | not implemented | — |
| Rejection | no playable video stream (`createHLSSession:946`) | — |

All five reachable combinations are genuinely distinct: `startHLSSession:557` sets
`copyAudio = audioCodec == "aac"` independently of the video decision, and
`buildHLSArgs:202` honours `CopyAudio` in every branch. So "HLS" spans everything from a
pure stream copy to a full re-encode, and a transcode profile on an AAC source correctly
copies the audio rather than needlessly re-encoding it.

### 3.2 What the decision considers

Considered: container (client-side only, for `direct`), video codec name, codec profile
string, bit depth, pixel format, source height (fallback selection), HDR `color_transfer`,
audio codec name, selected tracks, user-selected quality, encoder availability and
hardware runtime probes, CPU permit availability, per-user session capacity.

**Not considered:** video level, chroma subsampling other than via the pixel-format
allowlist, frame rate (except for GOP sizing), audio profile (§9.1), channel count for the
copy decision (§9.1), sample rate, subtitle codec (irrelevant — no subtitles in HLS), and
native-HLS support (the server has no idea whether the client is Safari).

### 3.3 The remux gate

Two layers, both in `createHLSSession:977-1016`:

- **Static:** `isBrowserSafeH264RemuxCandidate` (`hls_session.go:88`) rejects non-H.264
  codec names, `bit_depth > 8`, pixel formats outside the allowlist
  `{yuv420p, yuvj420p, nv12, nv21}`, and profile strings containing `10`/`4:2:2`/`422`/
  `4:4:4`/`444`. Rejection caches an unsafe verdict for 24 h and falls back.
- **Dynamic:** on cache miss, FFmpeg is started in remux mode, `waitForRemuxPreflight`
  blocks for `init.mp4` plus 4 complete segments (≤30 s), and `ValidateRemuxSafety` parses
  the fMP4 boxes asserting sync samples begin with IDR NALs. Failure to *validate* caches
  unsafe; failure to *wait* falls back without caching, correctly treating timeouts as
  transient.

This is a well-designed gate and it works. Verified: `remux safety validated … checked_segments=4
checked_sync_samples=4` for movie 57, and independently every segment of the offline
remux output begins with a keyframe (§5.4).

### 3.4 The unnecessary-transcode question

- **Video transcoded when only audio is incompatible?** No. Remux copies video and
  transcodes only audio.
- **Audio transcoded unnecessarily?** Yes, in one case: any non-AAC audio is re-encoded
  even when the container/codec pair would be fine, and it is always downmixed to stereo
  (§9.2).
- **Subtitles forcing video transcode?** No — subtitles never touch the HLS pipeline.
- **Requested remux silently becoming a full transcode?** **Yes, and invisibly** — see H3.

### 3.5 H3 — the effective profile is never exposed (Confirmed, High)

`effectiveProfile` exists only as a local in `createHLSSession`, a field on the internal
`hlsSessionStartParams` (`hls_session.go:70`), and a structured-log key. It is **not** a
field on `HLSSession`, **not** in `docs/openapi.json`, and **not referenced anywhere in
`web/src`**.

Reproduced against the live sandbox (artifact `C6-headers.txt`, `C6-hevc-remux.m3u8`):

```
GET /api/movies/120/hls/remux/playlist.m3u8?...   → HTTP 200
Cache-Control: no-store
Content-Type: application/vnd.apple.mpegurl
(no header naming the effective profile)

first segment URL: /api/movies/120/hls/remux/segment_0.m4s?...

server log: level=WARN msg="remux safety fallback engaged" movie_id=120
            requested_profile=remux effective_profile=2160p_16mbps
            validation_result=unsafe
            fallback_reason="requested remux is not supported for codec \"hevc\""
```

The client asked for a cheap stream copy and received a **2160p 16 Mbps libx264
transcode**. The response is a 200, the asset URLs still say `/hls/remux/`, and the player
badge — `modeLabel`, rendered at `MoviePlayerControls.tsx:143-146` from the *client-side*
resolved mode — still reads "Remux". The only observable difference is
`#EXT-X-TARGETDURATION:8` instead of `30`, which no client interprets as a mode signal.

This is precisely the mode-reporting defect that finding D9 of the direct-playback audit
fixed for direct play, left unfixed on the HLS side.

Severity is amplified by resource cost: on this machine a 1040p→720p CPU transcode ran at
4.38× realtime (§8.3); a 2160p→2160p 16 Mbps transcode over a 3.1 MB/s mount will not
sustain realtime, so the user experiences unexplained stalling on a mode they believed was
a stream copy.

### 3.6 Repeated recreation of a failing pipeline

The cache key uses the **requested** profile (`HLSSessionKey`, `hls_session.go:157`), so a
fallen-back session is cached under `…:remux:…` and reused — the fallback is computed once
per session, not per request. The 24 h `remuxSafetyVerdict` cache prevents re-running
preflight. Static rejections cache immediately. No recreation loop was found at the server
layer.

The client layer is different: see H2, where a 404 tail drives
`useHlsSessionRecovery` through three recreate-and-fail cycles.

---

## 4. Frontend and hls.js assessment

`hls.js` `^1.6.16`, imported as `hls.js/light` via a dynamic `import()`
(`VideoPlayer.tsx:44`). One instantiation site in the whole application.

### 4.1 Initialization and lifecycle

Correct: dynamic import keeps hls.js out of the main bundle; `xhrSetup` sets
`withCredentials`; `backBufferLength: 120` bounds memory; the `cancelled` flag plus
`disposeHls` closure handles React 19 Strict Mode double-invocation; `destroy()` runs on
unmount, source change, and `startSec` change. `startSec` is deliberately in the dependency
array so a resume-target change rebuilds the instance (locked by the test
`"rebuilds the hls.js instance when startSec changes on the same URL"`,
`src/test/playback/video-player.test.tsx:232`).

`loadSource()` is called before `attachMedia()` (`:199-200`), the inverse of the hls.js
documentation example. Both orders are supported; no defect.

**H10 (Likely, Low) — a late cleanup can null a live instance ref.** `hlsRef.current` is
assigned unconditionally at `:193` and set to `null` unconditionally at `:196`. If
instance A's cleanup runs after instance B was created (possible when the dynamic import
of the second effect run resolves before the first run's cleanup), the ref holding B is
nulled. The ref is only read by the start-seek effect (`:273`, `if (hlsRef.current) return`),
so the observable consequence is that the native seek path can compete with hls.js's
`startPosition`. Narrow, but the fix is a one-line identity check.

**H11 (Confirmed, Medium) — unsupported browsers get a blank player.** At `:180`,
`if (cancelled || !Hls.isSupported()) return;` exits silently: no `onError`, no message,
no fallback. The user sees a video element that never loads. Every browser that supports
neither MSE nor native HLS lands here.

### 4.2 Native HLS

`supportsNativeHLS` (`web/src/lib/playback.ts:42`) is a module-load IIFE testing
`canPlayType` for `application/vnd.apple.mpegurl` and `application/x-mpegURL`. When true,
HLS is handed to the native element via `video.src` (`VideoPlayer.tsx:256-266`).

Consequences not currently handled:

- On native HLS (Safari, iOS) **none of the hls.js error routing exists**. There is no
  session-lost detection, no 503 capacity handling, no `recoverMediaError`. A 404 tail
  (H2) surfaces as a generic media error.
- The server never learns the client is native, so profile decisions cannot account for
  Safari's codec support (e.g. HEVC in fMP4, which Safari could play directly and which
  Igloo always transcodes).

Both are graded **Confirmed** as code facts; their user impact on Safari was not exercised.

### 4.3 State synchronisation

The UI reflects the **requested** mode only. `modeLabel` derives from the client's resolved
mode; nothing consumes an effective profile because none is published (H3). Quality level,
available levels, current audio rendition and current subtitle rendition are not read from
hls.js at all — there is nothing to read, since the manifest is single-variant with no
renditions (§1.2). Buffering state comes from media-element events
(`VideoPlayer.tsx:72-99`), not from hls.js. Capacity state is surfaced (`play.tsx:701-715`
with a polite `LiveAnnouncer`). Recovery attempts are not surfaced at all.

No cross-movie state leakage was found: `sessionWindowKey`
(`useMoviePlaybackData.ts:171`) includes movie, mode, audio track, playback-session UUID
and floored start, and all three HLS hooks reset their counters when it changes.

### 4.4 Error handling

`VideoPlayer.tsx:209-238` routes: `FRAG_LOAD_ERROR` + HTTP 404 → `onSessionLost`;
fatal `MANIFEST_LOAD_ERROR`/`LEVEL_LOAD_ERROR` + HTTP 503 → `onCapacityBusy` with
`Retry-After`; fatal media error → one `recoverMediaError()`; everything else fatal →
`onError`. Non-fatal errors are left to hls.js. Budgets live in the hooks:
`HLS_SESSION_LOST_MAX_ATTEMPTS = 3` with a 2 s minimum interval,
`HLS_CAPACITY_RETRY_MAX_ATTEMPTS = 6`.

This is a sound structure and it matches hls.js 1.6 (no deprecated patterns;
`recoverMediaError` and `ErrorDetails` are current). Three gaps:

- **Error types are string-cast rather than taken from the enum** (`:48-49`,
  `const HLS_NETWORK_ERROR = "networkError" as ErrorData["type"]`). This compiles even if
  the upstream enum values change, and would silently stop matching. Use
  `Hls.ErrorTypes.NETWORK_ERROR`, which is already available in the closure.
- **`recoverMediaError` is budgeted per instance, not per window**, and there is no
  `swapAudioCodec` second-stage recovery. One attempt then a terminal error.
- **A 404 on a *segment* is unconditionally interpreted as "session lost"** even when it
  actually means "this segment never existed" (H2). The client's response — recreate the
  session — cannot help, and burns the budget.

### 4.5 Keepalive and recovery

`useHlsSessionKeepalive` refetches the manifest every 120 s while playback is ready
(server idle TTL is 300 s), swallowing errors. Because the keepalive hits `HLSManifest`,
which calls `GetOrCreateHLSSession`, it does not merely refresh a TTL — **it will recreate
an evicted session**, which is a deliberate and effective transparent-recovery mechanism.

`useHlsSessionRecovery` rate-limits to one attempt per 2 s and 3 per window, calling
`navigateToPlaybackPosition(t, { forceReload: true })`.

---

## 5. Manifest and rendition assessment

### 5.1 What is emitted

`generateVODPlaylist` (`hls_playlist.go:84`) synthesises, before FFmpeg has produced
anything:

```
#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:8   (30 when session.CopyVideo)
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-MAP:URI="<base>init.mp4<query>"
#EXTINF:4.000000,
<base>segment_0.m4s<query>
… ceil(duration/4) entries, all 4.000000 except a clamped last one …
#EXT-X-ENDLIST
```

Tag correctness: `#EXTM3U`, `#EXT-X-VERSION:7` (appropriate for fMP4 + `EXT-X-MAP`),
`MEDIA-SEQUENCE`, `PLAYLIST-TYPE`, `EXT-X-MAP` and `ENDLIST` are all well-formed and
correctly ordered. `#EXT-X-INDEPENDENT-SEGMENTS` is absent (see §5.5). No master-playlist
tags exist (§1.2).

### 5.2 H1 — synthesized segment durations are fiction for copy-video (Confirmed, Critical)

The playlist is generated from movie duration alone. For transcodes this is exact, because
`-force_key_frames:0 expr:gte(t,n_forced*4)` pins every keyframe to a 4 s boundary. For
**copy-video** the output can only split at *source* keyframes, which are wherever the
original encoder put them.

Measured with the production argv (artifacts `B1-extinf-drift.tsv`,
`B2-extinf-drift.tsv`):

| Source | Mode | Content | Real segs | Advertised | Mean `EXTINF` | Min–Max | Cumulative drift |
| --- | --- | ---: | ---: | ---: | ---: | --- | ---: |
| Movie 57 (h264 1040p 23.976) | remux | 600 s | 94 | 150 | **5.403 s** | 1.001–10.010 | **+131.9 s** |
| Movie 191 (h264 768p 29.97) | remux | 300 s | 74 | 75 | 4.055 s | 0.834–8.976 | ≈ 0 |
| Movie 131 (h264 752p) | remux | 120 s | **13** | 30 | **9.240 s** | 1.168–12.429 | large |
| Movie 57 | 720p transcode | 300 s | **75** | 75 | **4.000 s** | 3.962–4.004 | **+0.008 s** |

The transcode row is the control: the mechanism is sound, and the defect is **specific to
copy-video**. Its magnitude is entirely a property of the source's keyframe spacing, so it
ranges from harmless (movie 191) to severe (movie 131, where 13 real segments are
advertised as 30).

One deviation to record: the transcode control was run with `-c:a aac -ac 2 -b:a 320k`,
whereas the server would emit `-c:a copy` for this source (its audio is AAC, §3.1). HLS
segment boundaries are chosen at video keyframes, so the audio codec does not affect the
durations being measured; the remux rows, which carry the finding, used the argv verified
identical to production (§7.1).

Two consequences follow, and they differ in how certain they are:

- **Playlist timing is objectively wrong** (Confirmed). At segment 10 of movie 57 the
  playlist says content starts at 40.000 s; the fragment's own timestamp says 70.070 s
  (§5.4). A player that trusts `EXTINF` maps seek targets to the wrong segment.
- **Whether the *user* mis-seeks depends on hls.js** (Likely, not Confirmed). hls.js
  re-derives fragment start times from parsed fragment PTS after loading, so it may
  self-correct. Settling this needs the browser experiment in §20.3-B6. Note the
  correction cannot help the *first* seek into an unloaded region, which is precisely the
  common case.

### 5.3 H2 — the playlist advertises segments that will never exist (Confirmed, Critical)

Because the segment *count* is `ceil(duration/4)` while real segments are longer, FFmpeg
produces fewer files than the playlist lists. For movie 57 the live server emitted a
playlist with **1458** `#EXTINF` entries (artifact `C1-playlist-remux.m3u8`); at the
measured 5.403 s mean, FFmpeg will produce roughly **1079** — about 379 phantom entries.

Reproduced decisively on a short tail session (artifact `C5-tail.m3u8`):

```
GET /api/movies/57/hls/remux/playlist.m3u8?start=5800  → 8 #EXTINF entries
server log: msg="hls session finished" movie_id=57 elapsed=15s   (clean exit)
segments actually written to the temp dir: 1
GET segment_0 → 200
GET segment_1..7 → 404 404 404 404 404 404 404
```

FFmpeg's own playlist for that session contains exactly 1 `#EXTINF`. The server advertised
eight 4 s segments where the source's keyframe spacing produced a single 28 s one.

The 404 comes from `serveReadyHLSSegment` (`hls_handler.go:176-183`): the session has
exited cleanly and the file does not exist, so it returns `404 segment does not exist`.

**The client turns this into a recovery loop rather than a clean end.**
`VideoPlayer.tsx:210-217` classifies any `FRAG_LOAD_ERROR` with `response.code === 404` as
session-lost, so `useHlsSessionRecovery` recreates the session up to three times; each
recreation produces the same too-long playlist and the same 404. The user sees playback
fail near the end of the movie with a session error.

Crucially, **the corrected playlist never reaches the client.** Once FFmpeg exits cleanly
the server does serve `FinalPlaylist` with true durations (`buildHLSPlaylistBody:149`) —
but the client is holding a `VOD` + `ENDLIST` playlist, which RFC 8216 forbids reloading,
and hls.js does not reload it. The keepalive refetch (`useHlsSessionKeepalive`) is a bare
`fetch()` whose body is discarded. So the accurate playlist is generated and thrown away.

### 5.4 Fragment-level correctness (verified good)

Per-fragment probing of the offline remux output (embedded 7.1.4 binary):

| Segment | First video PTS | Keyframe? | Playlist cumulative start |
| --- | ---: | --- | ---: |
| 0 | 0.083 | yes | 0.000 |
| 1 | 8.550 | yes | 8.467 |
| 2 | 14.556 | yes | 14.473 |
| 3 | 24.566 | yes | 24.483 |
| 10 | 70.070 | yes | 69.987 |

Timestamps are continuous and monotonic across fragments, and **every segment begins on a
keyframe** — `movflags=+frag_discont` does not reset `baseMediaDecodeTime`. The output is
structurally sound; the defect is in the *description* of it, not the media.

(An earlier "non-monotonically increasing dts" signal was an artifact of naively
concatenating `init.mp4` with all segments and remuxing; it appears identically in the
known-good transcode output and is not a defect.)

### 5.5 Minor manifest gaps

- **`#EXT-X-INDEPENDENT-SEGMENTS` is not emitted** even though it is true for transcodes
  (every segment starts on a forced IDR) and verified true for the copy-video output
  above. Declaring it lets players start decoding at any segment without extra probing.
- **`#EXT-X-TARGETDURATION:30` for copy-video is a blunt instrument.** It is legal (it
  must be ≥ the largest rounded `EXTINF`, and the largest measured was 12.429 s), but it
  is a workaround for H1 rather than a fix, and it inflates the buffering hints players
  derive from it.
- **No `#EXT-X-START`**, so a rebased session gives players no hint that it represents a
  window rather than the whole asset.

### 5.6 Renditions

Not applicable — no audio, subtitle, or video renditions exist (§1.2). The audit brief's
rendition checklist (`GROUP-ID`, `DEFAULT`, `AUTOSELECT`, `FORCED`, `CHANNELS`,
duplicate-language handling, commentary and audio-description tracks) describes a feature
set Igloo has not implemented. Whether it *should* is addressed in §18.2.

---

## 6. FFprobe metadata assessment

### 6.1 What is captured

`ffprobe -v quiet -print_format json -show_streams -show_format -show_chapters`
(`ffprobe_metadata.go:186`), parsed into `Stream` (`:21`) with `index, codec_name, profile,
level, width, height, coded_*, display_aspect_ratio, avg_frame_rate, r_frame_rate,
bits_per_raw_sample, pix_fmt, color_range, color_transfer, color_primaries, color_space,
channels, channel_layout, sample_rate, bit_rate`, plus full `StreamDisposition` (`:51`:
`attached_pic, default, forced, comment, dub, original, hearing_impaired, visual_impaired`)
and tag-key normalisation (`StreamTags.UnmarshalJSON:70`, lowercasing, separator stripping,
`lang` → `language`).

Persisted by `processMovieStreams` (`movies_scanner.go:917`), which deletes and re-inserts
all stream rows and skips `attached_pic` and cover-art video codecs.

### 6.2 Sufficiency for HLS

Sufficient for what the HLS path actually does: pipeline choice reads `codec`,
`codec_profile`, `bit_depth`, `pixel_format`, `color_transfer`, `height`, `frame_rate`;
mapping reads `stream_index`; audio copy reads `codec`.

Not captured or not used, and needed for improvements recommended later: **`codec_level`
is stored but never read by the HLS path** (it is used only for the direct-play
`canPlayType` string), and audio `profile` is stored but not consulted for the copy
decision (§9.1). There is no stored keyframe index, which is the root enabler for fixing
H1/H2 properly (§18.1).

### 6.3 H14 — stream ordinals are not stable across re-scans (Confirmed, Medium)

`audio_track` is an ordinal into `stream_index`-ordered audio rows, resolved server-side
(`createHLSSession:966-968`). `processMovieStreams` deletes and re-inserts every stream row
on each scan, and there is no `UNIQUE(movie_id, stream_index)` constraint on
`video_streams`, `audio_streams`, or `subtitles` (only non-unique indices). A re-scan that
changes stream order silently repoints:

- a user's saved `preferred_audio_language`-derived selection,
- a deep link carrying `audio_track=N`,
- **a watch room's stored `audio_track`**, which is validated once at creation and never
  re-validated.

The direct-playback audit flagged this as a structural risk (its §6.2); it is confirmed
here as unmitigated on the HLS path. Note the ordinal is *also* the subtitle track index in
`/subtitles/{trackIndex}/web.vtt`, so the same drift silently changes which subtitle plays.

### 6.4 Verification of direct-audit claims

- "the `audio_track` API parameter is an ordinal … resolved to the absolute ffprobe index
  server-side" — **verified**: the live argv is `-map 0:0 -map 0:1` with indices taken from
  `stream_index`, not from ordinal position (§7.1).
- "`primaryVideoStream` is used by both watch-room creation and `createHLSSession`" —
  **verified** (`hls_session.go:945`, `watch_room_handler.go:413-438`).
- `hlsSegmentPoll = 25ms` and `hlsRemuxPreflightPoll = 250ms` split — **verified**
  (`hls_handler.go:26`, `:31`).

---

## 7. FFmpeg stream-mapping and command assessment

### 7.1 Fidelity of the reconstruction

Live capture from `/proc/<pid>/cmdline` for `GET /api/movies/57/hls/remux/playlist.m3u8`
(artifact `C1-real-argv.txt`):

```
/usr/bin/ffmpeg -y -fflags +genpts -analyzeduration 5000000 -probesize 5000000
  -readrate 4 -readrate_initial_burst 60
  -i /home/…/The.Santa.Clause.1994.mp4
  -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1
  -c:v copy -c:a copy
  -avoid_negative_ts make_zero -max_muxing_queue_size 1024
  -f hls -hls_segment_type fmp4 -hls_segment_options movflags=+frag_discont
  -hls_playlist_type event -hls_list_size 0 -hls_time 4
  -hls_segment_filename …/igloo-hls-1183807021/segment_%d.m4s
  -hls_fmp4_init_filename init.mp4 …/igloo-hls-1183807021/playlist.m3u8
```

### 7.2 Stream mapping — correct

Explicit `-map 0:<absolute ffprobe index>` for video and, when present, audio. No reliance
on FFmpeg's default stream selection. Attached pictures and cover-art codecs are excluded
at scan time and again by `primaryVideoStream` (`movie_media.go:42`). Video-only movies
omit the audio map and all audio options (`buildHLSArgs:156-159, 201-207`) — correct, and
covered by `ffmpeg_hls_args_additional_test.go`. Ordinal→absolute translation is validated
and range-checked (`createHLSSession:963`).

`-map_metadata -1` and `-map_chapters -1` intentionally drop language tags, titles and
dispositions. That is harmless *given* §1.2 — with one stream of each type and no
renditions, there is no place for that metadata to be expressed. It becomes a defect the
moment renditions are introduced.

### 7.3 Remux/transcode arguments — correct, with one omission

`-c:v copy` / `-c:a copy` are gated correctly; segment format is fMP4 throughout, which
suits both copied H.264 and encoded H.264; timestamps are normalised with
`-avoid_negative_ts make_zero` and `-fflags +genpts`; the output filename matches the
declared format. No bitstream filter is needed for H.264-in-fMP4 from an MP4/MKV source
(the copy path was verified to produce `avc1` with valid extradata, §5.4).

**Omission: no `-hls_flags temp_file`.** Segments are written directly to their final
names, so a partially written `segment_N.m4s` exists on disk under the name a client will
request. The server compensates behaviourally (`segmentComplete` treats N as complete only
once N+1 exists, or once FFmpeg has exited) rather than atomically. The compensation is
sound for the normal path, but it is the reason `init.mp4` needs its own special-casing
and it makes the readiness rule load-bearing. `temp_file` would make the invariant
structural.

### 7.4 H7 — the audio track is logged as a pointer (Confirmed, Low)

`hls_session.go:611` logs `"audio_track", params.AudioTrack` where the field is `*int`.
Live output:

```
msg="hls session starting" movie_id=57 … audio_track=0x21c92170d160 …
```

Every HLS session log line is unusable for diagnosing track selection — the single most
likely thing to need diagnosing. Fix is `*params.AudioTrack` with a nil guard.

---

## 8. Video decoding and encoding assessment

### 8.1 Build capability (both binaries verified)

Artifact `A1-caps.txt`. System 6.1.1 (dev/test) and embedded 7.1.4-Jellyfin (release) both
expose `-hls_segment_options`, `mov`'s `frag_discont`, `-readrate`,
`-readrate_initial_burst`, `libx264`, `h264_nvenc`, `h264_qsv`, `h264_vaapi`, and the
`cuda`/`qsv`/`vaapi` hwaccels. No option Igloo emits is missing from either build, so the
`SupportsCLIOption` gating is currently always-true on this host and the capability
plumbing is untested in its negative direction.

### 8.2 Decoding

Software decode by default. `-hwaccel cuda` is added for NVIDIA only when the `cuda`
hwaccel is probed, deliberately without `-hwaccel_output_format` so frames land in system
memory and the software filter chains keep working. QSV decode is deliberately disabled.
Apple gets `-hwaccel videotoolbox`. Corrupt-frame handling, interlaced content, rotation
and VFR are not specially handled anywhere: there is no `yadif`/`bwdif` deinterlacer, no
`-noautorotate`/rotation filter, and no `-vsync`/`fps` normalisation. Interlaced or rotated
sources will transcode to interlaced or mis-rotated output.

### 8.3 Encoding

`-profile:v high`, profile bitrate as `-b:v`/`-maxrate` with a 2× `-bufsize`, `scale=-2:H`,
explicit bt709 tagging on both the stream and the filter chain tail, `-pix_fmt yuv420p`
except for QSV and CUDA filter paths, GOP `ceil(4 × fps)`, and
`-force_key_frames:0 expr:gte(t,n_forced*4)` for every encoder with `-sc_threshold:v:0 0`
for libx264.

Verified working: the transcode control run produced 75 segments of mean 4.000 s
(3.962–4.004), i.e. the keyframe pinning does exactly what it claims (§5.2).

Concerns:

- **No `-level`.** `-profile:v high` is set but the level is left to the encoder. A 2160p
  16 Mbps libx264 output will land at level 5.1+; browsers generally cope, but the stored
  `codec_level` is available and unused, and level is what `canPlayType` strings key on.
- **CBR-style rate control** (`-b:v` == `-maxrate`) with no `-crf` means easy content is
  encoded at full profile bitrate. On a bandwidth-constrained home server this wastes both
  CPU and disk.
- **Capacity headroom is thin.** 1040p→720p ran at 4.38× realtime on this box.
  `HLS_MAX_CPU_TRANSCODES` here is 5 (from `.env`); five concurrent sessions of that shape
  would land near 0.9× each — below realtime. The default (`NumCPU()/4`) is more
  conservative than the configured value.

Output is browser-compatible in shape (High profile, `yuv420p`, AAC-LC), and the encoded
path is the *safer* of the two — the copy path is where the defects are.

---

## 9. Audio transcoding assessment

### 9.1 The copy decision

`startHLSSession:557`: `copyAudio = audioCodec == "aac"`. That is the entire test — no
profile, channel-count, or sample-rate check.

- **AAC-LC 5.1 copied into fMP4 is well-formed** (Confirmed). Probing the copied output:
  `codec_name=aac profile=LC codec_tag_string=mp4a channels=6 channel_layout=5.1
  sample_rate=48000`, extradata present, audio-only decode clean (artifact
  `B4-aac51-131`). Whether every browser's MSE accepts 6-channel AAC-LC was not tested
  (browser scope excluded); Chrome and Firefox decode and downmix it in practice. Graded
  **Likely fine**, with the settling test in §20.3.
- **HE-AAC / xHE-AAC would be copied blindly.** `profile` is stored but not consulted. 200
  AAC streams in this library are known `LC`, but **35 have no profile recorded at all**
  and are copied on the strength of the codec name alone. HE-AAC v2 in fMP4 fails on some
  browsers and xHE-AAC on most, so those 35 are unverified rather than known-safe. Graded
  **Hypothesis** — settling it means probing those 35 streams' actual profiles, which is a
  one-query change to the scanner's stored metadata.

### 9.2 The transcode path

Everything non-AAC becomes `-c:a aac -ac 2 -b:a 320k`.

- **Deterministic downmix**: always stereo, so multichannel sources are handled uniformly
  and no client can fail on an unsupported layout. Reasonable default.
- **320 kbps for stereo AAC is above the useful ceiling** of FFmpeg's native encoder; the
  bits are largely wasted. 160–192 kbps would be transparent.
- **No dialogue normalisation and no loudness handling.** AC-3/E-AC-3 carry `dialnorm`
  metadata that the native AAC encoder does not apply, so a downmixed AC-3 track can be
  noticeably quieter or louder than the source. Not tested; graded **Hypothesis**.
- **Atmos/TrueHD**: `truehd` and `dts` sources are transcoded (correct — no browser
  decodes them). The object metadata is necessarily lost; nothing in the code claims
  otherwise.
- **No `-af aresample=async` or explicit sample-rate normalisation**, so an odd source rate
  passes through to the encoder.

The selected track always reaches the output: mapping is explicit and absolute (§7.2).

---

## 10. Subtitle-processing assessment

Subtitles are entirely outside the HLS pipeline (§1.2). Extraction is
`ffmpeg -v error -i <src> -map 0:<abs index> -c:s webvtt -f webvtt pipe:1`
(`ffmpeg_subtitles.go:13`), served by `SubtitleWebVTT` (`subtitle_handler.go:35`) with a
1-hour cache, singleflight collapsing, a 60 s timeout, `\h` → space normalisation, and
explicit 415 rejection of `hdmv_pgs_subtitle`, `dvd_subtitle`, `dvb_subtitle`.

Classification is correct: text subtitles are convertible, bitmap subtitles are rejected
with a clear status rather than exposed as broken text, and disabled subtitles cost
nothing. Burn-in does not exist, so **no subtitle selection can ever cause a video
transcode** — a genuine strength of this design.

### 10.1 H4 — subtitles desync on every rebased HLS session (Likely, High)

The WebVTT is extracted from the source with **absolute** timestamps: a cue at 40 minutes
carries `00:40:00.000`. The HLS session, however, is **rebased**: `-ss <start>` plus
`-avoid_negative_ts make_zero` produces output whose first PTS is ~0 (measured: 0.083 s for
`-ss 600`, §11.2). The client attaches the track raw:

```ts
// VideoPlayer.tsx:311-317
const track = document.createElement("track");
track.src = sub.url;          // absolute-timestamp WebVTT
video.appendChild(track);
track.track.mode = "showing";
```

`play.tsx` compensates *displayed* time via `toAbsolutePlaybackTime`
(`movie-playback.ts:213`), but cue rendering is done by the browser against **media** time,
which is session-local. Nothing in `web/src` rewrites cue timings — a search for
`cue`/`TextTrack` across the source finds only unrelated YouTube and chapter code.

Therefore any HLS session that does not start at 0 renders subtitles offset by
`hlsStartSec`. That includes **every resume** and **every seek that rebases the session**
(beyond the 120 s forward threshold or backwards past the start). Resuming a film at 40
minutes shows the cues for minute 0 — or, more precisely, shows nothing, because the
session's media timeline never reaches the cue times at all.

Watch rooms are unaffected (always `startSec=0`). Direct play is unaffected (media time is
absolute).

Graded **Likely** rather than Confirmed only because the on-screen result was not observed
in a browser; the mechanism is established from code and the measured rebasing. §20.3
names the two-minute experiment that settles it.

The fix is cheap and belongs on the server: `SubtitleWebVTT` should accept the session
start and emit `-ss`-shifted cues, or the endpoint should emit
`X-TIMESTAMP-MAP=MPEGTS:0,LOCAL:00:00:00.000`-style offsetting. Alternatively the client
can shift cues after `load`, but that duplicates timing logic in the browser.

---

## 11. Segment-generation and seeking assessment

### 11.1 Options in use

`-f hls -hls_segment_type fmp4 -hls_segment_options movflags=+frag_discont
-hls_playlist_type event -hls_list_size 0 -hls_time 4`, with `init.mp4` and
`segment_%d.m4s`. VOD is synthesised on top of FFmpeg's event playlist (§5.1). Segment
numbering always restarts at 0 for a rebased session, and the manifest echoes the
*effective* start into every asset URL so segment lookups stay bound to the session that
generated the manifest (`hls_handler.go:78-87`).

### 11.2 H5 — `-ss` snaps backwards and the client never learns (Confirmed, Medium)

`-ss` is placed before `-i` (fast input seek), which snaps to the nearest keyframe **at or
before** the requested time. Measured on movie 57 (artifact `B3`):

```
true keyframes near t=600:  586.419  591.174  601.184  603.770  606.231
requested start:            600.000
first output PTS:           0.083  (flagged K)
first EXTINF:               10.010  == 601.184 − 591.174
⇒ the session actually begins at 591.174 s — 8.83 s before the request
```

The client then computes absolute time as `hlsStartSec + video.currentTime`
(`movie-playback.ts:213`) using the *requested* 600, so for the entire session the UI clock
— and every watch-progress value saved from it — is ~8.8 s ahead of the picture on screen.
The error is bounded by the source GOP length, which for sparse-keyframe sources (movie
131, keyframes up to 12.4 s apart) is over 12 s.

The server knows the true offset the moment the first segment exists; it does not measure
or publish it.

Transcode sessions are unaffected in a different way: `-force_key_frames` uses output time
`t`, which starts at 0 after the input seek, so boundaries stay aligned — but the same
backwards snap to the input keyframe still applies to where the content begins.

### 11.3 Readiness and partial-file exposure

`segmentComplete` (`hls_handler.go:321`): segment N is complete when N+1 exists, or when
FFmpeg has exited and the file is non-empty; `init.mp4` is complete once `segment_0`
exists. Polling is 25 ms, capped at 120 s, and aborts on `r.Context().Done()` so a scrub
does not leave goroutines polling. This is a sound design and prevents serving
partially-written segments in the normal case.

Two edges:

- **A killed session can expose a truncated final segment** only if `ExitErr` is nil, which
  cleanup makes impossible (`cleanupHLSSession` sets `ExpectedStop` and cancels, so
  `ExitErr != nil` → 500). Safe.
- **The last real segment of a clean session** is served on the "exited and file exists"
  branch without any completeness check beyond size > 0. It is complete in practice because
  FFmpeg closes it before exiting.

### 11.4 Seeking behaviour

- **Before generation completes?** Yes. The synthesised VOD playlist lists the whole movie
  from the first request, and `serveReadyHLSSegment` blocks until the requested segment is
  produced — up to 120 s. A forward seek beyond the encoder's position therefore *works*
  but can stall for a long time, bounded by how far ahead the seek is and by `-readrate 4`.
- **Does a seek create duplicate FFmpeg processes?** No. A rebase reuses the same
  `playback_session` UUID and `cleanupPersonalHLSSessionsForOwnerLocked` removes the
  superseded window. **Verified live**: two sequential manifests with the same UUID at
  `start=0` then `start=600` left the temp-dir count unchanged at 2 (§12.2).
- **Can a seek return stale segments?** No. Segment files are session-scoped by temp dir,
  and the asset URLs carry the effective start.
- **Does a seek restart too much work?** Yes, by design — any rebase discards all encoded
  output and restarts FFmpeg. The client mitigates with a 120 s forward-rebase threshold
  (`MOVIE_HLS_FORWARD_REBASE_THRESHOLD_SEC`) so short skips seek within the buffer.
- **Can existing segments be reused?** Never. There is no segment cache across sessions;
  two users watching the same movie at the same offset transcode it twice.

### 11.5 Unbounded generation (H8, Confirmed, Medium)

No `-t` or `-to` is passed, so a session encodes the entire remainder of the movie
regardless of what the user watches, at up to 4× realtime. For a 97-minute source this is
~24 minutes of continuous CPU and disk write per session. Disk cost per session is the
whole remaining movie: ~950 MB for the movie-57 remux, and ~11.7 GB for a
`2160p_16mbps` transcode of a 97-minute film. With `HLS_MAX_SESSIONS_PER_USER = 3` a single
user can hold three of those concurrently. Nothing bounds total transcode-directory usage,
and there is no disk-space check before starting a session.

---

## 12. Session and FFmpeg lifecycle assessment

This is the strongest part of the implementation. Everything below was verified live.

### 12.1 Session identity and isolation

`HLSSessionKey = movie:<id>:<profile>:audio:<n|none>:session:<uuid>:start:<sec>`
(`hls_session.go:157`); rooms use `room:<id>` (`:163`), a disjoint namespace. Ownership is
enforced on every segment request by `canAccessPersonalHLSSession` (`:417`), which compares
`IsRoom`, `MovieID` and `OwnerUserID`. Temp dirs are `os.MkdirTemp(root, "igloo-hls-*")`,
so filenames are unguessable and per-session.

**Cross-user or cross-configuration leakage: not possible.** A different user hitting a
known key gets 404 (ownership check); a different profile, audio track, start, or playback
session produces a different key and a different temp dir.

### 12.2 Live verification

| Scenario | Expected | Observed |
| --- | --- | --- |
| Same UUID, `start=0` then `start=600` (seek) | old session superseded | temp dirs 2 → 2 ✅ |
| Different UUID, same movie (second tab/device) | isolated new session | temp dirs 2 → 3 ✅ |
| 3 further sessions beyond the cap | LRU eviction holds at 3 | temp dirs 3, ffmpegs 3 ✅ |
| `POST /hls/session/stop` | prompt teardown | **84 ms**, process gone, temp dir removed ✅ |
| Abandoned session, no requests | reclaimed within ~6 min (300 s TTL + 60 s sweep) | fully reclaimed at **+7 min 32 s** — FFmpeg exited by +7 m 12 s, temp dir removed 20 s later ⚠️ |

The abandonment arm reclaimed everything, but took 7 m 32 s against the ~6 min the
documentation implies (`docs/ffmpeg.md` §HLS Playback: "fully reclaimed … within about six
minutes"). Other test sessions were being created and stopped during the window, which can
shift the sweep phase, so this is reported as an observation rather than a defect; the
guarantee held and nothing leaked. A dedicated single-session run would settle whether the
documented figure needs widening.

`HLS_MAX_SESSIONS_PER_USER` admission is real: after six distinct playback sessions for one
user, exactly three FFmpeg processes and three temp dirs remained.

### 12.3 Process lifecycle

`RunHLS` (`ffmpeg_hls.go:333`) owns `cmd.Wait` and delivers exactly one `onExit`; stderr is
drained into a 20-line ring buffer with a 1 MB token cap, so there is no pipe-backpressure
stall and no unbounded log growth. `cleanupHLSSession` (`hls_session.go:480`) is idempotent
via `CleanupOnce`: mark `ExpectedStop`, cancel the context (`CommandContext` kills), wait
2 s, `Process.Kill()`, wait 2 s, `RemoveAll(TempDir)`.

Cleanup triggers cover every path asked about: explicit stop, supersession, per-user cap
eviction, CPU-permit reclaim, TTL expiry via `OnEvicted` (wired at `application.go:197-208`
into a `app.Wait`-tracked goroutine), room deletion, shutdown (`shutdown.go`), and
boot-time orphan sweep (`startup.go:329`, globbing `igloo-hls-*`).

**Does FFmpeg outlive playback?** Only for the documented idle window. Closing the tab
fires `pagehide` → stop endpoint (84 ms). Losing that (crash, network drop, sleep) falls
back to the 5-minute idle TTL plus a ≤1-minute sweep. Not a leak; a bounded delay whose
cost is real on a home server given §11.5.

One gap: `cleanupHLSSession` ignores the `RemoveAll` error. If removal fails (file still
open, permissions), the directory leaks silently until the next boot sweep.

---

## 13. Watch-room HLS assessment

### 13.1 Creation-time validation (good)

`CreateWatchRoom` (`watch_room_handler.go:324`) validates in order: `movie_id > 0`, mode
present, `isValidPlaybackMode`, `audio_track >= 0`, `subtitle_track >= 0`, movie exists,
then — crucially — rejects a non-zero `audio_track` on a movie with **no** audio streams
(`:383`), an out-of-range `audio_track` (`:388`), a non-zero `audio_track` combined with
direct playback (`:396`), and ambiguous default dispositions for direct (`:402`). The
direct-only mirror block (`:413-438`) re-applies `movieContentType`, `primaryVideoStream`
and `isBrowserSafeH264RemuxCandidate` server-side because the server cannot probe.

**Video-only movies are handled correctly.** A movie with no audio streams accepts
`audio_track = 0` only in the sense that the field defaults to 0 and is rejected if
non-zero; `createHLSSession` then passes `audioTrack == nil` and `buildHLSArgs` omits the
audio map and all audio options entirely. No synthetic audio track is invented and no
failure occurs solely because audio is absent. This matches the personal path exactly.

### 13.2 Warm-up

`WarmUpRoomHLSSession` runs on a background context **after** `tx.Commit()`
(`:514-533`); on failure the room row is deleted and a 500 returned. Two observations:

- The rollback calls the `DeleteWatchRoom` **query**, not `CleanupRoomHLSSession`, so the
  tombstone/cache path is not exercised on this rollback. In practice there is nothing to
  clean (warm-up failed), but a partially-created session would be missed.
- Warm-up always starts at 0 and has no `-t`, so creating a room immediately begins
  encoding the entire film (§11.5) whether or not anyone presses play.

### 13.3 TTL — the premise checked and refuted

A 30-minute room TTL with no refresh would expire mid-film. It does not:
`getActiveRoomHLSSession` (`hls_session.go:455`) calls `RefreshHLSSessionTTL` on every hit,
and both `WatchRoomHLSManifest` (via `GetOrCreateRoomHLSSession`) and
`WatchRoomHLSSegment` route through it. Every segment fetch refreshes the room's 30
minutes. **Not a defect.**

The residual exposure is a room **paused** for over 30 minutes with a full buffer, which
issues no requests. It would be evicted, and recovery is poor: the room URL carries no
`start`, no `reload` and no `playback_session`, so the next manifest request creates a
fresh session **from t=0** while every participant's playhead is elsewhere. Graded
**Hypothesis** (the 35-minute idle arm was not run to completion).

Note the asymmetry worth locking with a test: `RefreshHLSSessionTTL` re-`Set`s room
sessions unconditionally, while the personal path re-`Get`s and identity-compares
(`raw != session`) before extending. A stale room pointer can therefore resurrect a cache
entry another path just removed.

### 13.4 H9 — the room player has no recovery wiring (Confirmed, High)

`WatchRoomPage.tsx:515-528` renders the shared `VideoPlayer` with `videoRef`, `src`,
`isHlsSource`, `title`, `isFullscreen`, `onError`, `onPlay`, `onPause` — and **omits
`onSessionLost`, `onCapacityBusy`, `startSec` and `onStartApplied`**. There is also no
`useHlsSessionKeepalive` anywhere in the watch-room tree.

Consequences for an HLS room:

- A segment 404 — including the H2 tail, which every remux room will hit — produces a
  generic "Stream error" with no recovery attempt.
- A 503 from the transcode limiter is a hard failure; there is no capacity retry and no
  "waiting for capacity" affordance.
- Participants have no keepalive; the room relies entirely on segment traffic for TTL.

Rooms are also never exercised by the HLS path in tests: `watch-room.spec.ts` creates
`mode: "direct"` only, and no watch-room unit test references hls.js.

### 13.5 Participant isolation and sharing

All participants share one session, one FFmpeg process and one temp dir — intended, and
efficient. Authorisation runs per request through `loadAuthorizedWatchRoomForRequest`, so a
non-member cannot fetch room segments. Room and personal namespaces cannot collide
(verified by `hls_room_test.go:13`). Deletion marks a tombstone, removes the cache entry,
kills FFmpeg and removes the temp dir.

---

## 14. Hardware-acceleration assessment

Configured device here is `cpu`, so the hardware paths were **not exercised at runtime**;
this section is a code assessment.

`ResolveHLSDevice` (`capabilities.go:137`) degrades to `cpu` whenever the encoder is
absent, a runtime probe failed, or the device is unknown, and records a `Reason` that is
logged (`hw_fallback_reason` — verified empty for cpu in the live log). Probing at startup
goes beyond name matching: `probeH264NVENC`, `probeNvidiaCUDAScale`,
`probeNvidiaCUDATonemap`, `probeH264QSV`, `probeQSVScale` each run a short real encode or
filter with a 5 s timeout (`capabilities.go:226-241`). Filter and encoder *options* are
probed individually before being emitted (`appendHLSIntelEncoderArgs`).

This directly addresses the classic failure the brief asks about — "hardware encoding
selected because an encoder name exists" — and does so properly.

Assessment of the specific risks:

- **Hardware decode with incompatible software filters**: avoided by design. CUDA decode is
  enabled without `-hwaccel_output_format`, so frames land in system memory; QSV decode is
  deliberately not enabled. HDR tone mapping on Intel never uses `scale_qsv`.
- **Software fallback retaining hardware-only arguments**: not possible — `hwLower` is the
  *resolved* device, and every hardware argument branches on it plus a capability check.
  `useNvidiaCUDAFilters` / `useIntelQSVScale` are additionally gated on `!copyVideo`.
- **Failed hardware commands retrying in software**: **not implemented.** Probes run at
  startup only. If the GPU is healthy at boot but the device is busy, the driver is
  upgraded, or `/dev/dri` permissions change at runtime, the session simply fails —
  `onExit` logs the stderr tail and the manifest request has already returned. There is no
  per-session retry that drops to `libx264`. Graded **Confirmed gap, Medium**; on this
  host it is unreachable because the device is `cpu`.
- **Different effective output between paths**: the pixel-format and colour handling
  differ deliberately (`nv12` for QSV, CUDA formats inside the filter chain) but converge
  on bt709 SDR H.264 High. Since the manifest declares no `CODECS` attribute (§1.2), there
  is nothing that could become inconsistent.
- **Device contention**: nothing serialises GPU access; `HLS_MAX_CPU_TRANSCODES` bounds
  only CPU sessions, and its name is accurate — hardware sessions still consume a permit
  because the limiter keys on `!copyVideo`, not on the device. That is the conservative
  choice and is fine.

The `2160p_16mbps` profile combined with a `cpu` device — the configuration on this
machine, and the one every HEVC 4K title in the library falls back to — is the most
resource-dangerous path in the system and has no guard beyond the permit count.

---

## 15. HLS player-controls assessment

### 15.1 What exists

All HLS-relevant selection happens **before** playback, in `PlaybackSettingsDialog.tsx`
(quality/mode, audio track, subtitle track), reached from the movie-details hero. In the
player itself, `MoviePlayerControls.tsx` provides seek ±10 s, play/pause, chapters, volume,
fullscreen and a read-only mode badge. There is **no in-player quality, audio, or subtitle
control at all**.

### 15.2 Accessibility

Good, and evidently maintained: `role="group"` with labels for the progress and control
clusters (`:76`, `:98-101`); keyboard hints inside `aria-label`s ("Pause (Space or K)",
"Seek forward 10 seconds (L or Right Arrow)"); `aria-pressed` on the fullscreen toggle;
all icons `aria-hidden`; the mode badge prefixed with an `sr-only` "Current playback mode:";
four `LiveAnnouncer` regions in `play.tsx:752-766` (playing/paused and capacity polite,
chapter and direct-play fallback assertive); an `sr-only` shortcut list; the buffering
spinner labelled and `pointer-events-none`. The settings dialog labels every control and
ties the audio-track/mode interaction note to both selects via `aria-describedby`.

Gaps, all HLS-specific:

- **Recovery is silent.** `useHlsSessionRecovery` retries up to three times with no
  announcement and no visible state; a screen-reader user gets nothing between "playing"
  and a terminal error. Capacity waiting, by contrast, is announced — the pattern to copy
  already exists.
- **The error screen is not announced.** `MoviePlaybackStatusScreen` renders an `<h1>` and
  `<p>` with no `role="alert"` / `aria-live`, so a mid-playback failure is silent for
  assistive tech.
- **The mode badge is inaccurate, not just terse** (H3) — an accessibility problem as much
  as a UI one, since it is the only mode affordance and it can be wrong.

### 15.3 Should the custom player remain?

**Yes.** The controls are accessible, small, and already integrated with the design system,
chapters, watch-progress, keyboard handling and the watch-room sync layer. Every defect in
this audit is a server-side or manifest-level defect, or a wiring omission in
`WatchRoomPage`. None of them is caused by the player being custom, and none would be fixed
by a package: a player library cannot correct a manifest that misdescribes its own
segments, cannot invent an effective-profile field the API does not return, and cannot
shift subtitle cues the server timestamps absolutely.

The one capability genuinely missing — in-player track and quality switching — is
achievable with existing shadcn/ui primitives, and with today's architecture each switch is
a new session anyway, so it is a navigation action rather than a player-internal one.
Adopting a package would mean re-solving the accessibility work already done here.

---

## 16. Confirmed bugs

**Status note (2026-07-28, same day).** The P0 and P1 items were fixed after this audit was
written; see §23 for what changed and how each fix was verified. The findings below are
kept as originally written so the evidence and the reasoning survive.

| ID | Severity | Summary | Evidence |
| --- | --- | --- | --- |
| **H1** | Critical | Synthesized VOD playlist declares uniform 4 s `#EXTINF` for copy-video sessions whose real segments follow source keyframes (measured mean 5.40 s, max 12.43 s); cumulative drift +131.9 s over 507.9 s of content. Transcode control drifts 0.008 s. | §5.2, `B1/B2-extinf-drift.tsv` |
| **H2** | Critical | The same playlist advertises `ceil(duration/4)` segments while FFmpeg produces far fewer; the surplus 404 after a clean exit, and the client misreads that as "session lost" and burns its 3-attempt recovery budget. Reproduced: 8 advertised, 1 produced, `segment_1..7 → 404`. | §5.3, `C5-tail.m3u8` |
| **H3** | High | The effective profile is never exposed. A `remux` request on HEVC silently became a `2160p_16mbps` CPU transcode with HTTP 200, `/hls/remux/` URLs, and a player badge still reading "Remux". | §3.5, `C6-*` |
| **H9** | High | The watch-room player omits `onSessionLost`, `onCapacityBusy`, `startSec` and the keepalive, so HLS rooms have no recovery from a 404 or a 503. | §13.4 |
| **H5** | Medium | `-ss` snaps back to the preceding keyframe (measured 600 → 591.17 s) but the client maps absolute time from the *requested* start, so the UI clock and saved progress are ahead of the picture by up to a GOP. | §11.2, `B3` |
| **H8** | Medium | No `-t`; every session encodes the whole remaining movie at up to 4× realtime (~950 MB remux, ~11.7 GB for a 4K transcode), with no disk-space check. | §11.5 |
| **H11** | Medium | `Hls.isSupported() === false` returns silently — blank player, no error, no fallback. | §4.1 |
| **H14** | Medium | Stream ordinals are not stable across re-scans (delete-and-reinsert, no `UNIQUE(movie_id, stream_index)`), silently repointing saved audio/subtitle selections and stored room tracks. | §6.3 |
| **H6** | Low–Med | First remux of a movie blocks the manifest response on preflight — measured **6.28 s** (30 s cap), cached 24 h thereafter. | §3.3, server log |
| **H7** | Low | `audio_track` is logged as a pointer (`audio_track=0x21c92170d160`), making session logs useless for diagnosing track selection. | §7.4 |
| **H13** | Low | `#EXT-X-INDEPENDENT-SEGMENTS` not emitted though true; no `#EXT-X-START` on rebased sessions. | §5.5 |

## 17. Likely bugs and risks

| ID | Grade | Summary | What would settle it |
| --- | --- | --- | --- |
| **H4** | Likely, High | Subtitles desync by `hlsStartSec` on every rebased HLS session (resume, seek-rebase), because WebVTT cues are absolute and the media timeline is rebased to 0. | Browser: resume a subtitled movie at 40 min and observe (§20.3) |
| **H10** | Likely, Low | A late `disposeHls` can null `hlsRef.current` while it holds a newer instance. | Unit test forcing overlapping effect runs |
| **H12** | Confirmed gap | No in-player quality/audio/subtitle switching; changing any of them means leaving playback. | Product decision (§18.2) |
| **H15** | Hypothesis | HE-AAC/xHE-AAC would be copied blindly (`copyAudio = codec == "aac"` ignores `profile`). Unreachable in this library — all 200 AAC streams are LC. | A HE-AAC sample through the remux path |
| **H16** | Hypothesis | AC-3/E-AC-3 `dialnorm` is not applied on downmix, so loudness can shift audibly. | A/B listening test on an AC-3 title |
| **H17** | Hypothesis | A watch room paused >30 min is evicted and restarts every participant from t=0 (no `start`, no `reload` in the room URL). | 35-minute idle room arm |
| **H18** | Confirmed gap | No runtime hardware fallback: probes run only at startup, so a device that fails later fails the session with no software retry. | Unreachable here (`cpu`); needs a GPU host |
| **H19** | Confirmed gap | Interlaced, rotated and VFR sources have no handling (no deinterlace, no rotation, no fps normalisation). | Sample files through transcode |
| **H20** | Low | `cleanupHLSSession` ignores the `RemoveAll` error, so a failed temp-dir removal leaks silently until the next boot sweep. | Code fix |

---

## 18. Recommended HLS architecture

The architecture is sound and should not be rewritten. Sessions, admission control,
process lifecycle, the remux safety gate, explicit stream mapping and hardware resolution
are all well built and were verified working. The problems are concentrated in one place:
**the server describes its output instead of measuring it.**

### 18.1 Make the playlist describe reality (fixes H1, H2, and most of H5)

Three options, in increasing order of cost:

1. **Serve FFmpeg's own playlist, always.** FFmpeg already writes an accurate event
   playlist and updates it atomically (`playlist.m3u8.tmp` → rename, observed live). Serve
   it as an **event** playlist during encoding — no `ENDLIST`, so clients legitimately
   reload it and pick up real durations as they are produced — and finalise to VOD on
   clean exit. This is the smallest change that eliminates both critical findings, at the
   cost of losing "whole movie is seekable immediately".
2. **Keep the synthetic VOD playlist for transcodes only** (where it is exact, measured
   0.008 s drift) and use option 1 for copy-video. This preserves today's seek behaviour
   for the majority of sessions and fixes the broken minority. **Recommended.**
3. **Index keyframes at scan time** and synthesise a correct copy-video playlist from the
   real GOP structure. This is the only option that keeps instant full-movie seeking on
   remux, and it also gives exact `-ss` targets (fixing H5 properly) and enables segment
   reuse later. It is the right long-term answer and matches the deferred
   "HLS copy-video playlist follow-up" already on record. Highest cost: a scanner change,
   a schema addition, and a re-scan.

Whatever is chosen, add `#EXT-X-INDEPENDENT-SEGMENTS`, and have the server publish the
**true** session start (measurable from the first segment) so the client can stop guessing.

### 18.2 Publish the effective profile (fixes H3)

Add `effective_profile` (and ideally `copy_video`, `effective_start_sec`) to the session,
return it from the manifest — a response header is the least invasive carrier for an
`.m3u8`, or a small `GET /api/movies/{id}/hls/session` companion endpoint — document it in
`docs/openapi.json`, and drive the player badge from it. The client should say
"Transcoded 2160p (remux unavailable: HEVC)" rather than "Remux".

Renditions (`EXT-X-MEDIA` audio groups, subtitle renditions, an ABR ladder) are **not**
recommended for a home server: they multiply concurrent FFmpeg work on a box that already
runs 4K CPU transcodes near its limit, and the current one-session-per-selection model is
honest about that cost. Revisit only if segment reuse (18.1 option 3) lands first.

### 18.3 Fix subtitle timing at the source (fixes H4)

Pass the session start to `SubtitleWebVTT` and emit cues shifted by it, cached per
(movie, stream, start). Keep the absolute-timestamp variant for direct play.

### 18.4 Bound the work (fixes H8)

Cap generation ahead of the playhead — either `-t` sized to a generous window with session
extension, or keep `-readrate` but add a disk-space precondition and a transcode-directory
budget. A home server should refuse a session it cannot store.

---

## 19. Prioritized action plan

**P0 — before HLS can be called production-ready**

1. **H2** — stop advertising segments that will not exist. Option 18.1(2) is the smallest
   correct fix. Until then, remux playback fails near the end of most films.
2. **H1** — stop declaring 4 s durations for copy-video (same change).
3. **H3** — publish and display the effective profile. Users are currently told a 4K CPU
   transcode is a stream copy.
4. **H4** — offset subtitle cues for rebased sessions. Resume + subtitles is a mainstream
   path and it is broken.

**P1 — high value, contained**

5. **H9** — wire `onSessionLost`, `onCapacityBusy`, `startSec` and the keepalive into
   `WatchRoomPage`, or extract the shared wiring so the two players cannot diverge again.
6. **H5** — publish the true session start; map absolute time from it.
7. **H11** — show a real error when neither MSE nor native HLS is available.
8. **H8** — add a disk precondition and bound generation.

**P2 — correctness and operability**

9. **H14** — add `UNIQUE(movie_id, stream_index)` and make re-scan preserve ordinals (the
   pre-production no-migration rule makes this cheap now and expensive later).
10. **H7** — dereference the audio-track pointer in logs.
11. **H6** — make preflight non-blocking (serve the manifest, decide before the first
    segment) or shorten the wait.
12. **H13, H20** — emit `#EXT-X-INDEPENDENT-SEGMENTS`; log `RemoveAll` failures.
13. **§15.2** — announce recovery attempts; add `role="alert"` to the playback error
    screen.

**P3 — deferred / product**

14. Keyframe indexing at scan time (18.1 option 3), enabling exact seeks and segment reuse.
15. In-player track/quality switching (H12).
16. Runtime hardware fallback (H18); interlaced/rotation/VFR handling (H19).

---

## 20. Test matrix and testing plan

### 20.1 Existing coverage

Substantial and good on the units: `hls_handler_test.go` (19 tests), `hls_session_test.go`
(27), `hls_playlist_test.go` (10), `hls_room_test.go` (9), `hls_remux_safety_test.go` (2),
plus `ffmpeg_hls_args_test.go` (20), `_additional` (4), `_hardware` (17), `_run` (11),
`remux_validator_test.go` (12), `remux_parser_test.go` (12) and a fuzz target. Web side:
`video-player.test.tsx`, `use-hls-capacity-retry`, `use-hls-session-keepalive`,
`movie-hls-playback-session`, `movie-playback-data`.

**The gap is systemic: nothing compares generated output to reality.** Every playlist test
asserts the synthesizer against its own formula, which is why H1 and H2 survived. No test
runs FFmpeg and checks the result against the manifest. No test covers
`useHlsSessionRecovery` directly, watch-room HLS end to end, or native HLS.

### 20.2 File test matrix

Expected effective profile assumes `HARDWARE_ACCELERATION_DEVICE=cpu`.

| # | Source | Requested | Effective | FFmpeg op | Audio | Manifest expectation | Auto? |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | H.264 8-bit + AAC stereo | remux | remux | `-c:v copy -c:a copy` | copy | `EXTINF` == real segment durations; segment count == files produced | yes |
| 2 | H.264 8-bit + AC-3 5.1 | remux | remux | `-c:v copy -c:a aac -ac 2` | stereo AAC | as #1 | yes |
| 3 | H.264 with sparse keyframes (movie 131) | remux | remux | copy | copy | **regression guard for H1/H2** | yes |
| 4 | H.264 failing IDR preflight | remux | best-fit transcode | full transcode | AAC | effective profile reported ≠ requested | yes |
| 5 | HEVC Main 10 SDR | remux | 2160p/1080p best-fit | full transcode | per codec | client shown effective profile (H3) | yes |
| 6 | HEVC Main 10 HDR10 (`smpte2084`) | 1080p_8mbps | same | tone-map chain | AAC | bt709 output tagging | partial |
| 7 | Dolby Vision | any | — | not represented in library | — | document as unsupported | no |
| 8 | Interlaced | 720p_3mbps | same | transcode | AAC | **currently no deinterlace (H19)** | manual |
| 9 | VFR | 720p_3mbps | same | transcode | AAC | segment boundaries stay 4 s | yes |
| 10 | Multiple audio tracks | remux, `audio_track=2` | remux | copy/transcode | track 2 reaches output | `-map 0:<abs idx of 3rd>` | yes |
| 11 | Duplicate-language audio | remux, track 1 vs 2 | remux | — | distinguishable in picker | ordinal→absolute correct | yes |
| 12 | Commentary / audio-description | remux | remux | — | disposition preserved in picker | — | yes |
| 13 | **Video-only** | remux | remux | no audio map | none | no `-c:a`, no `-map` for audio | yes |
| 14 | Multiple subtitle tracks | any | — | sidecar only | — | no subtitle rendition; correct VTT | yes |
| 15 | Forced / hearing-impaired subs | any | — | sidecar | — | disposition surfaced in picker | yes |
| 16 | ASS/SSA | any | — | `-c:s webvtt` | — | styling loss acceptable, timing correct | yes |
| 17 | PGS/DVD/DVB | any | — | rejected | — | HTTP 415, never a broken text track | yes |
| 18 | Missing language metadata | remux | remux | — | picker labels by index | — | yes |
| 19 | Missing/multiple default dispositions | direct → remux | remux | copy | — | direct refused, HLS unaffected | yes |
| 20 | Attached cover art | any | — | excluded | — | `primaryVideoStream` skips it | yes |
| 21 | Multiple video streams | any | — | first non-cover-art | — | correct `-map` | yes |
| 22 | DTS / TrueHD / E-AC-3 | remux | remux | `-c:v copy` + AAC stereo | transcoded | measure output bitrate | yes |
| 23 | Corrupt / truncated file | any | — | FFmpeg fails | — | manifest 4xx/5xx, no hung request, temp dir removed | yes |
| 24 | 4K over the slow mount | 2160p_16mbps | same | transcode | AAC | **below realtime — expect stall** | manual |
| 25 | Seek before generation completes | any | — | rebase | — | new session, old superseded | yes |
| 26 | Same UUID in two tabs | any | — | supersession | — | second kills first (documented behaviour) | yes |
| 27 | Two users, same movie | any | — | two sessions | — | isolation, no cross-access | yes |
| 28 | Watch room, HLS mode | room profile | same | shared session | stored track | **no coverage today** | yes |
| 29 | Resume at 40 min + subtitles | remux | remux | `-ss` | — | **H4 regression guard** | browser |

### 20.3 Recommended tests

**Golden-output tests (the missing layer, highest value).** A build-tagged suite that runs
the real FFmpeg against small fixture files and asserts the *manifest matches the media*:

- `TestGeneratedPlaylistMatchesRealSegments` — for a copy-video fixture, assert every
  `#EXTINF` is within tolerance of the produced segment and that the entry count equals the
  file count. This single test would have caught both H1 and H2.
- `TestCopyVideoSegmentCountMatchesFiles` — the count assertion alone, cheap enough to run
  on several fixtures.
- `TestSessionStartMatchesFirstSegmentPTS` — locks H5.

**Go unit tests** (no media required):

- `TestHLSSessionKey_UsesRequestedNotEffectiveProfile` — documents H3's cache-key
  consequence so a fix is deliberate.
- `TestStartHLSSession_CopyAudioOnlyForAAC` — extend to profile once H15 is decided.
- `TestBuildHLSArgs_SeekPlacedBeforeInput`, `TestNormalizedHLSStartSec_*`.
- `TestRefreshHLSSessionTTL_RoomIgnoresIdentity` — records the room/personal asymmetry
  (§13.3).
- `TestCleanupHLSSession_RemovesTempDirAfterKill` and an `OnEvicted` test driving a
  short-TTL cache — lock the lifecycle guarantees measured in §12.2.
- `TestCleanupPersonalHLSSessionsForOwner_KillsSameUUIDDifferentStart` — encodes the
  duplicate-tab consequence so a future per-tab UUID change breaks it deliberately.

**Web unit tests:** a `use-hls-session-recovery.test.tsx` (none exists); a VideoPlayer test
for `Hls.isSupported() === false` (H11); a test that `destroy()` runs on unmount; a
StrictMode double-mount test (H10).

**Playwright:** a watch-room HLS spec behind `E2E_BASE_URL` (case 28); extend
`hls-transcode.spec.ts` with a remux case and a remux-fallback case — today it only covers
transcode, and its `E2E_HLS_4K_MOVIE_ID` can never exercise remux because the library has
no H.264 above 1080p.

**Browser experiments to settle the Likely findings** (excluded from this audit's scope):

- **H4**: resume a subtitled movie at 40 minutes; observe whether cues appear.
- **H1 user impact**: serve the same segment files under (a) FFmpeg's real playlist and
  (b) the synthesized one from a static harness, seek to a known frame in each, and compare
  `video.currentTime` against ground truth — this determines whether hls.js's PTS
  re-derivation masks the drift.
- **H15**: 5.1 AAC-LC fMP4 in Firefox MSE.

**Not needed:** load/concurrency tests beyond what exists — §12.2 already demonstrates the
cap and eviction behave correctly.

---

## 21. Diagnostic changes

No production code was modified. `git status` shows only this new document plus the
`docs/ffmpeg.md` corrections listed below.

**Repository side effects:**

- `server/cmd/api/webdist/.keep` was created — the gitignored placeholder the build
  requires, per `CLAUDE.md`. Not tracked by git.
- The Go build cache was populated.

**Sandbox artifacts (outside the repository, in the session scratchpad):**

| Artifact | Contents |
| --- | --- |
| `A1-caps.txt` | Capability matrix, system 6.1.1 vs embedded 7.1.4-Jellyfin |
| `B1-extinf-drift.tsv` | Movie 57 remux, per-segment real vs synthesized durations |
| `B2-extinf-drift.tsv` | Movie 57 720p transcode control |
| `B3-ffmpeg.log`, `B3-seg0.mp4` | `-ss 600` keyframe-landing measurement |
| `B4-aac51-131` | 5.1 AAC-LC copy output and probe |
| `C1-real-argv.txt`, `C1-playlist-remux.m3u8` | Live argv capture; 1458-entry playlist |
| `C2a-abandon.log` | Abandoned-session reclamation timeline |
| `C5-tail.m3u8` | Tail session: 8 advertised vs 1 produced |
| `C6-headers.txt`, `C6-hevc-remux.m3u8` | HEVC remux → 2160p fallback, unreported |
| `server.log` | Sandbox server log |
| `igloo-copy.db`, `igloo-dev` | DB copy and throwaway binary |

**Isolation:** the sandbox ran on port 8099 against a *copy* of the database with
`transcode_dir` redirected to the scratchpad and a default admin seeded into the copy.
`db/igloo.db` and `./transcode` were never written. The sandbox server also ran a library
scan against the copy — expected, and confined to the copy.

**`docs/ffmpeg.md` corrections** (per the decision to reconcile rather than only report):
the §HLS Output Format and §Seeking and Resume Behavior sections stated the generated VOD
playlist and `-ss` behaviour without noting that copy-video segment durations and counts
are synthetic and diverge from the real output, or that `-ss` snaps backwards to a
keyframe. Both now say what the code does, each cross-referencing the finding here. These
notes should be revised again when H1/H2/H5 are fixed.

---

## 22. Final answers to the audit questions

**1. Is the current HLS implementation reliable?**
Partly. The session layer, process lifecycle, admission control, stream mapping and the
remux safety gate are well built and were verified working. The *transcode* delivery path
is reliable. The **copy-video (remux) delivery path is not**: its manifest misdescribes its
own output and playback breaks before the end of the film (H1, H2).

**2. Are remux, partial-transcode and full-transcode decisions correct?**
The decisions themselves are correct and conservative — the static gate plus IDR preflight
is a good design, and video is never transcoded merely because audio is incompatible. Two
qualifications: audio is transcoded on codec name alone, ignoring profile (H15), and always
downmixed to stereo at a wasteful 320 kbps; and a fallback from remux to a full transcode is
correct but invisible (H3).

**3. Are the selected audio and subtitle tracks always honoured?**
Yes at request time — mapping is explicit and absolute, ordinals are range-checked and
resolved server-side, and video-only movies are handled correctly. But the ordinal is not
stable across library re-scans (H14), so a saved or room-stored selection can silently come
to mean a different track.

**4. Are manifests valid and accurately describing the output?**
Valid, yes — the tags are well-formed and the fragments are sound. Accurate, **no** for
copy-video: durations are fiction (mean 5.40 s declared as 4 s) and the segment count is
inflated by up to 2.3×. For transcodes the manifest is accurate to 8 ms.

**5. Does seeking work correctly before generation completes?**
Mostly. A forward seek into unencoded territory blocks until the segment is produced (up to
120 s) rather than failing, and rebasing is clean with no duplicate processes. But the seek
lands up to a full GOP earlier than requested and the client is never told (H5), and on
copy-video the playlist's time→segment mapping is wrong to begin with (H1).

**6. Can sessions or segments collide or leak across users?**
No. Verified live: keys are scoped by movie, profile, audio track, playback-session UUID
and start; ownership is checked on every segment; rooms occupy a disjoint namespace; temp
dirs are randomised. Different users and different tabs are properly isolated, and the
per-user cap holds at 3.

**7. Can FFmpeg processes continue after playback ends?**
Not indefinitely. Explicit stop tears down in 84 ms; an abandoned session was measured
fully reclaimed — process killed, temp directory removed — 7 m 32 s after creation. The
real cost is not a leak but that an unbounded session writes the whole remaining movie to
disk in the meantime (H8).

**8. Is hardware acceleration safe and deterministic?**
Safe and deterministic at startup — real runtime encode/filter probes gate every hardware
path and everything degrades to `libx264` with a logged reason, which is better than most
implementations. It is not *resilient*: probes never re-run, so a device that fails after
boot fails the session with no software retry (H18). Unexercised here (`cpu`).

**9. Does the client display the actual effective playback mode?**
**No.** This is H3, and it is the single most misleading behaviour found: a request for
`remux` on an HEVC file returns 200 with `/hls/remux/` URLs while the server runs a 2160p
CPU transcode, and the badge reads "Remux".

**10. Should the custom player remain?**
Yes. It is accessible, integrated, and not the cause of any finding here. No player package
would fix a manifest that misdescribes its segments, an unpublished effective profile, or
absolutely-timestamped subtitles on a rebased timeline. The missing in-player track
switching is achievable with existing components.

**11. What must be fixed before HLS playback can be considered production-ready?**
The four P0 items: make the playlist describe the real output (H2, then H1), publish and
display the effective profile (H3), and offset subtitle cues for rebased sessions (H4).
Wiring watch-room recovery (H9) should follow immediately, since rooms currently have no
recovery from the very 404s H2 guarantees.

---

## 23. Remediation (2026-07-28)

P0 and P1 were fixed in the same session that produced this audit. P2 and P3 remain open.

### 23.1 What changed

| ID | Fix | Where |
| --- | --- | --- |
| **H1, H2** | Copy-video sessions now serve **FFmpeg's own playlist** (URLs rewritten, `EVENT` while encoding, finalized `VOD` after a clean exit) instead of a playlist synthesized from the movie duration. Transcodes keep the synthesized VOD playlist, which is exact for them. A manifest request for a copy-video session that has not published a segment yet waits, then returns `503` + `Retry-After: 1` — it never falls back to synthesizing. | `hls_handler.go` (`buildHLSPlaylistBody`, `readLiveHLSPlaylist`, `hasPlayableSegment`) |
| **H3** | Two-part. The client no longer offers `remux` for video the server will refuse to copy — `getAvailableModes` now applies `isBrowserSafeH264` to the remux branch, closing the common 10-bit case at source. For the remaining dynamic-preflight case the manifest carries `X-Igloo-Effective-Profile`, read via hls.js `MANIFEST_LOADED` and shown in the player badge through `effectiveModeLabel`. | `lib/playback.ts`, `VideoPlayer.tsx`, `play.tsx`, `hls_handler.go`, `hls_session.go` |
| **H4** | The WebVTT endpoint accepts `start` and shifts cue timings at serve time. The cache still holds one absolute-timestamp payload per (movie, stream), so there is no extra extraction and no cache multiplication. The client appends the session start to the subtitle URL. | `helpers/subtitle_shift.go`, `subtitle_handler.go`, `lib/movie-playback.ts` |
| **H5** | Copy-video sessions with a non-zero start measure where the media really begins, with a bounded ffprobe keyframe lookup running alongside FFmpeg, published as `X-Igloo-Actual-Start`. Advisory: on failure the header is omitted. Transcodes are unaffected — input seek is frame-accurate when re-encoding. | `ffprobe_keyframes.go`, `hls_session.go`, `hls_handler.go` |
| **H8** | A session is refused with `503` when the transcode filesystem has under 2 GB free. A filesystem that cannot be measured is not treated as full. | `disk_space.go`, `hls_session.go` |
| **H9** | The watch-room player is wired to `useHlsSessionRecovery`, `useHlsCapacityRetry` and `useHlsSessionKeepalive`, with a reload key that rebuilds the player; WebSocket sync then restores the host position. The keepalive also closes H17. | `WatchRoomPage.tsx`, `lib/watch-room.ts` |
| **H11** | An unsupported browser now gets an explicit error instead of a blank player. | `VideoPlayer.tsx` |

One defect was found *while* fixing H1/H2 and fixed with them: a session seeking past the
end of a stream exits cleanly having written a single empty segment, which FFmpeg declares
as `#EXTINF:0.000000` under an invalid `#EXT-X-TARGETDURATION:0`. Serving that verbatim
would have replaced one bad manifest with another, so a playlist is only accepted once it
declares at least one positive-duration segment; otherwise the session is reported as
having produced no playable media (`404`).

### 23.2 Verification

`make check` and `make test-openapi` pass. New tests: `ShiftWebVTT` (9 cases), the
copy-video playlist branch and its empty-output guard, `hasPlayableSegment`, the
actual-start probe and header, the subtitle `start` parameter, the corrected remux
eligibility gate, `effectiveModeLabel`, the subtitle URL offset, unsupported-browser
reporting, effective-profile reporting, and watch-room recovery wiring.

Re-running the experiments that found the bugs, against a sandbox server on a copy of the
database (artifacts `V-*`):

| Check | Before | After |
| --- | --- | --- |
| Copy-video `#EXTINF` values | uniform `4.000000` | identical to FFmpeg's own playlist (`8.466792`, `6.006000`, `10.010000`, …) |
| Copy-video segment count | 1458 advertised for movie 57 | exactly the segments FFmpeg has produced; playlist is `EVENT` with no premature `ENDLIST` |
| Advertised segments fetchable | tail 404s (8 advertised, 1 produced) | 7/7 returned `200` |
| Seek past end of stream | invalid manifest, dead segment URLs | `404 no playable media at this position` |
| `/hls/remux/` on HEVC | `200`, nothing named the transcode | `X-Igloo-Effective-Profile: 2160p_16mbps` |
| Copy-video session at `start=600` | client assumed 600 | `X-Igloo-Actual-Start: 591.174`, matching the keyframe measured in §11.2 |

**Not verified against live media:** the subtitle shift end to end over HTTP. Both attempts
hit the pre-existing subtitle-extraction timeout on the Samba mount (§1.4) — unrelated to
this change, and reproduced on the unmodified server. Coverage rests on the unit tests and
the handler tests, which exercise the real HTTP path and the real cache. The browser-side
confirmation of H4 (cues rendering in sync on resume) still needs a browser and remains
recorded in §20.2 row 29.

### 23.3 Known trade-off

Copy-video sessions are now seekable only across what FFmpeg has produced, because the
playlist no longer claims segments that do not exist. In practice the encoder runs several
times faster than realtime and any seek more than 120 s ahead already rebases the session
(`shouldRebaseHlsMovieSession`), so the exposure is a forward seek inside the no-rebase
window during a session's first seconds — where the previous behaviour was a blocking wait
followed by a `503`.

### 23.4 Still open

P2 and P3 are untouched: **H14** (stream-ordinal instability across re-scans — cheapest to
fix now, while migrations are not required), **H7** (`audio_track` logged as a pointer),
**H6** (preflight blocking the manifest), **H13**/**H20**, the accessibility gaps in
§15.2 (recovery is still not announced; the error screen still has no `role="alert"`), and
the P3 items including the scan-time keyframe index that would make H5 exact and enable
segment reuse.

# Web Direct Playback Audit

**Date:** 2026-07-27
**Branch audited:** `fix/playback-settings` (at `d0b96f35`)
**Scope:** direct playback (`mode=direct`) in the web client — from pressing Play to playback ending, failing, or changing mode.

This document is an investigation and recommendation report. At the time it was written, **no code was changed, and no diagnostic edits were made** (see §12).

> **Implementation status (2026-07-27, fifth revision):** the register stays closed; the fifth revision corrects two defects found by review *in the fixes themselves* — the D2 pixel-format rule misread 8-bit `nv12`/`yuv410p` as high bit depth, and the D1/D8 watch-room mirror could reach a different verdict than the client it mirrors. Details in §14.
>
> **Implementation status (2026-07-27, fourth revision):** the remaining register is closed — **D4, D5, D9, D10, D11, D13–D15, D17 and D-WR are fixed** on `fix/playback-settings` (see §14 for the commit-per-item mapping). Combined with the third revision (D1, D2, D3, D7, D8, D12, D16, D-FB, D-FB2, D-TEST), every finding is resolved except **D6** (sendfile/session-middleware rescoping, deliberately deferred — it touches the auth middleware layout for all stream routes) and **D-EXT** (sidecar subtitle indexing, a product feature for its own branch). Sections describing the pre-fix behaviour are preserved as written and describe the code **as audited**, not as it is now.

---

## 1. Scope and method

### What was audited

The complete `mode=direct` path: playback-mode selection, browser-compatibility evaluation, the direct-play URL, the backend file handler, `<video>` initialization, position restore, track behaviour, error handling, the boundary to the fallback mode, the custom player, and test coverage.

### What was deliberately not audited

Per the task boundaries: HLS manifest and segment generation, FFmpeg encoding quality, hardware acceleration, HLS session cleanup, FFmpeg process management, transcoding performance, and hls.js lifecycle/recovery internals. HLS, FFmpeg and ffprobe appear here **only** where they determine whether direct play is chosen, supply the metadata that decision rests on, identify streams, or serve as the alternative to direct play.

### Method and evidence quality

- Repository read as the primary source of truth; every claim below carries a `file:line` reference.
- One reproducible measurement was taken: a scratch Go program calling `mime.TypeByExtension` for each extension in `helpers.ValidVideoExtensions` on this host (Linux, `/etc/mime.types` present). Results in §3.2.
- Browser-capability claims are otherwise **documentation-sourced, not measured**. Sources are cited inline in §5. No live browser probing was performed by the audit, and no real media file was played by it. §10 lists the browser checks that should be run as manual validation; nothing here should be read as a claim that they were.
- **One claim was corrected after review by maintainer observation:** MKV playback in Chrome and Firefox. See §5.6 and the revision note in §14. The correction inverted the recommended fix for finding D1 and changed the priority order in §11.1, so it is worth reading before the rest of §3 and §5.
- **This report has been through a second, independent verification pass** (2026-07-27, §14). Every `file:line` in the §11 register was resolved against the working tree and the §3.2 measurement was re-run and reproduced exactly. That pass added one High finding the first draft missed (D16), demoted one finding to unverified (D11), and softened §5 from a table of facts to a table of assumptions. Details in §13 and §14.

---

## 2. Direct-playback architecture

### 2.1 The five playback modes as Igloo defines them

`STREAM_MODES` (`web/src/lib/constants.ts:156-209`) is the single source of truth, with a `type` discriminator:

| `id` | `type` | What actually happens |
|---|---|---|
| `direct` | `direct` | Original file, original container, original streams. `GET /api/movies/{id}/stream`. No FFmpeg. |
| `remux` | `remux` | HLS delivery. `-c:v copy`; audio copied if already AAC, otherwise re-encoded to stereo AAC. Selected streams re-muxed into fMP4. |
| `2160p_16mbps` … `720p_3mbps` | `transcode` | HLS delivery. Video re-encoded to H.264 at the profile bitrate; audio copied or converted. |

The taxonomy the task asked about maps on cleanly, with one gap:

- **Direct play** → `direct`. Correctly modelled.
- **Remux / direct stream** → `remux`. Correctly modelled, and correctly understood as *delivery format change without video re-encode*.
- **Partial transcode** (only the incompatible stream encoded) → **collapsed into `remux`**. Igloo's `remux` *is* the partial-transcode case when audio is not AAC. There is no separate mode and no user-visible distinction between "video copied, audio copied" and "video copied, audio re-encoded". This is a naming imprecision rather than a defect: both are cheap and both preserve video.
- **Full transcode** → the height-named profiles. Correctly modelled.
- **HLS as a delivery protocol** → correctly separated. `remux` and the profiles are both delivered over HLS; the client's only structural branch is `isHlsPlayback = resolvedMode !== "direct"` (`web/src/hooks/useMoviePlaybackData.ts:129`).

The one place the taxonomy leaks into the UI: the badge in the player footer shows `modeLabel`, which for `direct` reads *"Original file — plays as-is"*. That is a claim about the container, not about what the user hears — see §6.3.

### 2.2 Flow from Play to playing

```text
MovieDetailsHeroActions  →  Link to /movies/$id/play?mode&audio_track&subtitle_track
        │                       (search params come from PlaybackSettingsDialog,
        │                        or from getDefaultPlaybackSettings if untouched)
        ▼
routes/_auth/movies/$id/play.tsx
  Route.loader (only when `mode` is absent)
     ensureQueryData: authUser, movie details, technical-details, playback settings
     getPrimaryVideoStream → getAvailableModes → getDefaultPlaybackSettings
     throw redirect(...)  ← canonicalises mode / audio_track / subtitle_track into the URL
        │
        ▼
  PlayMoviePage → useMoviePlaybackData(...)
     mode = search.mode ?? "direct"                       ← direct is the implicit default
     availableModes = getAvailableModes(height, vcodec, audioStreams[0].codec, mime_type)
     resolved = resolvePlaybackSettings(...)              ← may downgrade mode, may upgrade direct→remux
     isHlsPlayback = resolvedMode !== "direct"
     streamUrl = buildMovieStreamUrl(...)                 ← "/api/movies/{id}/stream" for direct
        │
        ▼
  <VideoPlayer src={streamUrl} isHlsSource={isHlsPlayback} startSec={playbackStartSec} … />
     direct branch:  video.src = src            (VideoPlayer.tsx:248)
     start restore:  loadedmetadata → video.currentTime = min(startSec, duration)
     subtitles:      <track> injected pointing at /api/movies/{id}/subtitles/{n}/web.vtt
        │
        ▼
  GET /api/movies/{id}/stream
     chi: RequestID → preserveClientSocketIP → RealIP → RequestLogger → Recoverer
          → LoadAndSaveSession → DeviceTokenAuth → IsAuth
     StreamMovie (server/cmd/api/movie_handler.go:689) → http.ServeContent
```

### 2.3 Component inventory

**Routes**
- `web/src/routes/_auth/movies/$id/play.tsx` — the only video playback route. `Route.loader` canonicalises search params; `PlayMoviePage` owns refs, seek, resume, error state, keyboard, fullscreen.
- `web/src/routes/_auth/movies/$id/index.tsx` — details page; holds the pre-playback settings choice.
- `web/src/routes/_auth/settings/playback.tsx` — persisted user preferences.

**Hooks**
- `useMoviePlaybackData` (`web/src/hooks/useMoviePlaybackData.ts`) — the mode-resolution engine. Loads the four queries, computes `availableModes`, `resolvedMode`, `isHlsPlayback`, `streamUrl`, `subtitleInfo`, and syncs the resolution back into the URL.
- `useMovieResumeDecision`, `useMovieWatchProgressSaver`, `useVideoFullscreen`, `useVideoPlaybackKeyboard`, `useVideoMediaSession`, `useIdleControls` — direct-play-relevant.
- `useHlsSessionKeepalive`, `useHlsSessionRecovery`, `useHlsCapacityRetry` — HLS-only; all correctly inert on the direct path.

**Libraries**
- `web/src/lib/playback.ts` — `getAvailableModes`, `getPrimaryVideoStream`, `resolveModeForAudioTrack`, `resolveAudioTrackForMode`, `getDefaultPlaybackSettings`, `resolvePlaybackSettings`, `supportsNativeHLS`, `isBitmapSubtitleCodec`, `formatSubtitleLabel`. **This file contains the entire direct-play eligibility decision.**
- `web/src/lib/movie-playback.ts` — `buildMovieStreamUrl`, `buildMovieSubtitleTrackInfo`, time-base helpers, `persistMovieWatchProgress`, `deriveMoviePlaybackStatus`, `nativeMoviePlaybackErrorMessage`.
- `web/src/lib/route-search.ts` — `playSearchSchema`, `playbackSettingsToPlaySearch`, `subtitleTrackFromPlaySearch`.
- `web/src/lib/media-capabilities.ts` — despite the name, **display badges only**; never influences playback mode.

**Components**
- `web/src/components/playback/VideoPlayer.tsx` — the `<video>` element, source assignment, hls.js attachment, start-time restore, `<track>` injection, buffering spinner, native error mapping.
- `web/src/components/movies/MoviePlayerControls.tsx`, `ProgressBar.tsx`, `VolumeControl.tsx`, `ChapterMenu.tsx`, `MoviePlaybackStatus.tsx`, `ResumeDialog.tsx`, `PlaybackSettingsDialog.tsx`.

**Types**
- `web/src/types/movies.ts` — `VideoStreamType`, `AudioStreamType`, `SubtitleType`, `MovieTechnicalDetailsResponse`.
- `web/src/types/playback.ts` — `StreamModeId`, `PlaybackSettings`, `MoviePlaybackStatus`.

**Backend**
- `StreamMovie` (`server/cmd/api/movie_handler.go:689-742`) — serves the original file.
- `StreamWatchRoomMovie` (`server/cmd/api/watch_room_media_handler.go:73-123`) — the watch-room twin.
- `GetMovieTechnicalDetails` (`server/cmd/api/movie_handler.go:600-687`) — the metadata the decision rests on.
- Routes: `server/cmd/api/routes.go:172` (personal), `:215` (watch room).
- `resolveMovieFile` / `processMovieStreams` (`server/cmd/api/movies_scanner.go:269`, `:910`) — where `mime_type` and the stream rows are produced.
- `server/cmd/internal/ffprobe/ffprobe_metadata.go` — the `Stream` / `StreamDisposition` structs.
- `server/sqlc/schema.sql:207` (`movies`), `:288` (`video_streams`), `:320` (`audio_streams`), `:343` (`subtitles`).

### 2.4 Authentication

`/api/movies/{id}/stream` sits inside `registerAuthenticatedAPIRoutes` (`routes.go:51-72`): `DeviceTokenAuth` then `IsAuth`. For a browser this means **session cookie only** — there is no signed-URL or query-token escape hatch on any media route, and no CORS middleware is registered. A same-origin `<video src>` sends the cookie, so direct play works; anything cross-origin (casting, an external player, a `file://` test harness) cannot authenticate. That is a defensible design for a self-hosted app but it does bound where direct play can ever be used.

### 2.5 Position restore and progress

`start` (search param) → `playbackStartSec` (clamped to the movie duration) → `VideoPlayer.startSec` → applied on `loadedmetadata`, clamped again to `video.duration` (`VideoPlayer.tsx:109-114`, `:255-275`). Progress is saved by `useMovieWatchProgressSaver` on a 15 s interval plus pause/ended/`visibilitychange`/`pagehide`, serialized through a promise chain with a monotonic `save_sequence`. This part is sound and well tested.

### 2.6 Resource release

Direct play holds no server-side session, so teardown is purely client-side: the source effect's cleanup runs `video.removeAttribute("src"); video.load()` (`VideoPlayer.tsx:249-252`), which aborts the in-flight range request and lets the server's `defer file.Close()` fire. The HLS stop-session effect (`play.tsx:253-288`) is correctly gated on `isHlsPlayback` and never runs for direct. **No leak identified on the direct path.**

---

## 3. Direct-play eligibility audit

> **Status note (third revision):** this section describes the decision as audited. After the fixes, the gate additionally checks codec profile / bit depth / pixel format (D2), the audio `default`-disposition ambiguity table (D8), and a narrowing-only `canPlayType` probe (D3); the MIME allowlist is `["video/mp4"]` backed by a pinned server-side container map (D1); and the decision always runs before `/stream` is requested (D16).

### 3.1 The decision, in full

```ts
// web/src/lib/playback.ts:22-24
const BROWSER_COMPATIBLE_VIDEO_CODECS = ["h264", "h.264", "avc", "avc1"];
const BROWSER_COMPATIBLE_AUDIO_CODECS = ["aac", "mp3", "opus", "vorbis", "flac"];
const BROWSER_COMPATIBLE_MIME_TYPES  = ["video/mp4", "video/webm", "video/ogg"];

// web/src/lib/playback.ts:94-108
if (m.type === "direct") {
  if (!hasCodecInfo) return true;
  return isVideoDirectPlayable(videoCodec)
      && isAudioDirectPlayable(audioCodec)      // audioStreams[0].codec
      && isContainerDirectPlayable(mimeType ?? "");
}
```

That is the whole thing: three case-insensitive string-membership tests against ffprobe codec names and a stored MIME string. Coverage against the attributes the audit asked about:

| Attribute | Considered? | Notes |
|---|---|---|
| Container | ⚠️ indirectly | Via `mime_type`, which is an **extension guess**, not the probed container. See §3.2. |
| MIME type | ⚠️ | Checked, but the value is unreliable and host-dependent. |
| Video codec | ✅ | Name only; H.264 family. |
| Video profile | ❌ | `video_streams.codec_profile` is stored and ignored. High 10 / High 4:2:2 / High 4:4:4 pass. |
| Video level | ❌ | `codec_level` stored and ignored. |
| Pixel format | ❌ | `pixel_format` stored and ignored. |
| Bit depth | ❌ | `bit_depth` stored and ignored. 10-bit passes. |
| Chroma subsampling | ❌ | Derivable from `pixel_format`; ignored. |
| Resolution | ❌ | Not checked for direct (only used to filter transcode profiles). |
| Frame rate | ❌ | `frame_rate` stored and ignored. |
| HDR format | ❌ | `color_transfer` is stored and *is* used for display badges (`media-capabilities.ts`) but never for eligibility. An HDR10 file is offered as direct play. |
| Audio codec | ⚠️ | Name only, and only for `audioStreams[0]` — see §6.2. |
| Audio profile | ❌ | `codec_profile` stored and ignored (e.g. `HE-AACv2`). |
| Channel count | ❌ | `channels` stored and ignored. |
| Channel layout | ❌ | `channel_layout` stored and ignored. |
| Sample rate | ❌ | `sample_rate` stored and ignored. |
| Subtitle codec | ✅ (separately) | Bitmap codecs excluded from selection at both ends; does not gate direct play, and correctly so — subtitles are sideloaded as WebVTT. |
| Selected audio track | ✅ | A non-first selection forces `remux`. Correct and enforced in five places (listed in §3.6). |
| Selected subtitle track | ✅ | Independent of mode; sideloaded. |
| Number/arrangement of streams | ❌ | Multiple video or audio streams do not affect the decision. |
| Browser | ❌ | No detection of any kind on the direct path. |
| Operating system | ❌ | Not considered. |
| Native browser capabilities | ❌ | **No `canPlayType()` call exists on the direct path.** |
| User preferences | ✅ | `preferred_profile`, `preferred_audio_language`, `preferred_subtitle_language` all honoured. |

Checking the anti-patterns the audit asked about:

One structural gap in that snippet, separate from the attribute coverage above. `hasCodecInfo` is `videoCodec !== undefined` (`playback.ts:92`), and when it is false **every** mode is returned, direct included (`:96`, `:104`). The caller guards this with `techLoaded` (`useMoviePlaybackData.ts:100`) — but `techLoaded` only means the response *arrived*. A movie whose scan produced no `video_streams` rows yields `primaryVideo === undefined`, so `hasCodecInfo` is `false` with `techLoaded` `true`, and direct play is offered unconditionally for a file whose video codec is unknown. Recorded as **D17 (low)**: rare, but it is the same class of defect as the rest of this section and the fix is one line.

| Anti-pattern | Present? |
|---|---|
| Compatibility inferred from file extension | **Yes** — transitively, via `mime_type`. §3.2. |
| Only the container checked | No. |
| Only the video codec checked | No. |
| Audio compatibility ignored | No — but only the first stream's codec name. |
| Subtitle compatibility ignored | No (handled out of band). |
| Codec profile or level ignored | **Yes.** |
| Channel layout ignored | **Yes.** |
| Browser support inferred from user agent | No — there is no UA sniffing anywhere in `web/src`. |
| `canPlayType()` treated as authoritative | No — it is not called at all. |
| Invalid/incomplete MIME codec strings passed to browser APIs | No — no codec strings are ever built. |
| Hardware support confused with media-element support | No. |
| Android TV / native-client capabilities reused for web | No — `web/AGENTS.md` forbids it and the code complies. |
| Direct selected though the chosen audio can't be guaranteed | **Yes.** §6.2. |
| Browser-specific support represented as universal | **Yes** — a single static allowlist is applied to every browser. |
| OS-dependent support not considered | **Yes** (mostly benign, since HEVC is excluded anyway). |
| Failed direct play can repeatedly fall back and retry | No — there is no fallback at all. §9. |

### 3.2 Finding D1 (high) — `mime_type` is a host-dependent extension guess

`mime_type` is derived once, at scan time, from the file extension only:

```go
// server/cmd/api/movies_scanner.go:274
mimeType := mime.TypeByExtension("." + file.Ext)
if mimeType == "" {
    mimeType = "application/octet-stream"
}
```

The probed container is never consulted — `ffprobe.Format` does not even decode `format_name`. The value is then stored (`movies.mime_type`, `NOT NULL`, no `CHECK`), used verbatim as the eligibility gate on the client, **and** sent verbatim as `Content-Type` by `StreamMovie` (`movie_handler.go:730-739`).

`StreamMovie` also carries a **second, undocumented copy of the same derivation**: when `movie.MimeType` is empty it falls back to `mime.TypeByExtension(filepath.Ext(movie.FileName))` and then to `application/octet-stream` (`movie_handler.go:730-737`). That branch is unreachable today — `mime_type` is `NOT NULL` and the scanner never writes `""` — but it is dead code that re-implements the defect, and it should be deleted along with the fix.

`mime.TypeByExtension` is host-dependent: Go's builtin table is small, and on Unix it is *overridden* by `/etc/mime.types`, `/etc/apache2/mime.types`, `/etc/apache/mime.types`, `/etc/httpd/conf/mime.types` if present. Measured on this development host (Linux, `/etc/mime.types` present):

| Extension | Stored `mime_type` | In `BROWSER_COMPATIBLE_MIME_TYPES`? | Direct play offered |
|---|---|---|---|
| `.mp4` | `video/mp4` | yes | ✅ |
| `.m4v` | `video/mp4` | yes | ✅ |
| `.mkv` | `video/x-matroska` | **no** | ❌ — correct outcome (§5.6), reached by accident |
| `.webm` | **`audio/webm`** | **no** | ❌ |
| `.avi` | `video/vnd.avi` | no | ❌ (correct) |
| `.mov` | `video/quicktime` | no | ❌ (correct) |

Three defects fall out of this. Note that MKV is **not** among them — see the correction below.

**D1a — the MKV outcome is right for the wrong reason.** Neither Chrome nor Firefox plays MKV in a `<video>` element (§5.6), so refusing direct play for MKV is the correct behaviour. But Igloo arrives at it accidentally: not because any rule says "browsers do not support Matroska", but because a host-dependent extension lookup happened to return a string that is absent from a hardcoded allowlist. Change `/etc/mime.types`, run on a distro that maps `.mkv` differently, or add `video/x-matroska` to `BROWSER_COMPATIBLE_MIME_TYPES` in a well-meaning "MKV support" change, and Igloo starts serving MKV to browsers that cannot decode it — with the failure mode described in §5.6 (silent stall at 0 ms, **no `MediaError`**, so even the fallback recommended in §9.3 would not fire). Jellyfin shipped precisely this regression. The defect is that a correct playback decision rests on an accident rather than on a stated rule.

**D1b — WebM is served as audio.** `.webm` resolves to `audio/webm`, so direct play is refused *and*, if a WebM movie is ever played through any path that hits `/stream`, the browser is told the response is audio-only. Note that WebM is unreachable for direct play anyway, because `isVideoDirectPlayable` only accepts H.264 and WebM cannot legally carry it — so `video/webm` in the allowlist is dead code. Same for `video/ogg`: `.ogv` is not in `helpers.ValidVideoExtensions` (`server/cmd/internal/helpers/files.go:27-34`), so no movie can ever have that MIME type.

**D1c — behaviour is not reproducible across hosts.** On a minimal container image without `/etc/mime.types`, `.mkv` yields `""` → `application/octet-stream`, and `.avi`/`.mov` likewise. The *same library* produces different playback modes and different `Content-Type` headers on different machines. Nothing in the codebase pins the mapping. This is the defect that makes D1a dangerous: the accidental-but-correct MKV outcome is not stable.

**Scope consequence.** With MKV correctly excluded, direct play addresses **only `.mp4` and `.m4v` files with H.264 video and AAC/MP3 first-track audio**. In a typical home library — where MKV dominates — that is a minority of titles. This does not make direct play worthless (§4.5), but it does mean the mode list presents `direct` as the headline default (`STREAM_MODES[0]`, and the implicit fallback at `useMoviePlaybackData.ts:53`) for a path most files can never take. See §12, answer 4 for the honest trade-off.

Aggravating factor: `movieUnchanged` (path + size) short-circuits re-scanning, so a wrong `mime_type` persists indefinitely until the file size changes — fixing the derivation will require a forced re-scan or a one-off backfill.

The correct data is already in the row: `movies.container` is `NOT NULL` with `CHECK (container IN ('mkv','mp4','avi','mov','m4v','webm'))` (`schema.sql:215`). The project already has the right pattern for this — `helpers.AudioMimeTypes` (`files.go:21-25`) is an explicit map used by the music scanner. There is simply no `VideoMimeTypes` equivalent.

### 3.3 Finding D2 (high) — the codec gate is looser than the server's own remux gate

`isVideoDirectPlayable` accepts any stream whose codec *name* is in the H.264 family. Meanwhile `docs/ffmpeg.md` §"Remux, Transcode, and Fallback" documents that the server already refuses to *copy* H.264 that is "10-bit, 4:2:2, or 4:4:4 … identified from stored codec profile, bit depth, or pixel format metadata", because browsers are unlikely to play it.

So Igloo is **stricter about handing a browser copied H.264 over HLS than about handing it the original file directly.** An H.264 High 10 MP4 is offered as direct play, and neither Chrome nor Firefox can decode High 10 — the user gets `MEDIA_ERR_DECODE` or `MEDIA_ERR_SRC_NOT_SUPPORTED` and, because there is no fallback (§9), a dead end. The rules needed to fix this already exist server-side and are already tested.

Related, lower severity:
- `vorbis` in `BROWSER_COMPATIBLE_AUDIO_CODECS` is not a valid MP4 audio codec for any browser, and MP4 is now the only container that reaches direct play — so `vorbis` is a pure false positive.
- `flac` and `opus` in MP4 are supported by Chromium but with narrower support elsewhere; neither is common in a movie file, but both are accepted unconditionally.
- HEVC, VP9 and AV1 are excluded from direct play. This is *conservative but not wrong* — Chromium supports HEVC in MP4 only where the platform provides a decoder, which is exactly the OS-dependent case the audit warns against assuming. Leaving them on the HLS path is the safe call.

### 3.4 Finding D3 (high) — nothing ever asks the browser

There is exactly one browser capability probe in the entire frontend:

```ts
// web/src/lib/playback.ts:30-37
export const supportsNativeHLS = (() => {
  if (typeof document === "undefined") return false;
  const v = document.createElement("video");
  return v.canPlayType("application/vnd.apple.mpegurl") !== ""
      || v.canPlayType("application/x-mpegURL") !== "";
})();
```

It is evaluated once at module load and consumed only by the HLS branch (`VideoPlayer.tsx:169`). The direct path never calls `canPlayType`, `MediaSource.isTypeSupported`, or `navigator.mediaCapabilities.decodingInfo`. A static server-side allowlist is applied identically to every browser, so the decision cannot distinguish Chrome from Firefox, or a Firefox build with H.264 from one without.

This is the mechanism that would catch D2 for free: `canPlayType('video/mp4; codecs="avc1.6E0028, mp4a.40.2"')` returns `""` in a browser that cannot handle that profile. Igloo stores `codec_profile` and `codec_level` and could build a correct RFC 6381 string.

One constraint on the fix, which is easy to get wrong: **the probe must only ever narrow eligibility, never widen it.** The direct⊂remux invariant (§3.6) has two enforcers, and the server-side one — watch-room creation, `watch_room_handler.go:371-377` — cannot call `canPlayType`. If a browser probe were allowed to admit a mode the static rules reject, the client and the server would disagree about what is playable. The gate must therefore be `staticRules && canPlayType(...) !== ""`, never `||`. Related: the probe must be a plain function taking an injectable element rather than the module-load IIFE pattern `supportsNativeHLS` uses, or it cannot be unit-tested at all (§10.3).

### 3.5 Finding D16 (high) — a cold deep link starts direct play before eligibility is known

`getAvailableModes` decides whether direct play is allowed. On one reachable path, `GET /api/movies/{id}/stream` is issued **before that function has ever run.**

Two behaviours combine:

```ts
// web/src/routes/_auth/movies/$id/play.tsx:86
if (deps.mode !== undefined) return;      // ← the loader short-circuits entirely
```

```ts
// web/src/lib/movie-playback.ts:134
if (args.effectiveMode !== "direct" && args.techPending) {
  return { kind: "loading", message: "Preparing playback..." };
}
```

The route loader is what fetches technical details ahead of the player (`ensureQueryData(movieTechnicalDetailsQueryOpts)`), but it runs **only when `mode` is absent from the URL** — its whole job is to canonicalise a missing mode, so it returns immediately once one is present. And `deriveMoviePlaybackStatus` deliberately exempts `direct` from waiting on `techPending`, so the direct path mounts the player while metadata is still in flight.

For a URL that already carries `?mode=direct&audio_track=0` — a bookmark, a shared link, a back/forward navigation, a hard refresh — with a cold query cache, the sequence is: loader skips → `techLoaded` is `false` → `availableModes` is `null` → `provisionalMode` stays `direct` (`audio_track` is 0, so `resolveModeForAudioTrack` does nothing) → `streamUrl` is `/api/movies/{id}/stream` → `video.src` is assigned → the request goes out. Technical details land milliseconds later; if the file is MKV, AC-3 or HEVC the mode flips to `remux`, the URL is rewritten, and `VideoPlayer` tears the source down.

**Why this was missed on the first pass.** §3.6's two "what is correct" bullets are both real and both tested — but the E2E test is *"cold non-first audio … never requests the raw stream"*, which exercises `audio_track ≠ 0`, exactly the case `provisionalMode` handles. The `audio_track = 0` case has no equivalent guard and no test. The first draft generalised from the tested case to the untested one.

**Consequences.**

- A wasted range request against a file that was never eligible, on every cold load of such a link.
- A `MediaError` can fire from that request. Today it lands on the error screen; §9's "Try Again" then retries a mode the app has *already* decided against.
- **It constrains the D-FB fallback design.** A fallback keyed on `resolvedMode === "direct"` plus an error code cannot distinguish "the browser genuinely cannot play this eligible file" from "we optimistically started a request the app is about to supersede on its own". Without the distinction, every bookmarked direct link to an ineligible file produces a spurious "switched to remux for you" announcement. D-FB's trigger must additionally require `techLoaded` **and** that `direct` is in `availableModes`.

**Smallest correct fix.** Either drop the `!== "direct"` exemption at `movie-playback.ts:134` and let direct wait for technical details like every other mode, or — better for perceived latency, since the optimistic mount is the point — keep the early mount but suppress both the error screen and the D-FB fallback until `techLoaded`. The second preserves the fast path for the common case (eligible file, warm cache) while making the ineligible case silent rather than wrong.

### 3.6 What is correct

Worth stating plainly, because these were designed deliberately and hold up:

- **The `direct → remux` upgrade is right.** `resolveModeForAudioTrack` (`playback.ts:117-125`) forces `remux` whenever a non-first audio track is requested, because direct play cannot select a track. The documented invariant — remux requires a strict subset of direct's conditions, so remux is available whenever direct is — is real and is regression-tested (`playback-default-settings.test.ts`, *"never offers direct play without remux"*). It is enforced in five places: `getDefaultPlaybackSettings`, `resolvePlaybackSettings`, `useMoviePlaybackData`'s `provisionalMode`, the settings dialog, and server-side for watch rooms (`watch_room_handler.go:371-377`).
- **Cover art is excluded from primary-video selection** on both sides: `getPrimaryVideoStream` skips `mjpeg/png/gif/bmp` (`playback.ts:51-59`), and the scanner drops both `attached_pic == 1` and cover-art codecs before inserting rows (`movies_scanner.go:934-939`). An MP3-style embedded cover cannot be mistaken for the movie.
- **The preferences-before-stream gate is right.** `playbackPreferencesReady` (`useMoviePlaybackData.ts:89-90`) blocks the player from mounting until auth and playback settings resolve, so a cold load never fires a `/stream` request that the user's *preferences* would have redirected. This is covered by an E2E test (*"playback waits for preferences before requesting media"*). Note the boundary: it gates on **preferences**, not on **technical details**, and those are different gates — see D16.
- **A cold deep link with non-first audio never touches `/stream`.** When technical details are pending or errored, `provisionalMode` already resolves `direct + audio_track=1` to `remux` (`useMoviePlaybackData.ts:54`, `:121-125`). Tested both ways. This holds **only for `audio_track ≠ 0`**; the `audio_track = 0` case is D16.
- **Bitmap subtitles are refused** at selection (`resolvePlaybackSettings`), in the dialog (rendered `disabled`, labelled "image-based"), and at the server VTT endpoint.

---

## 4. HTTP media delivery audit

### 4.1 The handler

`StreamMovie` (`server/cmd/api/movie_handler.go:689-742`) is short and correct in structure: parse ID → `GetMovieForDirectStream` (three columns) → `os.Open` → `defer file.Close()` → `Stat` → set `Content-Type` → `http.ServeContent(w, r, movie.FileName, stat.ModTime(), file)`.

Delegating to `http.ServeContent` is the project's preferred native Go pattern (the sibling `StreamTrack` even documents it) and it is the right call — it means most of the checklist is correct for free:

| Behaviour | Status | Source |
|---|---|---|
| `GET` | ✅ | `routes.go:172` |
| `HEAD` | ❌ **405** | Finding D4 |
| Range parsing | ✅ | `ServeContent` |
| `206 Partial Content` | ✅ | `ServeContent` |
| `200 OK` when no range | ✅ | `ServeContent` |
| `416 Range Not Satisfiable` | ✅ | `ServeContent`, with `Content-Range: bytes */size` |
| `Accept-Ranges: bytes` | ✅ | `ServeContent` |
| `Content-Range` | ✅ | `ServeContent` |
| `Content-Length` | ✅ | per-range, correct |
| `Content-Type` | ⚠️ | set by the handler from `mime_type` — see D1 |
| `ETag` | ❌ | never set — Finding D5 |
| `Last-Modified` | ✅ | `stat.ModTime()` |
| Conditional requests | ⚠️ | `If-Modified-Since` / `If-Unmodified-Since` / `If-Range` work on date only |
| Cache headers | ❌ | no `Cache-Control` at all; browser heuristic caching applies |
| Open-ended ranges (`bytes=N-`) | ✅ | `ServeContent` |
| Suffix ranges (`bytes=-N`) | ✅ | `ServeContent` |
| Invalid ranges | ✅ | ignored or 416 per RFC 9110 |
| Multiple ranges | ✅ | `multipart/byteranges` |
| Seeking / repeated seeks | ✅ | stateless; each seek is a fresh range request |
| Request cancellation / client disconnect | ✅ | `io.Copy` returns on write error; `defer` closes the FD |
| Large-file handling | ✅ | streamed, never buffered whole |
| Memory usage | ✅ | 32 KiB copy buffer per request |
| FD lifecycle | ✅ | one `os.Open` per request, `defer file.Close()`, `ServeContent` is synchronous |
| Whole file accidentally buffered | ✅ no | scs abandoned its buffered writer in v2; only `Write`/`WriteHeader` are wrapped |
| Compression middleware on media | ✅ none | no `middleware.Compress`, no gzip handler anywhere in `server/cmd` |
| Auth middleware altering range behaviour | ✅ no | auth runs before any write and either 401s or passes through |
| Error middleware corrupting partial responses | ✅ no | every error path returns before the first write |
| Reverse proxy | ⚠️ | not accounted for; no `Cache-Control`/`ETag` to guide one, and `Vary: Cookie` is emitted per response |

### 4.2 Finding D4 (medium) — `HEAD /api/movies/{id}/stream` returns 405

The route is registered with `r.Get` only (`routes.go:172`) and `middleware.GetHead` is not installed anywhere in the router (`routes.go:10-26`). chi therefore rejects `HEAD` with 405 before `ServeContent` — which handles HEAD correctly — ever sees it. `docs/openapi.json` documents `get` only, so the spec and the implementation agree; both diverge from what media clients, link previewers, and download managers do. The browser `<video>` element does not HEAD-probe, so this does not break current playback; it will break anything else pointed at the URL.

### 4.3 Finding D5 (low) — no `ETag`

Validation is `Last-Modified` only, i.e. one-second granular. `If-Range` therefore cannot distinguish a file rewritten within the same second, and a client resuming a range after such a rewrite can splice bytes from two different files. For a media library this is a narrow window, but the fix is trivial and `ServeContent` will use a strong `ETag` if the handler sets one before calling it.

### 4.4 Finding D6 (low) — sendfile is defeated

`scs.LoadAndSave` wraps the `ResponseWriter` in `sessionResponseWriter`, which in `scs/v2@v2.9.0` implements `Write`, `WriteHeader` and `Unwrap` — and **not** `io.ReaderFrom`. It is the innermost wrapper, so `ServeContent`'s `io.CopyN` cannot reach a `ReadFrom` on anything beneath it, and every byte of every movie is copied through a 32 KiB userspace buffer instead of `sendfile(2)`. The same wrapper also re-commits the session and re-emits `Vary: Cookie` on each range response, and bearer-token clients pay one `GetDeviceByTokenHash` SELECT per range request.

Correctness is unaffected and memory is bounded. The cost is CPU and syscalls, and it scales with bitrate × concurrent viewers — precisely the 4K multi-user case. This is a real but low-priority efficiency finding; it also cannot be fixed inside `StreamMovie` (it needs the session middleware scoped off the media routes, or a `ReaderFrom` pass-through).

### 4.5 Efficiency verdict for large 4K files and multiple users

Direct play is by far the cheapest path Igloo has: no FFmpeg process, no transcode directory, no session-cache entry, no per-user session cap, no CPU permit. Serving a 60 GB file to three viewers costs three open FDs and three 32 KiB buffers. The only real costs are D6's lost zero-copy and the per-range session work. **Efficiency is not a reason to remove direct play — it is the strongest argument for keeping it.**

The limiting factor is reach rather than efficiency: because MKV cannot be served directly to a browser at all (§5.6), the large 4K files that would benefit most are usually the ones that must go through remux regardless. Direct play delivers its full efficiency advantage only for large MP4/M4V sources. See §12, answer 4.

### 4.6 Adjacent defects in `StreamWatchRoomMovie`

Not in the primary scope but on the same handler pattern: `StreamWatchRoomMovie` (`watch_room_media_handler.go:73-123`) does not special-case `sql.ErrNoRows`, so a room pointing at a deleted movie yields 500 instead of 404. And `watch_room_handler.go:359-364` bounds `audio_track` only from above — a negative value passes room creation.

---

## 5. Browser compatibility assumptions

**Read this section as a set of stated assumptions, not as evidence.** Every cell below is documentation-sourced and unmeasured except where §5.7 attributes it to a direct browser observation. One cell in the first draft was confidently wrong in a direction that would have shipped a playback regression (§5.6), which is the honest calibration for how much weight the rest deserves.

The tables exist to justify a recommendation — finding D3, *make the code ask the browser* — **not** to be transcribed into code. A hand-maintained browser support matrix is not a maintainable artifact in this repository: it has no owner, no test, and no expiry, and it drifts silently as browsers ship. `BROWSER_COMPATIBLE_MIME_TYPES` is that matrix, already committed, already partly accidental (§3.2). The conclusion to draw from §5 is that Igloo should stop maintaining one, not that this particular version of it is right.

`web/AGENTS.md` scopes the client to "standard desktop and mobile web browsers"; the repository names no browser support matrix, so Chrome/Chromium and Firefox are treated as the required targets (per the audit brief), with Safari noted where it changes a conclusion.

### 5.1 Definitions kept separate

- **Native `<video>` support** — what `src=` playback can decode. This is the only thing that matters for direct play.
- **MSE support** — what `MediaSource.isTypeSupported` accepts. Relevant to the HLS fallback, *not* to direct play; the two lists differ (notably for containers).
- **OS/hardware-dependent decode** — codecs a browser will expose only when the platform supplies a decoder (HEVC, and AV1 on older hardware). Never safe to assume.
- **Experimental / flag-gated APIs** — present in the spec and in the browser binary but off by default. `audioTracks` is the decisive case.

### 5.2 Containers, for `src=` playback

| Container | Chrome / Chromium | Firefox |
|---|---|---|
| MP4 | ✅ | ✅ |
| WebM | ✅ | ✅ |
| **MKV** | ❌ **not playable via `src=`** — see §5.6 | ⚠️ **version-dependent; treat as not playable** — `media.mkv.enabled` is default-on from Firefox 145, but its scope excludes H.264. See §5.6 |
| Ogg | ✅ | ✅ |
| AVI | ❌ | ❌ |
| MOV / QuickTime | ❌ (legacy Safari only) | ❌ |

### 5.3 Codecs, for `src=` playback

| Codec | Chrome / Chromium | Firefox | Notes |
|---|---|---|---|
| H.264 | ✅ in MP4 only (**not** MKV — §5.6) | ✅ in MP4 only (**not** MKV — §5.6) | The one codec Igloo allows for direct play. Codec support does not imply container support: the pair is what matters, which is what `canPlayType` takes and a static allowlist cannot express. |
| H.264 High 10 / 4:2:2 / 4:4:4 | ❌ | ❌ | Igloo does not exclude these — Finding D2 |
| HEVC | ⚠️ OS/hardware-dependent | ⚠️ | Correctly excluded by Igloo |
| VP9 | ✅ | ✅ | Excluded (conservative) |
| AV1 | ✅ | ✅ (with build caveats) | Excluded (conservative) |
| AAC | ✅ | ✅ | Allowed |
| MP3 | ✅ | ✅ | Allowed |
| Opus | ✅ in WebM; in MP4 flag-dependent | ✅ in WebM | Allowed unconditionally — mild over-reach |
| Vorbis | ✅ in WebM/Ogg only | ✅ in WebM/Ogg only | Allowed but unreachable — MP4 is the only container that gets here |
| FLAC | ✅ | ✅ | Allowed |
| AC-3 / E-AC-3 | ❌ | ❌ | Correctly excluded |
| DTS / DTS-HD | ❌ | ❌ | Correctly excluded |
| TrueHD | ❌ | ❌ | Correctly excluded |

### 5.4 Multi-channel audio

Neither Chrome nor Firefox exposes a channel-layout negotiation API to page script. A 5.1 AAC track in MP4 will decode, and the browser downmixes to the output device's layout. Igloo ignores `channels`/`channel_layout` for direct play; in practice this is benign for AAC, and the codecs where multi-channel actually breaks (AC-3, DTS, TrueHD) are already excluded by codec name.

### 5.5 Track APIs — the decisive constraint

**`HTMLMediaElement.audioTracks` / `AudioTrackList` is not usable in either target browser.**

| Browser | Status |
|---|---|
| Chrome | Not supported or disabled by default across **all** versions (37 → 153+); requires a runtime flag |
| Edge | Supported 12–18; disabled by default from 79 onward |
| Firefox | Disabled by default or unsupported across all versions (2 → 155+) |
| Safari / iOS Safari | ✅ fully supported since 7 |
| Chrome Android / Firefox Android / Samsung Internet | ✗ |

caniuse puts global availability of the feature at roughly 15% of usage, essentially all of it Safari. MDN marks both `HTMLMediaElement.audioTracks` and `AudioTrackList` as **not Baseline — limited availability**.

The consequence is structural and cannot be engineered around in the browser: **during direct playback in Chrome or Firefox, page script cannot enumerate the file's audio tracks, cannot know which one the browser chose, and cannot switch tracks.** No player library changes this (§8). The only mechanism that can deliver a chosen audio track to these browsers is a server-side re-mux — which is exactly what Igloo's `direct → remux` upgrade does. That design decision is correct and this audit endorses it.

**Text tracks.** `HTMLMediaElement.textTracks` and `<track>` are universally supported for *out-of-band* WebVTT. In-band text tracks are a different matter: Safari surfaces them for HLS, but Chrome and Firefox do not expose embedded SRT/ASS/PGS from MP4 or MKV as `TextTrack` objects. So embedded subtitle selection during direct play is also impossible natively — and again Igloo's design (server-side FFmpeg conversion to WebVTT, injected as `<track>`) is the correct workaround. Image-based subtitles (PGS/DVD/DVB) can never work in a `<track>` and are correctly rejected server-side.

**HDR.** Browsers will decode an HDR10 stream where the codec is supported, but tone mapping and display are entirely OS/display-dependent; there is no page-script capability query that answers "will this look right". Igloo's transcode path tone-maps deliberately (`docs/ffmpeg.md` §HDR Tone Mapping); direct play does not and cannot. Since HDR sources are overwhelmingly HEVC — already excluded — this rarely bites today, but an HDR10 H.264 file would be offered as direct play and rendered with wrong colours rather than failing outright.

### 5.6 MKV — a corrected claim, and why the source was misleading

**Neither Chrome nor Firefox plays MKV files in a `<video>` element.** This was reported from direct observation by the maintainer and is corroborated below. An earlier draft of this audit claimed the opposite; that claim is withdrawn, and it is worth recording *why* it was wrong, because the same trap is easy to fall into again.

**The observation's provenance matters and is worth stating explicitly**, because the obvious failure mode would be circular: Igloo refuses MKV for direct play via the MIME allowlist, so "MKV does not play in Igloo" would prove nothing about the browser. Confirmed with the maintainer (2026-07-27): the files were opened **directly in the browser, outside Igloo**. The observation is therefore evidence about the browser, not about this codebase, and it stands. Firefox remains the softer of the two cells — see the version caveat in §5.2 — but nothing about the recommendation depends on resolving it.

**What misled the analysis.** Chromium's `media/base/mime_util_internal.cc` does register Matroska, unconditionally and with no build flag or feature gate:

```cpp
AddContainerWithCodecs("audio/matroska",   mkv_audio_codecs);
AddContainerWithCodecs("video/matroska",   mkv_codecs);
AddContainerWithCodecs("audio/x-matroska", mkv_audio_codecs);
AddContainerWithCodecs("video/x-matroska", mkv_codecs);
```

Reading that table as "these are the containers `<video src=>` can play" is the error. **That map is not the media-element support list.** In practice `canPlayType("video/x-matroska")` returns `""` in Chrome and progressive playback of an MKV fails. This is exactly the distinction §5.1 sets out — MSE / internal container registration / demuxer capability / hardware decode are four different things from native `src=` support — and the audit fell into it anyway. The lesson for Igloo: **a browser-source lookup is not a substitute for `canPlayType()`.** This is now the strongest argument for finding D3.

Firefox is similar but for a different reason. Bugzilla 1422891 (`media.mkv.enabled`, default-on since Firefox 145) is a **meta bug that remains open**, and its scope is VP8/VP9/AV1/HEVC video with Opus/Vorbis/AAC audio. H.264-in-MKV is explicitly contested there on patent-licensing grounds — so the one codec combination Igloo would care about is the one Firefox does not cover. A pref being default-on is not the same as the format being playable.

**The failure mode is the dangerous part.** Jellyfin shipped an MKV entry in Chrome's DirectPlay profile and filed [jellyfin-web#7651](https://github.com/jellyfin/jellyfin-web/issues/7651) (opened 2026-03-03, still open): codec-compatible MKV files are selected for direct play and then fail **silently at 0 ms** — no UI error, no FFmpeg invocation, no server-side error log, only a playback-reporting entry. The quoted assessment is *"Chrome supports WebM but not MKV proper. The two standards are different and, while it may work for some files, it won't for others."* Their recommended fix is to remove MKV from the direct-play profile and add a server-side fallback.

Two consequences for this report:

1. **Do not add `video/x-matroska` to `BROWSER_COMPATIBLE_MIME_TYPES.`** Igloo's current refusal of MKV is correct; only the *mechanism* is wrong (D1a).
2. **A silent 0 ms stall produces no `MediaError`,** so an error-code-triggered fallback cannot catch it. The fallback design in §9.3 needs a stall guard as well as error codes.

A Jellyfin reference checkout exists at `/home/jose-ibanez/projects/jellyfin` per `server/AGENTS.md`; this issue is a useful cross-check when revisiting container decisions.

### 5.7 Sources

- Maintainer observation, 2026-07-27: neither Chrome nor Firefox plays MKV files, observed by opening the files **directly in the browser, outside Igloo** (confirmed 2026-07-27; provenance matters, see §5.6). This is the authoritative source for §5.6 and overrides the source reading below. It is also the only claim in §5 backed by an observation rather than by documentation.
- [jellyfin/jellyfin-web#7651](https://github.com/jellyfin/jellyfin-web/issues/7651) — "Chrome DeviceProfile hardcodes MKV in DirectPlayProfiles — causes silent 0ms playback failure for codec-compatible MKV files". Opened 2026-03-03, open. Retrieved 2026-07-27.
- Chromium `media/base/mime_util_internal.cc`, `AddSupportedMediaFormats()` — registers `video/x-matroska` unconditionally. **Cited as the source of a corrected error, not as evidence of playback support.** Retrieved 2026-07-27.
- Mozilla Bugzilla [1422891](https://bugzilla.mozilla.org/show_bug.cgi?id=1422891) — Matroska meta bug, **status NEW/open**; `media.mkv.enabled` default-on since Firefox 145; scope is VP8/VP9/AV1/HEVC + Opus/Vorbis/AAC; H.264-in-MKV contested on licensing grounds. Retrieved 2026-07-27.
- [caniuse: `mdn-api_htmlmediaelement_audiotracks`](https://caniuse.com/mdn-api_htmlmediaelement_audiotracks). Retrieved 2026-07-27.
- [MDN: `HTMLMediaElement.audioTracks`](https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/audioTracks) and [`AudioTrackList`](https://developer.mozilla.org/en-US/docs/Web/API/AudioTrackList) — Baseline status. Retrieved 2026-07-27.
- [MDN: Media container formats](https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Containers). Retrieved 2026-07-27.

---

## 6. Stream and track behaviour

### 6.1 Finding D7 (high) — ffprobe dispositions are discarded

```go
// server/cmd/internal/ffprobe/ffprobe_metadata.go:51-53
type StreamDisposition struct {
	AttachedPic int `json:"attached_pic"`
}
```

One field. `default`, `forced`, `comment`, `dub`, `original`, `hearing_impaired` and `visual_impaired` are all present in ffprobe's JSON and all silently dropped by the decoder. Downstream:

```go
// server/cmd/api/movies_scanner.go:1037-1051
IsForced:    false,
IsDefault:   false,
```

`subtitles.is_forced` and `subtitles.is_default` are therefore `false` for **every row in the database**, and `/technical-details` serves those constants to the client. `audio_streams` has no disposition columns at all (`schema.sql:320-341`), and neither does `video_streams`.

Visible consequences:
- `formatSubtitleLabel` (`playback.ts:346-347`) appends "Forced" and "Default" badges that can never render. Dead UI.
- Forced subtitles cannot be auto-enabled — the standard behaviour users expect for foreign-language segments.
- Commentary, audio description, dub and original tracks are indistinguishable from the main mix; the audio picker labels them only by language and channel layout (`formatPlaybackAudioLabel`), so three English tracks render as three identical "English · 5.1" entries.
- Hearing-impaired subtitle tracks are indistinguishable from regular ones.
- **The application cannot determine which stream the browser will play.** This is the root cause of D8.

Related: `Stream.Tags` has no case-normalising `UnmarshalJSON` (unlike `FormatTags`, which has one with alias fallbacks). Matroska muxers that write uppercase `TITLE`/`LANGUAGE` stream tags therefore produce rows with `NULL` language and title — the tracks lose their labels and stop matching `preferred_audio_language`/`preferred_subtitle_language`.

Because the project is pre-production and `server/AGENTS.md` explicitly permits direct schema edits with no migrations, adding these columns is cheap.

### 6.2 Finding D8 (high) — direct-play audio eligibility assumes "first stream = the one that plays"

```ts
// web/src/hooks/useMoviePlaybackData.ts:100-107
availableModes = getAvailableModes(
  primaryVideo?.height ?? 0,
  primaryVideo?.codec,
  audioStreams[0]?.codec,   // ← the assumption
  techData.data.movie?.mime_type,
);
```

`audioStreams` is `ORDER BY stream_index`, so `[0]` is the lowest ffprobe stream index — a *positional* choice, not a *disposition* choice. The comment in `playback.ts:80-85` states the intent honestly ("`audioCodec` must be the FIRST audio stream's codec"), and `DIRECT_PLAY_AUDIO_TRACK = 0` encodes the same assumption. But the browser selects by container track order and default disposition, and Igloo does not store dispositions (D7). Failure shapes:

- **First stream is AAC commentary, second is AC-3 main.** Direct play is offered (first codec is AAC). The user hears commentary, and the player has no control to change it — the only escape is to go back to the details page and pick track 2, which forces `remux`.
- **First stream is AC-3, second is AAC** — direct correctly refused. Right answer for the right reason.
- **Container default disposition points at stream 2.** Igloo evaluates stream 1 and the browser plays stream 2. Eligibility was computed against a stream that never plays.
- **Multiple default dispositions, or none.** Not represented at all; behaviour is browser-defined and unpredictable.

**The fix must refuse on ambiguity, not on absence.** The tempting rule — "require an unambiguous default disposition, else refuse direct" — over-corrects. Not every muxer writes a `default` flag, §3.2 has *already* narrowed direct play to `.mp4`/`.m4v`, and stacking a disposition requirement on top could leave the mode addressing almost nothing, which would undercut §12 answer 4's own case for keeping it. The rule that buys the certainty without the collapse:

| Audio streams | `default` dispositions | Direct play |
|---|---|---|
| exactly 1 | any | ✅ eligible — nothing to be ambiguous about |
| ≥ 2 | exactly one, on index 0 | ✅ eligible |
| ≥ 2 | exactly one, **not** on index 0 | ❌ refuse — this is the commentary-first failure above |
| ≥ 2 | more than one | ❌ refuse — browser-defined, unpredictable |
| ≥ 2 | none | ✅ eligible, evaluated against stream 0 |

The last row is the deliberate trade: with no flag present, container track order is what the browser follows, so stream 0 is the right stream to evaluate. That is the status quo, and D7 does not make it worse — it just stops being a guess in the three rows above it.

Note the identifier convention is otherwise sound and consistently documented: the `audio_track` API parameter is an **ordinal into the `stream_index`-ordered rows**, resolved to the absolute ffprobe index server-side at session creation (`hls_session.go:966-968`; `docs/ffmpeg.md` §Audio Track Selection). Subtitles use the same convention (`subtitle_handler.go:33`). Raw ffprobe indices are never exposed to the client. Stability across re-scans is a separate matter — `processMovieStreams` deletes and re-inserts all rows, so a saved ordinal survives only if the stream order does; there is no `UNIQUE(movie_id, stream_index)` constraint to protect it.

### 6.3 Finding D9 (medium) — "direct" describes the container, not the experience

The task asked whether the app labels playback as direct when the behaviour differs. The `direct → remux` direction is handled well: `?mode=direct&audio_track=1` resolves to `remux` *before* any media request, the URL is rewritten to match, and `modeLabel` reflects `resolvedMode`, not `search.mode` (`useMoviePlaybackData.ts:154-155`). This is tested at both unit and E2E level.

The genuine mislabel runs the other way. When a file *is* served directly, the badge reads **"Original file — plays as-is"** while the app has no idea which audio stream the browser selected (D7, D8) and offers no way to change it. The label is true about bytes and misleading about experience.

### 6.4 What happens for each file shape

| File contains | Actual behaviour today | Assessment |
|---|---|---|
| Multiple video streams | All non-cover-art streams stored; `getPrimaryVideoStream` picks the first non-cover-art; the browser picks its own. No coordination. | ⚠️ undefined for genuine multi-angle files (rare) |
| Multiple audio streams | Eligibility judged on `[0]`; browser plays its own choice; no in-player switching | ❌ D8 |
| Multiple subtitle streams | All listed; one selectable pre-playback; sideloaded as WebVTT | ✅ |
| Same-language duplicates | Rendered as identical picker entries ("English · 5.1" ×3) | ❌ D7 — needs title/disposition to disambiguate |
| Commentary audio | Indistinguishable from main audio | ❌ D7 |
| Audio description | Indistinguishable | ❌ D7 |
| Forced subtitles | `is_forced` always `false`; never auto-enabled; badge never renders | ❌ D7 |
| Hearing-impaired subtitles | Indistinguishable | ❌ D7 |
| Missing language metadata | `formatPlaybackAudioLabel` falls back to "Track N · <layout>"; `formatSubtitleLabel` to "Track N"; `normalizeLang` returns `undefined` and `<track srclang="">` | ✅ graceful |
| Missing default dispositions | Not represented | ❌ D7 |
| Multiple default dispositions | Not represented | ❌ D7 |
| Incorrect default dispositions | Not represented; and Igloo would not detect the mismatch | ❌ D7 |
| Attached pictures | `attached_pic == 1` filtered at scan (`movies_scanner.go:934-939`) | ✅ |
| Cover art | `mjpeg/png/gif/bmp` filtered at scan **and** skipped by `getPrimaryVideoStream` | ✅ belt and braces |
| Chapters | Stored, normalized against duration, rendered in `ChapterMenu`, seek works in direct play | ✅ |
| External subtitle files | **Not supported at all** — the scanner indexes no sidecar `.srt`/`.ass`; only embedded streams reach `subtitles` | ➖ gap, not a defect |

### 6.5 Subtitle behaviour in direct play

Injection is imperative (`VideoPlayer.tsx:286-312`): remove any existing `track[data-subtitle]`, create a `<track kind="subtitles">` pointing at `/api/movies/{id}/subtitles/{n}/web.vtt`, append, then set `track.track.mode = "showing"`.

- **Enabling / switching:** works; the effect keys on the URL so a change swaps the track.
- **Disabling:** only via the URL (`subtitle_track=off`) or the details-page dialog. There is **no in-player subtitle control** (§7), so a user cannot turn subtitles off mid-film.
- **Surviving seeks:** yes for direct play — a seek is just `video.currentTime`, and the `<track>` is untouched.
- **Surviving playback restoration:** ⚠️ **unverified — see D11.** A `start` change does tear down and reload the media element (D10, confirmed) while the subtitle effect does not re-run. Whether `mode = "showing"` actually survives that is a prediction this audit did not test, and the HTML spec suggests it does.
- **Styled (ASS/SSA) subtitles:** converted to WebVTT server-side, losing positioning, karaoke and styling. Text survives; presentation does not. This is inherent to the `<track>` approach and acceptable.
- **Image-based (PGS/DVD/DVB):** can never work in direct play. Correctly rejected at both ends.

The `srclang` attribute is set from `normalizeLang`, which maps ISO 639-2 → 639-1 and returns `undefined` for anything else, producing `srclang=""`. Harmless but not spec-clean.

---

## 7. Custom player audit

### 7.1 Finding D10 (medium) — the direct-play source is torn down whenever `startSec` changes

```ts
// web/src/components/playback/VideoPlayer.tsx:165-253
useEffect(() => {
  const video = videoRef.current;
  if (!video || !src) return;

  if (isHlsSource && !supportsNativeHLS) { /* … hls.js … */ }

  video.src = src;
  return () => {
    video.removeAttribute("src");
    video.load();
  };
}, [isHlsSource, src, startSec, videoRef]);   // ← startSec
```

`startSec` belongs in this dependency list only for the hls.js branch, which needs `startPosition` at construction time. For direct play the source URL is a constant (`/api/movies/{id}/stream`), so any change to `start` — the resume dialog calls `navigateToPlaybackPosition`, which rewrites the search param — runs the cleanup and then re-assigns the *same* URL. The result is `removeAttribute("src")` + `load()` + fresh resource selection: the buffer is discarded, a new range request is issued from byte 0, and the element flashes.

The separate start-restore effect (`:255-275`) then correctly re-attaches its `loadedmetadata` listener, so the seek itself still lands. The defect is wasted work and a visible glitch, not a wrong position.

### 7.2 Finding D11 (medium, **unverified**) — subtitles may silently drop after that reload

The `<track>` effect depends on `[subtitleUrl, videoRef]` (`VideoPlayer.tsx:312`). During a D10 reload the subtitle URL does not change, so the effect does not re-run and `track.track.mode = "showing"` is never re-applied after the media element's resource selection restarts. The concern is that subtitles end up present in the DOM but disabled.

**This is a prediction, and the spec argues against it.** The media load algorithm that `video.load()` runs forgets only the element's *media-resource-specific* text tracks — that set is in-band tracks. A `TextTrack` created from a `<track>` element is not in it, so neither the track nor its `mode` should be discarded, and `"showing"` should survive. No browser was driven by this audit (§13), so neither reading is confirmed.

Keep the finding, but **check it before fixing it**: the trigger (D10) is real and confirmed, the failure would be silent if it exists, and the check is a single test — the subtitle-persistence spec already listed in §10.3 settles it either way. If the spec reading holds, D11 costs nothing but the test; the test is worth having regardless, because D10's reload is exactly the kind of teardown that browsers have historically disagreed about.

### 7.3 Finding D12 (low) — reload and auto-resume machinery is inert for direct play

`buildMovieStreamUrl` returns a constant for direct (`movie-playback.ts:176`), so:
- `streamReloadKey` / `forceReload` cannot bust anything on the direct path.
- The auto-resume effect keyed on `[streamUrl]` (`play.tsx:492-516`) never re-runs, so `pendingAutoPlayOnLoadRef` is never consumed for direct play.

In practice nothing breaks today, because the only direct-play caller of `navigateToPlaybackPosition` is the resume dialog, which fires while paused. But the mechanism is dead code on this path and would silently fail to resume playback if a fallback (§9) ever navigated mid-playback — which is exactly the change this report recommends.

### 7.4 Control surface

| Control | Present | Notes |
|---|---|---|
| Play / pause | ✅ | button + Space/K + click-to-toggle in fullscreen + Media Session |
| Seek bar | ✅ | native `input[type=range]` (`ProgressBar.tsx:200`) — correct per prior VoiceOver work |
| Skip ±10 s | ✅ | buttons + J/L + arrows |
| Volume / mute | ✅ | `VolumeControl`, arrows + M |
| Fullscreen | ✅ | `useVideoFullscreen`, F, with an immersive-viewport fallback for iOS |
| Chapters | ✅ | `ChapterMenu`, keyboard-operable, announced assertively on jump |
| Buffering indicator | ✅ | 300 ms debounce so sub-perceptual stalls don't flash |
| Stream-mode badge | ⚠️ | present but not announced — D13 |
| **Audio track** | ❌ | **absent** — only settable pre-playback, and only by forcing `remux` |
| **Subtitle track** | ❌ | **absent** — cannot be toggled off mid-film |
| **Playback speed** | ❌ | absent |
| **Picture-in-picture** | ❌ | absent |

The two absences that matter are audio and subtitles. For **subtitles this is a pure UI gap** — the data and the delivery mechanism already exist, and adding a menu is a contained change. For **audio it is not** — switching tracks requires changing the playback mode (§5.5), so an in-player audio menu must trigger a `direct → remux` navigation, i.e. a stream restart with position preservation.

### 7.5 Lifecycle and React behaviour

| Aspect | Status |
|---|---|
| Event listener cleanup | ✅ all imperative listeners removed; JSX handlers managed by React |
| `useEffectEvent` usage | ✅ correct — keeps callbacks out of dep arrays without stale closures |
| React Compiler compliance | ✅ no manual memoization added |
| hls.js disposal | ✅ `cancelled` flag + `destroy()` guards the async import race |
| Buffering timer cleanup | ✅ cleared on unmount and on `src` change |
| Source changes | ⚠️ over-eager for direct — D10 |
| Component remounts | ✅ `videoRef` is owned by the route, so the element survives player re-renders |
| Position restoration | ✅ correct, double-clamped |
| Direct-play error mapping | ✅ all four `MediaError` codes mapped to distinct messages |

### 7.6 Accessibility

Strong points: every button has a descriptive accessible name including its shortcut; the seek bar is a real range input with `aria-valuetext`; `role="group"` wraps the progress and control clusters; three `LiveAnnouncer` regions cover play/pause, capacity waits, and chapter jumps; decorative icons are `aria-hidden`; keyboard coverage is complete and documented in an `sr-only` paragraph; focus rings are preserved throughout.

Defects found:

- **Finding D13 (low).** `aria-label="Current stream quality"` is applied to a plain `<span>` (`MoviePlayerControls.tsx:143-148`). `aria-label` on a generic-role element is ignored by most assistive technology, so the badge is announced as bare text ("Original file — plays as-is") with no indication of what it means.
- **Finding D14 (low).** The fullscreen playback surface is a `<div role="button" tabIndex={0}>` wrapping the `<video>` and the capacity overlay (`play.tsx:738-749`). A `role="button"` containing a media element is not a valid pattern; the same toggle is already available on the footer play button and on Space/K, so the div could be presentational with a click handler and no role.
- **Finding D15 (trivial).** The `sr-only` shortcut text renders "…J or Left arrow to rewind10 seconds…" — JSX strips the newline before `{MOVIE_SEEK_STEP_SEC}` at `play.tsx:700`. The very next interpolation uses `{" "}` correctly.
- `aria-label` on the `<video>` element itself (`VideoPlayer.tsx:333`) is largely inert without `controls`, but harmless.

### 7.7 Maintainability

The player is well-factored: eight focused hooks, a thin `VideoPlayer`, a presentational controls component, and pure functions for every decision, all unit-tested. Extending it with a subtitle menu or a speed control is a normal-sized change. Nothing here argues for replacement (§8).

---

## 8. Player-package evaluation

### 8.1 The question a package cannot answer

The confirmed direct-play problems are: a wrong MIME derivation (server), an incomplete codec gate (server metadata + client logic), discarded ffprobe dispositions (server), an unpredictable browser audio-track selection (browser limitation), and a missing fallback (client routing). **None of these is a UI problem.**

Specifically: `HTMLMediaElement.audioTracks` is unavailable in Chrome and Firefox (§5.5). A player library runs in the same page, on the same element, against the same API surface. It cannot enumerate or switch embedded audio tracks that the browser does not expose. Every player that offers an "audio track" menu does so for **adaptive-streaming renditions** (HLS/DASH, via hls.js/shaka), not for tracks embedded in a progressively-served file. That is a capability Igloo already has, through `?audio_track=N` on the HLS path.

### 8.2 Candidates

| | Vidstack | Media Chrome | Video.js v10 | Plyr |
|---|---|---|---|---|
| Role in the stack | UI components + player hooks | UI web components only | UI + engine | UI wrapper |
| Native `<video>` support | ✅ | ✅ | ✅ | ✅ |
| Works with existing hls.js path | ✅ | ✅ (bring your own engine) | ✅ (own HLS engine — would displace hls.js) | ✅ |
| Direct-play audio-track control | ❌ impossible | ❌ impossible | ❌ impossible | ❌ impossible |
| Direct-play subtitle control | ✅ (`<track>` UI) | ✅ | ✅ | ✅ |
| HLS audio-rendition control | ✅ | ✅ | ✅ | partial |
| HLS subtitle-rendition control | ✅ | ✅ | ✅ | partial |
| React integration | ✅ first-class hooks | ✅ web components | ⚠️ wrapper | ⚠️ wrapper |
| Keyboard / SR / focus | ✅ states WCAG 2.2, WAI-ARIA, CVAA compliance | ✅ | ✅ | ✅ |
| Maintenance | active; **merging with Media Chrome and Plyr** | active; merging | **v10 is a ground-up Mux rewrite, GA targeted mid-2026** | merging into the above |
| Bundle impact | modular but non-trivial | small | large | medium |
| Migration difficulty | high | high | high | medium |
| Solves any confirmed problem? | **no** | **no** | **no** | **no** |

Two further considerations specific to this repository:

- `web/AGENTS.md` states: *"Do not add a new UI dependency when shadcn/ui, Radix UI, or an existing project component can provide the required behavior."* A subtitle menu is a Radix dropdown; a speed control is a Radix menu. Both already have precedent in `ChapterMenu`.
- The candidate landscape is mid-consolidation. Vidstack, Media Chrome and Plyr have announced they are merging, and Video.js v10 is a rewrite targeting GA mid-2026. Adopting any of them now means adopting a moving target.

### 8.3 Recommendation

**Keep the custom player.** Replacing it would cost a large migration and a new UI dependency, would not fix a single confirmed defect, and would land on a library ecosystem that is actively reorganising. The real gap is *two missing menus* in a player that is otherwise accessible, tested and well-factored — a far smaller job than a migration, and one that keeps the direct/remux coupling (which is Igloo-specific and cannot be expressed in a generic player) inside Igloo's own code.

This recommendation should be revisited if Igloo ever needs DASH, DRM, or a multi-platform client shell.

---

## 9. Fallback behaviour

### 9.1 There is no fallback

> **Status note (third revision):** no longer true. The fallback described in §9.3 is implemented — `shouldDirectPlayFallback` / `useDirectPlayFallback` — including the D16 arming conditions and the D-FB2 stall guard. This section describes the pre-fix behaviour.

The three `direct → remux` transitions are all **pre-emptive**, computed before any media request:

1. **Audio-track upgrade** — `resolveModeForAudioTrack` (`playback.ts:117-125`).
2. **Codec/container ineligibility** — `getAvailableModes` drops `direct`; `resolvePlaybackSettings:223-225` substitutes the default mode.
3. **Metadata pending or failed** — `provisionalMode` (`useMoviePlaybackData.ts:54`, `:121-125`) resolves conservatively so a cold deep link never hits `/stream` with a non-first audio track.

A fourth transition is **not** pre-emptive, and it is the one D16 describes: on a cold deep link carrying `?mode=direct&audio_track=0`, the stream request goes out before eligibility is evaluated, and the switch to `remux` happens *after* the browser has already been handed bytes it may not be able to decode. That is a mode change driven by metadata arriving late, not by an error — but it is indistinguishable from an error-driven fallback if you only look at the `MediaError`.

**Nothing is error-driven.** A direct-play failure reaches `onNativeError` → `nativeMoviePlaybackErrorMessage(code)` → `setPlaybackError` → the error screen. The "Try Again" handler (`play.tsx:649-654`) clears `playbackError`, `playing`, `currentTime` and `duration` and **re-attempts the identical mode**. For a codec-incompatibility failure that is an infinite manual retry loop with a guaranteed identical outcome; the only escape is to navigate back and change the mode by hand.

### 9.2 Assessment against the brief

| Question | Answer |
|---|---|
| Which errors trigger fallback | None |
| Which media-element events are observed | `error` only (all four `MediaError` codes), plus `waiting`/`stalled`/`seeking`/`playing`/`canplay`/`seeked` for the spinner |
| Fallback too early / too late | N/A |
| Network errors confused with codec errors | **For fallback purposes, yes** — all four codes take the identical path. Messages differ; behaviour does not. |
| Auth failures confused with incompatibility | A 401 mid-stream surfaces as `MEDIA_ERR_NETWORK` ("A network error interrupted video playback"), which is misleading but at least not a codec claim |
| Position preserved | N/A — no fallback. `currentTime` is reset to 0 by "Try Again", losing the position the user had reached |
| Audio/subtitle preferences preserved | N/A. They live in the URL and would survive a navigation |
| Failed direct request cancelled | ✅ — the source effect cleanup aborts it |
| Fallback can run more than once | N/A |
| Fallback can loop back to direct | N/A (and the pre-emptive paths cannot: they only ever move *away* from direct) |
| Duplicate playback sessions | ✅ no — direct play creates no server session |
| Actionable error when both modes fail | ❌ — the error screen offers only "Try Again" and "Back". It does not say "this file cannot be played directly; try a different quality", and it does not link to the settings dialog |
| Fallback path deterministic | N/A. The *pre-emptive* paths are fully deterministic and well tested |

### 9.3 What a correct fallback looks like

Because `MEDIA_ERR_SRC_NOT_SUPPORTED` and `MEDIA_ERR_DECODE` are the only two codes that unambiguously mean "this browser cannot play these bytes", the fallback can be both narrow and deterministic:

- Trigger **only** on those two codes, and **only** when `resolvedMode === "direct"`.
- **Require `techLoaded`, and require `direct` to be in `availableModes`, before the trigger arms.** Without this, D16's optimistic pre-eligibility request poisons the fallback: every cold load of a bookmarked direct link to an ineligible file would raise a `MediaError`, fire the fallback, and announce a mode switch the app was about to make on its own. The fallback must only fire for a file the app has *affirmatively decided* is direct-playable. Fix D16 first, or this condition is doing D16's job badly.
- **Add a stall guard alongside the error codes.** §5.6 shows the important container failure is *silent*: Jellyfin's MKV case stalls at 0 ms and never sets `MediaError`, so an error-only trigger would hang forever. A bounded "direct play produced no `loadedmetadata` (or no `timeupdate` past 0) within N seconds" condition should fall back on the same one-shot budget. This also covers a truncated or corrupt file that never progresses.
- Fire **at most once per stream window** (the existing `sessionWindowKey` is the natural guard, mirroring `useHlsSessionRecovery`'s attempt budget).
- Navigate to `mode=remux` at the current absolute position, preserving `audio_track` and `subtitle_track`.
- Never fall back on `MEDIA_ERR_NETWORK` or `MEDIA_ERR_ABORTED`; never fall back *to* `direct`; never fall back from an HLS mode into direct.
- Announce the switch (a `LiveAnnouncer` message and a visible note), because the user's chosen mode changed underneath them.

Note that **D12 and D16 must both be fixed first**: the auto-resume effect keys on `streamUrl`, which is constant for direct play, so a fallback navigation would currently fail to resume playback (D12); and without D16 the fallback cannot tell a real incompatibility from a request the app issued before it knew better.

---

## 10. Testing strategy

### 10.1 Existing coverage

**Good** — the mode-resolution logic is genuinely well tested:
- `web/src/test/playback/playback-default-settings.test.ts` — `getDefaultPlaybackSettings`, `resolvePlaybackSettings`, the direct↔remux interlocks, and the *"never offers direct play without remux"* invariant across five video codecs.
- `web/src/test/playback/movie-playback-data.test.tsx` — clamped stale `start`, bitmap subtitle rejection, preference-vs-explicit precedence, direct deep link with non-first audio → remux, provisional remux while metadata is pending, no raw-stream fallback after metadata failure, cold direct play staying ready for track 0.
- `web/src/test/playback/video-player.test.tsx` — subtitle `<track>` inject/swap/remove, buffering spinner, hls.js error routing.
- `web/e2e/movie-player.spec.ts` — the preferences-before-media gate, search-param canonicalisation, *"cold non-first audio … never requests the raw stream"*, chapter seek and menu a11y.

**Missing entirely** *(status note, third revision: this gap is closed — the §10.3 suites below were implemented alongside the fixes, except the subtitle-persistence spec that decides D11)*:
- **Server:** no test exercises `StreamMovie` at all. No test sends a `Range` header or asserts `206`, `Content-Range`, `Accept-Ranges`, `416`, `304` or `HEAD` anywhere in the codebase. No test asserts what `resolveMovieFile` computes for `mime_type` — movie fixtures hard-code it (`movies_scanner_test.go` pre-bakes `video/x-matroska`), so D1 is invisible to the suite. `ffprobe_metadata_test.go` never tests `StreamDisposition` or stream classification.
- **Client:** nothing tests `supportsNativeHLS`/`canPlayType` (it is a module-load IIFE and hard to stub). Nothing asserts the direct-play `video.src = src` assignment or its `removeAttribute("src") + load()` cleanup — which is why D10 and D11 are unobserved. **Nothing covers a cold deep link with `mode=direct` and `audio_track=0`** — the existing cold-deep-link tests all use a non-first audio track, which is the case `provisionalMode` already handles, so D16 is invisible to the suite in exactly the way D1 is invisible to the Go suite. No test exercises the native `onError` → `nativeMoviePlaybackErrorMessage` path end to end.
- **E2E:** every direct-play request in `movie-player.spec.ts` is `page.route`d and never fulfilled, so **no automated test has ever decoded a single frame of real media.**

### 10.2 File test matrix

Legend — **Elig.**: expected direct-play eligibility *after* the recommended fixes. **Auto**: automatable without real media (unit/integration on metadata + HTTP), vs. **Manual**: requires a real file in a real browser.

| # | File | Elig. | Expected video | Expected audio | Subtitles | Browser behaviour | Fallback | Browsers | Auto/Manual |
|---|---|---|---|---|---|---|---|---|---|
| 1 | MP4 H.264 High + AAC stereo | ✅ direct | stream 0 | stream 0 | none unless selected | plays | none | Ch, FF | Auto (elig.) + Manual (playback) |
| 2 | MP4 H.264 + 3× AAC (eng/fra/commentary) | ✅ direct **only if** an unambiguous default exists | stream 0 | the default-disposition track | — | plays default | none; selecting another track → remux | Ch, FF | Auto + Manual |
| 3 | MP4 H.264 + AAC + embedded `mov_text` subs | ✅ direct | stream 0 | stream 0 | sideloaded WebVTT only; embedded never exposed | plays; subs via `<track>` | none | Ch, FF | Auto + Manual |
| 4 | MP4 H.264 + AC-3 | ❌ → remux | — | — | — | never attempted | n/a (pre-emptive) | Ch, FF | **Auto** |
| 5 | MP4 HEVC + AAC | ❌ → transcode/remux | — | — | — | never attempted | n/a | Ch, FF | **Auto** |
| 6 | WebM VP9 + Opus | ❌ today (codec gate) | — | — | — | never attempted | n/a | Ch, FF | **Auto** — also asserts `mime_type == "video/webm"`, catching D1b |
| 7 | **MKV H.264 + AAC** | ❌ **must stay refused** (§5.6) | — | — | — | never attempted; if ever served, **silent 0 ms stall with no `MediaError`** | n/a pre-emptively; stall guard if it ever leaks | Ch, FF | **Auto** — must assert `getAvailableModes` excludes `direct` *and* that no `/stream` request is made. Highest-value regression guard in the matrix. |
| 8 | MKV HEVC + TrueHD | ❌ → transcode | — | — | — | never attempted | n/a | Ch, FF | **Auto** |
| 9 | MKV HEVC + E-AC-3 | ❌ → transcode | — | — | — | never attempted | n/a | Ch, FF | **Auto** |
| 10 | MKV H.264 + DTS | ❌ → remux | — | — | — | never attempted | n/a | Ch, FF | **Auto** |
| 11 | Forced subtitle track present | n/a | — | — | forced track auto-enabled *after D7 fix* | — | — | Ch, FF | **Auto** (needs disposition columns) |
| 12 | ASS/SSA subtitles | ✅ if A/V allow | — | — | converted to WebVTT; styling lost | subs render unstyled | none | Ch, FF | Auto (conversion) + Manual (legibility) |
| 13 | PGS subtitles | ✅ if A/V allow | — | — | track disabled in picker, labelled "image-based" | no subs offered | none | Ch, FF | **Auto** |
| 14 | 3× English audio tracks | ✅ if default unambiguous | stream 0 | default track | — | plays default | — | Ch, FF | **Auto** — assert picker labels are distinguishable |
| 15 | No language metadata on any track | ✅ if A/V allow | stream 0 | stream 0 | — | plays | — | Ch, FF | **Auto** — assert "Track N" fallback labels |
| 16 | Multiple audio streams, no default dispositions | ✅ **stays eligible**, evaluated against stream 0 (§6.2) | stream 0 | stream 0 | — | plays stream 0 | — | Ch, FF | **Auto** — guards against over-correcting D8 into refusing everything |
| 16b | Multiple audio streams, single default on a non-zero index | ❌ **refuse direct** *after fix* | — | — | — | never attempted | n/a | Ch, FF | **Auto** |
| 16c | Multiple audio streams, multiple defaults | ❌ **refuse direct** *after fix* | — | — | — | never attempted | n/a | Ch, FF | **Auto** |
| 17 | Incorrect default disposition | ✅ direct | stream 0 | whatever the container says | — | plays the container's choice | — | Ch, FF | Manual |
| 18 | Commentary as first audio stream, main track flagged `default` | ❌ **refuse direct** *after fix*; ✅ today (bug) | — | — | — | today: user hears commentary | n/a | Ch, FF | **Auto** — regression test for D8; same shape as row 16b |
| 18b | Movie with zero `video_streams` rows | ❌ **refuse direct** *after fix*; ✅ today (bug) | — | — | — | — | n/a | Ch, FF | **Auto** — regression test for D17 |
| 18c | **Cold deep link `?mode=direct&audio_track=0` to an ineligible file** | ❌ must not request `/stream` | — | — | — | today: `/stream` is requested before eligibility is known | must not announce a fallback | Ch, FF | **Auto (E2E)** — regression test for D16; the existing cold-link specs all use `audio_track ≠ 0` and miss this |
| 19 | 2 video streams (multi-angle) | ⚠️ define behaviour | first non-cover-art | — | — | browser picks | — | Ch, FF | Auto (elig.) + Manual |
| 20 | Attached cover art + real video | ✅ if A/V allow | the real stream | stream 0 | — | plays the real video | — | Ch, FF | **Auto** — already partly covered |
| 21 | External `.srt` sidecar | n/a | — | — | **not indexed at all** | no subs offered | — | — | **Auto** — documents the gap |
| 22 | 10-bit H.264 (High 10) MP4 | ❌ **refuse direct** *after fix*; ✅ today (bug) | — | — | — | today: decode error, dead end | must fall back to remux/transcode | **Ch, FF** | Auto (elig.) + **Manual (decisive for D2)** |
| 23 | HDR10 (H.264, `smpte2084`) | ❌ **refuse direct** *after fix* | — | — | — | today: wrong colours | → transcode (tone-mapped) | Ch, FF | Auto (elig.) + Manual (colour) |
| 24 | Dolby Vision | ❌ | — | — | — | never attempted | n/a | Ch, FF | **Auto** — note: not represented in the schema; only `color_transfer` is stored |
| 25 | Variable frame rate | ✅ if A/V allow | stream 0 | stream 0 | — | plays; A/V sync is the browser's problem | — | Ch, FF | Manual |
| 26 | Corrupt / truncated file | ✅ (metadata may be stale) | — | — | — | `MEDIA_ERR_DECODE` mid-playback | must **not** loop; one remux attempt then a clear error | Ch, FF | **Auto** (HTTP + error mapping) + Manual |
| 27 | Large 4K file (> 40 GB) | ✅ if A/V allow | stream 0 | stream 0 | — | plays; repeated seeks issue many ranges | — | Ch, FF | **Auto** (range/concurrency) + Manual (seek feel) |

### 10.3 Recommended tests

**Unit — capability decisions** (`web/src/test/playback/`)
- Table-driven `getAvailableModes` cases for every row of the matrix above, including MKV, 10-bit, 4:2:2, HDR, and the multi-audio shapes. This is the highest-value new coverage and needs no media.
- Round-trip test asserting the container→MIME map matches `helpers.ValidVideoExtensions` exactly, so adding an extension without a MIME entry fails.
- Tests for a `canPlayType`-based probe with an injectable `HTMLVideoElement` (avoid the module-load IIFE pattern — make the probe a function so it is testable).

**Go — ffprobe parser** (`server/cmd/internal/ffprobe/`)
- Decode a fixture with `default`, `forced`, `comment`, `hearing_impaired` dispositions and assert each survives.
- Decode a fixture with uppercase `TITLE`/`LANGUAGE` stream tags and assert they are normalized (currently they are dropped).
- Classification: attached-pic video, cover-art codec, `data` and `attachment` stream types.

**Go — `mime_type` derivation** (`server/cmd/api/movies_scanner_test.go`)
- Assert `resolveMovieFile` produces the expected MIME for each of the six valid extensions, **without** hard-coding it in the fixture. This test fails today and is the direct regression guard for D1.

**Go — HTTP range** (new `server/cmd/api/movie_stream_handler_test.go`)
Using `httptest` and a small temp file:
- Plain `GET` → 200, `Accept-Ranges: bytes`, correct `Content-Length`, correct `Content-Type` from the container map.
- `Range: bytes=0-99` → 206 + `Content-Range: bytes 0-99/N`, exact bytes.
- Open-ended `bytes=100-`, suffix `bytes=-100`, multi-range → `multipart/byteranges`.
- Out-of-range → 416 + `Content-Range: bytes */N`.
- `If-Modified-Since` → 304; `If-Range` behaviour once an ETag exists.
- `HEAD` → 200 with headers and no body (fails today — guards D4).
- 401 for an unauthenticated request; 404 for a missing row; 404 for a row whose file is gone.

**API integration**
- `/technical-details` shape and ordering, including that dispositions are populated once D7 is fixed.

**Browser / Playwright** (`web/e2e/`)
- Extend `movie-player.spec.ts`: assert the direct-play request carries a `Range` header and that a `206` reply reaches the element; assert `<video>` gets an `src` at all (no current test does).
- New fallback spec: stub `/stream` to return a body the browser cannot decode, assert exactly **one** navigation to `mode=remux`, at the preserved position, with `audio_track`/`subtitle_track` intact, and assert it never bounces back to `direct`.
- New cold-deep-link spec: load `?mode=direct&audio_track=0` for a movie whose technical details make it ineligible, with a cold query cache, and assert **no** request to `/api/movies/{id}/stream` is ever made and no fallback is announced (guards D16). The mocked-e2e stack must serve technical details with a delay for this to be meaningful — an instantly-resolved stub hides the race.
- New subtitle-persistence spec: change `start` mid-playback and assert the `<track>` is still `showing`. This **settles** D11 rather than guarding a known defect — write it before writing the fix.
- A real-media suite behind `E2E_BASE_URL`, mirroring `hls-transcode.spec.ts`'s opt-in pattern, with env-provided movie IDs for at least: MKV H.264+AAC, 10-bit MP4, multi-audio MP4. **This is the only way rows 7, 22 and 26 get real coverage.**

**Accessibility**
- Extend the existing player tests: assert the mode badge is announced (guards D13), and that the fullscreen surface exposes no invalid role nesting (guards D14).

**Large-file / seeking / concurrency**
- Go test: 10 concurrent range requests against one file, asserting all succeed with correct bytes and that the FD count returns to baseline.
- Go benchmark or timed test: sequential seek pattern (100 random ranges) to catch a regression if `ServeContent` is ever replaced by a hand-rolled handler.

---

## 11. Findings register

Severity reflects user impact on the direct-playback feature. ✅ marks findings fixed by the third- and fourth-revision implementations (§14); the Evidence and Smallest-correct-fix columns describe the code as audited.

| ID | Sev | Finding | Evidence | Smallest correct fix |
|---|---|---|---|---|
| **D1** ✅ | **High** | `mime_type` is a host-dependent extension guess: `.webm` is served as `audio/webm`, missing `/etc/mime.types` yields `application/octet-stream`, and the correct exclusion of MKV rests on an accident rather than a rule | `movies_scanner.go:274`; `playback.ts:24`; `movie_handler.go:730-739`; measured table §3.2 | Add `helpers.VideoMimeTypes` keyed on `container` (mirroring `AudioMimeTypes`, `files.go:21`); use it in the scanner and as the `Content-Type` source; delete the dead `mime.TypeByExtension` fallback in `StreamMovie`; force a re-scan or backfill. **Do not add `video/x-matroska` to `BROWSER_COMPATIBLE_MIME_TYPES`** (§5.6). State the MP4-only rule explicitly with a comment citing §5.6 |
| **D2** ✅ | High | Direct-play codec gate ignores profile, bit depth and pixel format — looser than the server's own remux gate | `playback.ts:61-63`, `:94-108`; `docs/ffmpeg.md` §Remux | Reuse the server's 10-bit / 4:2:2 / 4:4:4 exclusion rules on the direct path, from the already-stored `codec_profile`, `bit_depth`, `pixel_format` |
| **D7** ✅ | High | ffprobe dispositions are discarded; `is_forced`/`is_default` are `false` for every row; audio has no disposition columns | `ffprobe_metadata.go:51-53`; `movies_scanner.go:1044-1045`; `schema.sql:320-341` | Add the disposition fields to `StreamDisposition`, add columns to `audio_streams`/`subtitles`, populate them in the inserts (schema edits are free pre-production). Also give `Stream.Tags` the case-normalising unmarshaller `FormatTags` already has |
| **D8** ✅ | High | Direct-play audio eligibility assumes `audioStreams[0]` is what the browser plays | `useMoviePlaybackData.ts:104`; `playback.ts:40`, `:80-85` | After D7: choose the candidate by default disposition, using the refuse-on-ambiguity table in §6.2. **Do not** refuse merely because no stream is flagged `default` — that over-corrects into refusing nearly everything |
| **D16** ✅ | **High** | A cold deep link carrying `?mode=direct&audio_track=0` requests `/stream` before eligibility is evaluated: the route loader short-circuits when `mode` is present, and `deriveMoviePlaybackStatus` exempts direct from waiting on `techPending` | `play.tsx:86`; `movie-playback.ts:134`; §3.5 | Either make direct wait for `techLoaded` like every other mode, or keep the optimistic mount and suppress the error screen and the D-FB fallback until `techLoaded`. Must land before D-FB |
| **D-FB** ✅ | High | No fallback from direct play — "Try Again" retries the identical mode | `play.tsx:649-654`; `VideoPlayer.tsx:359-388` | One-shot, per-stream-window navigation to `remux` on `MEDIA_ERR_SRC_NOT_SUPPORTED` / `MEDIA_ERR_DECODE` only, gated on `techLoaded` and `direct ∈ availableModes`, preserving position and track selection; announce the switch. Fix D12 and D16 first |
| **D3** ✅ | **High** | No `canPlayType()` on the direct path; one static allowlist stands in for every browser. §5.6 shows why a hardcoded container list is unsafe — it was wrong here and it was wrong in Jellyfin | `playback.ts:30-37` (the only probe, HLS-only) | Add a **narrowing-only** second gate: build an RFC 6381 codec string from `codec_profile`/`codec_level` and require a non-empty `canPlayType`. Must be `staticRules && probe`, never a disjunction — the watch-room server enforces the same invariant and cannot run a probe (§3.4). Make it an injectable function, not a module-load IIFE, or it is untestable |
| **D-FB2** ✅ | Medium | A silent container failure produces no `MediaError`, so an error-code-only fallback cannot fire | §5.6; jellyfin-web#7651 | Add a bounded stall guard (no `loadedmetadata` / no progress past 0 within N s) to the D-FB fallback |
| **D4** ✅ | Medium | `HEAD /api/movies/{id}/stream` returns 405 | `routes.go:172`; no `GetHead` in `routes.go:10-26` | Add `middleware.GetHead` (or an explicit `r.Head`), and document `head` in `docs/openapi.json` |
| **D9** ✅ | Medium | "Original file — plays as-is" describes the container, not the audio the user hears | `constants.ts:159`; `MoviePlayerControls.tsx:147` | After D7/D8, name the selected audio in the badge or its tooltip |
| **D10** ✅ | Medium | Direct-play source is torn down and reloaded whenever `startSec` changes | `VideoPlayer.tsx:253` | Split the effect, or drop `startSec` from the dep array on the direct branch |
| **D11** ✅ | Medium, **unverified** | Subtitles *may* silently stop showing after that reload. The HTML load algorithm forgets only in-band text tracks, so a `<track>`-derived one probably survives — this audit did not test it | `VideoPlayer.tsx:312` | **Write the §10.3 subtitle-persistence spec first and let it decide.** If confirmed: re-apply `track.track.mode = "showing"` after resource selection restarts (or fix D10, which removes the trigger). If not: keep the test, drop the finding |
| **D5** ✅ | Low | No `ETag`; `If-Range` validation is date-granular | `movie_handler.go:689-742` | Set a strong ETag from size + mtime before `ServeContent` |
| **D6** | Low | scs's writer defeats sendfile; session re-committed per range response | scs `sessionResponseWriter` lacks `io.ReaderFrom` | Scope the session middleware off media routes, or pass `ReadFrom` through |
| **D12** ✅ | Low | `streamReloadKey` and the auto-resume effect are inert for direct play | `movie-playback.ts:176`; `play.tsx:492-516` | Key auto-resume on `sessionWindowKey` rather than `streamUrl` |
| **D13** ✅ | Low | `aria-label` on a plain `<span>` is not announced | `MoviePlayerControls.tsx:143-148` | Use visually-hidden text, or move the label onto a focusable/labelled element |
| **D14** ✅ | Low | `div role="button"` wraps the `<video>` in fullscreen | `play.tsx:738-749` | Drop the role and `tabIndex`; keep the click handler (the toggle is already reachable via the footer button and Space/K) |
| **D15** ✅ | Trivial | `sr-only` text renders "rewind10 seconds" | `play.tsx:700` | Add `{" "}` |
| **D17** ✅ | Low | `getAvailableModes` offers every mode, direct included, when `videoCodec` is `undefined` — reachable with `techLoaded` true for a movie with zero `video_streams` rows | `playback.ts:92`, `:96`; `useMoviePlaybackData.ts:100` | Treat "metadata loaded but no video stream" as ineligible for direct rather than as "no codec info yet" |
| **D-TEST** ✅ | High | No test covers `StreamMovie`, any `Range` header, `mime_type` derivation, the direct-play `src` assignment, or a cold `mode=direct&audio_track=0` deep link | §10.1 | The suites in §10.3 — landed with the fixes, except the D11 subtitle-persistence spec (deferred with D11) |
| **D-EXT** | Info | External subtitle sidecar files are not indexed | scanner reads embedded streams only | Out of scope; noted as a product gap |
| **D-WR** ✅ | Info | `StreamWatchRoomMovie` returns 500 for a missing movie; negative `audio_track` unvalidated at room creation | `watch_room_media_handler.go:84-89`; `watch_room_handler.go:359-364` | Adjacent, out of scope |

### 11.1 Recommended sequence

> **Status note (fourth revision):** items 1–6 landed with the third revision; items 7–9 landed with the fourth (mapping in §14). The whole sequence is complete except D6, which remains deferred.

1. **D1** — pin the container→MIME map. Cheapest of the five, purely server-side, independently testable without a browser, fixes the `Content-Type` on the wire (which matters for every consumer that is not a `<video>` element), kills the duplicate derivation inside `StreamMovie`, and states the MP4-only rule that §3.2 shows is currently accidental. Lock it with the D-TEST regression guard for matrix row 7 in the same change.
2. **D3** — add the `canPlayType` gate, **narrowing-only**. It is the only change that makes the container/codec decision self-correcting, and §5.6 is the proof that a hand-maintained list will drift. Land it after D1 so the static rule it narrows is the pinned one, not the accidental one.
3. **D7** → **D8** — the metadata foundation; without dispositions, direct play cannot be made deterministic. Apply the §6.2 ambiguity table, not a blanket disposition requirement.
4. **D2** — exclude 10-bit / 4:2:2 / 4:4:4, reusing the server's existing remux rules. (Largely subsumed by D3 if the codec string is built correctly; keep it as an explicit rule for defence in depth.)
5. **D16** — stop the pre-eligibility `/stream` request. Must precede D-FB, whose trigger depends on it.
6. **D-FB** + **D-FB2** (after **D12** and **D16**) — make residual failures recoverable, including silent ones.
7. **D4**, **D10** — small correctness fixes. **Check D11** with its test before deciding whether it needs a fix at all.
8. **D-TEST** — written alongside each of the above, not after.
9. **D13**–**D15**, **D17**, **D5**, **D6** — cleanup.

Items 1–5 are the ones that change the answer to "is direct playback correct?" from *no* to *yes*.

Two reorderings are worth recording. The first revision moved **D3** to the front once §5.6 established that the failure being guarded against is a hardcoded container list going stale. The second moves **D1** back ahead of it — not because D3 matters less, but because D1 is a server-side change with a cheap deterministic test and no browser dependency, and D3's gate is more defensible when the static rule underneath it is one someone chose rather than one `/etc/mime.types` chose. **D16** is new to this list and sits immediately before the fallback work it constrains.

---

## 12. Answers to the audit questions

1. **Is direct playback implemented correctly?** Partly. The transport is correct (`http.ServeContent` gives ranges, seeking and large-file streaming for free) and the mode-resolution logic is well-structured and well-tested. The **eligibility decision is not correct**: it rests on a host-dependent extension guess (D1), ignores profile and bit depth (D2), never asks the browser (D3), assumes the first audio stream is the one that plays (D8), and on one reachable path does not run at all before the file is requested (D16). Its container verdicts happen to be right today, but by accident rather than by rule (§3.2, D1a).

2. **Does the app choose direct playback only when the browser can reliably play the media?** No, in two distinct ways. *When the decision runs*, its container exclusions are correct — MKV, WebM, AVI and MOV are all rightly refused (§5.2, §5.6) — but it **offers direct play for 10-bit, 4:2:2 and HDR H.264 MP4 files that Chrome and Firefox cannot decode** (D2, §5.3), and it cannot guarantee the audio stream the browser will select (D8). That reliability gap is inside MP4, not across containers. *Separately*, on a cold deep link carrying an explicit `mode=direct`, **the decision does not run before the file is requested at all** (D16) — so for that path the app is not choosing direct play on any basis, correct or otherwise.

3. **Is audio, video and subtitle track behaviour deterministic?** Video: yes. Subtitles: yes for the sideloaded WebVTT path, with one *possible* restoration bug that this audit did not verify (D11). **Audio: no.** With dispositions discarded (D7) and `audioTracks` unavailable in Chrome and Firefox (§5.5), the application can neither predict nor observe which audio stream is playing.

4. **Should direct playback remain in the web application?** **Yes — but this is a closer call than it first appears, and the counter-argument deserves to be stated.**

   *For keeping it:* it is the only path with no FFmpeg process, no transcode temp directory, no per-user session slot and no CPU permit. Serving a 60 GB file to three viewers costs three FDs and three 32 KiB buffers (§4.5). Where it applies, nothing else comes close.

   *Against:* §5.6 narrows its addressable set to `.mp4`/`.m4v` with H.264 and AAC/MP3 first-track audio — a minority of a typical MKV-dominated library. For that minority it carries real complexity: a whole eligibility system, the `direct → remux` coupling, a separate `<video>` code path in the player, an un-selectable audio track, and a failure mode with no fallback. A maintainer could reasonably decide that always remuxing is simpler and that the CPU saving does not justify the surface area.

   *Why keep it anyway:* remux is not free — it spends an FFmpeg process and one of three per-user session slots per stream, and `docs/ffmpeg.md` documents that even H.264 remux needs fMP4 preflight validation and can still fall back to a full transcode. Direct play is the only mode with no failure modes of its own once eligibility is correct. Keep it, but **narrow it deliberately**: refuse when the audio selection cannot be guaranteed (D8), when profile/bit depth make decoding unlikely (D2), and when `canPlayType` says no (D3) — and state the MP4-only rule in code rather than letting a MIME-string accident enforce it.

   If the fixes in §11.1 items 1–5 are not going to be done, removing direct play is the safer option than leaving it in its current state.

5. **Should the custom player be replaced by a package?** **No.** Every confirmed defect is a server-metadata, browser-capability or routing problem; no player library addresses any of them, because none can expose embedded audio tracks that Chrome and Firefox do not surface. The real gap is two missing menus in an otherwise accessible, well-tested player, and `web/AGENTS.md` explicitly prefers building those on Radix over adding a UI dependency. See §8.3.

6. **What is required before direct playback can be considered reliable?** Items 1–6 of §11.1: pin the container→MIME map, add a narrowing-only `canPlayType` gate, store and use ffprobe dispositions, exclude the profiles browsers cannot decode, stop requesting the file before eligibility is known, and add a single deterministic fallback with a stall guard — each with the tests in §10.3. The regression guard for matrix row 7 (MKV must never reach `/stream`) should land with the first of these, and row 18c (a cold `mode=direct` deep link must not reach `/stream` either) with D16.

---

## 13. Diagnostic changes

**None.** No files in the repository were modified during this audit. The one measurement taken (§3.2) used a throwaway Go program in a scratch directory outside the repository, calling `mime.TypeByExtension` for each extension in `helpers.ValidVideoExtensions`. It is reproducible in four lines and touched nothing in `Igloo/`.

No browser was driven by the audit, no server was started, and no media file was played by it. Except where §5.6 records a maintainer observation, the browser-behaviour claims in §5 and the "expected browser behaviour" column in §10.2 are documentation-derived predictions, not observations.

### 13.1 Verification pass (2026-07-27)

The second revision (§14) was a verification pass, not a re-audit. What it did:

- Resolved **every `file:line` reference in the §11 findings register** against the working tree. All resolved to the code described, with ≤ 2 lines of drift. Specifically re-confirmed: the three allowlists (`playback.ts:22-24`); `audioStreams[0]?.codec` (`useMoviePlaybackData.ts:104`); `startSec` in the source-effect dep array (`VideoPlayer.tsx:253`); `[subtitleUrl, videoRef]` (`:312`); `StreamDisposition` carrying only `AttachedPic`; `IsForced: false, IsDefault: false` hardcoded in `insertSubtitleStream`; `audio_streams` having no disposition columns; `r.Get("/{id}/stream")` with no `middleware.GetHead` in `InitRouter`; the absence of any compression or CORS middleware in `server/cmd`; `sessionResponseWriter` lacking `io.ReaderFrom` in `scs/v2@v2.9.0`; and the server-side `isNonBrowserH264Profile` / `isNonBrowserH264PixelFormat` gate that D2 says the direct path is looser than.
- **Re-ran the §3.2 measurement.** It reproduces exactly on this host, including `.webm → audio/webm`.
- Re-confirmed every "no test exists" claim in §10.1, including that `movies_scanner_test.go` hardcodes `video/x-matroska` at four call sites.
- Confirmed the provenance of the §5.6 MKV observation (§5.7) — the one thing in §5 that is evidence rather than documentation.

What it did **not** do: drive a browser, start a server, or play a media file. That boundary is unchanged, which is why D11 was demoted rather than resolved, and why §5 was reframed rather than corrected.

## 14. Revision history

**2026-07-27 (fifth revision) — review corrections.** Code review of the branch found two defects in the fixes themselves, both in rules the report had specified correctly but the implementation matched loosely.

*The D2 pixel-format rule refused files it should allow.* Both copies matched `pixel_format` with substrings (`"10"`, `"12"`, `"14"`, `"16"`, `"422"`, `"444"`), so the 8-bit 4:2:0 names `nv12` and `yuv410p` read as high bit depth. Direct play was silently refused for them on the web side, and the server's *remux* gate demoted them to a full transcode. Both are now an allowlist of the formats browsers actually decode for H.264 — `yuv420p`, `yuvj420p`, `nv12`, `nv21` — which also makes an unrecognised format fall back rather than be assumed safe. The `codec_profile` marker list is unchanged: profile strings like `High 10` and `High 4:2:2` genuinely need substring matching.

*The watch-room mirror could disagree with the client it mirrors.* Two parity gaps: the client gates on the stored `movie.mime_type` while the handler re-derived the MIME type from `movie.Container`, and the handler judged `videoStreams[0]` while the client resolves the feature through `getPrimaryVideoStream`, so a file whose attached picture sorts first reached opposite verdicts. One `movieContentType` helper now answers for the direct-stream `Content-Type`, the watch-room stream `Content-Type`, the `mime_type` field of technical-details, and the D1 container check — the client's input and the server's validation are the same string by construction. `primaryVideoStream` mirrors the client's cover-art skip and is used by both watch-room creation and `createHLSSession`.

Also fixed from the same review: watch-room creation is now disabled until technical details resolve (it could otherwise post an optimistic `direct` for the server to reject), the player badge's visually-hidden label says "Current playback mode" rather than "Current stream quality" now that it names the audio track, and a `handleAudioTrackChange` guard made unreachable by the `direct ⊂ remux` invariant was deleted. The room-presets section states why creation is blocked — the shared "Loading playback options…" line while the metadata is in flight, the API's own message if it fails — so the disabled button is never unexplained, the way `PlaybackSettingsDialog` already behaved.

**2026-07-27 (fourth revision) — implementation of the remaining register.** D4, D5, D9, D10, D11, D13–D15, D17 and D-WR were implemented on `fix/playback-settings`, one commit per item:

| Item | Findings | Commit |
|---|---|---|
| 7a | D4 | `0a56602a` — explicit `r.Head` on the movie, watch-room, and music track stream routes; `head` operations in `openapi.json`; HEAD httptest cases |
| 7b | D5 | `96778af0` — `strongFileETag` (size + nanosecond mtime) set before `ServeContent` in all three stream handlers; `If-None-Match`/`If-Range` tests |
| 7c | D-WR | `455025a6` — `sql.ErrNoRows` → 404 in `StreamWatchRoomMovie`; its leftover `mime.TypeByExtension` Content-Type fallback replaced with the pinned `helpers.VideoMimeTypes` map (D1 residue) |
| 9a | D17 | `fc163adf` — required `videoStreamsLoaded` flag on `AvailableModesArgs`; loaded-but-no-video refuses direct **and remux**, yielding the empty-modes → `modeUnavailable` screen; matrix row 18b tests |
| 9b | D17 mirror | `66730961` — watch-room creation refuses a direct room for a zero-video-stream movie (the server gate previously skipped its safety check in that case) |
| 8a | D9 | `486520e5` — `directPlayModeLabel` names the ordinal-0 audio language ("Original file — English audio"), language-only per the user's choice; generic label kept as the fallback and in the settings dialog |
| 7d | D10 | `f8a94475` — source effect split in two: the hls.js effect keeps `startSec` (rewind-buffer rebuilds are how HLS seeks on an unchanged URL), the native effect drops it; regression tests written first |
| 7e | D11 | `ab28a621` — deciding browser spec added to the opt-in real-media suite (`E2E_DIRECT_SUBTITLE_MOVIE_ID`); D10's fix removed the only trigger, so D11 closes either way |
| 9c | D13–D15 | `1988ca1c` — badge announced via visually-hidden text; fullscreen surface `role="button"`/`tabIndex`/keydown removed; "rewind10 seconds" whitespace fixed |

Design deviations from the report, recorded: **D4** used explicit `r.Head` registrations rather than `middleware.GetHead` — the OpenAPI coverage test is bidirectional and `GetHead` registers nothing with chi, so documented `head` operations would have failed its stale check. **D17** refuses remux as well as direct when metadata is loaded with no video stream (nothing can remux a movie with no video), which routes into the existing `modeUnavailable` screen; the same rule was mirrored into watch-room creation, where the server gate silently allowed the zero-stream case. **D11**'s verdict is that neither jsdom (stubbed `load()`, polyfilled `track`) nor the mocked e2e stack (no decodable media — the D-FB fallback navigates away mid-spec) can decide it; the jsdom regression test guards the D10 trigger and the browser truth lives in the opt-in real-media suite. The §10.3 idea of asserting "exactly one `/stream` request" was dropped from the browser spec — a real element issues many range requests for one source, so the spec asserts the seek landing and the subtitle staying `showing` instead.

Still open after this revision: **D6** (deferred: efficiency-only, and the fix rescopes the session middleware for every stream route) and **D-EXT** (a product feature, not a defect).

**2026-07-27 (third revision) — implementation.** §11.1 items 1–6 were implemented on `fix/playback-settings`, one commit per item:

| Item | Findings | Commit |
|---|---|---|
| 1 | D1 | `c63a6aa3` — pinned `helpers.VideoMimeTypes`; scanner + `StreamMovie` Content-Type from the container; dead `mime.TypeByExtension` fallback deleted; web MIME allowlist narrowed to `video/mp4`; scanner/parity/httptest-range suites |
| 2 | D3 | `99644d27` — `lib/direct-play-probe.ts` (RFC 6381 builder + injectable `canPlayType` probe); `getAvailableModes` options-object refactor; narrowing-only `staticRules && probe` gate |
| 3 | D7 | `32f22f47` — full `StreamDisposition` decode; `audio_streams.is_default` column; subtitle `is_forced`/`is_default` populated; `StreamTags` key normalization |
| 4 | D8 | `b2bbcf4a` — §6.2 refuse-on-ambiguity table (`directPlayAudioSelectionEligible`), mirrored server-side for watch-room creation |
| 5 | D2 | `d01d179b` — web `isBrowserSafeH264` mirroring the server remux gate; `pixel_format` added to the web types; watch-room container + H.264-safety mirror |
| 6 | D16 | `ecb5e91d` — direct waits for `techLoaded`; loader warms caches without blocking navigation; locked-in tests inverted; matrix row 18c mocked E2E |
| 6b | D12, D-FB, D-FB2 | `352b2651` — auto-resume re-keyed on the stream window; `shouldDirectPlayFallback` + `useDirectPlayFallback` with the 10s stall guard; one-shot announced switch to remux |
| D-TEST | real-media suite | `31a2cef3` — opt-in `direct-play-media.spec.ts` (matrix rows 7, 22, multi-audio, happy path) behind `E2E_BASE_URL` + `E2E_DIRECT_*` movie IDs |

Design deviations from the report, recorded: the stall guard disarms on `loadedmetadata` only, not on "no `timeupdate` past 0" — a player left paused would otherwise trip a false fallback; failures after metadata surface as `MEDIA_ERR_DECODE` and take the error path. D16 landed as the simpler wait-for-`techLoaded` variant with a non-blocking loader prefetch as the latency mitigation (§3.5 preferred the optimistic mount; blocking the loader on the prefetch was tried and rejected — it kept the player skeleton from rendering). The fallback preserves the *requested* `start` when the failure precedes `loadedmetadata`, since `currentTime` is still 0 then. `docs/ffmpeg.md` gained a "Direct Play Eligibility and Fallback" section documenting the new rules.

Still open after this revision: D4, D5, D6, D9, D10, D11 (unverified — its deciding spec was not written), D13–D15, D17, D-EXT, D-WR.

**2026-07-27 (second revision) — verification pass.** Every claim in the findings register was re-checked against the working tree (§13.1). The report's substance held: all `file:line` references resolved, the one measurement reproduced exactly, and §4, §7 and §8 needed no change. Four things did change.

*One finding was added.* **D16 (High):** a cold deep link carrying `?mode=direct&audio_track=0` requests `/stream` before eligibility is evaluated, because the route loader short-circuits when `mode` is present (`play.tsx:86`) and `deriveMoviePlaybackStatus` exempts direct from waiting on `techPending` (`movie-playback.ts:134`). The first draft missed this by generalising from a test that covers only `audio_track ≠ 0` — the case `provisionalMode` already handles. It constrains the D-FB fallback design and now sits ahead of it in §11.1.

*One finding was demoted.* **D11** was stated in §6.5 as observed fact ("subtitle selection does not survive playback restoration"). It is a prediction, and the HTML media load algorithm argues against it: `load()` forgets only *media-resource-specific* text tracks, and a `<track>`-derived one is not in that set. Marked unverified; the §10.3 spec now settles it rather than guarding it.

*One recommendation was narrowed.* **D8's** fix — "refuse direct when there are multiple audio streams and no unambiguous default" — over-corrected. With direct play already limited to `.mp4`/`.m4v` (§3.2), requiring a `default` flag that many muxers never write could have reduced the mode to near-nothing, contradicting §12 answer 4's own case for keeping it. Replaced with the refuse-on-ambiguity table in §6.2; matrix rows 16 and 18 were split accordingly.

*One section changed status.* **§5** was reframed from a table of facts to a table of stated assumptions, with the Firefox MKV cell softened to version-dependent (`media.mkv.enabled` is default-on from Firefox 145, though its scope excludes H.264). The §5.6 conclusion is unchanged and is now better supported: the maintainer's observation was confirmed to have been made in the browser directly, outside Igloo, which is what makes it evidence about browsers rather than about this codebase's own MIME allowlist.

Also: **D3** gained the constraint that it must narrow eligibility and never widen it, because the watch-room server enforces the same direct⊂remux invariant and cannot run a browser probe. **D17 (low)** was added — `getAvailableModes` offers direct play when `videoCodec` is `undefined`, reachable for a movie with zero `video_streams` rows. **D1** moved ahead of D3 in §11.1 and gained the dead `mime.TypeByExtension` fallback inside `StreamMovie`. Three factual nits corrected: the §2.2 middleware chain omitted `preserveClientSocketIP`; §4.4 overstated which methods `sessionResponseWriter` implements; §3.1 said "four places" where §3.6 lists five.

**2026-07-27 (first revision) — MKV claim corrected.** The first version of this report stated that Chrome and Firefox play MKV, rated the resulting MKV exclusion a **Critical** defect, and recommended adding `video/x-matroska` to `BROWSER_COMPATIBLE_MIME_TYPES`. That recommendation would have broken playback. The maintainer reported from direct observation that neither browser plays MKV; corroboration and root cause are in §5.6.

Changes made: §5.2/§5.3 container and codec tables; new §5.6 and renumbered §5.7; §3.2 D1a rewritten and the "largest loss of value" framing removed; D1 downgraded Critical → High and its fix inverted; D3 upgraded Medium → High and moved to the front of §11.1; new finding D-FB2 (silent failures produce no `MediaError`); §9.3 gained a stall-guard requirement; matrix row 7 inverted from "should become eligible" to "must stay refused, highest-value regression guard"; §12 answers 1, 2, 4 and 6 revised, with the counter-argument for removing direct play now stated in §12, answer 4.

The error's cause is worth keeping on record: Chromium's `mime_util_internal.cc` MIME table was read as if it were the `<video src=>` support list. It is not. §5.1 draws that exact distinction and the analysis violated it anyway — which is now the report's central argument for replacing hardcoded container lists with a runtime `canPlayType` probe (D3).

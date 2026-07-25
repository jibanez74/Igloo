# Playback Review — Follow-up Issues (2026-07-25)

Issues found while implementing and live-verifying the audio-language fix on
`fix/playback-settings`, deliberately **not** fixed there to keep that change
focused. That change fixed: direct play silently ignoring the selected audio
track (selecting any track other than the container's first now resolves the
mode from `direct` to `remux`, so the chosen language actually plays),
watch-room `audio_track` validation (out-of-range, non-zero on a movie without
audio, and `direct` + non-zero are now rejected at creation instead of failing
later), a stale `hls_profiles.go` comment claiming remux always transcodes
audio, and `docs/ffmpeg.md` §Audio Track Selection and Direct Play.

Findings 1 and 3 below are pre-existing and reproducible on `main`. Finding 2
is pre-existing, but the parent change made it self-contradicting — see the
note in that section.

Line references were verified against the working tree **after** the parent
change landed; they will drift as the files move.

---

## 1. `formatLanguageName` truncates ISO 639-2 codes, mislabelling real languages

Observed as a cosmetic defect — an audio track tagged `spa` renders as
`SPA · Stereo` instead of `Spanish · Stereo` — but it is a correctness bug that
can name the **wrong language**.

`web/src/lib/playback.ts:251-261` (module-private):

```ts
function formatLanguageName(raw: string | undefined): string | undefined {
  const code = raw?.trim().toLowerCase();
  if (!code) return undefined;
  const two = code.slice(0, 2);                    // <-- 3-letter code truncated
  if (LANGUAGE_NAMES[two]) return LANGUAGE_NAMES[two];
  return code.length <= 3
    ? code.toUpperCase()
    : code.charAt(0).toUpperCase() + code.slice(1);
}
```

It treats the first two characters of an ISO 639-2 code as an ISO 639-1 code.
It never calls `normalizeLang` (`playback.ts:132-138`), which sits 119 lines
above it **in the same file**, does the correct `ISO_639_2_TO_1` lookup, and is
already used by the sibling *selection* logic. Track selection is therefore
correct while the label describing it is not.

Two failure classes, both present in the dev library:

- **Wrong language named**, where the truncation collides with a real ISO 639-1
  key: `est` (Estonian) → "Spanish", `rum` (Romanian) → "Russian", `arc`
  (Aramaic) and `arm` (Armenian) → "Arabic", `fil` (Filipino) → "Finnish".
- **Uppercase-code fallback**: `spa` → "SPA", plus `chi`, `swe`, `dut`, `pol`,
  `tur`, `cze`, `gre`, `bul`, `lav`, `hrv`, `lit`, `ice`, `may`, `isl`, `srp`,
  `slo`, `msa`, `cat` and ~20 more singletons.

That French renders correctly is coincidence — `fra` and `fre` both truncate to
`fr`. The same accident covers `eng`, `deu`, `ita`, `rus`, `jpn`, `kor`, `nld`,
`por`, `ron`, `ukr`, `ell`, `vie`, `hun`, `hin`, `dan`, `fin`, `nor`, `ara`,
`zho`, `heb`, `tha`, `nob`.

**Every label in the app takes the broken path.** The scanner stores whatever
ffprobe reports, which is 3-letter codes; the dev database contains no 2-letter
language values at all, so the function's intended 2-letter branch is dead in
practice. `spa` is the second-most-common audio language (35 rows) and the
second-most-common subtitle language (349 rows) in the library.

Reproduce the census:

```bash
sqlite3 "file:db/igloo.db?mode=ro" \
  "SELECT DISTINCT language, COUNT(*) FROM audio_streams GROUP BY language ORDER BY 2 DESC;"
sqlite3 "file:db/igloo.db?mode=ro" \
  "SELECT DISTINCT language, COUNT(*) FROM subtitles GROUP BY language ORDER BY 2 DESC;"
```

At time of writing: 21 distinct audio codes, 68 distinct subtitle codes, all
3-letter (plus empty, `und`, `zxx` — those correctly fall through to `Track N`).

### Blast radius

`formatLanguageName` is private with two call sites, both in `playback.ts`:
`formatPlaybackAudioLabel:284` and `formatSubtitleLabel:332`. Transitively it
reaches:

| Surface | Path |
|---|---|
| Audio track options (the reported symptom) | `components/movies/PlaybackSettingsDialog.tsx` |
| Subtitle options | `components/movies/PlaybackSettingsDialog.tsx` |
| "You'll hear: …" summary prose | `describePlaybackExperience`, `playback.ts` |
| Create-watch-room audio + subtitle summary | `components/watch-room/CreateWatchRoomDialog.tsx` |
| **The browser's own subtitle menu** | `lib/movie-playback.ts:253` — the `label` on the `<track>` element |

`movie-playback.ts` is the sharpest illustration: line 253 builds the `label`
through the broken formatter while line 254 builds `srclang` with
`normalizeLang`. The same object gets a correct 2-letter code and a wrong
display name.

### Suggested fix

Route the lookup through the existing helper, keeping the uppercase fallback:

```ts
const two = normalizeLang(code);
if (two && LANGUAGE_NAMES[two]) return LANGUAGE_NAMES[two];
```

`normalizeLang` passes 2-letter input through unvalidated (`"zz"` → `"zz"`), so
the fallback must still survive a `LANGUAGE_NAMES` miss.

**Open decision — how far to take it:**

- **Option A (recommended): rewire and extend the maps.** 22 of the 68 subtitle
  codes in the library are absent from `ISO_639_2_TO_1` entirely — `ice`/`isl`,
  `per`/`fas`, `slo`/`slk`, `srp`, `bul`, `lav`, `hrv`, `est`, `lit`, `may`/`msa`,
  `cat`, `baq`, `geo`, `alb`, `mac`, `mkd`, `kir`, `khm`, `kaz`, `kan`, `glg`,
  `tel`, `tam`, `sqi`, `mal`, `aze`, `fil`, `nob`, `srp`, `ind`. Note the map
  already handles bibliographic/terminological pairs inconsistently (it has
  `ger`/`deu`, `dut`/`nld`, `chi`/`zho`, `cze`/`ces`, `fre`/`fra`, `gre`/`ell`,
  `rum`/`ron` but not `ice`/`isl`, `per`/`fas`, `slo`/`slk`, `wel`/`cym`).
  Extending both maps eliminates every wrong-language collision *and* every
  uppercase fallback.
- **Option B: rewire only.** Kills all five wrong-language collisions and fixes
  the ~10 most common codes including `spa`, `chi`, `swe`, `dut`, `pol`, `tur`,
  `cze`, `gre`. The ~35 rarer codes keep rendering as uppercase codes — which is
  at least honest, unlike today.

The maps live at `web/src/lib/constants.ts:316` (`ISO_639_2_TO_1`, 34 keys) and
`:354` (`LANGUAGE_NAMES`, 27 keys). Today every value of the first is a key of
the second; any extension must preserve that.

### Why it survived

**There is no test coverage at all** for `formatLanguageName`,
`formatPlaybackAudioLabel`, `formatSubtitleLabel`, `describePlaybackExperience`
or `normalizeLang` — verified by grep across `web/src/test/`. Several tests use
`spa` fixtures, but they assert only the *selected index* or the `<select>`'s
own accessible name, never the option text:

- `test/playback/playback-default-settings.test.ts` — `spa`/`fra`/`eng`
  fixtures, asserts selected index only (exercises `normalizeLang`, which is
  correct).
- `test/playback/playback-settings-dialog.test.tsx` — has a `spa` stream with
  `title: "Spanish Stereo"`, but that string is a fixture title and is never
  asserted.
- `test/playback/video-player.test.tsx` — asserts `<track>` labels, but they are
  passed in as literals, bypassing `formatSubtitleLabel`.

A fix should add label-level unit tests (including the collision cases) and a
contract test that every value of `ISO_639_2_TO_1` is a key of `LANGUAGE_NAMES`.
`web/src/test/lib/constants-contracts.test.ts` already exists and has no
language assertions.

### Related, not the same bug

`components/movies/MovieAboutSection.tsx:57` renders the movie's original
language as `{language.trim().toUpperCase()}` — a bare code with no name lookup
at all. `TechnicalDetailsDialog.tsx` does likewise for stream chips, which is
arguably intentional for a technical view.

---

## 2. Every HLS watch room on a video-only movie fails at creation

`GetOrCreateRoomHLSSession` takes `audioTrack int` — a plain int with no way to
express "this movie has no audio" — and always passes a non-nil pointer
(`server/cmd/api/hls_session.go:858-859`):

```go
audioTrackCopy := audioTrack
session, createErr := app.createHLSSession(ctx, &movie, profile, &audioTrackCopy, "", 0, true)
```

`createHLSSession` (`hls_session.go:955-957`) rejects exactly that:

```go
if len(audioStreams) == 0 {
    if audioTrack != nil {
        return nil, fmt.Errorf("audio_track is not valid for video-only movie %d", movieID)
    }
}
```

`WarmUpRoomHLSSession` (`hls_session.go:810`) is a thin wrapper over the same
function, and `CreateWatchRoom` rolls the room back and returns 500 when warm-up
fails (`watch_room_handler.go:454`). So an HLS watch room for a silent or
video-only file cannot be created at all, and any pre-existing room would fail
identically at `WatchRoomHLSManifest` (`watch_room_media_handler.go:27`).

**The parent change made this self-contradicting.** The new validation at
`watch_room_handler.go:358` deliberately permits `audio_track == 0` on a movie
with no audio:

```go
movieHasAudio := len(audioStreams) > 0
if !movieHasAudio && req.AudioTrack != 0 { /* 400 */ }
```

…and then warm-up 500s on that very case. The validation layer and the session
layer now disagree in writing. Whoever fixes this should make them agree rather
than tightening the validation, because video-only rooms are a legitimate case.

The personal playback path is the correct reference and already works:
`parseHLSParams` leaves `AudioTrack` as a nil `*int` when the query parameter is
absent, and the client omits it for movies with no audio streams
(`web/src/hooks/useMoviePlaybackData.ts:137` → `lib/movie-playback.ts:177-179`).

### Suggested fix

Normalize the pointer in `GetOrCreateRoomHLSSession`, leaving
`createHLSSession`'s strict nil/non-nil contract untouched. `loadHLSMovieForSession`
(`hls_session.go:896`) only reads the movie row, so the room path currently has
no stream knowledge:

```go
audioStreams, audioErr := app.Queries.GetAudioStreamsByMovieID(ctx, movieID)
if audioErr != nil {
    return nil, fmt.Errorf("failed to load audio streams: %w", audioErr)
}

var audioTrackPtr *int
if len(audioStreams) > 0 {
    audioTrackCopy := audioTrack
    audioTrackPtr = &audioTrackCopy
}
```

This costs one extra query on cold sessions only — the lookup sits inside the
singleflight body, so cache hits skip it. Better still, extract the
"resolve audio pointer from stream count" step into one helper shared with the
creation-time validator so the two cannot drift apart again.

### How a movie legitimately has zero audio streams

`movies_scanner.go:945` writes an audio row only inside `case "audio":`; there
is no synthetic default row, and only *video* is required
(`movies_scanner.go:643`: `"no video stream found - invalid movie file"`). Home
video, B-roll and muxed-out sources all land with zero audio rows.

### Test gap

No test covers a video-only watch room. A fix should add one to
`server/cmd/api/hls_room_test.go` asserting that an HLS room on a movie with no
`audio_streams` rows warms up successfully.

---

## 3. Remux can silently downgrade to a full transcode while the UI says otherwise

When the remux safety preflight rejects a source, Igloo restarts the session on
a transcode profile — but nothing tells the client, so the player keeps showing
the remux badge, "Original video, adjusted audio". A user on a 10-bit H.264
source watching a full 720p re-encode is told the picture is untouched.

The distinction exists internally and then dies. `HLSSession`
(`hls_session.go:46-63`) has **no profile fields at all**; `RequestedProfile` and
`EffectiveProfile` live only on the transient `hlsSessionStartParams`
(`hls_session.go:65-76`). `startHLSSession` logs both (`:608-612`) and copies only
`CopyVideo` onto the session (`:603`), so once `createHLSSession` returns, the
effective profile is unrecoverable. Nothing reaches the wire either: both
playlist handlers set only `Content-Type` and `Cache-Control`
(`hls_handler.go:87-88`, `watch_room_media_handler.go:39-40`), and there is no
session-state endpoint. Only the server log carries the truth
(`"remux safety fallback engaged"`, with `requested_profile` / `effective_profile`).

The client labels from its own request: `useMoviePlaybackData.ts:146` derives
`modeLabel` from `resolvedMode`, rendered as the quality badge at
`MoviePlayerControls.tsx:147`. Watch rooms have the same hole
(`WatchRoomPage.tsx` builds the URL from `room.playback_mode`).

`session.CopyVideo` is the one surviving signal — it is `false` exactly when the
fallback engaged (`hls_session.go:564`) — but it is never surfaced.

### What triggers the fallback

Static, `isBrowserSafeH264RemuxCandidate` (`hls_session.go:88`):

- codec is not `h264` / `h.264` / `avc` / `avc1`
- `BitDepth > 8` (10-bit)
- a non-browser H.264 pixel format
- a codec profile string containing `10`, `4:2:2`, `422`, `4:4:4` or `444`.
  Note this is a **substring** match, so it would false-positive on any future
  profile string that happens to contain "10".

Dynamic preflight:

- timeout after `hlsRemuxPrevalidateTimeout` (30s, `hls_session.go:21`)
- ffmpeg exiting before `init.mp4` plus `HLS_REMUX_PREVALIDATE_SEGMENTS` (4)
  complete segments exist
- `ValidateRemuxSafety` finding an unreadable init segment, an unparseable
  `avcC`, or any of the 4 segments with no IDR sync sample

The fallback target is `BestFitHLSFallbackProfile(primaryVideo.Height)` — the
highest transcode profile fitting the source height, else `720p_3mbps`.

Two behaviours worth knowing before touching this:

- **Cost.** On fallback the already-started session is torn down and ffmpeg
  restarts from scratch, so the user can wait up to 30s of dead air before the
  transcode even begins — with the UI still claiming remux.
- **Caching asymmetry.** Static-unsafe and validation-failure verdicts are
  cached for 24h, but preflight *wait* failures are deliberately not cached
  (`hls_session.go:1048`: "Preflight wait failures can be transient…"). A loaded
  server can therefore re-stall on the same movie indefinitely.

### Suggested fix

Smallest honest surface, two parts:

1. Add `EffectiveProfile string` to `HLSSession` and set it beside `CopyVideo`
   at `hls_session.go:603`. Purely additive — the value is already in scope.
2. Emit it as a response header (e.g. `X-Igloo-Effective-Profile`) from both
   playlist handlers, alongside the existing `Cache-Control`.

A header is the right vehicle because the playlist is fetched by hls.js, not by
TanStack Query. hls.js exposes the `XMLHttpRequest` as `data.networkDetails` on
`Hls.Events.MANIFEST_LOADED`, readable inside the existing `new Hls({...})` block
at `web/src/components/playback/VideoPlayer.tsx:178-201`, which already registers
`MANIFEST_PARSED` and `ERROR` handlers. A JSON session-state route would mean a
new endpoint, a new poll, and a race with manifest load for the same information.

**Caveat:** on Safari's native-HLS path (`supportsNativeHLS`, `playback.ts:26`)
the `src` is assigned straight to the `<video>` element, so there is no XHR and
the header cannot be read. The fix must degrade to the requested label there
rather than break.

**Contract:** `docs/openapi.json` is authoritative. One `headers` entry on the
shared `HLSPlaylistResponse` component (`openapi.json:5264`, which already has a
`Cache-Control` header block) covers both `/api/movies/{id}/hls/{profile}/playlist.m3u8`
and `/api/watch-rooms/{id}/hls/playlist.m3u8`, since both `$ref` it. Precedent
for documenting a non-content header: `Retry-After` at `openapi.json:5777`.
`make test-openapi` checks route coverage rather than headers, so it will not
object — but it must still be run.

---

## Before fixing any of these

Findings 2 and 3 change behaviour governed by `docs/ffmpeg.md`, which is
authoritative — update it in the same task and run the full playback pass per
`server/AGENTS.md`: focused server tests, `make check`, and the relevant
Playwright media specs, reporting any media combination or hardware path left
untested.

Finding 3 additionally changes the HTTP contract, so `docs/openapi.json` must be
updated in the same task, followed by `make test-openapi`.

Finding 1 is web-only, but it touches playback surfaces — see
`docs/design-system.md` §3.5 — so verify the settings dialog and the native
subtitle menu in a browser, not just in unit tests.

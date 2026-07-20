# Audio Player Review — Follow-up Issues (2026-07-19)

Issues observed during the audio-player UI/UX review on `fix/audio-player-ui`
that were deliberately **not** fixed there, to keep that change focused. The
review itself fixed: the mixed focus-ring recipes (now `FOCUS_VISIBLE_RING_CLASS`
via `PLAYER_ICON_BUTTON_CLASS` / `PLAYER_PRIMARY_BUTTON_CLASS`), `disabled` on
play/pause (now `aria-disabled` + guard), raw `bg-white` progress thumbs (now
`bg-foreground`), the duplicated play/toggle/seek logic, the per-page
current-track wiring (now `useTrackPlaybackMatcher`), and global shortcuts
being dead while the player's own sliders had focus.

Everything below is pre-existing and reproducible on `main`.

## 1. Movie player: duplicate "Playback progress" group + doubled time readouts

`MoviePlayerControls.tsx:76` wraps `ProgressBar` in its own
`role="group" aria-label="Playback progress"`, but `ProgressBar` already
renders an identically-labelled group internally
(`ProgressBar.tsx:101` default `groupLabel`). Screen-reader users hear two
nested "Playback progress" groups. The footer also renders its own
`0:01:24 / 2:04:34` readout while the `video` variant of `ProgressBar` shows
the same times below the bar — two visible time displays for one position.

Fix ideas: drop the wrapper `div`'s group role (keep it purely for layout),
and either pass `showTimes`-off styling for the video variant or remove the
footer's separate readout.

**Caution:** this is playback chrome — per `docs/ffmpeg.md` / design-system
§3.5 it needs the full playback test pass (direct stream, HLS, fullscreen
idle-hide, watch rooms use the same bar via `WatchRoomPage`).

## 2. Movie player announces its seek slider as "Seek through track"

`MoviePlayerControls` does not override `ProgressBar`'s default
`ariaLabel = "Seek through track"` (`ProgressBar.tsx:100`), so on a *movie*
the slider is announced with music wording. Pass
`ariaLabel="Seek through movie"` (and review the watch-room usage) when
touching item 1.

## 3. Playlist drag-and-drop still threads current-track props manually

The playlist page passes `currentTrackId` / `isPlaying` down through
`DraggableTrackList` → `SortableTrackItem` instead of using the new
`useTrackPlaybackMatcher` hook (`web/src/hooks/useTrackPlaybackMatcher.ts`)
that the other four lists now share. Left alone because adopting the hook
changes the DnD components' public props, their tests would need an
`AudioPlayerProvider` wrapper, and the drag overlay intentionally renders
`isCurrentTrack={false}` (`DraggableTrackList.tsx:187`) — that special case
must survive any refactor.

## 4. No e2e coverage of actual audio playback

No Playwright spec ever clicks through to a playing `<audio>` element — the
music specs assert that Play buttons exist and stop there.
`web/e2e/mock-api-server.ts` has no handler for
`GET /api/music/tracks/{id}/stream` (only the movie stream is stubbed), so
streaming audio is currently untestable in the mocked harness. A future spec
should stub the stream route (a stalled response is enough — the movie play
spec proves seek/transport assertions work at `readyState 0`) and drive:
play from an album → fullscreen dialog → minimize → mini-bar transport →
current-row highlight.

## 5. Dev database is never migrated (`make dev` startup failure)

The embedded startup schema only creates missing tables. After
`server/sqlc/schema.sql` gains a column, an existing `db/igloo.db` (repo
root) makes the server die at boot:
`failed to prepare database queries: … no such column: save_session_id`
(hit 2026-07-19; fixed by `ALTER TABLE movie_watch_progress ADD COLUMN …`
matching the schema's type/default — don't delete the db, it holds the
scanned library). Pre-production "no migrations" is policy, but a dev-only
mitigation (a `make db-reset` target, or a startup log line naming the
mismatched table) would save the next person the diagnosis.

## 6. Dev-only console warning: Inter preload unused

Every page in dev logs
`The resource /fonts/InterVariable.woff2 was preloaded using link preload
but not used within a few seconds…` (often repeated, and again after each
HMR update). The preload markup itself is correct — `index.html:29` has
`as="font" type="font/woff2" crossorigin`, which matches the anonymous-mode
fetch the `@font-face` in `boot.css:4` performs — so the usual
crossorigin-mismatch cause is ruled out. The likely culprit is dev-mode
serving: Vite injects `boot.css` via JS, so the `@font-face` registers (and
the font request fires) later than the browser's grace window. Verify
whether the warning appears on a production build (`make build`, served by
the Go binary); if prod is clean this is pure dev noise and just needs a
note wherever "console must be clean" audits are run, not a code change.

# Movie Watch Progress — Work Plan

## Overview

Track per-user playback position for movies so users can resume where they left off. When a user returns to a movie that has saved progress, a dialog asks whether to resume or start from the beginning. Users can also manually mark movies as watched/unwatched from the movie details page.

## Current state

- No watch progress infrastructure exists (no table, no endpoints, no frontend logic).
- The play page already tracks `currentTime` via the `timeupdate` event and supports a `start` search param for seek-offset playback.
- User auth / session ID extraction is well-established (`SessionManager.GetInt64`).
- The project has `AlertDialog` and `Dialog` UI components available from shadcn.
- The project uses the React compiler — no `useCallback`, `useMemo`, or `React.memo` needed.

## Design decisions

- **Save interval**: every 15 seconds during playback, and also on pause.
- **Minimum threshold**: only start saving progress after 180 seconds (3 minutes) of playback.
- **Completion threshold**: when playback reaches 98% or higher, the movie is considered watched — progress is cleared and the `watched` flag is set.
- **Per-user**: each user has independent watch progress and watched status.
- **Direct stream resume**: for direct mode, the resume dialog sets `video.currentTime` directly instead of using the `start` search param (which only applies to HLS).
- **"Continue Watching" section**: deferred to a future work plan.

## Implementation plan

### 1. Database — new table

Add a `movie_watch_progress` table to `server/sqlc/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS movie_watch_progress (
  user_id       INTEGER NOT NULL,
  movie_id      INTEGER NOT NULL,
  progress_sec  REAL    NOT NULL DEFAULT 0,
  duration_sec  REAL    NOT NULL DEFAULT 0,
  watched       INTEGER NOT NULL DEFAULT 0,
  updated_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, movie_id),
  FOREIGN KEY (user_id)  REFERENCES users (id)  ON DELETE CASCADE ON UPDATE CASCADE,
  FOREIGN KEY (movie_id) REFERENCES movies (id) ON DELETE CASCADE ON UPDATE CASCADE
);
```

- `progress_sec` and `duration_sec` are REAL (float64) to preserve sub-second precision from the video element's `currentTime` and `duration`.
- `watched` is an INTEGER used as a boolean (0 = not watched, 1 = watched). Set automatically when playback reaches 98%, or toggled manually by the user.

### 2. Database — sqlc queries

Add a new query file `server/sqlc/queries/movie_watch_progress.sql` with these operations:

- **UpsertMovieWatchProgress** — insert or update on conflict `(user_id, movie_id)`. Sets `progress_sec`, `duration_sec`, and `updated_at = CURRENT_TIMESTAMP`. Does NOT touch the `watched` column.
- **GetMovieWatchProgress** — select `progress_sec`, `duration_sec`, `watched`, `updated_at` for a given `(user_id, movie_id)`.
- **DeleteMovieWatchProgress** — delete the row for a given `(user_id, movie_id)`.
- **MarkMovieWatched** — upsert a row setting `watched = 1`, clearing `progress_sec = 0`, and updating `updated_at`. Used both when playback reaches 98% and when the user manually marks a movie as watched.
- **MarkMovieUnwatched** — update `watched = 0` for a given `(user_id, movie_id)`. Used when the user manually unmarks a movie. If no row exists, this is a no-op (the movie is already effectively unwatched).

Run `sqlc generate` after adding the queries.

### 3. Backend — handler endpoints

Add a new handler file `server/cmd/api/watch_progress_handler.go` with these handlers:

| Method   | Route                                    | Handler                    | Purpose                                         |
|----------|------------------------------------------|----------------------------|--------------------------------------------------|
| `GET`    | `/api/movies/{id}/watch-progress`        | `GetMovieWatchProgress`    | Return saved progress and watched status         |
| `PUT`    | `/api/movies/{id}/watch-progress`        | `UpdateMovieWatchProgress` | Upsert the current playback position             |
| `DELETE` | `/api/movies/{id}/watch-progress`        | `DeleteMovieWatchProgress` | Clear progress (user chose "start over")         |
| `PUT`    | `/api/movies/{id}/watch-progress/watched`| `ToggleMovieWatched`       | Toggle the watched flag for the current user     |

All handlers extract `userID` from the session and `movieID` from the URL parameter, following the same pattern as `ToggleLikeMovie` in `movie_handler.go`.

**`UpdateMovieWatchProgress` (PUT progress):**
- Accepts `{ progress_sec: number, duration_sec: number }`.
- Guards against `duration_sec <= 0` to prevent division by zero.
- If `duration_sec > 0` and `progress_sec / duration_sec >= 0.98`, calls `MarkMovieWatched` instead of the regular upsert (auto-marks as watched AND clears progress).
- Otherwise, calls `UpsertMovieWatchProgress`.

**`GetMovieWatchProgress` (GET):**
- Returns `{ progress_sec, duration_sec, watched, updated_at }` or a 404 if no record exists.

**`ToggleMovieWatched` (PUT watched):**
- Reads the current record via `GetMovieWatchProgress`.
- If the record exists and `watched = 1`, calls `MarkMovieUnwatched`.
- If the record does not exist or `watched = 0`, calls `MarkMovieWatched`.
- Returns the new `watched` state in the response so the frontend can update without a refetch.

Wire the routes in `InitRouter` under the existing `/api/movies` route group.

### 4. Frontend — API helpers

Add fetch helpers in `web/src/lib/api.ts` for the endpoints, following the existing `apiRequest` pattern:

- `getMovieWatchProgress(movieId)` — `GET`
- `updateMovieWatchProgress(movieId, progressSec, durationSec)` — `PUT`
- `deleteMovieWatchProgress(movieId)` — `DELETE`
- `toggleMovieWatched(movieId)` — `PUT`

### 5. Frontend — progress reporting from the play page

In `play.tsx`, add logic that periodically saves the current playback position to the server while the movie is playing:

- **Periodic save**: use a `setInterval` (15 seconds) that fires while the video is playing. On each tick, call `updateMovieWatchProgress` with the current `currentTime` and `duration`. Also save on pause.
- **Minimum threshold**: only start saving progress after `currentTime >= 180` seconds (3 minutes) to avoid creating entries for accidental clicks or brief previews.
- **Completion detection — periodic save**: when a periodic save fires with `currentTime / duration >= 0.98`, the server auto-marks the movie as watched and clears progress.
- **Completion detection — `ended` event**: listen for the `ended` event on the video element. When the movie finishes naturally, send a final progress update with the full duration to guarantee the server marks it as watched. This handles the case where the last periodic save fires before the 98% threshold and the movie ends before the next interval.
- **On page unload**: use `fetch` with `keepalive: true` in a `beforeunload` listener to save the final position when the user closes the tab or navigates away. Must include `credentials: "include"` for session auth. (`navigator.sendBeacon` cannot be used here because it only supports POST and our endpoint is PUT.)

### 6. Frontend — resume dialog on the play page

When `PlayMoviePage` mounts, fetch `GET /api/movies/{movieId}/watch-progress`.

- If progress exists, `progress_sec >= 180`, and `progress_sec / duration_sec < 0.98`, show a dialog before playback starts.
- The dialog displays the saved position formatted as time (e.g. "Resume from 1:23:45?") and offers two choices:
  - **Resume**: for HLS mode, navigate with `start` set to the saved `progress_sec`, which triggers the existing seek-offset flow. For direct mode, set `video.currentTime` directly after the video loads (since the `start` search param only applies to HLS).
  - **Start from beginning**: call `DELETE /api/movies/{movieId}/watch-progress` and start playback from 0.
- If no progress exists or it falls outside the thresholds, skip the dialog and play normally.
- Use the existing `AlertDialog` component from shadcn for consistent styling.

### 7. Frontend — watched toggle on movie details page

In `MovieDetailsHeroActions`, add a button between the Play button and the Like button that toggles the movie's watched status:

- On mount, use the `GET` watch progress endpoint to determine if the movie is marked as watched.
- Display a check/eye icon to indicate watched state. The button toggles the state via `PUT /api/movies/{movieId}/watch-progress/watched`.
- Use the `watched` value returned in the toggle response to update the UI immediately (optimistic or response-driven).

### Files to modify or create

| File | Action |
|------|--------|
| `server/sqlc/schema.sql` | Add `movie_watch_progress` table |
| `server/sqlc/queries/movie_watch_progress.sql` | **New** — sqlc queries |
| `server/cmd/api/watch_progress_handler.go` | **New** — HTTP handlers |
| `server/cmd/api/main.go` | Wire routes in `InitRouter` |
| `web/src/lib/api.ts` | Add fetch helpers for the endpoints |
| `web/src/routes/_auth/movies/$id/play.tsx` | Progress reporting, resume dialog, completion cleanup |
| `web/src/components/MovieDetailsHeroActions.tsx` | Add watched toggle button |

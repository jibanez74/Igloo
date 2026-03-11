# Movie Details Page — More Menu Work Plan

This document outlines the work plan for implementing the "More" button menu on the movie details page (`/movies/:id`). The menu currently shows disabled placeholders ("Add to list", "Share"); it will be replaced with the items below.

---

## Current State

- **Location**: `web/src/routes/_auth/movies/$id/index.tsx` — `DropdownMenu` with `MoreVertical` trigger (lines ~362–373).
- **Data**: Page uses `libraryMovieDetailsQueryOpts(movieId)`; response includes `movie`, `cast`, `crew`, `genres`, `production_companies`, `extra_videos`.
- **Backend**: `GetMovieDetails` in `server/cmd/api/movie_handler.go`; no existing API for updating movie metadata, fetching ffprobe for display, or deleting a movie. User has `is_admin` (e.g. `web/src/types/user.ts`, `server/sqlc/schema.sql`).

---

## 1. Playback Settings

**Goal**: Open a dialog with playback options: video quality, audio track, and subtitles.

**Scope**:

- **Video**: Reuse/align with existing stream modes used on the play page (`STREAM_MODES`: Direct, HLS 1080p 8 Mbps, HLS 1080p 4 Mbps, HLS 720p 3 Mbps). The dialog could either:
  - Set a **default** for this movie (would require storing per-movie or global preference and applying it when navigating to play), or
  - Act as a **pre-play** settings screen (user picks options then clicks "Play" to go to `/movies/:id/play` with those choices).
- **Audio**: List audio tracks for the file. Today play uses `audio_track=0` only. Backend already has ffprobe (and HLS session) knowledge of streams; need an API that returns audio stream count (and optionally labels) for the movie file so the dialog can offer track selection. Play page would then accept a chosen track (e.g. query or route param).
- **Subtitles**: Add UI for subtitle selection (off / none, or list of tracks if we have them). Backend has `DeleteMovieSubtitles` and scanner-side subtitle handling; need to confirm if subtitles are stored and exposed per movie and whether playback (direct and/or HLS) supports them.

**Tasks**:

1. Add **Playback Settings** menu item; on click open a dialog (e.g. `PlaybackSettingsDialog`).
2. Implement **PlaybackSettingsDialog**:
   - Video: dropdown of stream modes (same list as play page).
   - Audio: dropdown of audio tracks (labels like "Track 1", "Track 2", or language/title if available). Data from new or existing API (see below).
   - Subtitles: dropdown (e.g. "None" + any subtitle tracks if we expose them).
3. Decide behavior: **pre-play** (navigate to play with chosen video/audio/subtitle) vs **saved default** (store preference, play page reads it). Implement the chosen behavior.
4. Backend (if needed): endpoint to return audio (and optionally subtitle) stream count/labels for a movie file (e.g. `GET /api/movies/:id/streams` or include in existing details). Use existing ffprobe integration (`app.Ffprobe.GetMetadata(movie.FilePath)`) and filter by `codec_type`.
5. Play page: accept selected audio track (and subtitle if applicable) and pass to HLS URL / direct stream or apply in player.

**Dependencies**: None beyond existing movie details and play route.

---

## 2. Watch Together

**Goal**: Menu item only; no functionality (placeholder for future).

**Tasks**:

1. Add **Watch Together** menu item (e.g. second item in the dropdown).
2. On click: no-op or show a "Coming soon" toast/small message so the user gets feedback that the feature is planned.

**Dependencies**: None.

---

## 3. Edit

**Goal**: Allow the user to edit movie metadata either by **identifying via TMDB** (search by title, year, or TMDB ID) or by **entering data manually** (including poster and backdrop).

**Scope**:

- **Identify via TMDB**: Form with fields title, year, and optionally TMDB ID. Call TMDB search/fetch; on success, populate metadata (and optionally poster/backdrop URLs) and save to DB.
- **Manual entry**: Form with fields for all editable metadata (title, year, overview, tagline, certification, poster path/URL, backdrop path/URL, etc. as needed). Save to DB without TMDB.
- **Persistence**: Backend must support updating the movie record. Scanner uses `UpsertMovie` and related queries; we need a dedicated **update** API that:
  - Takes movie id + payload (metadata fields).
  - Updates only the movie row (and optionally related entities if we allow editing genres/cast/etc. later). For a first version, updating the movie table fields is enough.

**Tasks**:

1. Add **Edit** menu item; on click open an **Edit Movie** dialog (or slide-over).
2. Design dialog with two modes (tabs or steps):
   - **Identify with TMDB**: fields Title, Year, TMDB ID (optional). "Search" or "Fetch" uses existing TMDB integration (`SearchMoviesByTitleAndYear`, `GetTmdbMovieByID`). On result, show preview and "Apply" to write to DB.
   - **Manual**: form with title, year, release date, overview, tagline, certification, poster URL/path, backdrop URL/path, etc. "Save" writes to DB.
3. Backend:
   - Add `PATCH` or `PUT` endpoint, e.g. `PATCH /api/movies/:id` (or `PUT /api/movies/:id`). Require auth. Body: partial movie metadata (nullable fields for optional). Implement handler that:
     - Loads movie by id (return 404 if not found).
     - Updates only provided fields (e.g. title, year, overview, poster_path, backdrop_path, …) using a new sqlc query `UpdateMovie` (or reuse/adapt existing update logic if any).
   - Optional: add `GET /api/movies/:id/tmdb-search` or use a generic TMDB search endpoint that returns candidates so the UI can show a picker before applying. Alternatively, do search in the existing backend and return a single "best match" for the given title/year/tmdb_id.
4. After successful save, invalidate movie details query and close dialog so the details page reflects new data.

**Dependencies**: TMDB already used in scanner; new API for update and optional TMDB search endpoint.

---

## 4. Technical Details

**Goal**: Open a modal that shows a formatted list of technical details for the movie file, based on ffprobe output.

**Scope**:

- Use existing ffprobe types: `Format` (filename, duration, size, bit_rate, format_name, format_long_name, tags), `Stream` (index, codec_name, codec_type, profile, bit_rate; for video: width, height, aspect_ratio, frame rate; for audio: sample_rate, channels, channel_layout), and `Chapters` if desired.
- Exclude or clearly label attached-picture streams (e.g. cover art) so they don’t look like video tracks.

**Tasks**:

1. Add **Technical Details** menu item; on click open a **TechnicalDetailsModal**.
2. Backend: add `GET /api/movies/:id/technical-details`. Handler:
   - Resolve movie by id, get `file_path`.
   - Call `app.Ffprobe.GetMetadata(movie.FilePath)`.
   - Return a JSON structure suitable for display (e.g. format section + list of streams with human-readable labels). Filter out `disposition.attached_pic == 1` from streams or mark them as "Attachment" so UI can hide or group them.
3. Frontend: fetch technical details when modal opens (or on menu click). Render in a readable layout:
   - **Format**: filename, duration, size, bit rate, format name.
   - **Video streams**: index, codec, resolution, aspect ratio, frame rate, bit rate, profile, etc.
   - **Audio streams**: index, codec, channels, sample rate, bit rate, language/title if present.
   - **Subtitles** (if any): index, codec, language/title.
   - **Chapters** (optional): list with start/end/time.
4. Use existing `Dialog` (or equivalent) for the modal; ensure accessibility (focus trap, close on Escape, aria labels).

**Dependencies**: Existing ffprobe integration; no new dependencies.

---

## 5. Delete

**Goal**: Allow **admin users only** to delete the movie (library record and optionally the file).

**Scope**:

- Only show the **Delete** menu item when the current user has `is_admin === true` (use existing `AuthUser` / session data).
- Delete means: remove movie row and related data (cast, crew, genres, production companies, extra videos, streams, chapters, subtitles, etc.). Optionally delete the file on disk (configurable or separate "Delete file" choice) — to be decided.

**Tasks**:

1. Add **Delete** menu item; render it only if `user.is_admin` (get user from auth context or existing hook that provides current user).
2. On click: open a confirmation dialog ("Delete this movie? This cannot be undone."). If "Delete file from disk" is in scope, add a checkbox and pass the choice to the API.
3. Backend: add `DELETE /api/movies/:id`. Handler:
   - Require auth.
   - Check `user.is_admin`; if not admin, return 403.
   - Load movie by id; if not found return 404.
   - In a transaction: delete related rows in order (e.g. cast, crew, genres, production companies, extra videos, video/audio streams, chapters, subtitles — match scanner delete order), then delete the movie row. If "delete file" is requested, remove the file at `movie.FilePath` after DB commit (and handle errors appropriately).
4. On success: redirect to `/movies` (or parent list) and invalidate movie list/details queries.

**Dependencies**: Existing `is_admin` on user; no new schema if we only delete DB rows and optionally the file.

---

## Implementation Order (Suggested)

1. **Watch Together** — trivial placeholder.
2. **Technical Details** — one new API, one modal, no auth rules.
3. **Playback Settings** — dialog + play page integration; may need one small API for streams.
4. **Edit** — new update API + TMDB/manual forms; more moving parts.
5. **Delete** — admin check + delete API + confirmation; do after Edit so movie management is consistent.

---

## Menu Structure (Final)

- Playback Settings → opens Playback Settings dialog
- Watch Together → placeholder (e.g. toast "Coming soon")
- Edit → opens Edit Movie dialog (TMDB identify or manual)
- Technical Details → opens Technical Details modal
- Delete → only visible when `user.is_admin`; opens confirmation then calls delete API

Use `DropdownMenuSeparator` between logical groups if desired (e.g. before Delete).

---

## Open Questions

1. **Playback Settings**: Should the dialog only affect the *next* play (user clicks "Play" from dialog and goes to play page with those options), or should we persist a default (e.g. in user settings or per-movie) and have the play page read it automatically?
2. **Subtitles**: Are subtitle tracks currently stored and exposed for library movies? If yes, where (DB table, ffprobe-only)? Should playback (direct and HLS) support subtitles in this phase, or should the Playback Settings dialog only show a "None" option for now?
3. **Edit — poster/backdrop**: For "manual" edit, do we store poster/backdrop as full URLs (e.g. TMDB CDN) or as path segments (e.g. `/xy/abc.jpg`) and let the frontend prefix with `TMDB_IMAGE_BASE`? Same for user-uploaded images if we allow that later.
4. **Edit — TMDB**: When identifying by TMDB, should we overwrite *all* metadata (cast, crew, genres, production companies, extra videos) by re-running the same logic as the scanner for that movie, or only update the main movie row and leave related entities unchanged until a full rescan?
5. **Delete**: Should "Delete" remove only the library entry (DB) and leave the file on disk, or should we offer (or default to) deleting the file as well? Any need for a "trash" or soft-delete?
6. **Technical Details**: Should we cache ffprobe results (e.g. in DB or in-memory per movie) to avoid running ffprobe on every modal open, or is running it on demand acceptable for your library sizes?

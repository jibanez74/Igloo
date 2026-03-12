# Movie Details Page — More Menu Work Plan

This document outlines the work plan for implementing the "More" button menu on the movie details page (`/movies/:id`). The menu currently shows disabled placeholders ("Add to list", "Share"); it will be replaced with the items below.

**Progress checklist**: `MOVIE_DETAILS_MORE_MENU_CHECKLIST.txt` — tracks task completion across all four implementation phases.

---

## Current State

- **Location**: `web/src/routes/_auth/movies/$id/index.tsx` — `DropdownMenu` with `MoreVertical` trigger (lines ~362–373). Two disabled items: "Add to list", "Share".
- **Data**: Page uses `libraryMovieDetailsQueryOpts(movieId)`; response includes `movie`, `cast`, `crew`, `genres`, `production_companies`, `extra_videos`.
- **Backend**: `GetMovieDetails` in `server/cmd/api/movie_handler.go`; no existing API for updating movie metadata or deleting a movie.
- **DB**: Stream metadata (video\_streams, audio\_streams, subtitles, chapters) is **already stored** by the scanner via ffprobe. However, there are **no SELECT queries** for these tables yet — only INSERT and DELETE.
- **UI**: shadcn/ui (New York style) with Radix primitives, Tailwind v4, lucide-react icons. Components at `web/src/components/ui/`.
- **Auth**: User has `is_admin` field (schema, types, session).

---

## 1. Playback Settings

**Goal**: Dialog to configure video quality, audio track, and subtitles for the next playback. Settings are stored **in component state only** (not persisted) and are discarded when the movie details page unmounts.

**Behavior**: User opens the dialog from the menu, picks options, closes the dialog. The settings are held in React state on the movie details page. When the user later clicks the existing **Play** button, it navigates to `/movies/:id/play` passing those settings as search params. If the user never opens Playback Settings, the Play button uses defaults (Direct stream, audio track 0).

### Frontend

1. Add **Playback Settings** menu item in the dropdown.
2. On click, open `PlaybackSettingsDialog` (new component using shadcn `Dialog`).
3. Dialog contents:
   - **Video Quality**: `Select` with stream mode options. Default: "Direct stream".
     - Direct stream
     - HLS 2160p 16 Mbps *(new — `2160p_16mbps`)*
     - HLS 1080p 8 Mbps (`1080p_8mbps`)
     - HLS 1080p 6 Mbps *(new — `1080p_6mbps`)*
     - HLS 1080p 4 Mbps (`1080p_4mbps`)
     - HLS 720p 3 Mbps (`720p_3mbps`)
   - **Audio Track**: `Select` populated from audio\_streams for this movie (fetched from backend). Labels: `"{language} — {codec} {channels}ch"` or `"Track {stream_index}"` if no language. Default: first track (stream\_index 0).
   - **Subtitles**: `Select` with only `"None"` for now (placeholder for future).
   - **"Save" / "Done" button**: closes the dialog (does **not** start playback).
4. State management on movie details page:
   - Add `useState` for `playbackSettings: { mode: StreamModeId, audioTrack: number }` with default `{ mode: "direct", audioTrack: 0 }`.
   - `PlaybackSettingsDialog` receives current settings and an `onSave` callback that updates state and closes the dialog.
   - Settings are ephemeral — discarded on unmount (no localStorage, no URL state, no persistence).
5. Update the **Play** `<Link>` on the movie details page:
   - Change from a plain `<Link to="/movies/$id/play">` to include search params from `playbackSettings`: `search: { mode, audio_track }`.
6. Modify play page route (`web/src/routes/_auth/movies/$id/play.tsx`):
   - Add `validateSearch` to accept optional `mode` (StreamModeId) and `audio_track` (number).
   - Use search params as initial state for `streamMode` and `audioTrack` instead of hardcoded defaults.
   - Update `buildStreamUrl` to use the `audioTrack` state instead of hardcoded `0`.
   - Keep existing in-page stream mode dropdown functional (user can still change during playback).
7. Update `STREAM_MODES` constant on the play page to include the two new profiles:
   ```ts
   const STREAM_MODES = [
     { id: "direct", label: "Direct stream" },
     { id: "2160p_16mbps", label: "HLS 2160p 16 Mbps" },
     { id: "1080p_8mbps", label: "HLS 1080p 8 Mbps" },
     { id: "1080p_6mbps", label: "HLS 1080p 6 Mbps" },
     { id: "1080p_4mbps", label: "HLS 1080p 4 Mbps" },
     { id: "720p_3mbps", label: "HLS 720p 3 Mbps" },
   ] as const;
   ```

### Backend

8. Add the two new profiles to `HLSAllowedProfiles` and `HLSProfileConfigs` in `server/cmd/internal/helpers/hls_profiles.go`:
   - `2160p_16mbps`: Width 3840, Height 2160, VideoBitrate "16M", Bufsize "32M".
   - `1080p_6mbps`: Width 1920, Height 1080, VideoBitrate "6M", Bufsize "12M".
9. Add sqlc query `GetAudioStreamsByMovieID` in `server/sqlc/queries/movies.sql`:
   ```sql
   SELECT id, movie_id, stream_index, codec, codec_profile, bit_rate,
          sample_rate, channels, channel_layout, language, title
   FROM audio_streams WHERE movie_id = ? ORDER BY stream_index;
   ```
10. Add `GET /api/movies/:id/audio-streams` endpoint (or include audio streams in the existing movie details response). Returns the list of audio streams for the movie.
11. Run `sqlc generate`.

### Dependencies

None beyond existing movie details and play route.

---

## 2. Watch Together

**Goal**: Menu item only; placeholder for future feature.

### Tasks

1. Add **Watch Together** menu item (after Playback Settings).
2. On click: show a toast — "Coming soon" — using existing toast system.

### Dependencies

None.

---

## 3. Edit

**Goal**: Allow **admin users only** to edit movie metadata via two modes: **Identify with TMDB** (full replacement like the scanner) or **Manual** (update individual fields).

### Decisions (from answers)

- **TMDB mode**: Replace *everything* — movie row + cast, crew, genres, production companies, extra videos — same as scanner.
- **Manual mode**: Update only the movie row fields provided.
- **Poster/backdrop**: Store TMDB path segments (e.g. `/abcdef.jpg`); frontend completes the URL.

### Frontend

1. Add **Edit** menu item in the dropdown; render only if `user.is_admin`.
2. On click, open `EditMovieDialog` (shadcn `Dialog` with `Tabs`):
   - **Tab 1 — Identify with TMDB**:
     - Fields: Title (pre-filled), Year (pre-filled), TMDB ID (optional).
     - "Search" button → calls `POST /api/movies/:id/tmdb-search` with `{ title, year, tmdb_id }`.
     - Show search results as selectable cards (poster, title, year, overview snippet).
     - User selects a result → "Apply" calls `PUT /api/movies/:id/identify` with the chosen TMDB ID.
     - On success: invalidate movie details query, close dialog.
   - **Tab 2 — Manual**:
     - Form fields: title, year, release\_date, overview, tagline, certification, poster\_path, backdrop\_path, language.
     - Pre-filled with current movie data.
     - "Save" calls `PATCH /api/movies/:id` with changed fields.
     - On success: invalidate movie details query, close dialog.

### Backend

3. **New sqlc query** — `UpdateMovie` in `server/sqlc/queries/movies.sql`:
   - Dedicated UPDATE (not the COALESCE-style `UpsertMovie`).
   - Updates: title, tmdb\_id, imdb\_id, poster\_path, backdrop\_path, adult, language, year, release\_date, overview, tag\_line, certification, critic\_rating, audience\_rating, revenue, budget, run\_time.
   - Note: The existing `UpsertMovie` **does not update `backdrop_path`** on conflict — this is a known gap.

4. **New sqlc queries** — `DeleteMovieCast` and `DeleteMovieCrew`:
   - `DELETE FROM cast WHERE movie_id = ?`
   - `DELETE FROM crew WHERE movie_id = ?`
   - Needed because the scanner uses `UpsertCast`/`UpsertCrew` (which don't remove stale rows). For "replace everything", we need to delete first.

5. **TMDB Search endpoint** — `POST /api/movies/:id/tmdb-search`:
   - Body: `{ title: string, year?: int, tmdb_id?: int }`.
   - If `tmdb_id` provided: call `GetTmdbMovieByID` directly, return single result.
   - If title provided: call `SearchMoviesByTitleAndYear(title, year)`, return list of candidates (id, title, year, overview, poster\_path).
   - Require auth + `is_admin` → 403 if not admin.

6. **TMDB Identify endpoint** — `PUT /api/movies/:id/identify`:
   - Body: `{ tmdb_id: int }`.
   - Fetches full TMDB data via `GetTmdbMovieByID`.
   - In a transaction:
     - Update the movie row with TMDB metadata (use `UpdateMovie`).
     - Delete and re-insert related entities (reuse scanner functions):
       - `DeleteMovieProductionCompanies` → `processProductionCompanies`
       - `DeleteMovieCast` → `processCast`
       - `DeleteMovieCrew` → `processCrew`
       - `DeleteMovieGenres` → `processMovieGenres`
       - `DeleteMovieExtraVideos` → `processExtraVideos`
     - Do **not** re-process streams/chapters (file hasn't changed).
   - Require auth + `is_admin` → 403 if not admin.

7. **Manual Update endpoint** — `PATCH /api/movies/:id`:
   - Body: partial movie metadata (only fields to update).
   - Handler loads movie by ID (404 if not found), applies updates via `UpdateMovie`.
   - Require auth + `is_admin` → 403 if not admin.

8. Run `sqlc generate` after adding queries.

### Dependencies

- Existing TMDB integration (`SearchMoviesByTitleAndYear`, `GetTmdbMovieByID`).
- Existing scanner entity-processing functions (may need to extract/refactor for reuse outside the scan loop).

---

## 4. Technical Details

**Goal**: Open a modal showing formatted technical details for the movie file, using data **already stored in the database** (no ffprobe call at runtime).

### Data Sources (all in DB)

| Section | Source Table | Key Fields |
|---------|-------------|------------|
| **File** | `movies` | `file_name`, `file_path`, `size`, `container`, `mime_type`, `run_time` |
| **Video** | `video_streams` | `stream_index`, `codec`, `codec_profile`, `codec_level`, `bit_rate`, `width`, `height`, `aspect_ratio`, `frame_rate`, `bit_depth`, `color_space`, `color_range`, `color_primaries`, `color_transfer`, `language`, `title` |
| **Audio** | `audio_streams` | `stream_index`, `codec`, `codec_profile`, `bit_rate`, `sample_rate`, `channels`, `channel_layout`, `language`, `title` |
| **Subtitles** | `subtitles` | `stream_index`, `codec`, `language`, `title`, `is_forced`, `is_default` |
| **Chapters** | `chapters` | `title`, `start_time`, `thumb` |

### Frontend

1. Add **Technical Details** menu item in the dropdown.
2. On click, open `TechnicalDetailsDialog` (shadcn `Dialog`).
3. Fetch data from `GET /api/movies/:id/technical-details`.
4. Render in a clean, readable layout with sections:
   - **File**: filename, size (human-readable), container, duration.
   - **Video Streams**: one card/row per stream — codec, resolution, aspect ratio, frame rate, bit rate, profile, bit depth, color info.
   - **Audio Streams**: one card/row per stream — codec, channels, channel layout, sample rate, bit rate, language.
   - **Subtitles**: one row per track — codec, language, forced/default flags.
   - **Chapters** (if any): list with title and timestamp.

### Backend

5. Add sqlc queries in `server/sqlc/queries/movies.sql`:
   - `GetVideoStreamsByMovieID`: `SELECT * FROM video_streams WHERE movie_id = ? ORDER BY stream_index`
   - `GetAudioStreamsByMovieID`: `SELECT * FROM audio_streams WHERE movie_id = ? ORDER BY stream_index` (shared with Playback Settings)
   - `GetSubtitlesByMovieID`: `SELECT * FROM subtitles WHERE movie_id = ? ORDER BY stream_index`
   - `GetChaptersByMovieID`: `SELECT * FROM chapters WHERE movie_id = ? ORDER BY start_time`
6. Add `GET /api/movies/:id/technical-details` endpoint. Handler:
   - Load movie by ID (404 if not found).
   - Query all four stream/chapter tables.
   - Return JSON with `{ movie: {...}, video_streams: [...], audio_streams: [...], subtitles: [...], chapters: [...] }`.
7. Run `sqlc generate`.

### Dependencies

None; all data already in DB.

---

## 5. Delete

**Goal**: Allow **admin users only** to delete the movie. Default: DB-only. Optional checkbox to also delete the file from disk.

### Frontend

1. Add **Delete** menu item; render only if `user.is_admin`. Style destructively (red text). Separated from other items with `DropdownMenuSeparator`.
2. On click, open `DeleteMovieDialog` (shadcn `AlertDialog`):
   - Message: "Are you sure you want to delete **{title}**? This action cannot be undone."
   - Checkbox: "Also delete the movie file from disk" (unchecked by default).
   - "Cancel" and "Delete" buttons.
3. On confirm: call `DELETE /api/movies/:id` with body `{ delete_file: boolean }`.
4. On success: navigate to `/movies` (or parent route), invalidate movie list queries, show success toast.

### Backend

5. Add `DELETE /api/movies/:id` endpoint. Handler:
   - Require auth; check `user.is_admin` → 403 if not admin.
   - Load movie by ID → 404 if not found.
   - Read `delete_file` from request body.
   - Delete the movie row. Related data (cast, crew, genres, production companies, extra videos, video\_streams, audio\_streams, subtitles, chapters) will be cascade-deleted via `ON DELETE CASCADE` foreign keys already defined in the schema.
   - If `delete_file` is true: remove the file at `movie.FilePath` after DB delete. Log but don't fail the request if file removal fails (file may already be gone).
6. No new sqlc queries needed — a simple `DELETE FROM movies WHERE id = ?` suffices since all related tables have `ON DELETE CASCADE`.

### Dependencies

Existing `is_admin` on user; existing schema cascade deletes.

---

## Implementation Phases

Work is grouped into four phases (see `MOVIE_DETAILS_MORE_MENU_CHECKLIST.txt` for task-level tracking):

1. **Phase 1: Watch Together + Technical Details** — trivial placeholder + read-only feature. Sets up menu structure and creates shared sqlc SELECT queries (audio\_streams) reused by Phase 2.
2. **Phase 2: Playback Settings** — dialog + play page search params + new HLS profiles. Reuses `GetAudioStreamsByMovieID` from Phase 1.
3. **Phase 3: Edit** — most complex: two modes (TMDB / Manual), three endpoints, scanner function refactoring. Admin only.
4. **Phase 4: Delete** — admin check + cascade delete + optional file removal. Last because it's destructive.

---

## Menu Structure (Final)

```
┌─────────────────────────┐
│ Playback Settings       │  → opens PlaybackSettingsDialog
│ Watch Together          │  → toast "Coming soon"
│ Edit                    │  → opens EditMovieDialog (admin only, TMDB / Manual tabs)
│ Technical Details       │  → opens TechnicalDetailsDialog
│─────────────────────────│  ← DropdownMenuSeparator
│ Delete                  │  → opens DeleteMovieDialog (admin only, red text)
└─────────────────────────┘
```

---

## New Backend Queries Summary

| Query | Table | Type | Used By |
|-------|-------|------|---------|
| `GetVideoStreamsByMovieID` | video\_streams | SELECT | Technical Details |
| `GetAudioStreamsByMovieID` | audio\_streams | SELECT | Technical Details, Playback Settings |
| `GetSubtitlesByMovieID` | subtitles | SELECT | Technical Details |
| `GetChaptersByMovieID` | chapters | SELECT | Technical Details |
| `UpdateMovie` | movies | UPDATE | Edit (Manual), Edit (TMDB Identify) |
| `DeleteMovieCast` | cast | DELETE | Edit (TMDB Identify) |
| `DeleteMovieCrew` | crew | DELETE | Edit (TMDB Identify) |
| `DeleteMovie` | movies | DELETE | Delete |

---

## New API Endpoints Summary

| Method | Path | Auth | Used By |
|--------|------|------|---------|
| `GET` | `/api/movies/:id/audio-streams` | Yes | Playback Settings |
| `GET` | `/api/movies/:id/technical-details` | Yes | Technical Details |
| `POST` | `/api/movies/:id/tmdb-search` | Admin | Edit (TMDB tab) |
| `PUT` | `/api/movies/:id/identify` | Admin | Edit (TMDB apply) |
| `PATCH` | `/api/movies/:id` | Admin | Edit (Manual) |
| `DELETE` | `/api/movies/:id` | Admin | Delete |

---

## New Frontend Components Summary

| Component | Type | Used By |
|-----------|------|---------|
| `PlaybackSettingsDialog` | Dialog | Playback Settings menu item |
| `EditMovieDialog` | Dialog + Tabs | Edit menu item |
| `TechnicalDetailsDialog` | Dialog | Technical Details menu item |
| `DeleteMovieDialog` | AlertDialog | Delete menu item |

# Server Follow-up Findings

This document records deferred findings from the server constants audit performed on 2026-07-10. These items are intentionally outside the bounded constants cleanup and remain open for later work.

## API-owned constants in the shared helpers package

`server/cmd/internal/helpers/constants.go` still contains constants referenced only by `server/cmd/api`. Under the rule in `server/AGENTS.md`, these should remain close to their owning file or narrowly scoped package instead of being exported from `helpers`.

The remaining candidates are grouped by domain:

- Runtime configuration: `DEFAULT_APP_PORT`, `DEFAULT_DB_PATH`, `DEFAULT_STATIC_DIR`, `DEFAULT_LOGS_DIR`, `DEFAULT_TRANSCODE_DIR`, `ENV_DB_PATH`, `ENV_STATIC_DIR`, `ENV_LOGS_DIR`, `ENV_TRANSCODE_DIR`, `ENV_SESSION_COOKIE_SECURE`, `ENV_LOG_TO_STDOUT`, `ENV_PORT`.
- Authentication and bootstrap: `COOKIE_USER_ID`, `DEFAULT_ADMIN_NAME`, `DEFAULT_ADMIN_EMAIL`, `DEFAULT_ADMIN_PASSWORD`, `ENV_DEFAULT_ADMIN_NAME`, `ENV_DEFAULT_ADMIN_EMAIL`, `ENV_DEFAULT_ADMIN_PASSWORD`, `INTERNAL_SERVER_ERROR`, `NOT_AUTHORIZED_MESSAGE`, `INVALID_CREDENTIALS_MESSAGE`.
- Scanning and listing: `SCANNER_BATCH_SIZE`, `MOVIES_LIBRARY_DEFAULT_PER_PAGE`, `MOVIES_LIBRARY_MAX_PER_PAGE`, `SEARCH_DEFAULT_PER_PAGE`, `SEARCH_MAX_PER_PAGE`, `SEARCH_ALL_TOP_N`.
- Playlists and notifications: `PLAYLIST_CONTENT_TYPE_TRACK`, `PLAYLIST_CONTENT_TYPE_MOVIE`, `MAX_PLAYLIST_REQUEST_SIZE`, `NOTIFICATION_TITLE_MOVIE_REQUEST`, `NOTIFICATION_TITLE_ALBUM_REQUEST`, `NOTIFICATION_TITLE_TRACK_REQUEST`, `NOTIFICATION_TITLE_OTHER`.
- TMDB presentation: `TMDB_IMAGE_BASE_URL`, `TMDB_IMAGE_SIZE`, `TMDB_BACKDROP_SIZE`, `TMDB_POSTER_SIZE`, `TMDB_PROFILE_SIZE`, `TMDB_LOGO_SIZE`, `TMDB_MAX_ITEMS`, `TMDB_YEAR_MATCH_SCORE`.
- HLS and playback: `HLS_COPY_VIDEO_TARGET_DURATION`, `HLS_REMUX_PREVALIDATE_TIMEOUT`, `HLS_REMUX_SAFETY_CACHE_TTL`, `HLS_REMUX_SAFETY_CACHE_SWEEP`, `ENV_HLS_MAX_CPU_TRANSCODES`, `HLS_CPU_TRANSCODE_DEFAULT_DIVISOR`, `HLS_TRANSCODE_BUSY_RETRY_AFTER_SEC`, `HDR_TRANSFER_PQ`, `HDR_TRANSFER_HLG`, `HLS_PLAYBACK_SESSION_ID_PATTERN`, `HLS_SESSION_TTL`, `HLS_SESSION_CACHE_SWEEP`, `HLS_SEGMENT_WAIT`, `HLS_SEGMENT_POLL`, `HLS_PLAYLIST_CONTENT_TYPE`, `HLS_SEGMENT_HTTP_CONTENT_TYPE`.
- Watch rooms, progress, and subtitles: `WATCH_ROOM_PLAYBACK_MODE_DIRECT`, `MAX_WATCH_ROOM_REQUEST_SIZE`, `WATCH_COMPLETION_THRESHOLD`, `SUBTITLE_WEBVTT_CONTENT_TYPE`, `SUBTITLE_EXTRACT_TIMEOUT`, `SUBTITLE_CACHE_TTL`, `SUBTITLE_CACHE_CLEANUP`.

Future cleanup should move each group to its owning API file, make names unexported, update references, and preserve every value and behavior. Re-run the reference audit afterward so genuinely cross-package constants remain in `helpers/constants.go`.

## Duplicated HLS playlist filename

The shared HLS output contract includes `playlist.m3u8`, but that filename is duplicated as a literal in API handlers, HLS session code, FFmpeg argument construction, and FFmpeg fixture code.

A future cleanup should add a descriptively named shared filename constant and replace production duplicates. Tests may retain literals when they intentionally verify the external HLS contract. Because this is media-related, review `docs/ffmpeg.md` and run the relevant HLS tests plus `make check`.

## Mutable codec maps in `constants.go`

`BitmapSubtitleCodecs` and `CoverArtVideoCodecs` are mutable exported maps in `server/cmd/internal/helpers/constants.go`, but each is used only by one helper implementation.

Move the bitmap subtitle map beside `IsBitmapSubtitleCodec` and the cover-art map beside `IsCoverArtVideoCodec`, make both unexported, and preserve their entries. This reduces exported mutable state and keeps the lookup data close to its behavior.

## Inline assignments in control flow

A static scan found 154 production, non-generated `if` statements with inline assignments, contrary to the server control-flow rule. Generated sqlc files and test files were excluded.

The largest concentrations at audit time were:

- `playlist_handler.go`: 12
- `watch_progress_handler.go`: 11
- `playback_settings_handler.go`: 11
- `user_handler.go`: 10
- `music_scanner_helpers.go`: 10

Reproduce the scan with:

```bash
rg -n --glob '*.go' --glob '!*_test.go' --glob '!**/database/**' '^\s*if\b[^\n]*:=' server/cmd
```

Address these incrementally by assigning function results, errors, map lookups, and comparisons before the `if`. Keep each cleanup behavior-preserving and run focused tests followed by `make check`.


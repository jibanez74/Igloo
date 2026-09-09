# Query data audit

Audit scope: all **243 original sqlc queries**, including explicit projections, plus raw application SQL and schema triggers. Two focused reads were added, for **245 current named queries**. Production callers, response mapping, handwritten fetch functions, frontend transformations and playback/cache paths were traced; generated types and test fixtures were used for verification, not as proof of application use.

**76 existing query projections narrowed**, removing **421 returned column slots** when each changed original query definition is counted once. **56 stored columns across 27 tables** were removed directly from the current schema. These are structural counts, not measured latency, allocation, database-byte or network-byte improvements. The two new focused reads are reported separately below and are not added to the 421 count.

No routes, migrations, compatibility layers or database resets were introduced. Filtering, ordering, foreign keys, uniqueness, authentication, scanner reconciliation and media protocol requirements count as uses even when values are not rendered. Existing databases were not modified by this work.

## Consumer evidence and response changes

- **Scanner writes:** [movie persistence](../server/cmd/internal/scanner/movie/persistence.go), [stream persistence](../server/cmd/internal/scanner/movie/streams.go), [music persistence](../server/cmd/internal/scanner/music/persistence.go), [music identity/cache helpers](../server/cmd/internal/scanner/music/relationships.go) and [retry](../server/cmd/internal/scanner/music/retry.go) discard write rows or use only IDs/identity/image comparisons. Narrowed results retain transaction rollback, conflict updates and cache publication after commit.
- **Music responses:** [album](../server/cmd/api/album_handler.go), [musician](../server/cmd/api/musician_handler.go), [track](../server/cmd/api/track_handler.go), playlist, search and statistics handlers feed [audio conversion](../web/src/lib/audio-utils.ts), the music routes and [AudioPlayer](../web/src/components/playback/AudioPlayer.tsx). Streaming uses `/api/tracks/{id}/stream`, not `file_path`. Album tracks keep MIME, index/disc, duration and quality fields for playback and list presentation. Album/musician genre arrays remain strings, including empty arrays.
- **Movie responses:** [details/technical handlers](../server/cmd/api/movie_handler.go) feed the movie details route, cast/crew credits, trailers, technical-details dialog and [playback data hook](../web/src/hooks/useMoviePlaybackData.ts). Descriptive details no longer carry filesystem metadata/timestamps; technical details keep filename, size, container, MIME, runtime and exact duration, without the absolute path. Descriptive duration is retained because playback uses it as a fallback. Cast/crew join IDs, crew portrait, company provider/logo/country fields and extra-video bookkeeping fields are removed from responses.
- **Notifications:** [handler](../server/cmd/api/notifications_handler.go) and the movie/album/track request dialogs consume creation success/error only. HTTP 201 now has the success envelope without `data`. The bell still gets sender name from the creator join, creation time, read state, content and action identity.
- **Authentication and rooms:** [device issuance](../server/cmd/api/quick_connect_handler.go), [middleware](../server/cmd/api/middleware.go), user response mapping and [watch rooms](../server/cmd/api/watch_room_handler.go) establish the narrowed fields. Device ownership/hash remain stored for authorization; expiry still uses last-used time. Room media pins remain in the member-authorized queries; list and delete reads are narrower.
- **Contract:** [OpenAPI](openapi.json), generated TypeScript, handwritten types/calls and affected fixtures were updated together. SQL nullable wrappers, response nullability, pagination and ordering remain unchanged. Generated empty slices preserve `[]` genre responses.

## Stored-column removals

The following columns had only writes, defaults, obsolete maintenance, or projections eliminated above. Corresponding write parameters and timestamp assignments were removed. Every other column, index, relationship and constraint was retained.

| Table | Removed columns | Evidence |
| --- | --- | --- |
| `settings` | `created_at`, `updated_at` | Configuration snapshot/manager does not read or expose these times; singleton key and all configuration values remain. |
| `production_companies` | `logo`, `country`, `created_at`, `updated_at` | Movie details consumes only local ID/name; TMDB ID remains the conflict key. Logo/country and timestamps had no downstream use. |
| `artist` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `genres` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `extra_videos` | `official`, `created_at`, `updated_at` | Trailer selection/link building consumes key/type/site/title. External ID uniqueness remains; official flag/timestamps unused. |
| `track_musicians` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `musician_genres` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `musician_albums` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `track_genres` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `album_genres` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `music_spotify_matches` | `spotify_id`, `score`, `threshold_value`, `candidate_name`, `candidate_artist`, `search_query`, `strategy`, `error`, `updated_at` | Resolution/retry/compound repair consume status/reason only. Provider ID is stored on musician/album; diagnostic candidate/search/score fields were write-only. |
| `video_streams` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `audio_streams` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `subtitles` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `remux_safety_verdicts` | `created_at`, `updated_at` | Fingerprint determines validity; payload is consumed. Movie timestamps in fingerprints remain. |
| `keyframe_indexes` | `created_at`, `updated_at` | Fingerprint determines validity; payload is consumed. Movie timestamps in fingerprints remain. |
| `cast` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `crew` | `created_at`, `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `movie_production_companies` | `created_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `movie_genres` | `created_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `movie_extra_videos` | `created_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `user_liked_movies` | `created_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `user_track_stats` | `first_played_at`, `updated_at` | Aggregates and last_played_at feed stats; neither removed time participates in a read or ordering. |
| `watch_rooms` | `updated_at` | created_at orders room lists; live WebSocket playback timestamps are independent of this unused SQL column. |
| `watch_room_members` | `updated_at` | created_at still orders members; updated_at was never read or maintained by behavior. |
| `notifications` | `updated_at` | No production read, filter, order, identity, constraint or cache dependency on these row timestamps. |
| `notification_reads` | `read_at` | Read state is row existence; no read-time display, ordering or expiry. |

## Retained dependencies and unresolved fields

- File paths, filenames, sizes, catalog timestamps, file hashes and filesystem identity remain stored for serving, invalidation, quiet-period detection and missing-file cleanup. Video/audio/subtitle codec metadata, dispositions, absolute stream indices, rotation, interlace, color and frame-rate data remain available to media planning and technical details. No FFmpeg arguments or codec eligibility rules changed.
- Normalized identity aliases, sort/date contribution tables, local/Spotify genre provenance, FTS content and generation triggers, playlist positions/access relationships, progress save-session/sequence fields and token hashes are internal uses. Their absence from a UI does not justify deletion.
- The standalone track-details endpoint has no current web fetch consumer. Its remaining technical/tag fields are retained to preserve the documented endpoint feature. This audit does not claim they are all displayed. Album-list consumers now use a separate narrower projection.
- Album/musician details still expose sort/provider/popularity/follower/catalog timestamps; playlist and collaborator rows expose association/creation metadata; user profiles expose timestamps; statistics expose metrics/history even where current web screens do not consume every field. Their complete downstream requirement is unresolved. These remaining fields are retained conservatively rather than using serialization or generated types as proof of application use.
- `user_play_history.completed` remains part of the accepted play-event recording feature even though current statistical reads do not filter on it. Chapter parent IDs and stream movie IDs remain in the existing technical-detail protocol. User mutation results retain PIN nullness in the existing sql nullable representation; PIN contents are never serialized.

## Query-by-query inventory

Counts are returned SQL columns, excluding affected-row counts. “None” means no unused result field was established (or the query already returns no row), not a claim that every exposed field is visibly rendered. Links identify current production callers; the sections above trace their downstream consumers. Field names below are SQL-style names derived from generated scan destinations.


### [albums.sql](../server/sqlc/queries/albums.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetAlbumByID` | 12 → 12 | None | Detail endpoint exposes catalog presentation/provider metadata; delete/identity logic also uses relevant IDs. Preserve broad detail feature where downstream use is unresolved; see retained uncertainties. [album_handler.go](../server/cmd/api/album_handler.go) |
| `GetAlbumBySpotifyID` | 12 → 3 | `title`, `sort_title`, `spotify_popularity`, `musician`, `release_date`, `year`, `total_tracks`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and cover comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `GetAlbumsBySpotifyIDs` | 2 → 2 | None | Provider search maps provider ID to local ID for already-in-library status and navigation; retain both identities. [spotify_handler.go](../server/cmd/api/spotify_handler.go) |
| `GetLatestAlbums` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [album_handler.go](../server/cmd/api/album_handler.go) |
| `GetAlbumsAlphabetical` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [album_handler.go](../server/cmd/api/album_handler.go) |
| `UpdateAlbumSpotifyCover` | 12 → 3 | `title`, `sort_title`, `spotify_popularity`, `musician`, `release_date`, `year`, `total_tracks`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and cover comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `UpsertAlbum` | 12 → 3 | `title`, `sort_title`, `spotify_popularity`, `musician`, `release_date`, `year`, `total_tracks`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and cover comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `DeleteAlbum` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [album_handler.go](../server/cmd/api/album_handler.go) |

### [devices.sql](../server/sqlc/queries/devices.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `CreateDevice` | 8 → 6 | `user_id`, `token_hash` | Token issuance owns the user ID and plaintext token. Response mapper consumes the six device summary values; no need to return the hash or ownership ID. [quick_connect_handler.go](../server/cmd/api/quick_connect_handler.go) |
| `GetDeviceByTokenHash` | 8 → 3 | `name`, `platform`, `app_version`, `token_hash`, `created_at` | Authentication caches/checks device ID, user ID, and last-use expiry. Hash remains the indexed predicate; descriptive device fields belong to device-list responses. [middleware.go](../server/cmd/api/middleware.go) |
| `GetDevicesByUser` | 7 → 6 | `user_id` | Settings device list needs the six summary values. Ownership is supplied by the authenticated query predicate. [device_handler.go](../server/cmd/api/device_handler.go) |
| `UpdateDeviceLastUsed` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [middleware.go](../server/cmd/api/middleware.go) |
| `RenameDevice` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [device_handler.go](../server/cmd/api/device_handler.go) |
| `DeleteDeviceForUser` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [auth_handler.go](../server/cmd/api/auth_handler.go), [device_handler.go](../server/cmd/api/device_handler.go) |
| `DeleteDevice` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [middleware.go](../server/cmd/api/middleware.go) |
| `DeleteDevicesUnusedSince` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [device_expiry.go](../server/cmd/api/device_expiry.go) |

### [file_fingerprints.sql](../server/sqlc/queries/file_fingerprints.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `UpsertTrackFileFingerprint` | 1 → 0 | `track_id` | Only success was consumed. Use `:execrows`; scanner maps zero affected rows to `sql.ErrNoRows`, preserving missing-catalog failure. [fingerprint.go](../server/cmd/internal/scanner/music/fingerprint.go) |
| `UpsertMovieFileFingerprint` | 1 → 0 | `movie_id` | Only success was consumed. Use `:execrows`; scanner maps zero affected rows to `sql.ErrNoRows`, preserving missing-catalog failure. [fingerprint.go](../server/cmd/internal/scanner/movie/fingerprint.go) |
| `DeleteMovieRemuxSafetyVerdicts` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovieKeyframeIndexes` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |

### [genres.sql](../server/sqlc/queries/genres.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetOrCreateGenre` | 5 → 1 | `tag`, `genre_type`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go), [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `GetGenresByMusicianID` | 2 → 1 | `id` | Detail handlers previously mapped each row to Tag; return tag strings directly, keeping DISTINCT, ordering, and empty arrays. [musician_handler.go](../server/cmd/api/musician_handler.go) |
| `GetAlbumGenres` | 2 → 1 | `id` | Detail handlers previously mapped each row to Tag; return tag strings directly, keeping DISTINCT, ordering, and empty arrays. [album_handler.go](../server/cmd/api/album_handler.go) |

### [keyframe_indexes.sql](../server/sqlc/queries/keyframe_indexes.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetKeyframeIndex` | 7 → 3 | `movie_id`, `stream_index`, `created_at`, `updated_at` | Cache key is already an input. Consumer validates fingerprint and reads payload (duration/keyframes or safe/reason); row timestamps have no cache-validity role. [hls_keyframes.go](../server/cmd/api/hls_keyframes.go) |
| `UpsertKeyframeIndex` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [hls_keyframes.go](../server/cmd/api/hls_keyframes.go) |

### [movie_watch_progress.sql](../server/sqlc/queries/movie_watch_progress.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetMovieWatchProgress` | 8 → 4 | `user_id`, `movie_id`, `save_session_id`, `save_sequence` | Response mapper needs progress, duration, watched and updated time only. User/movie predicates and save-session/sequence write ordering remain stored. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |
| `UpsertMovieWatchProgress` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |
| `DeleteMovieWatchProgress` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |
| `MarkMovieWatched` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |
| `MarkMovieWatchedFromProgress` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |
| `GetContinueWatchingMovies` | 6 → 6 | None | Home watching cards consume movie identity/artwork/title/year and resume position/duration; watched state and freshness still filter/order in SQL. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |
| `MarkMovieUnwatched` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go) |

### [movies.sql](../server/sqlc/queries/movies.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetMovieForDirectStream` | 4 → 4 | None | Existing focused read: path opens the file, name drives serving, MIME sets Content-Type; all three remain necessary. [stream_file.go](../server/cmd/api/stream_file.go) |
| `GetMovieByID` | 26 → 26 | None | Shared by HLS, subtitles, Identify, edit, deletion and technical details. File metadata/fingerprint timestamps and descriptive fields have different internal consumers. Keep shared row; details now use the separate focused query. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go), [movie_handler.go](../server/cmd/api/movie_handler.go), [movie_edit_handler.go](../server/cmd/api/movie_edit_handler.go), [subtitle_handler.go](../server/cmd/api/subtitle_handler.go), [hls_session.go](../server/cmd/api/hls_session.go) |
| `MovieExists` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [watch_progress_handler.go](../server/cmd/api/watch_progress_handler.go), [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go), [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMoviesByTmdbIDs` | 2 → 2 | None | Provider search maps provider ID to local ID for already-in-library status and navigation; retain both identities. [tmdb_handler.go](../server/cmd/api/tmdb_handler.go) |
| `GetMoviesByIDs` | 3 → 3 | None | Batched watch-room list hydration needs movie title/poster by ID; all projected values used. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `GetMovieScanIndex` | 10 → 10 | None | Catalog identity/path, size, filesystem identity and fingerprint values drive skip/hash/probe decisions and missing-file reconciliation; retain every projected value. [lifecycle.go](../server/cmd/internal/scanner/movie/lifecycle.go) |
| `GetLatestMovies` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `UpsertMovie` | 26 → 1 | `title`, `file_path`, `file_name`, `size`, `container`, `mime_type`, `adult`, `tmdb_id`, `imdb_id`, `poster_path`, `backdrop_path`, `language`, `year`, `release_date`, `overview`, `tag_line`, `certification`, `critic_rating`, `audience_rating`, `revenue`, `budget`, `run_time`, `duration`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `UpsertProductionCompany` | 7 → 1 | `name`, `tmdb_id`, `logo`, `country`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `UpsertArtist` | 6 → 1 | `name`, `tmdb_id`, `profile`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `UpsertCast` | 7 → 0 | `id`, `movie_id`, `artist_id`, `character`, `cast_order`, `created_at`, `updated_at` | Scanner discarded the row; keep error propagation and transaction boundaries, use `:exec`. Stored technical/relationship values remain consumed separately. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `UpsertCrew` | 7 → 0 | `id`, `movie_id`, `artist_id`, `job`, `department`, `created_at`, `updated_at` | Scanner discarded the row; keep error propagation and transaction boundaries, use `:exec`. Stored technical/relationship values remain consumed separately. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `CreateMovieProductionCompany` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovieProductionCompanies` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovieVideoStreams` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [streams.go](../server/cmd/internal/scanner/movie/streams.go) |
| `InsertVideoStream` | 26 → 0 | `id`, `movie_id`, `stream_index`, `codec`, `codec_profile`, `codec_level`, `bit_rate`, `width`, `height`, `coded_width`, `coded_height`, `aspect_ratio`, `frame_rate`, `avg_frame_rate`, `bit_depth`, `pixel_format`, `color_range`, `color_space`, `color_primaries`, `color_transfer`, `field_order`, `rotation`, `language`, `title`, `created_at`, `updated_at` | Scanner discarded the row; keep error propagation and transaction boundaries, use `:exec`. Stored technical/relationship values remain consumed separately. [streams.go](../server/cmd/internal/scanner/movie/streams.go) |
| `DeleteMovieAudioStreams` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [streams.go](../server/cmd/internal/scanner/movie/streams.go) |
| `InsertAudioStream` | 14 → 0 | `id`, `movie_id`, `stream_index`, `codec`, `codec_profile`, `bit_rate`, `sample_rate`, `channels`, `channel_layout`, `language`, `title`, `is_default`, `created_at`, `updated_at` | Scanner discarded the row; keep error propagation and transaction boundaries, use `:exec`. Stored technical/relationship values remain consumed separately. [streams.go](../server/cmd/internal/scanner/movie/streams.go) |
| `DeleteMovieSubtitles` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [streams.go](../server/cmd/internal/scanner/movie/streams.go) |
| `InsertSubtitle` | 10 → 0 | `id`, `movie_id`, `stream_index`, `codec`, `language`, `title`, `is_forced`, `is_default`, `created_at`, `updated_at` | Scanner discarded the row; keep error propagation and transaction boundaries, use `:exec`. Stored technical/relationship values remain consumed separately. [streams.go](../server/cmd/internal/scanner/movie/streams.go) |
| `DeleteMovieChapters` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [chapters.go](../server/cmd/internal/scanner/movie/chapters.go) |
| `InsertChapter` | 5 → 0 | `id`, `title`, `start_time`, `thumb`, `movie_id` | Scanner discarded the row; keep error propagation and transaction boundaries, use `:exec`. Stored technical/relationship values remain consumed separately. [chapters.go](../server/cmd/internal/scanner/movie/chapters.go) |
| `CreateMovieGenre` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovieGenres` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `UpsertExtraVideo` | 9 → 1 | `title`, `external_id`, `key`, `type`, `site`, `official`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `CreateMovieExtraVideo` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovieExtraVideos` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `GetCastByMovieID` | 7 → 5 | `movie_id`, `artist_id` | Movie cast cards use cast identity, character/order and artist name/profile. Movie and artist foreign keys are join inputs, not client fields. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetCrewByMovieID` | 7 → 4 | `movie_id`, `artist_id`, `artist_profile` | Movie credit presentation uses identity, job, department and name; it does not render crew portraits or use the joined IDs. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetGenresByMovieID` | 2 → 2 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetProductionCompaniesByMovieID` | 5 → 2 | `tmdb_id`, `logo`, `country` | Movie details renders company names keyed by local ID; provider ID, logo and country have no client consumer. Provider ID remains the stored upsert identity. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMovieExtraVideos` | 9 → 5 | `external_id`, `official`, `created_at`, `updated_at` | Trailer links need ID/title/key/type/site. Provider external_id remains the stored uniqueness key; official flag and row timestamps have no consumer. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `UpdateMovie` | 26 → 0 | `id`, `title`, `file_path`, `file_name`, `size`, `container`, `mime_type`, `adult`, `tmdb_id`, `imdb_id`, `poster_path`, `backdrop_path`, `language`, `year`, `release_date`, `overview`, `tag_line`, `certification`, `critic_rating`, `audience_rating`, `revenue`, `budget`, `run_time`, `duration`, `created_at`, `updated_at` | Edit handler discarded the row. Use `:execrows`; zero affected rows still fail and roll back the edit transaction. [movie_edit_handler.go](../server/cmd/api/movie_edit_handler.go) |
| `DeleteMovieCast` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovieCrew` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `DeleteMovie` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_edit_handler.go](../server/cmd/api/movie_edit_handler.go) |
| `GetVideoStreamsByMovieID` | 26 → 24 | `created_at`, `updated_at` | Keep technical fields for capability checks, HLS mapping, selectors and technical details. Removed row timestamps were never used for playback validity; movie fingerprint data remains. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go), [movie_handler.go](../server/cmd/api/movie_handler.go), [hls_session.go](../server/cmd/api/hls_session.go) |
| `GetAudioStreamsByMovieID` | 14 → 12 | `created_at`, `updated_at` | Keep technical fields for capability checks, HLS mapping, selectors and technical details. Removed row timestamps were never used for playback validity; movie fingerprint data remains. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go), [movie_handler.go](../server/cmd/api/movie_handler.go), [movie_streams_cache.go](../server/cmd/api/movie_streams_cache.go), [hls_session.go](../server/cmd/api/hls_session.go) |
| `GetSubtitlesByMovieID` | 10 → 8 | `created_at`, `updated_at` | Keep technical fields for capability checks, HLS mapping, selectors and technical details. Removed row timestamps were never used for playback validity; movie fingerprint data remains. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go), [movie_handler.go](../server/cmd/api/movie_handler.go), [movie_streams_cache.go](../server/cmd/api/movie_streams_cache.go), [subtitle_handler.go](../server/cmd/api/subtitle_handler.go) |
| `GetChaptersByMovieID` | 5 → 5 | None | Technical details/player uses chapter IDs, times, titles and thumbnails. Parent association remains in existing technical protocol; no storage removal inferred from display alone. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMoviesCount` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMoviesLibraryAsc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMoviesLibraryDesc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMovieGenresWithCounts` | 3 → 3 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `CountMoviesForGenre` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMoviesByGenreAsc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetMoviesByGenreDesc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `DeleteMissingMovie` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [cleanup.go](../server/cmd/internal/scanner/movie/cleanup.go) |
| `GetMovieByPath` | 26 → 3 | `title`, `file_name`, `size`, `container`, `mime_type`, `adult`, `imdb_id`, `poster_path`, `backdrop_path`, `language`, `year`, `release_date`, `overview`, `tag_line`, `certification`, `critic_rating`, `audience_rating`, `revenue`, `budget`, `run_time`, `duration`, `created_at`, `updated_at` | Resolution and commit refresh consume only ID, stored path, and observed TMDB identity to reject stale/deleted/reidentified work. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go), [resolution.go](../server/cmd/internal/scanner/movie/resolution.go), [fingerprint.go](../server/cmd/internal/scanner/movie/fingerprint.go) |
| `MarkMovieTmdbRetry` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `ClearMovieTmdbRetry` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `CountMovieTmdbRetries` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [lifecycle.go](../server/cmd/internal/scanner/movie/lifecycle.go) |
| `UpdateMovieTmdbMetadata` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |
| `HasMovieTmdbRetry` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [persistence.go](../server/cmd/internal/scanner/movie/persistence.go) |

### [music_metadata.sql](../server/sqlc/queries/music_metadata.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `FindMusicArtistIdentity` | 10 → 3 | `name`, `sort_name`, `summary`, `spotify_popularity`, `spotify_followers`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and thumbnail comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [resolution.go](../server/cmd/internal/scanner/music/resolution.go) |
| `SaveMusicArtistIdentity` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `FindMusicAlbumIdentity` | 12 → 3 | `title`, `sort_title`, `spotify_popularity`, `musician`, `release_date`, `year`, `total_tracks`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and cover comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [resolution.go](../server/cmd/internal/scanner/music/resolution.go) |
| `SaveMusicAlbumIdentity` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `FindMusicGenreIdentity` | 5 → 1 | `tag`, `genre_type`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `SaveMusicGenreIdentity` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `SaveMusicTrackMetadata` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `DeleteMusicCreditMetadata` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `SaveMusicCreditMetadata` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `SaveMusicAlbumDate` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `ReconcileMusicArtistSort` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go), [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [retry.go](../server/cmd/internal/scanner/music/retry.go), [cleanup.go](../server/cmd/internal/scanner/music/cleanup.go) |
| `ReconcileMusicAlbumSort` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `ReconcileMusicAlbumDate` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `ReconcileMusicAlbumYear` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `MusicTrackAffectedArtists` | 1 → 1 | None | IDs select affected reconciliation owners or post-commit cache invalidation; retain relationship lookup and values. [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [cleanup.go](../server/cmd/internal/scanner/music/cleanup.go) |
| `MusicTrackAffectedAlbum` | 1 → 1 | None | IDs select affected reconciliation owners or post-commit cache invalidation; retain relationship lookup and values. [persistence.go](../server/cmd/internal/scanner/music/persistence.go), [cleanup.go](../server/cmd/internal/scanner/music/cleanup.go) |
| `DeleteMusicArtistSpotifyGenres` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `SaveMusicArtistSpotifyGenre` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `DeleteMusicAlbumSpotifyGenres` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `SaveMusicAlbumSpotifyGenre` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [relationships.go](../server/cmd/internal/scanner/music/relationships.go) |
| `MoveMusicArtistAliases` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MoveMusicArtistTracks` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MoveMusicArtistGenres` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `DeleteMergedMusicArtist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MusicArtistRetryCandidates` | 10 → 2 | `sort_name`, `summary`, `spotify_id`, `spotify_popularity`, `spotify_followers`, `thumb`, `created_at`, `updated_at` | Retry loop pages by ID and resolves local name. Spotify absence/status and track references remain SQL filters; other artist metadata is not consumed. [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `MoveMusicAlbumAliases` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MoveMusicAlbumTracks` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MoveMusicAlbumGenres` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `DeleteMergedMusicAlbum` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MusicAlbumRetryCandidates` | 12 → 3 | `sort_title`, `spotify_id`, `spotify_popularity`, `release_date`, `year`, `total_tracks`, `cover`, `created_at`, `updated_at` | Retry loop pages by ID and resolves local title/musician. Other metadata is reloaded through focused identity reads or written after resolution. [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `MoveMusicArtistCredits` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MoveMusicArtistContributions` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MoveMusicAlbumFallback` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `DeleteMergedMusicMatch` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MusicArtistTrackMetadata` | 5 → 3 | `artist_key`, `album_sort` | Compound-credit repair reads track ID, original artist tag and artist sort only. Artist identity key and album sort remain stored for separate reconciliation/identity uses. [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `UpdateMusicTrackPrimaryArtist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `MusicArtistTrackIDs` | 1 → 1 | None | IDs select affected reconciliation owners or post-commit cache invalidation; retain relationship lookup and values. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `MusicAlbumTrackIDs` | 1 → 1 | None | IDs select affected reconciliation owners or post-commit cache invalidation; retain relationship lookup and values. [merges.go](../server/cmd/internal/scanner/music/merges.go) |
| `UpdateMusicArtistEnrichment` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `UpdateMusicAlbumEnrichment` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `MusicCompoundReconciliationCandidates` | 10 → 1 | `name`, `sort_name`, `summary`, `spotify_id`, `spotify_popularity`, `spotify_followers`, `thumb`, `created_at`, `updated_at` | Repair needs only candidate IDs. Name/sort/provider/presentation fields do not reach the repair loop; status/reason filters remain. [retry.go](../server/cmd/internal/scanner/music/retry.go) |
| `SetMusicArtistSpotifyID` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `SetMusicAlbumSpotifyID` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |

### [music_spotify_matches.sql](../server/sqlc/queries/music_spotify_matches.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetMusicSpotifyMatch` | 13 → 2 | `entity_type`, `entity_id`, `spotify_id`, `score`, `threshold_value`, `candidate_name`, `candidate_artist`, `search_query`, `strategy`, `error`, `updated_at` | Resolution/retry uses status and reason. Entity key is input; catalog owns Spotify ID. Other outcome diagnostics had only write/test uses and are removed from storage. [resolution.go](../server/cmd/internal/scanner/music/resolution.go) |
| `UpsertMusicSpotifyMatch` | 0 → 0 | None | Keep status/reason write and entity-key conflict update; remove duplicate provider ID and unused candidate/search/score/error/timestamp bookkeeping parameters. [spotify.go](../server/cmd/internal/scanner/music/spotify.go) |

### [musicians.sql](../server/sqlc/queries/musicians.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetMusicianBySpotifyID` | 10 → 3 | `name`, `sort_name`, `summary`, `spotify_popularity`, `spotify_followers`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and thumbnail comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `UpdateMusicianSpotifyThumb` | 10 → 3 | `name`, `sort_name`, `summary`, `spotify_popularity`, `spotify_followers`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and thumbnail comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `UpsertMusician` | 10 → 3 | `name`, `sort_name`, `summary`, `spotify_popularity`, `spotify_followers`, `created_at`, `updated_at` | Music resolution/persistence needs ID, Spotify identity, and thumbnail comparison. Other metadata is written or read by detail endpoints separately; preserve changed-image timestamp behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `GetMusiciansByAlbumID` | 3 → 3 | None | Detail endpoint exposes catalog presentation/provider metadata; delete/identity logic also uses relevant IDs. Preserve broad detail feature where downstream use is unresolved; see retained uncertainties. [album_handler.go](../server/cmd/api/album_handler.go) |
| `GetMusiciansAlphabetical` | 6 → 5 | `sort_name` | Musician cards need ID/name/thumb and album/track counts. sort_name still drives SQL ordering and search but is not returned. [musician_handler.go](../server/cmd/api/musician_handler.go) |
| `GetMusicianByID` | 10 → 10 | None | Detail endpoint exposes catalog presentation/provider metadata; delete/identity logic also uses relevant IDs. Preserve broad detail feature where downstream use is unresolved; see retained uncertainties. [musician_handler.go](../server/cmd/api/musician_handler.go) |
| `GetAlbumsByMusicianID` | 6 → 6 | None | Detail endpoint exposes catalog presentation/provider metadata; delete/identity logic also uses relevant IDs. Preserve broad detail feature where downstream use is unresolved; see retained uncertainties. [musician_handler.go](../server/cmd/api/musician_handler.go) |
| `GetTracksByMusicianID` | 12 → 8 | `sort_title`, `file_path`, `track_index`, `disc` | Track rows/queue use display, duration, quality and album identity/artwork. Ordering still uses sort_title/disc/index; remove those unused result values and internal path. [musician_handler.go](../server/cmd/api/musician_handler.go) |

### [notifications.sql](../server/sqlc/queries/notifications.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `CreateNotification` | 7 → 0 | `id`, `created_by_user_id`, `title`, `message`, `is_admin`, `created_at`, `updated_at` | All three request dialogs branch on success/error and close/invalidate, without consuming a created object. Insert is `:exec`; HTTP 201 returns `{error:false}` without data. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |
| `ListNotificationsForUser` | 9 → 7 | `created_by_user_id`, `updated_at` | Notification bell uses title/message, sender name, created time, read state and ID for actions. Creator ID still joins users; remove it and unused updated time from results. Admin visibility remains enforced. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |
| `CountUnreadNotificationsForUser` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |
| `MarkNotificationReadForUser` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |
| `MarkAllNotificationsReadForUser` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |
| `DeleteNotificationForUser` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |
| `GetNotificationBadgeForUser` | 2 → 1 | `is_admin` | Handler serializes only unread count. Keep admin short-circuit and stale-user no-row semantics in SQL; remove the returned admin flag. [notifications_handler.go](../server/cmd/api/notifications_handler.go) |

### [playlist_collaborators.sql](../server/sqlc/queries/playlist_collaborators.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `AddCollaborator` | 6 → 6 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `RemoveCollaborator` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `GetPlaylistCollaborators` | 8 → 8 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go), [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |

### [playlist_movies.sql](../server/sqlc/queries/playlist_movies.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `AddMovieToPlaylist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |
| `RemoveMovieFromPlaylist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |
| `CountPlaylistMovies` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |
| `GetPlaylistMoviesPaginatedAsc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |
| `GetPlaylistMoviesPaginatedDesc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |

### [playlist_tracks.sql](../server/sqlc/queries/playlist_tracks.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `AddTrackToPlaylist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `RemoveTrackFromPlaylist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `GetPlaylistTracksInfinite` | 15 → 14 | `file_path` | Track display/queue conversion uses identity, display metadata, duration and available quality/album/artist fields; streaming URLs use ID, never filesystem path. Retain playlist/statistics fields under the documented endpoint feature; see uncertainties. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `CountPlaylistTracks` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `GetPlaylistTrackSummary` | 2 → 2 | None | Both track count and total duration are serialized for playlist details and rendered in the playlist header. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `UpdateTrackPosition` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |

### [playlists.sql](../server/sqlc/queries/playlists.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `CreatePlaylist` | 10 → 10 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `GetPlaylistWithAccess` | 11 → 11 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [playlist_handler.go](../server/cmd/api/playlist_handler.go) |
| `GetPlaylistsWithCollaboratorAccess` | 14 → 14 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `UpdatePlaylist` | 10 → 10 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `DeletePlaylist` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go), [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `UpdatePlaylistTimestamp` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go), [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `CreateMoviePlaylist` | 10 → 10 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |
| `UpdateMoviePlaylist` | 10 → 10 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |
| `GetMoviePlaylistsWithCollaboratorAccess` | 13 → 13 | None | Playlist ownership, visibility, edit permissions, content type and related identities control access/features. Lists/details and mutation responses expose metadata; update timestamps order lists. Retain remaining broader metadata pending resolution; see uncertainties. [movie_playlist_handler.go](../server/cmd/api/movie_playlist_handler.go) |

### [remux_safety_verdicts.sql](../server/sqlc/queries/remux_safety_verdicts.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetRemuxSafetyVerdict` | 7 → 3 | `movie_id`, `stream_index`, `created_at`, `updated_at` | Cache key is already an input. Consumer validates fingerprint and reads payload (duration/keyframes or safe/reason); row timestamps have no cache-validity role. [hls_remux_safety.go](../server/cmd/api/hls_remux_safety.go) |
| `UpsertRemuxSafetyVerdict` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [hls_remux_safety.go](../server/cmd/api/hls_remux_safety.go) |

### [settings.sql](../server/sqlc/queries/settings.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetSettings` | 19 → 17 | `created_at`, `updated_at` | Settings manager publishes the complete configuration snapshot to startup/scanners/integrations/playback. Row creation/update timestamps never participate in configuration or responses; remove storage and maintenance. [startup.go](../server/cmd/api/startup.go) |
| `CreateSettings` | 19 → 17 | `created_at`, `updated_at` | Settings manager publishes the complete configuration snapshot to startup/scanners/integrations/playback. Row creation/update timestamps never participate in configuration or responses; remove storage and maintenance. [startup.go](../server/cmd/api/startup.go) |
| `UpdateGeneralSettings` | 19 → 17 | `created_at`, `updated_at` | Settings manager publishes the complete configuration snapshot to startup/scanners/integrations/playback. Row creation/update timestamps never participate in configuration or responses; remove storage and maintenance. [settings_handler.go](../server/cmd/api/settings_handler.go) |
| `UpdateLibrarySettings` | 19 → 17 | `created_at`, `updated_at` | Settings manager publishes the complete configuration snapshot to startup/scanners/integrations/playback. Row creation/update timestamps never participate in configuration or responses; remove storage and maintenance. [settings_handler.go](../server/cmd/api/settings_handler.go) |
| `UpdatePlaybackServerSettings` | 19 → 17 | `created_at`, `updated_at` | Settings manager publishes the complete configuration snapshot to startup/scanners/integrations/playback. Row creation/update timestamps never participate in configuration or responses; remove storage and maintenance. [playback_settings_handler.go](../server/cmd/api/playback_settings_handler.go) |

### [track_genres.sql](../server/sqlc/queries/track_genres.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `CreateTrackGenre` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `DeleteTrackGenresExcept` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `DeleteTrackGenres` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `GetGenresByAlbumID` | 3 → 2 | `genre_id` | Album track grouping needs track_id and tag only; genre ID is neither grouped nor rendered. [album_handler.go](../server/cmd/api/album_handler.go) |

### [track_musicians.sql](../server/sqlc/queries/track_musicians.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `CreateTrackMusician` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `DeleteTrackMusicians` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `DeleteTrackMusiciansExcept` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |

### [tracks.sql](../server/sqlc/queries/tracks.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetTrack` | 25 → 24 | `file_path` | Dedicated track-details API retains technical/tag fields to preserve that feature; no current web fetch proves a narrower shape safe. File path moves to the separate streaming query and stays internal. See retained uncertainties. [track_handler.go](../server/cmd/api/track_handler.go) |
| `TrackExists` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [track_handler.go](../server/cmd/api/track_handler.go), [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `ListMusicTrackScanIndex` | 8 → 8 | None | Catalog identity/path, size, filesystem identity and fingerprint values drive skip/hash/probe decisions and missing-file reconciliation; retain every projected value. [lifecycle.go](../server/cmd/internal/scanner/music/lifecycle.go) |
| `UpsertTrack` | 25 → 1 | `title`, `sort_title`, `file_path`, `file_name`, `container`, `mime_type`, `codec`, `size`, `track_index`, `duration`, `disc`, `channels`, `channel_layout`, `bit_rate`, `profile`, `release_date`, `year`, `composer`, `copyright`, `language`, `album_id`, `musician_id`, `created_at`, `updated_at` | Persistence, relationship writes, and scan caches need only the returned identity. Return scalar ID; retain conflict keys and write behavior. [persistence.go](../server/cmd/internal/scanner/music/persistence.go) |
| `GetTracksByAlbumID` | 25 → 11 | `sort_title`, `file_path`, `file_name`, `container`, `size`, `channels`, `profile`, `release_date`, `year`, `composer`, `copyright`, `language`, `created_at`, `updated_at` | Album track rows and queue consume the eleven retained values, including MIME for AudioPlayer, index/disc ordering display and channel/bitrate quality. Technical/tag details remain stored and available from track details. [album_handler.go](../server/cmd/api/album_handler.go) |
| `GetTracksAlphabetical` | 11 → 10 | `file_path` | Track display/queue conversion uses identity, display metadata, duration and available quality/album/artist fields; streaming URLs use ID, never filesystem path. Retain playlist/statistics fields under the documented endpoint feature; see uncertainties. [track_handler.go](../server/cmd/api/track_handler.go) |
| `GetTracksCount` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [track_handler.go](../server/cmd/api/track_handler.go) |
| `GetAlbumsCount` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [album_handler.go](../server/cmd/api/album_handler.go) |
| `GetMusiciansCount` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [musician_handler.go](../server/cmd/api/musician_handler.go) |
| `GetMusicLibraryCounts` | 3 → 3 | None | Dashboard uses all three library counts; retain combined query. [track_handler.go](../server/cmd/api/track_handler.go) |
| `GetRandomTracks` | 11 → 10 | `file_path` | Track display/queue conversion uses identity, display metadata, duration and available quality/album/artist fields; streaming URLs use ID, never filesystem path. Retain playlist/statistics fields under the documented endpoint feature; see uncertainties. [track_handler.go](../server/cmd/api/track_handler.go) |
| `DeleteMissingTrack` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [cleanup.go](../server/cmd/internal/scanner/music/cleanup.go) |

### [user_liked_movies.sql](../server/sqlc/queries/user_liked_movies.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `LikeMovie` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `UnlikeMovie` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `IsMovieLiked` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `CountUserLikedMovies` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetLikedMoviesForUserAsc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |
| `GetLikedMoviesForUserDesc` | 5 → 5 | None | Already projects card/genre identity and displayed labels/artwork/year/certification or counts; preserve SQL pagination, filtering and order. [movie_handler.go](../server/cmd/api/movie_handler.go) |

### [user_liked_tracks.sql](../server/sqlc/queries/user_liked_tracks.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `LikeTrack` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [track_handler.go](../server/cmd/api/track_handler.go) |
| `UnlikeTrack` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [track_handler.go](../server/cmd/api/track_handler.go) |
| `GetLikedTrackIDsByUserID` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [track_handler.go](../server/cmd/api/track_handler.go) |
| `GetLikedTracksForUser` | 11 → 10 | `file_path` | Track display/queue conversion uses identity, display metadata, duration and available quality/album/artist fields; streaming URLs use ID, never filesystem path. Retain playlist/statistics fields under the documented endpoint feature; see uncertainties. [track_handler.go](../server/cmd/api/track_handler.go) |
| `CountUserLikedTracks` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [track_handler.go](../server/cmd/api/track_handler.go) |

### [user_stats.sql](../server/sqlc/queries/user_stats.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `RecordPlayEvent` | 0 → 0 | None | No result row. Keep accepted duration/completed recording semantics and history timestamps; completed is retained as part of the recorded play-event feature, not treated as display-only data. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `UpsertUserTrackStats` | 0 → 0 | None | No result row. Aggregate count/time and last-played time are consumed by statistics; first-played/updated timestamps had no read or ordering use and are removed. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `GetUserTopTracks` | 12 → 11 | `file_path` | Track display/queue conversion uses identity, display metadata, duration and available quality/album/artist fields; streaming URLs use ID, never filesystem path. Retain playlist/statistics fields under the documented endpoint feature; see uncertainties. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `GetUserTopMusicians` | 6 → 6 | None | Statistics endpoint consumes aggregate counts/time and entity display fields. Preserve independent statistics feature even where the web lacks a current display for every metric; see uncertainties. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `GetUserTopGenres` | 5 → 5 | None | Statistics endpoint consumes aggregate counts/time and entity display fields. Preserve independent statistics feature even where the web lacks a current display for every metric; see uncertainties. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `GetUserListeningStats` | 4 → 4 | None | Statistics endpoint consumes aggregate counts/time and entity display fields. Preserve independent statistics feature even where the web lacks a current display for every metric; see uncertainties. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `GetUserRecentlyPlayed` | 11 → 10 | `file_path` | Track display/queue conversion uses identity, display metadata, duration and available quality/album/artist fields; streaming URLs use ID, never filesystem path. Retain playlist/statistics fields under the documented endpoint feature; see uncertainties. [stats_handler.go](../server/cmd/api/stats_handler.go) |
| `GetUserTopAlbums` | 8 → 8 | None | Statistics endpoint consumes aggregate counts/time and entity display fields. Preserve independent statistics feature even where the web lacks a current display for every metric; see uncertainties. [stats_handler.go](../server/cmd/api/stats_handler.go) |

### [users.sql](../server/sqlc/queries/users.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `GetUser` | 9 → 9 | None | Shared authentication/profile/admin handlers use password verification, identity, role, PIN presence and avatar cleanup; retain the shared row and sanitized response mapping. [auth_handler.go](../server/cmd/api/auth_handler.go), [admin_user_handler.go](../server/cmd/api/admin_user_handler.go), [user_handler.go](../server/cmd/api/user_handler.go) |
| `GetUserIsAdmin` | 1 → 1 | None | Focused privilege/PIN check; scalar result is used for authorization or verification, never inferred from fixtures. [notifications_handler.go](../server/cmd/api/notifications_handler.go), [middleware.go](../server/cmd/api/middleware.go) |
| `UserExists` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [track_playlist_handler.go](../server/cmd/api/track_playlist_handler.go) |
| `GetUserPin` | 1 → 1 | None | Focused privilege/PIN check; scalar result is used for authorization or verification, never inferred from fixtures. [user_pin_handler.go](../server/cmd/api/user_pin_handler.go) |
| `GetAdminUser` | 9 → 1 | `name`, `email`, `password`, `is_admin`, `avatar`, `pin`, `created_at`, `updated_at` | Startup checks only admin existence; scalar ID preserves sql.ErrNoRows without copying profile/password/PIN. [startup.go](../server/cmd/api/startup.go) |
| `GetUserByEmail` | 9 → 9 | None | Shared authentication/profile/admin handlers use password verification, identity, role, PIN presence and avatar cleanup; retain the shared row and sanitized response mapping. [auth_handler.go](../server/cmd/api/auth_handler.go) |
| `CreateUser` | 9 → 8 | `password` | Profile response mapping consumes returned profile fields and PIN nullness, never password hash. Keep PIN validity representation and omit password from RETURNING. [admin_user_handler.go](../server/cmd/api/admin_user_handler.go), [startup.go](../server/cmd/api/startup.go) |
| `UpdateUserName` | 9 → 8 | `password` | Profile response mapping consumes returned profile fields and PIN nullness, never password hash. Keep PIN validity representation and omit password from RETURNING. [user_handler.go](../server/cmd/api/user_handler.go) |
| `UpdateUserEmail` | 9 → 8 | `password` | Profile response mapping consumes returned profile fields and PIN nullness, never password hash. Keep PIN validity representation and omit password from RETURNING. [user_handler.go](../server/cmd/api/user_handler.go) |
| `UpdateUserPassword` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [admin_user_handler.go](../server/cmd/api/admin_user_handler.go), [user_handler.go](../server/cmd/api/user_handler.go) |
| `UpdateUserPin` | 9 → 8 | `password` | Profile response mapping consumes returned profile fields and PIN nullness, never password hash. Keep PIN validity representation and omit password from RETURNING. [user_pin_handler.go](../server/cmd/api/user_pin_handler.go) |
| `UpdateUserAvatar` | 9 → 8 | `password` | Profile response mapping consumes returned profile fields and PIN nullness, never password hash. Keep PIN validity representation and omit password from RETURNING. [user_handler.go](../server/cmd/api/user_handler.go) |
| `DeleteUser` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [admin_user_handler.go](../server/cmd/api/admin_user_handler.go), [user_handler.go](../server/cmd/api/user_handler.go) |
| `GetAllUsers` | 8 → 8 | None | Admin management lists profiles and PIN presence; password/PIN contents already excluded. Profile timestamps retained under existing contract; see uncertainties. [admin_user_handler.go](../server/cmd/api/admin_user_handler.go) |
| `AdminUpdateUser` | 9 → 8 | `password` | Profile response mapping consumes returned profile fields and PIN nullness, never password hash. Keep PIN validity representation and omit password from RETURNING. [admin_user_handler.go](../server/cmd/api/admin_user_handler.go) |
| `CountAdmins` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [admin_user_handler.go](../server/cmd/api/admin_user_handler.go) |
| `GetUsersExcluding` | 4 → 4 | None | User picker consumes identity, name/email search labels and avatar, with current user excluded in SQL. [user_list_handler.go](../server/cmd/api/user_list_handler.go) |

### [watch_rooms.sql](../server/sqlc/queries/watch_rooms.sql)

| Query | Columns before → after | Unused results removed | Consumer evidence and decision |
| --- | --- | --- | --- |
| `CreateWatchRoom` | 12 → 11 | `updated_at` | Room lifecycle/media authorization retains owner/movie/mode, pinned audio/subtitle identities and ordinals; WebSocket join also needs member presence fields. Stored updated_at was unused; live playback state timestamps remain. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `AddWatchRoomMember` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `GetWatchRoomByID` | 12 → 1 | `id`, `movie_id`, `playback_mode`, `audio_track`, `subtitle_track`, `audio_stream_index`, `audio_language`, `subtitle_stream_index`, `subtitle_language`, `created_at`, `updated_at` | Join path checks existence; delete checks owner. Return only owner_user_id and preserve missing-room behavior. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `GetWatchRoomsForUser` | 12 → 5 | `audio_track`, `subtitle_track`, `audio_stream_index`, `audio_language`, `subtitle_stream_index`, `subtitle_language`, `updated_at` | List mapper uses ID, owner, movie, mode and created time. Media pins are read by member-authorized playback queries instead. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `GetWatchRoomMembers` | 3 → 3 | None | Presence summaries need ID/name/avatar and batch grouping needs room ID. Membership created time still orders participants. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `GetWatchRoomMembersByRoomIDs` | 4 → 4 | None | Presence summaries need ID/name/avatar and batch grouping needs room ID. Membership created time still orders participants. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `GetWatchRoomForMember` | 12 → 11 | `updated_at` | Room lifecycle/media authorization retains owner/movie/mode, pinned audio/subtitle identities and ordinals; WebSocket join also needs member presence fields. Stored updated_at was unused; live playback state timestamps remain. [watch_room_auth.go](../server/cmd/api/watch_room_auth.go) |
| `GetWatchRoomForMemberWithSummary` | 14 → 13 | `updated_at` | Room lifecycle/media authorization retains owner/movie/mode, pinned audio/subtitle identities and ordinals; WebSocket join also needs member presence fields. Stored updated_at was unused; live playback state timestamps remain. [watch_room_ws.go](../server/cmd/api/watch_room_ws.go) |
| `IsWatchRoomMember` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `DeleteWatchRoom` | 0 → 0 | None | No returned row to trim. Preserve the mutation, predicates, constraints and affected-row/error semantics used by the caller. [watch_room_auth.go](../server/cmd/api/watch_room_auth.go) |
| `CountUsersByIDs` | 1 → 1 | None | Scalar identity/count/existence value is consumed for validation, pagination, retry progress or feature state; retain. [watch_room_handler.go](../server/cmd/api/watch_room_handler.go) |
| `ListWatchRoomIDsByMovieID` | 1 → 1 | None | Movie cleanup invalidates each affected room runtime cache only after commit; keep IDs. [cleanup.go](../server/cmd/internal/scanner/movie/cleanup.go) |

### Added focused reads

| Query | Columns | Consumer evidence |
| --- | --- | --- |
| `GetTrackForDirectStream` | `file_path`, `file_name`, `mime_type` (3, replacing a 25-column read) | `stream_file.go` opens the path and preserves filename/MIME, range serving and cache behavior. |
| `GetMovieDetails` | 19 descriptive/identity/duration fields (replacing a 26-column read) | `movie_handler.go` details response; internal HLS/edit/technical callers keep their required metadata. |

## Raw SQL and schema review

| Location | Review and result |
| --- | --- |
| [search_handler.go](../server/cmd/api/search_handler.go) | Reviewed four FTS count queries and four ranked page queries, including Scan destinations and entity adapters. Track pages omit file_path; musician pages omit sort_name from SELECT/Scan. Movie/album pages already return card fields. FTS matching, ranking, counts, stable order and pagination stay intact. |
| [search_vocab.go](../server/cmd/api/search_vocab.go) | Both generation reads are necessary for cached lookup and transactional snapshot validation. Vocabulary SELECT term/doc (for each of four allowlisted vocabularies) supplies token matching and frequency ranking; the extra bounded row detects truncation. Retain. |
| [startup.go](../server/cmd/api/startup.go) | WAL/configuration PRAGMAs, schema application and analysis/optimize operations consume side effects. No unused row payload to remove. No live schema application or reset was performed during the audit. |
| [session_store_cache.go](../server/cmd/api/session_store_cache.go) and the pinned scs/sqlite3store dependency | Session Find returns data filtered by token/expiry; All returns token/data; Commit/Delete/expiry cleanup mutate storage. Token/data/expiry are all auth/cache/expiry dependencies. Dependency SQL was inspected locally and left intact. |
| [schema.sql](../server/sqlc/schema.sql) | Reviewed foreign keys, CHECK/unique constraints, indexes, FTS synchronization/generation triggers and music relationship/sort/date maintenance. Stored-column deletions above do not remove referenced constraint/order/identity fields. Relationship provenance, timestamp ordering on catalog/playlist/member/notification/history rows, and session expiry remain. |

## Validation

- `make generate` passed with the existing sqlc v1.31.1 binary; `make generate-openapi` passed. Generated Go and TypeScript were regenerated, not edited by hand.
- `make check` passed: OpenAPI lint and route coverage, generated-contract consistency, server vet/deadcode checks and full tests, frontend lint, **83 test files / 746 tests**, TypeScript compilation and production build. The standalone `make test-server` also passed.
- Because `check-openapi` compares generated output with the Git index, it ran with a temporary alternate index containing the newly generated TypeScript snapshot. Regeneration produced no drift; the real Git index was not staged or changed.
- Focused database/scanner coverage verifies insertion and conflict updates, metadata/identity preservation, constraints, transaction rollback, cache publication, and both fingerprint writers' missing-catalog `sql.ErrNoRows` behavior. The movie-edit regression test verifies that zero affected rows fail with HTTP 500 and roll back the transaction. Existing device authentication/expiry, notification creation/list/read-state and OpenAPI response tests passed with the narrowed rows. Notification creation explicitly asserts absent data and checks the persisted request.
- Chromium mock-API browser suites for album details, musician details, movie details, movie player and music tracks: **28 passed, 2 failed**. Both failures are the existing `direct playback waits for preferences before requesting media` and `HLS playback waits for preferences before requesting media` cases at `web/e2e/movie-player.spec.ts:75`. They time out waiting for “Preparing playback...”. Both failures reproduced with the same assertion on a separate untouched archive of HEAD, using separate ports. The current playback hook only waits for the server catalog when device download-speed preferences require it; these cases do not establish that condition. No unrelated playback behavior or assertions were changed.
- Browser checks used the repository mock API, not a live library. Real browser codec combinations, macOS ARM64 and hardware decoding/encoding paths were not exercised. FFmpeg arguments, media formats and capability rules were not changed.
- `git diff --check` passed. No dependency changes, migrations, database resets or live-library writes were performed.

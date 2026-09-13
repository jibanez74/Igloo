-- name: GetShow :one
SELECT * FROM shows WHERE id = ?;

-- name: GetLatestShows :many
-- created_at is written once by UpsertLocalShow, so it orders by first discovery.
-- CURRENT_TIMESTAMP only has second resolution, so id breaks the ties a bulk scan creates.
SELECT id, name, poster_path, premiere_year FROM shows ORDER BY created_at DESC, id DESC LIMIT 12;

-- name: UpsertLocalShow :one
INSERT INTO shows (directory_path, local_name, premiere_year, name) VALUES (?, ?, ?, ?) ON CONFLICT (directory_path) DO UPDATE SET id = shows.id RETURNING *;

-- name: UpdateShowMetadata :exec
UPDATE shows SET name = ?, tmdb_id = ?, imdb_id = ?, original_name = ?, overview = ?, tagline = ?, language = ?, origin_countries = ?, first_air_date = ?, last_air_date = ?, status = ?, type = ?, adult = ?, poster_path = ?, backdrop_path = ?, homepage = ?, vote_average = ?, vote_count = ?, popularity = ?, certification = ?, tmdb_season_count = ?, tmdb_episode_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: MarkShowRetry :exec
INSERT INTO show_tmdb_retries (show_id) VALUES (?) ON CONFLICT (show_id) DO UPDATE SET attempts = 0, last_attempt_at = NULL;

-- name: RecordShowTmdbMiss :exec
INSERT INTO show_tmdb_retries (show_id, attempts, last_attempt_at) VALUES (?, 1, ?)
ON CONFLICT (show_id) DO UPDATE SET attempts = show_tmdb_retries.attempts + 1, last_attempt_at = excluded.last_attempt_at;

-- name: ClearShowRetry :exec
DELETE FROM show_tmdb_retries WHERE show_id = ?;

-- name: GetShowSeason :one
SELECT * FROM show_seasons WHERE id = ?;

-- name: UpsertLocalShowSeason :one
INSERT INTO show_seasons (show_id, season_number, name) VALUES (?, ?, ?) ON CONFLICT (show_id, season_number) DO UPDATE SET id = show_seasons.id RETURNING *;

-- name: UpdateShowSeasonMetadata :exec
UPDATE show_seasons SET name = ?, tmdb_id = ?, overview = ?, air_date = ?, poster_path = ?, vote_average = ?, tmdb_episode_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: MarkShowSeasonRetry :exec
INSERT INTO show_season_tmdb_retries (season_id) VALUES (?) ON CONFLICT DO NOTHING;

-- name: ClearShowSeasonRetry :exec
DELETE FROM show_season_tmdb_retries WHERE season_id = ?;

-- name: GetShowEpisode :one
SELECT * FROM show_episodes WHERE id = ?;

-- name: UpsertLocalShowEpisode :one
INSERT INTO show_episodes (season_id, episode_number, name) VALUES (?, ?, ?) ON CONFLICT (season_id, episode_number) DO UPDATE SET id = show_episodes.id RETURNING *;

-- name: UpdateShowEpisodeMetadata :exec
UPDATE show_episodes SET name = ?, tmdb_id = ?, overview = ?, air_date = ?, still_path = ?, production_code = ?, tmdb_runtime = ?, vote_average = ?, vote_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: MarkShowEpisodeRetry :exec
INSERT INTO show_episode_tmdb_retries (episode_id) VALUES (?) ON CONFLICT DO NOTHING;

-- name: ClearShowEpisodeRetry :exec
DELETE FROM show_episode_tmdb_retries WHERE episode_id = ?;

-- name: GetShowSeasons :many
SELECT * FROM show_seasons WHERE show_id = ? ORDER BY season_number;

-- name: GetShowEpisodes :many
SELECT * FROM show_episodes WHERE season_id = ? ORDER BY episode_number;

-- name: GetPendingShows :many
SELECT s.* FROM shows s WHERE s.id > sqlc.arg(after_id) AND (
 EXISTS (SELECT 1 FROM show_tmdb_retries r WHERE r.show_id = s.id) OR
 EXISTS (SELECT 1 FROM show_seasons se JOIN show_season_tmdb_retries r ON r.season_id = se.id WHERE se.show_id = s.id) OR
 EXISTS (SELECT 1 FROM show_seasons se JOIN show_episodes e ON e.season_id = se.id JOIN show_episode_tmdb_retries r ON r.episode_id = e.id WHERE se.show_id = s.id))
 ORDER BY s.id LIMIT 100;

-- name: GetShowRetry :one
SELECT attempts, last_attempt_at FROM show_tmdb_retries WHERE show_id = ?;

-- name: GetShowPendingSeasonIDs :many
SELECT r.season_id FROM show_season_tmdb_retries r JOIN show_seasons se ON se.id = r.season_id WHERE se.show_id = ?;

-- name: GetShowPendingEpisodeIDs :many
SELECT r.episode_id FROM show_episode_tmdb_retries r JOIN show_episodes e ON e.id = r.episode_id JOIN show_seasons se ON se.id = e.season_id WHERE se.show_id = ?;

-- name: CountShowRetries :one
SELECT (SELECT COUNT(*) FROM show_tmdb_retries) + (SELECT COUNT(*) FROM show_season_tmdb_retries) + (SELECT COUNT(*) FROM show_episode_tmdb_retries);

-- name: GetShowFileByPath :one
SELECT * FROM show_files WHERE file_path = ?;

-- name: UpsertShowFile :one
INSERT INTO show_files (season_id, file_path, file_name, size, container, mime_type, duration) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (file_path) DO UPDATE SET size = excluded.size, container = excluded.container, mime_type = excluded.mime_type, duration = excluded.duration, updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: LinkShowEpisodeFile :exec
INSERT INTO show_episode_files (episode_id, file_id, season_id, episode_order) VALUES (?, ?, ?, ?);

-- name: DeleteShowFileLinks :exec
DELETE FROM show_episode_files WHERE file_id = ?;

-- name: GetShowScanEpisodeLinks :many
SELECT file_id, episode_id FROM show_episode_files ORDER BY file_id, episode_order;

-- name: UpsertShowFingerprint :exec
INSERT INTO show_file_fingerprints (file_id, mtime_ns, ctime_ns, device, inode) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (file_id) DO UPDATE SET mtime_ns = excluded.mtime_ns, ctime_ns = excluded.ctime_ns, device = excluded.device, inode = excluded.inode;

-- name: GetShowScanIndex :many
SELECT f.*, fp.mtime_ns, fp.ctime_ns, fp.device, fp.inode FROM show_files f LEFT JOIN show_file_fingerprints fp ON fp.file_id = f.id;

-- name: DeleteMissingShowFile :one
DELETE FROM show_files WHERE id = ? AND file_path = ? RETURNING season_id;

-- Pruning is scoped to the season a file change touched: cascades leave the
-- catalog rows behind, and a full-table NOT EXISTS per file did not scale.
-- name: PruneShowSeasonEpisodes :exec
DELETE FROM show_episodes WHERE show_episodes.season_id = ? AND NOT EXISTS (SELECT 1 FROM show_episode_files l WHERE l.episode_id = show_episodes.id);

-- name: PruneShowSeason :one
DELETE FROM show_seasons WHERE show_seasons.id = ? AND NOT EXISTS (SELECT 1 FROM show_files f WHERE f.season_id = show_seasons.id) RETURNING show_id;

-- name: PruneShow :exec
DELETE FROM shows WHERE shows.id = ? AND NOT EXISTS (SELECT 1 FROM show_seasons s WHERE s.show_id = shows.id);

-- name: DeleteShowFileVideoStreams :exec
DELETE FROM show_video_streams
WHERE file_id = ?;

-- name: InsertShowVideoStream :exec
INSERT INTO show_video_streams (
  file_id,
  stream_index,
  codec,
  codec_profile,
  codec_level,
  bit_rate,
  width,
  height,
  coded_width,
  coded_height,
  aspect_ratio,
  frame_rate,
  avg_frame_rate,
  bit_depth,
  pixel_format,
  color_range,
  color_space,
  color_primaries,
  color_transfer,
  field_order,
  rotation,
  language,
  title
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteShowFileAudioStreams :exec
DELETE FROM show_audio_streams
WHERE file_id = ?;

-- name: InsertShowAudioStream :exec
INSERT INTO show_audio_streams (
  file_id,
  stream_index,
  codec,
  codec_profile,
  bit_rate,
  sample_rate,
  channels,
  channel_layout,
  language,
  title,
  is_default
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteShowFileSubtitles :exec
DELETE FROM show_subtitles
WHERE file_id = ?;

-- name: InsertShowSubtitle :exec
INSERT INTO show_subtitles (
  file_id,
  stream_index,
  codec,
  language,
  title,
  is_forced,
  is_default
)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteShowFileChapters :exec
DELETE FROM show_chapters
WHERE file_id = ?;

-- name: InsertShowChapter :exec
INSERT INTO show_chapters (
  file_id,
  title,
  start_time,
  thumb
)
VALUES
  (?, ?, ?, ?);

-- name: UpsertNetwork :one
INSERT INTO networks (tmdb_id, name, logo, country) VALUES (?, ?, ?, ?) ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name, logo = excluded.logo, country = excluded.country RETURNING *;

-- name: DeleteShowCast :exec
DELETE FROM show_cast WHERE show_id = ?;

-- name: CreateShowCast :exec
INSERT INTO show_cast (show_id, artist_id, character, cast_order, credit_id, episode_count) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowCrew :exec
DELETE FROM show_crew WHERE show_id = ?;

-- name: CreateShowCrew :exec
INSERT INTO show_crew (show_id, artist_id, department, job, credit_id, episode_count) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowGenre :exec
DELETE FROM show_genres WHERE show_id = ?;

-- name: CreateShowGenre :exec
INSERT INTO show_genres (show_id, genre_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowProductionCompany :exec
DELETE FROM show_production_companies WHERE show_id = ?;

-- name: CreateShowProductionCompany :exec
INSERT INTO show_production_companies (show_id, production_company_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowNetwork :exec
DELETE FROM show_networks WHERE show_id = ?;

-- name: CreateShowNetwork :exec
INSERT INTO show_networks (show_id, network_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowCreator :exec
DELETE FROM show_creators WHERE show_id = ?;

-- name: CreateShowCreator :exec
INSERT INTO show_creators (show_id, artist_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowExtraVideo :exec
DELETE FROM show_extra_videos WHERE show_id = ?;

-- name: CreateShowExtraVideo :exec
INSERT INTO show_extra_videos (show_id, extra_video_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowSeasonCast :exec
DELETE FROM show_season_cast WHERE season_id = ?;

-- name: CreateShowSeasonCast :exec
INSERT INTO show_season_cast (season_id, artist_id, character, cast_order, credit_id, episode_count) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowSeasonCrew :exec
DELETE FROM show_season_crew WHERE season_id = ?;

-- name: CreateShowSeasonCrew :exec
INSERT INTO show_season_crew (season_id, artist_id, department, job, credit_id, episode_count) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowSeasonExtraVideo :exec
DELETE FROM show_season_extra_videos WHERE season_id = ?;

-- name: CreateShowSeasonExtraVideo :exec
INSERT INTO show_season_extra_videos (season_id, extra_video_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowEpisodeCrew :exec
DELETE FROM show_episode_crew WHERE episode_id = ?;

-- name: CreateShowEpisodeCrew :exec
INSERT INTO show_episode_crew (episode_id, artist_id, department, job, credit_id, episode_count) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- name: DeleteShowEpisodeGuestCast :exec
DELETE FROM show_episode_guest_cast WHERE episode_id = ?;

-- name: CreateShowEpisodeGuestCast :exec
INSERT INTO show_episode_guest_cast (episode_id, artist_id, character, cast_order, credit_id, episode_count) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;

-- ============================================================================
-- Show details page reads.
--
-- The queries below build the show details payload. Like GetLatestShows above,
-- they are explicit projections that never expose directory_path or local_name,
-- and they address a season by its number rather than by an internal row id;
-- the rest of the queries above serve the scanner.
-- ============================================================================

-- name: GetShowDetails :one
-- Show header for the details page. Omits the filesystem columns and the
-- fields the page does not render (adult, imdb_id, homepage, popularity,
-- timestamps).
SELECT
  id,
  name,
  original_name,
  premiere_year,
  tmdb_id,
  overview,
  tagline,
  language,
  origin_countries,
  first_air_date,
  last_air_date,
  status,
  type,
  poster_path,
  backdrop_path,
  vote_average,
  vote_count,
  certification,
  tmdb_season_count,
  tmdb_episode_count
FROM shows
WHERE id = ?
LIMIT 1;

-- name: GetShowSeasonSummaries :many
-- Seasons with the count of episodes actually present. Pruning deletes episodes
-- that lost their last file, so a stored episode always has one and the stored
-- count is the available count; TMDB's tmdb_episode_count rides alongside it so
-- the page can state how much of a season is here. Specials (season 0) sort
-- last rather than first.
SELECT
  s.id,
  s.season_number,
  s.name,
  s.overview,
  s.air_date,
  s.poster_path,
  s.tmdb_episode_count,
  COUNT(e.id) AS available_episode_count
FROM show_seasons AS s
LEFT JOIN show_episodes AS e
  ON e.season_id = s.id
WHERE s.show_id = ?
GROUP BY s.id
ORDER BY
  s.season_number = 0,
  s.season_number;

-- name: GetShowSeasonSummaryByNumber :one
-- The GetShowSeasonSummaries aggregate for one season, addressed by number.
SELECT
  s.id,
  s.season_number,
  s.name,
  s.overview,
  s.air_date,
  s.poster_path,
  s.tmdb_episode_count,
  COUNT(e.id) AS available_episode_count
FROM show_seasons AS s
LEFT JOIN show_episodes AS e
  ON e.season_id = s.id
WHERE s.show_id = ?
  AND s.season_number = ?
GROUP BY s.id;

-- name: GetShowEpisodesBySeasonNumber :many
-- Episodes of one season, addressed by (show_id, season_number) so the route
-- never exposes an internal season id. Carries no file, stream, codec, or
-- chapter data: those belong to a physical file, and one file can back several
-- episodes. The requesting user's watch progress rides along so the episode
-- list can show resume and watched state without a query per row; the CAST
-- keeps sqlc typing `watched` as a plain bool across the LEFT JOIN.
SELECT
  e.id,
  e.episode_number,
  e.name,
  e.overview,
  e.air_date,
  e.still_path,
  e.tmdb_runtime,
  e.vote_average,
  e.vote_count,
  wp.progress_sec,
  wp.duration_sec,
  CAST((wp.watched IS NOT NULL AND wp.watched) AS BOOLEAN) AS watched
FROM show_episodes AS e
INNER JOIN show_seasons AS s
  ON s.id = e.season_id
LEFT JOIN show_episode_watch_progress AS wp
  ON wp.episode_id = e.id
  AND wp.user_id = sqlc.arg(user_id)
WHERE s.show_id = sqlc.arg(show_id)
  AND s.season_number = sqlc.arg(season_number)
ORDER BY e.episode_number;

-- name: GetShowEpisodePlaybackDetails :one
-- Player header for one episode: the episode row plus the season number and
-- show identity the page titles itself with and navigates back to.
SELECT
  e.id,
  e.episode_number,
  e.name,
  e.overview,
  e.air_date,
  e.still_path,
  e.tmdb_runtime,
  e.vote_average,
  e.vote_count,
  s.season_number,
  s.name AS season_name,
  sh.id AS show_id,
  sh.name AS show_name,
  sh.poster_path AS show_poster_path,
  sh.backdrop_path AS show_backdrop_path
FROM show_episodes AS e
INNER JOIN show_seasons AS s
  ON s.id = e.season_id
INNER JOIN shows AS sh
  ON sh.id = s.show_id
WHERE e.id = ?
LIMIT 1;

-- name: GetShowFileForEpisode :one
-- The physical file behind an episode. An episode may be linked to more than
-- one file (duplicate copies), so the lowest file id is the deterministic
-- playback target; a combined file is returned whole, playback never seeks to
-- a guessed episode offset.
SELECT
  f.id,
  f.season_id,
  f.file_path,
  f.file_name,
  f.size,
  f.container,
  f.mime_type,
  f.duration,
  f.updated_at
FROM show_episode_files AS l
INNER JOIN show_files AS f
  ON f.id = l.file_id
WHERE l.episode_id = ?
ORDER BY f.id
LIMIT 1;

-- name: GetShowEpisodeForDirectStream :one
-- Direct-stream twin of GetMovieForDirectStream, resolved through the same
-- lowest-file-id rule as GetShowFileForEpisode.
SELECT
  f.file_path,
  f.file_name,
  f.container,
  f.mime_type
FROM show_episode_files AS l
INNER JOIN show_files AS f
  ON f.id = l.file_id
WHERE l.episode_id = ?
ORDER BY f.id
LIMIT 1;

-- name: GetShowFileEpisodeIDs :many
-- Episodes linked to a file, read before a deletion cascades the links away so
-- the per-episode runtime caches can be evicted after commit.
SELECT episode_id
FROM show_episode_files
WHERE file_id = ?
ORDER BY episode_order;

-- name: GetShowVideoStreamsByFileID :many
-- Video streams of a show file, in the same order the movie twin uses.
SELECT
  id,
  file_id,
  stream_index,
  codec,
  codec_profile,
  codec_level,
  bit_rate,
  width,
  height,
  coded_width,
  coded_height,
  aspect_ratio,
  frame_rate,
  avg_frame_rate,
  bit_depth,
  pixel_format,
  color_range,
  color_space,
  color_primaries,
  color_transfer,
  field_order,
  rotation,
  language,
  title
FROM show_video_streams
WHERE file_id = ?
ORDER BY stream_index;

-- name: GetShowAudioStreamsByFileID :many
SELECT
  id,
  file_id,
  stream_index,
  codec,
  codec_profile,
  bit_rate,
  sample_rate,
  channels,
  channel_layout,
  language,
  title,
  is_default
FROM show_audio_streams
WHERE file_id = ?
ORDER BY stream_index;

-- name: GetShowSubtitlesByFileID :many
SELECT
  id,
  file_id,
  stream_index,
  codec,
  language,
  title,
  is_forced,
  is_default
FROM show_subtitles
WHERE file_id = ?
ORDER BY stream_index;

-- name: GetShowChaptersByFileID :many
SELECT
  id,
  title,
  start_time,
  thumb,
  file_id
FROM show_chapters
WHERE file_id = ?
ORDER BY start_time;

-- name: GetCastByShowID :many
-- Aggregate cast with artist name and profile. show_cast has no row id, so
-- credit_id is the stable identity; TMDB can credit one artist with several
-- roles, which is why the artist id alone will not do.
--
-- Capped at 100 rows. TMDB aggregate credits for a long-running show reach into
-- the thousands, while the details page bills a few dozen; the cap follows
-- cast_order, so the billing TMDB considers most relevant is what survives it.
-- The cap is part of the HTTP contract - see docs/openapi.json.
SELECT
  sc.credit_id,
  sc.artist_id,
  sc.character,
  sc.cast_order,
  sc.episode_count,
  a.name AS artist_name,
  a.profile AS artist_profile
FROM show_cast AS sc
INNER JOIN artist AS a
  ON a.id = sc.artist_id
WHERE sc.show_id = ?
ORDER BY
  sc.cast_order,
  a.name
LIMIT 100;

-- name: GetCrewByShowID :many
-- Aggregate crew with artist name. credit_id is the stable identity, as in
-- GetCastByShowID, and the same 100-row cap applies for the same reason.
SELECT
  sc.credit_id,
  sc.artist_id,
  sc.department,
  sc.job,
  sc.episode_count,
  a.name AS artist_name,
  a.profile AS artist_profile
FROM show_crew AS sc
INNER JOIN artist AS a
  ON a.id = sc.artist_id
WHERE sc.show_id = ?
ORDER BY
  sc.department,
  sc.job,
  a.name
LIMIT 100;

-- name: GetCreatorsByShowID :many
-- Series creators, billed ahead of the aggregate crew on the details page.
SELECT
  a.id,
  a.name,
  a.profile
FROM artist AS a
INNER JOIN show_creators AS sc
  ON sc.artist_id = a.id
WHERE sc.show_id = ?
ORDER BY a.name;

-- name: GetGenresByShowID :many
-- Genres linked to a show (for details view).
SELECT
  g.id,
  g.tag
FROM genres AS g
INNER JOIN show_genres AS sg
  ON sg.genre_id = g.id
WHERE sg.show_id = ?
ORDER BY g.tag;

-- name: GetNetworksByShowID :many
-- Networks linked to a show, with the logo and country the About section
-- renders. The only query in the project that reads networks.
SELECT
  n.id,
  n.name,
  n.logo,
  n.country
FROM networks AS n
INNER JOIN show_networks AS sn
  ON sn.network_id = n.id
WHERE sn.show_id = ?
ORDER BY n.name;

-- name: GetProductionCompaniesByShowID :many
-- Production companies linked to a show (for details view).
SELECT
  pc.id,
  pc.name
FROM production_companies AS pc
INNER JOIN show_production_companies AS spc
  ON spc.production_company_id = pc.id
WHERE spc.show_id = ?
ORDER BY pc.name;

-- name: GetShowExtraVideos :many
-- List all extra videos (trailers, special features) linked to a show.
SELECT
  ev.id,
  ev.title,
  ev.key,
  ev.type,
  ev.site
FROM extra_videos AS ev
INNER JOIN show_extra_videos AS sev
  ON sev.extra_video_id = ev.id
WHERE sev.show_id = ?
ORDER BY
  ev.type,
  ev.title;

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
INSERT INTO show_tmdb_retries (show_id) VALUES (?) ON CONFLICT DO NOTHING;

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

-- name: HasShowRetry :one
SELECT EXISTS(SELECT 1 FROM show_tmdb_retries WHERE show_id = ?);

-- name: HasShowSeasonRetry :one
SELECT EXISTS(SELECT 1 FROM show_season_tmdb_retries WHERE season_id = ?);

-- name: HasShowEpisodeRetry :one
SELECT EXISTS(SELECT 1 FROM show_episode_tmdb_retries WHERE episode_id = ?);

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

-- name: UpsertShowFingerprint :exec
INSERT INTO show_file_fingerprints (file_id, mtime_ns, ctime_ns, device, inode, sha256) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (file_id) DO UPDATE SET mtime_ns = excluded.mtime_ns, ctime_ns = excluded.ctime_ns, device = excluded.device, inode = excluded.inode, sha256 = excluded.sha256;

-- name: GetShowScanIndex :many
SELECT f.*, fp.mtime_ns, fp.ctime_ns, fp.device, fp.inode, fp.sha256 FROM show_files f LEFT JOIN show_file_fingerprints fp ON fp.file_id = f.id;

-- name: DeleteMissingShowFile :execrows
DELETE FROM show_files WHERE id = ? AND file_path = ?;

-- name: PruneShowEpisodes :exec
DELETE FROM show_episodes WHERE NOT EXISTS (SELECT 1 FROM show_episode_files l WHERE l.episode_id = show_episodes.id);

-- name: PruneShowSeasons :exec
DELETE FROM show_seasons WHERE NOT EXISTS (SELECT 1 FROM show_files f WHERE f.season_id = show_seasons.id);

-- name: PruneShows :exec
DELETE FROM shows WHERE NOT EXISTS (SELECT 1 FROM show_seasons s WHERE s.show_id = shows.id);

-- name: DeleteShowFileVideoStreams :exec
DELETE FROM show_video_streams
WHERE file_id = ?;

-- name: InsertShowVideoStream :one
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
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteShowFileAudioStreams :exec
DELETE FROM show_audio_streams
WHERE file_id = ?;

-- name: InsertShowAudioStream :one
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
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteShowFileSubtitles :exec
DELETE FROM show_subtitles
WHERE file_id = ?;

-- name: InsertShowSubtitle :one
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
  (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteShowFileChapters :exec
DELETE FROM show_chapters
WHERE file_id = ?;

-- name: InsertShowChapter :one
INSERT INTO show_chapters (
  file_id,
  title,
  start_time,
  thumb
)
VALUES
  (?, ?, ?, ?)
RETURNING *;

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

-- name: GetShowFileEpisodes :many
SELECT e.* FROM show_episodes e JOIN show_episode_files l ON l.episode_id = e.id WHERE l.file_id = ? ORDER BY l.episode_order;

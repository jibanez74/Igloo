-- ============================================================================
-- PLAY HISTORY RECORDING
-- ============================================================================

-- name: RecordPlayEvent :exec
-- Records a new play event when a track is played
INSERT INTO user_play_history (user_id, track_id, duration_played, completed)
VALUES (?, ?, ?, ?);

-- name: UpsertUserTrackStats :exec
-- Updates aggregated stats when a play event is recorded
INSERT INTO user_track_stats (user_id, track_id, play_count, total_time_played, last_played_at, first_played_at)
VALUES (?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (user_id, track_id) DO UPDATE SET
    play_count = user_track_stats.play_count + 1,
    total_time_played = user_track_stats.total_time_played + excluded.total_time_played,
    last_played_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- USER STATISTICS QUERIES
-- ============================================================================

-- name: GetUserTopTracks :many
-- Returns the user's most played tracks
SELECT
    uts.play_count,
    uts.total_time_played,
    uts.last_played_at,
    t.id,
    t.title,
    t.duration,
    t.file_path,
    a.id AS album_id,
    a.title AS album_title,
    a.cover AS album_cover,
    m.id AS musician_id,
    m.name AS musician_name
FROM user_track_stats uts
INNER JOIN tracks t ON uts.track_id = t.id
LEFT JOIN albums a ON t.album_id = a.id
LEFT JOIN musicians m ON t.musician_id = m.id
WHERE uts.user_id = ?
ORDER BY uts.play_count DESC
LIMIT ? OFFSET ?;

-- name: GetUserTopMusicians :many
-- Returns the user's most listened musicians by total play count
SELECT
    m.id,
    m.name,
    m.thumb,
    SUM(uts.play_count) AS total_play_count,
    SUM(uts.total_time_played) AS total_time_listened,
    COUNT(DISTINCT t.id) AS unique_tracks_played
FROM user_track_stats uts
INNER JOIN tracks t ON uts.track_id = t.id
INNER JOIN musicians m ON t.musician_id = m.id
WHERE uts.user_id = ?
GROUP BY m.id, m.name, m.thumb
ORDER BY total_play_count DESC
LIMIT ? OFFSET ?;

-- name: GetUserTopGenres :many
-- Returns the user's most listened genres
SELECT
    g.id,
    g.tag,
    SUM(uts.play_count) AS total_play_count,
    SUM(uts.total_time_played) AS total_time_listened,
    COUNT(DISTINCT t.id) AS unique_tracks_played
FROM user_track_stats uts
INNER JOIN tracks t ON uts.track_id = t.id
INNER JOIN track_genres tg ON t.id = tg.track_id
INNER JOIN genres g ON tg.genre_id = g.id
WHERE uts.user_id = ? AND g.genre_type = 'music'
GROUP BY g.id, g.tag
ORDER BY total_play_count DESC
LIMIT ?;

-- name: GetUserListeningStats :one
-- Returns overall listening statistics for a user
SELECT
    COALESCE(SUM(uts.play_count), 0) AS total_plays,
    COALESCE(SUM(uts.total_time_played), 0) AS total_time_listened,
    COUNT(DISTINCT uts.track_id) AS unique_tracks_played,
    (SELECT COUNT(*) FROM user_liked_tracks ult WHERE ult.user_id = ?) AS liked_tracks_count
FROM user_track_stats uts
WHERE uts.user_id = ?;

-- name: GetUserRecentlyPlayed :many
-- Returns the user's recently played tracks
SELECT
    uph.played_at,
    uph.duration_played,
    t.id,
    t.title,
    t.duration,
    t.file_path,
    a.id AS album_id,
    a.title AS album_title,
    a.cover AS album_cover,
    m.id AS musician_id,
    m.name AS musician_name
FROM user_play_history uph
INNER JOIN tracks t ON uph.track_id = t.id
LEFT JOIN albums a ON t.album_id = a.id
LEFT JOIN musicians m ON t.musician_id = m.id
WHERE uph.user_id = ?
ORDER BY uph.played_at DESC
LIMIT ? OFFSET ?;

-- name: GetUserTrackPlayCount :one
-- Returns the play count for a specific track
SELECT play_count
FROM user_track_stats
WHERE user_id = ? AND track_id = ?;

-- name: GetUserTopAlbums :many
-- Returns the user's most listened albums
SELECT
    a.id,
    a.title,
    a.cover,
    a.musician,
    a.year,
    SUM(uts.play_count) AS total_play_count,
    SUM(uts.total_time_played) AS total_time_listened,
    COUNT(DISTINCT t.id) AS unique_tracks_played
FROM user_track_stats uts
INNER JOIN tracks t ON uts.track_id = t.id
INNER JOIN albums a ON t.album_id = a.id
WHERE uts.user_id = ?
GROUP BY a.id, a.title, a.cover, a.musician, a.year
ORDER BY total_play_count DESC
LIMIT ? OFFSET ?;

-- name: GetUserListeningHistoryByPeriod :many
-- Returns listening stats grouped by date for charts
SELECT
    DATE(uph.played_at) AS play_date,
    COUNT(*) AS play_count,
    SUM(uph.duration_played) AS total_duration
FROM user_play_history uph
WHERE uph.user_id = ?
  AND uph.played_at >= ?
  AND uph.played_at <= ?
GROUP BY DATE(uph.played_at)
ORDER BY play_date ASC;

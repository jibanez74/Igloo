-- name: FindMusicArtistIdentity :one
SELECT e.id, e.spotify_id, e.thumb FROM musicians e JOIN music_artist_identity i ON i.musician_id=e.id WHERE i.identity_key = ?;

-- name: SaveMusicArtistIdentity :exec
INSERT INTO music_artist_identity(identity_key, musician_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: FindMusicAlbumIdentity :one
SELECT e.id, e.spotify_id, e.cover FROM albums e JOIN music_album_identity i ON i.album_id=e.id WHERE i.title_key = ? AND i.artist_key = ?;

-- name: SaveMusicAlbumIdentity :exec
INSERT INTO music_album_identity(title_key, artist_key, album_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: FindMusicGenreIdentity :one
SELECT e.id FROM genres e JOIN music_genre_identity i ON i.genre_id=e.id WHERE i.identity_key = ?;

-- name: SaveMusicGenreIdentity :exec
INSERT INTO music_genre_identity(identity_key, genre_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: SaveMusicTrackMetadata :exec
INSERT INTO music_track_metadata(track_id,artist_tag,artist_key,artist_sort,album_sort) VALUES(?,?,?,?,?)
ON CONFLICT(track_id) DO UPDATE SET artist_tag=excluded.artist_tag,artist_key=excluded.artist_key,artist_sort=excluded.artist_sort,album_sort=excluded.album_sort;

-- name: DeleteMusicCreditMetadata :exec
DELETE FROM music_credit_metadata WHERE track_id=?;

-- name: SaveMusicCreditMetadata :exec
INSERT OR IGNORE INTO music_credit_metadata(track_id,musician_id,sort_name) VALUES(?,?,?);

-- name: SaveMusicAlbumDate :exec
INSERT INTO music_album_metadata(album_id,spotify_date) VALUES(?,?)
ON CONFLICT(album_id) DO UPDATE SET spotify_date=excluded.spotify_date;

-- name: ReconcileMusicArtistSort :exec
UPDATE musicians SET sort_name=COALESCE((SELECT vote FROM (SELECT MIN(sort_name COLLATE BINARY) AS vote FROM music_credit_metadata
WHERE musician_id=musicians.id AND sort_name<>'' GROUP BY track_id) GROUP BY vote
ORDER BY COUNT(*) DESC, vote COLLATE BINARY LIMIT 1), name), updated_at = CURRENT_TIMESTAMP
WHERE id=? AND sort_name IS NOT COALESCE((SELECT vote FROM (SELECT MIN(sort_name COLLATE BINARY) AS vote FROM music_credit_metadata
WHERE musician_id=musicians.id AND sort_name<>'' GROUP BY track_id) GROUP BY vote
ORDER BY COUNT(*) DESC, vote COLLATE BINARY LIMIT 1), name);

-- name: ReconcileMusicAlbumSort :exec
UPDATE albums SET sort_title=COALESCE((SELECT m.album_sort FROM music_track_metadata m JOIN tracks t ON t.id=m.track_id
WHERE t.album_id=albums.id AND m.album_sort<>'' GROUP BY m.album_sort ORDER BY COUNT(*) DESC,m.album_sort COLLATE BINARY LIMIT 1),title), updated_at = CURRENT_TIMESTAMP
WHERE albums.id=? AND sort_title IS NOT COALESCE((SELECT m.album_sort FROM music_track_metadata m JOIN tracks t ON t.id=m.track_id
WHERE t.album_id=albums.id AND m.album_sort<>'' GROUP BY m.album_sort ORDER BY COUNT(*) DESC,m.album_sort COLLATE BINARY LIMIT 1),title);

-- name: ReconcileMusicAlbumDate :exec
UPDATE albums SET release_date=COALESCE((SELECT t.release_date FROM tracks t WHERE t.album_id=albums.id AND t.release_date IS NOT NULL
GROUP BY t.release_date ORDER BY COUNT(*) DESC,t.release_date LIMIT 1),(SELECT spotify_date FROM music_album_metadata WHERE album_id=albums.id)), updated_at = CURRENT_TIMESTAMP
WHERE albums.id=? AND release_date IS NOT COALESCE((SELECT t.release_date FROM tracks t WHERE t.album_id=albums.id AND t.release_date IS NOT NULL
GROUP BY t.release_date ORDER BY COUNT(*) DESC,t.release_date LIMIT 1),(SELECT spotify_date FROM music_album_metadata WHERE album_id=albums.id));

-- name: ReconcileMusicAlbumYear :exec
UPDATE albums SET year=CAST(substr(release_date,1,4) AS INTEGER), updated_at = CURRENT_TIMESTAMP WHERE id=? AND year IS NOT CAST(substr(release_date,1,4) AS INTEGER);

-- name: MusicTrackAffectedArtists :many
SELECT musician_id FROM track_musicians WHERE track_id=(SELECT id FROM tracks WHERE file_path=?);

-- name: MusicTrackAffectedAlbum :one
SELECT album_id FROM tracks WHERE file_path=?;

-- name: DeleteMusicArtistSpotifyGenres :exec
DELETE FROM musician_genres WHERE musician_id=? AND source='spotify';

-- name: SaveMusicArtistSpotifyGenre :exec
INSERT OR IGNORE INTO musician_genres(musician_id,genre_id,source) VALUES(?,?,'spotify');

-- name: DeleteMusicAlbumSpotifyGenres :exec
DELETE FROM album_genres WHERE album_id=? AND source='spotify';

-- name: SaveMusicAlbumSpotifyGenre :exec
INSERT OR IGNORE INTO album_genres(album_id,genre_id,source) VALUES(?,?,'spotify');

-- name: MoveMusicArtistAliases :exec
UPDATE music_artist_identity SET musician_id=sqlc.arg(owner) WHERE musician_id=sqlc.arg(redundant);

-- name: MoveMusicArtistTracks :exec
UPDATE tracks SET musician_id=sqlc.arg(owner) WHERE musician_id=sqlc.arg(redundant);

-- name: MoveMusicArtistGenres :exec
INSERT OR IGNORE INTO musician_genres(musician_id,genre_id,source)
SELECT sqlc.arg(owner),genre_id,source FROM musician_genres WHERE musician_genres.musician_id=sqlc.arg(redundant);

-- name: DeleteMergedMusicArtist :exec
DELETE FROM musicians WHERE id=?;

-- name: MusicArtistRetryCandidates :many
SELECT e.id, e.name FROM musicians e LEFT JOIN music_spotify_matches m ON m.entity_id=e.id AND m.entity_type='musician'
WHERE e.id>sqlc.arg(after_id) AND (m.status IS NULL OR m.status='failed')
AND EXISTS(SELECT 1 FROM track_musicians t WHERE t.musician_id=e.id)
ORDER BY e.id LIMIT 100;

-- name: MoveMusicAlbumAliases :exec
UPDATE music_album_identity SET album_id=sqlc.arg(owner) WHERE album_id=sqlc.arg(redundant);

-- name: MoveMusicAlbumTracks :exec
UPDATE tracks SET album_id=sqlc.arg(owner) WHERE album_id=sqlc.arg(redundant);

-- name: MoveMusicAlbumGenres :exec
INSERT OR IGNORE INTO album_genres(album_id,genre_id,source)
SELECT sqlc.arg(owner),genre_id,source FROM album_genres WHERE album_genres.album_id=sqlc.arg(redundant);

-- name: DeleteMergedMusicAlbum :exec
DELETE FROM albums WHERE id=?;

-- name: MusicAlbumRetryCandidates :many
SELECT e.id, e.title, e.musician FROM albums e LEFT JOIN music_spotify_matches m ON m.entity_id=e.id AND m.entity_type='album'
WHERE e.id>sqlc.arg(after_id) AND (m.status IS NULL OR m.status='failed')
AND EXISTS(SELECT 1 FROM tracks t WHERE t.album_id=e.id)
ORDER BY e.id LIMIT 100;

-- name: MoveMusicArtistCredits :exec
INSERT OR IGNORE INTO track_musicians(track_id,musician_id)
SELECT track_id,sqlc.arg(owner) FROM track_musicians WHERE track_musicians.musician_id=sqlc.arg(redundant);

-- name: MoveMusicArtistContributions :exec
INSERT OR IGNORE INTO music_credit_metadata(track_id,musician_id,sort_name)
SELECT track_id,sqlc.arg(owner),sort_name FROM music_credit_metadata WHERE music_credit_metadata.musician_id=sqlc.arg(redundant);

-- name: MoveMusicAlbumFallback :exec
INSERT OR IGNORE INTO music_album_metadata(album_id,spotify_date)
SELECT sqlc.arg(owner),spotify_date FROM music_album_metadata WHERE music_album_metadata.album_id=sqlc.arg(redundant);

-- name: DeleteMergedMusicMatch :exec
DELETE FROM music_spotify_matches WHERE entity_type=? AND entity_id=?;

-- name: MusicArtistTrackMetadata :many
SELECT m.track_id, m.artist_tag, m.artist_sort FROM music_track_metadata m JOIN track_musicians tm ON tm.track_id=m.track_id
WHERE tm.musician_id=? AND m.track_id>sqlc.arg(after_id) ORDER BY m.track_id LIMIT 100;

-- name: UpdateMusicTrackPrimaryArtist :exec
UPDATE tracks SET musician_id=? WHERE id=?;

-- name: MusicArtistTrackIDs :many
SELECT track_id FROM track_musicians WHERE musician_id=?;

-- name: MusicAlbumTrackIDs :many
SELECT id FROM tracks WHERE album_id=?;

-- name: UpdateMusicArtistEnrichment :exec
-- COALESCE like UpsertMusician: the scanner maps an empty summary and a zero
-- popularity/follower count to NULL, and an obscure artist legitimately reports
-- both, so an unguarded SET would erase values a previous match stored.
UPDATE musicians SET
 summary = COALESCE(sqlc.narg(summary), summary),
 spotify_popularity = COALESCE(sqlc.narg(spotify_popularity), spotify_popularity),
 spotify_followers = COALESCE(sqlc.narg(spotify_followers), spotify_followers),
 updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: UpdateMusicAlbumEnrichment :exec
-- Same NULL-coercion guard as UpdateMusicArtistEnrichment.
UPDATE albums SET
 spotify_popularity = COALESCE(sqlc.narg(spotify_popularity), spotify_popularity),
 total_tracks = COALESCE(sqlc.narg(total_tracks), total_tracks),
 updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: MusicCompoundReconciliationCandidates :many
-- Driven from musicians so the keyset cursor rides the primary key, like
-- MusicArtistRetryCandidates. Joining from music_spotify_matches instead forced
-- a temp b-tree sort of every remaining candidate on each 100-row page.
SELECT e.id FROM musicians e
WHERE e.id>sqlc.arg(after_id)
AND EXISTS (SELECT 1 FROM music_spotify_matches m WHERE m.entity_type='musician' AND m.entity_id=e.id
AND m.status='unmatched' AND m.reason IN ('no_results','score_below_threshold'))
AND EXISTS (SELECT 1 FROM track_musicians tm JOIN music_track_metadata local ON local.track_id=tm.track_id
JOIN music_artist_identity i ON i.musician_id=tm.musician_id AND i.identity_key=local.artist_key
WHERE tm.musician_id=e.id AND (instr(local.artist_tag,' & ')>0 OR instr(local.artist_tag,',')>0))
ORDER BY e.id LIMIT 100;

-- name: SetMusicArtistSpotifyID :exec
UPDATE musicians SET spotify_id=?, updated_at = CURRENT_TIMESTAMP WHERE id=?;

-- name: SetMusicAlbumSpotifyID :exec
UPDATE albums SET spotify_id=?, updated_at = CURRENT_TIMESTAMP WHERE id=?;

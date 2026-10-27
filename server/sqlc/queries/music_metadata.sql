-- name: FindMusicArtistIdentity :one
SELECT e.id, e.spotify_id, e.thumb_source FROM musicians e JOIN music_artist_identity i ON i.musician_id=e.id WHERE i.identity_key = ?;

-- name: SaveMusicArtistIdentity :exec
INSERT INTO music_artist_identity(identity_key, musician_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: FindMusicAlbumIdentity :one
SELECT e.id, e.spotify_id, e.cover_source FROM albums e JOIN music_album_identity i ON i.album_id=e.id WHERE i.title_key = ? AND i.artist_key = ?;

-- name: SaveMusicAlbumIdentity :exec
INSERT INTO music_album_identity(title_key, artist_key, album_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING;

-- name: FindMusicGenreIdentity :one
SELECT e.id FROM genres e JOIN music_genre_identity i ON i.genre_id=e.id WHERE i.identity_key = ?;

-- name: SaveMusicGenreIdentity :exec
INSERT INTO music_genre_identity(identity_key, genre_id) VALUES (?, ?) ON CONFLICT DO NOTHING;

-- name: SaveMusicTrackMetadata :exec
INSERT INTO music_track_metadata(track_id,artist_tag,artist_key,artist_sort,album_sort,apple_track_id,apple_album_id) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(track_id) DO UPDATE SET artist_tag=excluded.artist_tag,artist_key=excluded.artist_key,artist_sort=excluded.artist_sort,album_sort=excluded.album_sort,
apple_track_id=excluded.apple_track_id,apple_album_id=excluded.apple_album_id;

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

-- name: MusicTrackAffected :many
-- The album and artists a track contributes to, read before a rewrite or delete
-- so both sides can be reconciled. One row per artist (album_id repeats); a
-- single row with a NULL musician_id means no artists; no rows means no track.
SELECT t.album_id, tm.musician_id FROM tracks t LEFT JOIN track_musicians tm ON tm.track_id=t.id
WHERE t.file_path=? ORDER BY tm.musician_id;

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
SELECT e.id, e.name FROM musicians e LEFT JOIN music_provider_matches m ON m.entity_id=e.id AND m.entity_type='musician'
WHERE e.id>sqlc.arg(after_id) AND (m.status IS NULL OR m.status='failed' OR EXISTS(SELECT 1 FROM music_spotify_adoptions p WHERE p.musician_id=e.id))
AND (EXISTS(SELECT 1 FROM track_musicians t WHERE t.musician_id=e.id)
OR EXISTS(SELECT 1 FROM music_spotify_adoptions p
    JOIN music_artist_identity ai ON ai.musician_id=p.musician_id
    JOIN music_album_identity al ON al.artist_key=ai.identity_key
    JOIN tracks t ON t.album_id=al.album_id WHERE p.musician_id=e.id))
ORDER BY e.id LIMIT 100;

-- name: MusicArtistRetryCandidatesIn :many
-- MusicArtistRetryCandidates limited to the artists a run of watcher-detected
-- changes touched; the set is small, so it is one unpaged query.
SELECT e.id, e.name FROM musicians e LEFT JOIN music_provider_matches m ON m.entity_id=e.id AND m.entity_type='musician'
WHERE e.id IN (sqlc.slice(ids)) AND (m.status IS NULL OR m.status='failed' OR EXISTS(SELECT 1 FROM music_spotify_adoptions p WHERE p.musician_id=e.id))
AND (EXISTS(SELECT 1 FROM track_musicians t WHERE t.musician_id=e.id)
OR EXISTS(SELECT 1 FROM music_spotify_adoptions p
    JOIN music_artist_identity ai ON ai.musician_id=p.musician_id
    JOIN music_album_identity al ON al.artist_key=ai.identity_key
    JOIN tracks t ON t.album_id=al.album_id WHERE p.musician_id=e.id))
ORDER BY e.id;

-- name: CountMusicArtistRetryCandidates :one
SELECT COUNT(*) FROM musicians e LEFT JOIN music_provider_matches m ON m.entity_id=e.id AND m.entity_type='musician'
WHERE (m.status IS NULL OR m.status='failed' OR EXISTS(SELECT 1 FROM music_spotify_adoptions p WHERE p.musician_id=e.id))
AND (EXISTS(SELECT 1 FROM track_musicians t WHERE t.musician_id=e.id)
OR EXISTS(SELECT 1 FROM music_spotify_adoptions p
    JOIN music_artist_identity ai ON ai.musician_id=p.musician_id
    JOIN music_album_identity al ON al.artist_key=ai.identity_key
    JOIN tracks t ON t.album_id=al.album_id WHERE p.musician_id=e.id));

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
-- Albums for Spotify to search: a failed search, an album Apple handed over
-- because it no longer lists it, or one nothing has settled yet, unless that
-- one waits for Apple: lookups are on and one of its files carries an Apple
-- collection id.
SELECT e.id, e.title, e.musician, e.year FROM albums e LEFT JOIN music_provider_matches m ON m.entity_id=e.id AND m.entity_type='album'
WHERE e.id>sqlc.arg(after_id)
AND (m.status='failed' OR (m.provider='apple' AND m.reason='not_listed')
OR (m.status IS NULL AND NOT (CAST(sqlc.arg(apple_enabled) AS BOOLEAN) AND EXISTS(SELECT 1 FROM tracks t
    JOIN music_track_metadata h ON h.track_id=t.id WHERE t.album_id=e.id AND h.apple_album_id IS NOT NULL))))
AND EXISTS(SELECT 1 FROM tracks t WHERE t.album_id=e.id)
ORDER BY e.id LIMIT 100;

-- name: MusicAlbumRetryCandidatesIn :many
-- MusicAlbumRetryCandidates limited to the albums a run of watcher-detected
-- changes touched. The ids arrive as a JSON array rather than sqlc.slice():
-- sqlc numbers apple_enabled ?2, and an expanded slice renumbers the
-- anonymous placeholders before it, so ?2 would bind the second album id.
SELECT e.id, e.title, e.musician, e.year FROM albums e LEFT JOIN music_provider_matches m ON m.entity_id=e.id AND m.entity_type='album'
WHERE e.id IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT)))
AND (m.status='failed' OR (m.provider='apple' AND m.reason='not_listed')
OR (m.status IS NULL AND NOT (CAST(sqlc.arg(apple_enabled) AS BOOLEAN) AND EXISTS(SELECT 1 FROM tracks t
    JOIN music_track_metadata h ON h.track_id=t.id WHERE t.album_id=e.id AND h.apple_album_id IS NOT NULL))))
AND EXISTS(SELECT 1 FROM tracks t WHERE t.album_id=e.id)
ORDER BY e.id;

-- name: CountMusicAlbumRetryCandidates :one
SELECT COUNT(*) FROM albums e LEFT JOIN music_provider_matches m ON m.entity_id=e.id AND m.entity_type='album'
WHERE (m.status='failed' OR (m.provider='apple' AND m.reason='not_listed')
OR (m.status IS NULL AND NOT (CAST(sqlc.arg(apple_enabled) AS BOOLEAN) AND EXISTS(SELECT 1 FROM tracks t
    JOIN music_track_metadata h ON h.track_id=t.id WHERE t.album_id=e.id AND h.apple_album_id IS NOT NULL))))
AND EXISTS(SELECT 1 FROM tracks t WHERE t.album_id=e.id);

-- name: MoveMusicArtistCredits :exec
INSERT OR IGNORE INTO track_musicians(track_id,musician_id)
SELECT track_id,sqlc.arg(owner) FROM track_musicians WHERE track_musicians.musician_id=sqlc.arg(redundant);

-- name: MoveMusicArtistContributions :exec
INSERT OR IGNORE INTO music_credit_metadata(track_id,musician_id,sort_name)
SELECT track_id,sqlc.arg(owner),sort_name FROM music_credit_metadata WHERE music_credit_metadata.musician_id=sqlc.arg(redundant);

-- name: MoveMusicAlbumFallback :exec
INSERT OR IGNORE INTO music_album_metadata(album_id,spotify_date)
SELECT sqlc.arg(owner),spotify_date FROM music_album_metadata WHERE music_album_metadata.album_id=sqlc.arg(redundant);

-- name: MusicArtistTrackMetadata :many
-- The range and order ride idx_track_musicians_musician_track, so keep them on tm.
SELECT m.track_id, m.artist_tag, m.artist_sort FROM track_musicians tm JOIN music_track_metadata m ON m.track_id=tm.track_id
WHERE tm.musician_id=? AND tm.track_id>sqlc.arg(after_id) ORDER BY tm.track_id LIMIT 100;

-- name: UpdateMusicTrackPrimaryArtist :exec
-- Guarded so an unchanged primary artist does not fire the search triggers.
UPDATE tracks SET musician_id = sqlc.arg(musician_id) WHERE id = sqlc.arg(id) AND musician_id IS NOT sqlc.arg(musician_id);

-- name: MusicArtistTrackIDs :many
SELECT track_id FROM track_musicians WHERE musician_id=?;

-- name: MusicAlbumTrackIDs :many
SELECT id FROM tracks WHERE album_id=?;

-- name: UpdateMusicAlbumEnrichment :exec
-- COALESCE like UpsertAlbum: the scanner maps a zero track count and an
-- unknown album type to NULL, so an unguarded SET would erase values a
-- previous match stored.
UPDATE albums SET
 total_tracks = COALESCE(sqlc.narg(total_tracks), total_tracks),
 album_type = COALESCE(sqlc.narg(album_type), album_type),
 updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: MusicTrackLinkAlbumCandidates :many
-- Matched albums with a track still to pair with Spotify's track list: no
-- 'track' outcome yet, or a failed one. Unmatched tracks are final, so an
-- album whose extras are not on Spotify drops out once they are recorded.
SELECT e.id, e.title, e.musician, e.spotify_id FROM albums e
WHERE e.id>sqlc.arg(after_id) AND e.spotify_id IS NOT NULL
AND EXISTS(SELECT 1 FROM tracks t LEFT JOIN music_provider_matches m ON m.entity_type='track' AND m.entity_id=t.id
WHERE t.album_id=e.id AND (m.status IS NULL OR m.status='failed'))
ORDER BY e.id LIMIT 100;

-- name: MusicTrackLinkAlbumCandidatesIn :many
-- MusicTrackLinkAlbumCandidates limited to the albums a run of
-- watcher-detected changes touched.
SELECT e.id, e.title, e.musician, e.spotify_id FROM albums e
WHERE e.id IN (sqlc.slice(ids)) AND e.spotify_id IS NOT NULL
AND EXISTS(SELECT 1 FROM tracks t LEFT JOIN music_provider_matches m ON m.entity_type='track' AND m.entity_id=t.id
WHERE t.album_id=e.id AND (m.status IS NULL OR m.status='failed'))
ORDER BY e.id;

-- name: CountMusicTrackLinkAlbumCandidates :one
SELECT COUNT(*) FROM albums e
WHERE e.spotify_id IS NOT NULL
AND EXISTS(SELECT 1 FROM tracks t LEFT JOIN music_provider_matches m ON m.entity_type='track' AND m.entity_id=t.id
WHERE t.album_id=e.id AND (m.status IS NULL OR m.status='failed'));

-- name: MusicAlbumTracksToLink :many
-- Every track of the album with its outcome: one without an outcome, or with
-- a failed one, is paired; a matched one holds its Spotify track.
SELECT t.id, t.title, t.disc, t.track_index, t.duration, t.spotify_id, m.status FROM tracks t
LEFT JOIN music_provider_matches m ON m.entity_type='track' AND m.entity_id=t.id
WHERE t.album_id=?
ORDER BY t.disc, t.track_index, t.id;

-- name: ClearMusicAlbumSpotify :exec
-- Drops what a match its tracks contradicted stored on the album; the next
-- three statements drop its date and its tracks' pairings and outcomes.
UPDATE albums SET spotify_id=NULL, cover_source=NULL, album_type=NULL, total_tracks=NULL, updated_at=CURRENT_TIMESTAMP WHERE id=?;

-- name: ClearMusicAlbumSpotifyDate :exec
UPDATE music_album_metadata SET spotify_date=NULL WHERE album_id=?;

-- name: ClearMusicAlbumTrackLinks :exec
UPDATE tracks SET spotify_id=NULL, explicit=false, updated_at=CURRENT_TIMESTAMP
WHERE album_id=? AND (spotify_id IS NOT NULL OR explicit);

-- name: DeleteMusicAlbumTrackMatches :exec
DELETE FROM music_provider_matches WHERE entity_type='track' AND entity_id IN (SELECT id FROM tracks WHERE album_id=?);

-- name: MusicCompoundReconciliationCandidates :many
-- Driven from musicians so the keyset cursor rides the primary key, like
-- MusicArtistRetryCandidates. Joining from music_provider_matches instead forced
-- a temp b-tree sort of every remaining candidate on each 100-row page.
SELECT e.id FROM musicians e
WHERE e.id>sqlc.arg(after_id)
AND EXISTS (SELECT 1 FROM music_provider_matches m WHERE m.entity_type='musician' AND m.entity_id=e.id
AND m.status='unmatched' AND m.reason IN ('no_results','score_below_threshold'))
AND EXISTS (SELECT 1 FROM track_musicians tm JOIN music_track_metadata local ON local.track_id=tm.track_id
JOIN music_artist_identity i ON i.musician_id=tm.musician_id AND i.identity_key=local.artist_key
WHERE tm.musician_id=e.id AND (instr(local.artist_tag,' & ')>0 OR instr(local.artist_tag,',')>0))
ORDER BY e.id LIMIT 100;

-- name: MusicCompoundReconciliationCandidatesIn :many
-- MusicCompoundReconciliationCandidates limited to the artists a run of
-- watcher-detected changes touched.
SELECT e.id FROM musicians e
WHERE e.id IN (sqlc.slice(ids))
AND EXISTS (SELECT 1 FROM music_provider_matches m WHERE m.entity_type='musician' AND m.entity_id=e.id
AND m.status='unmatched' AND m.reason IN ('no_results','score_below_threshold'))
AND EXISTS (SELECT 1 FROM track_musicians tm JOIN music_track_metadata local ON local.track_id=tm.track_id
JOIN music_artist_identity i ON i.musician_id=tm.musician_id AND i.identity_key=local.artist_key
WHERE tm.musician_id=e.id AND (instr(local.artist_tag,' & ')>0 OR instr(local.artist_tag,',')>0))
ORDER BY e.id;

-- name: SetMusicArtistSpotifyID :exec
UPDATE musicians SET spotify_id=?, updated_at = CURRENT_TIMESTAMP WHERE id=?;

-- name: SetMusicAlbumSpotifyID :exec
UPDATE albums SET spotify_id=?, updated_at = CURRENT_TIMESTAMP WHERE id=?;

-- name: GetMusicSpotifyAdoption :one
SELECT spotify_id FROM music_spotify_adoptions WHERE musician_id=?;

-- name: SaveMusicSpotifyAdoption :exec
INSERT INTO music_spotify_adoptions(musician_id, spotify_id) VALUES (?, ?)
ON CONFLICT(musician_id) DO NOTHING;

-- name: DeleteMusicSpotifyAdoption :exec
DELETE FROM music_spotify_adoptions WHERE musician_id=?;

-- name: MusicAlbumIdentityKeys :many
SELECT title_key, artist_key FROM music_album_identity WHERE album_id=?;

-- name: MusicAppleAlbumCandidates :many
-- Albums for the Apple pass: one nothing has settled whose files carry an
-- Apple collection id, and an Apple-matched one with tracks still to pair
-- (files added or changed since it matched).
SELECT e.id, e.apple_id FROM albums e
LEFT JOIN music_provider_matches m ON m.entity_type='album' AND m.entity_id=e.id
WHERE e.id>sqlc.arg(after_id)
AND ((m.entity_id IS NULL AND EXISTS(SELECT 1 FROM tracks t JOIN music_track_metadata h ON h.track_id=t.id
    WHERE t.album_id=e.id AND h.apple_album_id IS NOT NULL))
OR (e.apple_id IS NOT NULL AND EXISTS(SELECT 1 FROM tracks t LEFT JOIN music_provider_matches tm
    ON tm.entity_type='track' AND tm.entity_id=t.id WHERE t.album_id=e.id AND tm.entity_id IS NULL)))
ORDER BY e.id LIMIT 100;

-- name: MusicAppleAlbumCandidatesIn :many
-- MusicAppleAlbumCandidates limited to the albums a run of watcher-detected
-- changes touched.
SELECT e.id, e.apple_id FROM albums e
LEFT JOIN music_provider_matches m ON m.entity_type='album' AND m.entity_id=e.id
WHERE e.id IN (sqlc.slice(ids))
AND ((m.entity_id IS NULL AND EXISTS(SELECT 1 FROM tracks t JOIN music_track_metadata h ON h.track_id=t.id
    WHERE t.album_id=e.id AND h.apple_album_id IS NOT NULL))
OR (e.apple_id IS NOT NULL AND EXISTS(SELECT 1 FROM tracks t LEFT JOIN music_provider_matches tm
    ON tm.entity_type='track' AND tm.entity_id=t.id WHERE t.album_id=e.id AND tm.entity_id IS NULL)))
ORDER BY e.id;

-- name: CountMusicAppleAlbumCandidates :one
SELECT COUNT(*) FROM albums e
LEFT JOIN music_provider_matches m ON m.entity_type='album' AND m.entity_id=e.id
WHERE (m.entity_id IS NULL AND EXISTS(SELECT 1 FROM tracks t JOIN music_track_metadata h ON h.track_id=t.id
    WHERE t.album_id=e.id AND h.apple_album_id IS NOT NULL))
OR (e.apple_id IS NOT NULL AND EXISTS(SELECT 1 FROM tracks t LEFT JOIN music_provider_matches tm
    ON tm.entity_type='track' AND tm.entity_id=t.id WHERE t.album_id=e.id AND tm.entity_id IS NULL));

-- name: MusicAlbumAppleCollections :many
-- The album's Apple collection ids, the one most of its tracks carry first.
SELECT h.apple_album_id FROM tracks t JOIN music_track_metadata h ON h.track_id=t.id
WHERE t.album_id=? AND h.apple_album_id IS NOT NULL
GROUP BY h.apple_album_id ORDER BY COUNT(*) DESC, h.apple_album_id;

-- name: MusicAlbumHasAppleHint :one
SELECT EXISTS(SELECT 1 FROM tracks t JOIN music_track_metadata h ON h.track_id=t.id
WHERE t.album_id=? AND h.apple_album_id IS NOT NULL);

-- name: MusicAlbumTracksToPairApple :many
-- The album's tracks without an outcome, with the Apple track id each file
-- carries.
SELECT t.id, h.apple_track_id FROM tracks t
LEFT JOIN music_track_metadata h ON h.track_id=t.id
LEFT JOIN music_provider_matches m ON m.entity_type='track' AND m.entity_id=t.id
WHERE t.album_id=? AND m.entity_id IS NULL ORDER BY t.id;

-- name: GetAlbumByAppleID :one
SELECT id FROM albums WHERE apple_id=? LIMIT 1;

-- name: SetMusicAlbumAppleID :exec
UPDATE albums SET apple_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?;

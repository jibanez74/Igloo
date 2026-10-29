-- Library artwork, kept by the artwork syncer. Each image column is the value
-- the API returns and its _source column the provider reference; the triggers
-- at the end of schema.sql keep the two in step for every other writer. The
-- syncer's writes leave updated_at alone, like the triggers.
--
-- A full pass lists every table with List*Artwork. A pass of changes lists the
-- images that still return their source with ListPending*Artwork, through the
-- partial indexes they name, then every row of the folders those belong in
-- with List*ArtworkInFolders. Each repeats its table's List*Artwork columns,
-- so one mapper reads all three; keys arrive as a JSON array of strings, which
-- integer columns compare as numbers.

-- name: GetArtworkStaticDir :one
SELECT static_dir FROM artwork_state WHERE id = 1;

-- name: SetArtworkStaticDir :exec
INSERT INTO artwork_state (id, static_dir) VALUES (1, ?)
ON CONFLICT (id) DO UPDATE SET static_dir = excluded.static_dir;

-- name: ClearArtworkStaticDir :exec
DELETE FROM artwork_state;

-- name: ListMovieArtwork :many
SELECT id, tmdb_id, title, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM movies
WHERE id > sqlc.arg(after_id)
  AND (poster_path IS NOT NULL OR poster_path_source IS NOT NULL OR backdrop_path IS NOT NULL OR backdrop_path_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListPendingMovieArtwork :many
SELECT id, tmdb_id, title, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM movies INDEXED BY idx_movies_artwork_pending
WHERE id > sqlc.arg(after_id)
  AND ((poster_path_source IS NOT NULL AND poster_path IS poster_path_source)
   OR (backdrop_path_source IS NOT NULL AND backdrop_path IS backdrop_path_source))
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListMovieArtworkInFolders :many
SELECT id, tmdb_id, title, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM movies
WHERE tmdb_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (poster_path IS NOT NULL OR poster_path_source IS NOT NULL OR backdrop_path IS NOT NULL OR backdrop_path_source IS NOT NULL)
ORDER BY id;

-- name: SetMoviePosterArtwork :execrows
-- Applies only while the source the value was made from is still the row's,
-- so a source a scanner wrote meanwhile wins, and a copy only while the row's
-- provider ID still names the folder key it is in: a new ID returned the
-- source, and a copy in the old folder would be taken for an orphan.
UPDATE movies SET poster_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND poster_path_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR CAST(tmdb_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: SetMovieBackdropArtwork :execrows
UPDATE movies SET backdrop_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND backdrop_path_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR CAST(tmdb_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertMoviePosterArtwork :execrows
UPDATE movies SET poster_path = poster_path_source WHERE poster_path LIKE '/api/static/%';

-- name: RevertMovieBackdropArtwork :execrows
UPDATE movies SET backdrop_path = backdrop_path_source WHERE backdrop_path LIKE '/api/static/%';

-- name: ListArtistArtwork :many
SELECT id, tmdb_id, name, profile, profile_source
FROM artist
WHERE id > sqlc.arg(after_id) AND (profile IS NOT NULL OR profile_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListPendingArtistArtwork :many
SELECT id, tmdb_id, name, profile, profile_source
FROM artist INDEXED BY idx_artist_artwork_pending
WHERE id > sqlc.arg(after_id) AND profile_source IS NOT NULL AND profile IS profile_source
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListArtistArtworkInFolders :many
SELECT id, tmdb_id, name, profile, profile_source
FROM artist
WHERE tmdb_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (profile IS NOT NULL OR profile_source IS NOT NULL)
ORDER BY id;

-- name: SetArtistProfileArtwork :execrows
UPDATE artist SET profile = sqlc.narg(value) WHERE id = sqlc.arg(id) AND profile_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR CAST(tmdb_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertArtistProfileArtwork :execrows
UPDATE artist SET profile = profile_source WHERE profile LIKE '/api/static/%';

-- name: ListShowArtwork :many
SELECT id, tmdb_id, name, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM shows
WHERE id > sqlc.arg(after_id)
  AND (poster_path IS NOT NULL OR poster_path_source IS NOT NULL OR backdrop_path IS NOT NULL OR backdrop_path_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListPendingShowArtwork :many
SELECT id, tmdb_id, name, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM shows INDEXED BY idx_shows_artwork_pending
WHERE id > sqlc.arg(after_id)
  AND ((poster_path_source IS NOT NULL AND poster_path IS poster_path_source)
   OR (backdrop_path_source IS NOT NULL AND backdrop_path IS backdrop_path_source))
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListShowArtworkInFolders :many
SELECT id, tmdb_id, name, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM shows
WHERE tmdb_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (poster_path IS NOT NULL OR poster_path_source IS NOT NULL OR backdrop_path IS NOT NULL OR backdrop_path_source IS NOT NULL)
ORDER BY id;

-- name: SetShowPosterArtwork :execrows
UPDATE shows SET poster_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND poster_path_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR CAST(tmdb_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: SetShowBackdropArtwork :execrows
UPDATE shows SET backdrop_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND backdrop_path_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR CAST(tmdb_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertShowPosterArtwork :execrows
UPDATE shows SET poster_path = poster_path_source WHERE poster_path LIKE '/api/static/%';

-- name: RevertShowBackdropArtwork :execrows
UPDATE shows SET backdrop_path = backdrop_path_source WHERE backdrop_path LIKE '/api/static/%';

-- name: ListShowSeasonArtwork :many
-- Season and episode copies live in their show's folder, named by its TMDB id.
SELECT se.id, sh.tmdb_id AS show_tmdb_id, se.season_number, se.poster_path, se.poster_path_source
FROM show_seasons se
JOIN shows sh ON sh.id = se.show_id
WHERE se.id > sqlc.arg(after_id) AND (se.poster_path IS NOT NULL OR se.poster_path_source IS NOT NULL)
ORDER BY se.id
LIMIT sqlc.arg(page_size);

-- name: ListPendingShowSeasonArtwork :many
SELECT se.id, sh.tmdb_id AS show_tmdb_id, se.season_number, se.poster_path, se.poster_path_source
FROM show_seasons se INDEXED BY idx_show_seasons_artwork_pending
JOIN shows sh ON sh.id = se.show_id
WHERE se.id > sqlc.arg(after_id) AND se.poster_path_source IS NOT NULL AND se.poster_path IS se.poster_path_source
ORDER BY se.id
LIMIT sqlc.arg(page_size);

-- name: ListShowSeasonArtworkInFolders :many
SELECT se.id, sh.tmdb_id AS show_tmdb_id, se.season_number, se.poster_path, se.poster_path_source
FROM show_seasons se
JOIN shows sh ON sh.id = se.show_id
WHERE sh.tmdb_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (se.poster_path IS NOT NULL OR se.poster_path_source IS NOT NULL)
ORDER BY se.id;

-- name: SetShowSeasonPosterArtwork :execrows
UPDATE show_seasons SET poster_path = sqlc.narg(value)
WHERE show_seasons.id = sqlc.arg(id) AND show_seasons.poster_path_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source)
    OR (SELECT CAST(sh.tmdb_id AS TEXT) FROM shows sh WHERE sh.id = show_seasons.show_id) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertShowSeasonPosterArtwork :execrows
UPDATE show_seasons SET poster_path = poster_path_source WHERE poster_path LIKE '/api/static/%';

-- name: ListShowEpisodeArtwork :many
SELECT e.id, sh.tmdb_id AS show_tmdb_id, se.season_number, e.episode_number, e.still_path, e.still_path_source
FROM show_episodes e
JOIN show_seasons se ON se.id = e.season_id
JOIN shows sh ON sh.id = se.show_id
WHERE e.id > sqlc.arg(after_id) AND (e.still_path IS NOT NULL OR e.still_path_source IS NOT NULL)
ORDER BY e.id
LIMIT sqlc.arg(page_size);

-- name: ListPendingShowEpisodeArtwork :many
SELECT e.id, sh.tmdb_id AS show_tmdb_id, se.season_number, e.episode_number, e.still_path, e.still_path_source
FROM show_episodes e INDEXED BY idx_show_episodes_artwork_pending
JOIN show_seasons se ON se.id = e.season_id
JOIN shows sh ON sh.id = se.show_id
WHERE e.id > sqlc.arg(after_id) AND e.still_path_source IS NOT NULL AND e.still_path IS e.still_path_source
ORDER BY e.id
LIMIT sqlc.arg(page_size);

-- name: ListShowEpisodeArtworkInFolders :many
SELECT e.id, sh.tmdb_id AS show_tmdb_id, se.season_number, e.episode_number, e.still_path, e.still_path_source
FROM show_episodes e
JOIN show_seasons se ON se.id = e.season_id
JOIN shows sh ON sh.id = se.show_id
WHERE sh.tmdb_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (e.still_path IS NOT NULL OR e.still_path_source IS NOT NULL)
ORDER BY e.id;

-- name: SetShowEpisodeStillArtwork :execrows
UPDATE show_episodes SET still_path = sqlc.narg(value)
WHERE show_episodes.id = sqlc.arg(id) AND show_episodes.still_path_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source)
    OR (SELECT CAST(sh.tmdb_id AS TEXT) FROM show_seasons se JOIN shows sh ON sh.id = se.show_id WHERE se.id = show_episodes.season_id) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertShowEpisodeStillArtwork :execrows
UPDATE show_episodes SET still_path = still_path_source WHERE still_path LIKE '/api/static/%';

-- name: ListNetworkArtwork :many
SELECT id, tmdb_id, name, logo, logo_source
FROM networks
WHERE id > sqlc.arg(after_id) AND (logo IS NOT NULL OR logo_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListPendingNetworkArtwork :many
SELECT id, tmdb_id, name, logo, logo_source
FROM networks INDEXED BY idx_networks_artwork_pending
WHERE id > sqlc.arg(after_id) AND logo_source IS NOT NULL AND logo IS logo_source
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListNetworkArtworkInFolders :many
SELECT id, tmdb_id, name, logo, logo_source
FROM networks
WHERE tmdb_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (logo IS NOT NULL OR logo_source IS NOT NULL)
ORDER BY id;

-- name: SetNetworkLogoArtwork :execrows
UPDATE networks SET logo = sqlc.narg(value) WHERE id = sqlc.arg(id) AND logo_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR CAST(tmdb_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertNetworkLogoArtwork :execrows
UPDATE networks SET logo = logo_source WHERE logo LIKE '/api/static/%';

-- name: ListAlbumArtwork :many
-- An album is matched by Spotify or by Apple, never both; the ID of the one
-- that matched it names its folder.
SELECT id, spotify_id, apple_id, title, cover, cover_source
FROM albums
WHERE id > sqlc.arg(after_id) AND (cover IS NOT NULL OR cover_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListPendingAlbumArtwork :many
SELECT id, spotify_id, apple_id, title, cover, cover_source
FROM albums INDEXED BY idx_albums_artwork_pending
WHERE id > sqlc.arg(after_id) AND cover_source IS NOT NULL AND cover IS cover_source
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListAlbumArtworkInFolders :many
-- Either ID may name a folder, so both are matched; the syncer keeps the
-- albums whose own folder it asked for.
SELECT id, spotify_id, apple_id, title, cover, cover_source
FROM albums
WHERE (spotify_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
    OR apple_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT))))
  AND (cover IS NOT NULL OR cover_source IS NOT NULL)
ORDER BY id;

-- name: SetAlbumCoverArtwork :execrows
UPDATE albums SET cover = sqlc.narg(value) WHERE id = sqlc.arg(id) AND cover_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR spotify_id IS CAST(sqlc.arg(key) AS TEXT) OR CAST(apple_id AS TEXT) IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertAlbumCoverArtwork :execrows
UPDATE albums SET cover = cover_source WHERE cover LIKE '/api/static/%';

-- name: ListMusicianArtwork :many
SELECT id, spotify_id, name, thumb, thumb_source
FROM musicians
WHERE id > sqlc.arg(after_id) AND (thumb IS NOT NULL OR thumb_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListPendingMusicianArtwork :many
SELECT id, spotify_id, name, thumb, thumb_source
FROM musicians INDEXED BY idx_musicians_artwork_pending
WHERE id > sqlc.arg(after_id) AND thumb_source IS NOT NULL AND thumb IS thumb_source
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: ListMusicianArtworkInFolders :many
SELECT id, spotify_id, name, thumb, thumb_source
FROM musicians
WHERE spotify_id IN (SELECT value FROM json_each(CAST(sqlc.arg(keys) AS TEXT)))
  AND (thumb IS NOT NULL OR thumb_source IS NOT NULL)
ORDER BY id;

-- name: SetMusicianThumbArtwork :execrows
UPDATE musicians SET thumb = sqlc.narg(value) WHERE id = sqlc.arg(id) AND thumb_source IS sqlc.narg(source)
  AND (sqlc.narg(value) IS sqlc.narg(source) OR spotify_id IS CAST(sqlc.arg(key) AS TEXT));

-- name: RevertMusicianThumbArtwork :execrows
UPDATE musicians SET thumb = thumb_source WHERE thumb LIKE '/api/static/%';

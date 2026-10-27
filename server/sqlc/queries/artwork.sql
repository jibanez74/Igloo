-- Library artwork, kept by the artwork syncer. Each image column is the value
-- the API returns and its _source column the provider reference; the triggers
-- at the end of schema.sql keep the two in step for every other writer. The
-- syncer's writes leave updated_at alone, like the triggers.

-- name: ListMovieArtwork :many
SELECT id, tmdb_id, title, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM movies
WHERE id > sqlc.arg(after_id)
  AND (poster_path IS NOT NULL OR poster_path_source IS NOT NULL OR backdrop_path IS NOT NULL OR backdrop_path_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: SetMoviePosterArtwork :execrows
-- Applies only while the source the value was made from is still the row's,
-- so a source a scanner wrote meanwhile wins.
UPDATE movies SET poster_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND poster_path_source IS sqlc.narg(source);

-- name: SetMovieBackdropArtwork :execrows
UPDATE movies SET backdrop_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND backdrop_path_source IS sqlc.narg(source);

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

-- name: SetArtistProfileArtwork :execrows
UPDATE artist SET profile = sqlc.narg(value) WHERE id = sqlc.arg(id) AND profile_source IS sqlc.narg(source);

-- name: RevertArtistProfileArtwork :execrows
UPDATE artist SET profile = profile_source WHERE profile LIKE '/api/static/%';

-- name: ListShowArtwork :many
SELECT id, tmdb_id, name, poster_path, poster_path_source, backdrop_path, backdrop_path_source
FROM shows
WHERE id > sqlc.arg(after_id)
  AND (poster_path IS NOT NULL OR poster_path_source IS NOT NULL OR backdrop_path IS NOT NULL OR backdrop_path_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: SetShowPosterArtwork :execrows
UPDATE shows SET poster_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND poster_path_source IS sqlc.narg(source);

-- name: SetShowBackdropArtwork :execrows
UPDATE shows SET backdrop_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND backdrop_path_source IS sqlc.narg(source);

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

-- name: SetShowSeasonPosterArtwork :execrows
UPDATE show_seasons SET poster_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND poster_path_source IS sqlc.narg(source);

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

-- name: SetShowEpisodeStillArtwork :execrows
UPDATE show_episodes SET still_path = sqlc.narg(value) WHERE id = sqlc.arg(id) AND still_path_source IS sqlc.narg(source);

-- name: RevertShowEpisodeStillArtwork :execrows
UPDATE show_episodes SET still_path = still_path_source WHERE still_path LIKE '/api/static/%';

-- name: ListNetworkArtwork :many
SELECT id, tmdb_id, name, logo, logo_source
FROM networks
WHERE id > sqlc.arg(after_id) AND (logo IS NOT NULL OR logo_source IS NOT NULL)
ORDER BY id
LIMIT sqlc.arg(page_size);

-- name: SetNetworkLogoArtwork :execrows
UPDATE networks SET logo = sqlc.narg(value) WHERE id = sqlc.arg(id) AND logo_source IS sqlc.narg(source);

-- name: RevertNetworkLogoArtwork :execrows
UPDATE networks SET logo = logo_source WHERE logo LIKE '/api/static/%';

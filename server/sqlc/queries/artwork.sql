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

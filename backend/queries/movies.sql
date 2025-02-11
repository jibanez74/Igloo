-- name: GetMovieCount :one
SELECT COUNT(*) FROM movies;

-- name: GetMovieByTmdbID :one
SELECT id, tmdb_id FROM movies
WHERE tmdb_id = $1;

-- name: GetLatestMovies :many
SELECT 
    id,
    title,
    thumb,
    year
FROM movies
ORDER BY created_at DESC
LIMIT 12;

-- name: GetMoviesPaginated :many
SELECT 
    id,
    title,
    thumb,
    year,
    audience_rating,
    critic_rating
FROM movies
ORDER BY title ASC
LIMIT $1 OFFSET $2;

-- name: GetMovieDetails :one
SELECT 
    m.*,
    COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', g.id,
            'tag', g.tag
        )) FILTER (WHERE g.id IS NOT NULL), '[]'
    ) as genres,
    COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', s.id,
            'name', s.name,
            'logo', s.logo
        )) FILTER (WHERE s.id IS NOT NULL), '[]'
    ) as studios,
    COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', a_cast.id,
            'name', a_cast.name,
            'thumb', a_cast.thumb,
            'character', cl.character,
            'sort_order', cl.sort_order
        )) FILTER (WHERE a_cast.id IS NOT NULL), '[]'
    ) as cast,
    COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', a_crew.id,
            'name', a_crew.name,
            'thumb', a_crew.thumb,
            'job', cw.job,
            'department', cw.department
        )) FILTER (WHERE a_crew.id IS NOT NULL), '[]'
    ) as crew,
    COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', me.id,
            'title', me.title,
            'url', me.url,
            'kind', me.kind
        )) FILTER (WHERE me.id IS NOT NULL), '[]'
    ) as extras
FROM movies m
LEFT JOIN movie_genres mg ON mg.movie_id = m.id
LEFT JOIN genres g ON g.id = mg.genre_id
LEFT JOIN movie_studios ms ON ms.movie_id = m.id
LEFT JOIN studios s ON s.id = ms.studio_id
LEFT JOIN cast_list cl ON cl.movie_id = m.id
LEFT JOIN artists a_cast ON a_cast.id = cl.artist_id
LEFT JOIN crew_list cw ON cw.movie_id = m.id
LEFT JOIN artists a_crew ON a_crew.id = cw.artist_id
LEFT JOIN movie_extras me ON me.movie_id = m.id
WHERE m.id = $1
GROUP BY m.id;

-- name: CreateMovie :one
INSERT INTO movies (
    title,
    file_path,
    file_name,
    container,
    size,
    content_type,
    run_time,
    adult,
    tag_line,
    summary,
    art,
    thumb,
    tmdb_id,
    imdb_id,
    year,
    release_date,
    budget,
    revenue,
    content_rating,
    audience_rating,
    critic_rating,
    spoken_languages
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22
)
RETURNING *;

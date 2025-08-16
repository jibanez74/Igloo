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

-- name: GetMovieDetails :one
WITH movie_base AS (
    SELECT m.*
    FROM movies m
    WHERE m.id = $1
),
movie_genres AS (
    SELECT COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', g.id,
            'tag', g.tag
        )) FILTER (WHERE g.id IS NOT NULL), '[]'
    ) as genres
    FROM movie_base m
    LEFT JOIN movie_genres mg ON mg.movie_id = m.id
    LEFT JOIN genres g ON g.id = mg.genre_id
),
movie_studios AS (
    SELECT COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', s.id,
            'name', s.name,
            'logo', s.logo
        )) FILTER (WHERE s.id IS NOT NULL), '[]'
    ) as studios
    FROM movie_base m
    LEFT JOIN movie_studios ms ON ms.movie_id = m.id
    LEFT JOIN studios s ON s.id = ms.studio_id
),
movie_cast AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', a.id,
                'name', a.name,
                'thumb', a.thumb,
                'character', cl.character,
                'sort_order', cl.sort_order
            )
            ORDER BY cl.sort_order
        ) FILTER (WHERE a.id IS NOT NULL), '[]'
    ) as cast
    FROM movie_base m
    LEFT JOIN cast_list cl ON cl.movie_id = m.id
    LEFT JOIN artists a ON a.id = cl.artist_id
),
movie_crew AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', a.id,
                'name', a.name,
                'thumb', a.thumb,
                'job', cw.job,
                'department', cw.department
            )
        ) FILTER (WHERE a.id IS NOT NULL), '[]'
    ) as crew
    FROM movie_base m
    LEFT JOIN crew_list cw ON cw.movie_id = m.id
    LEFT JOIN artists a ON a.id = cw.artist_id
),
movie_extras AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', me.id,
                'title', me.title,
                'url', me.url,
                'kind', me.kind
            )
        ) FILTER (WHERE me.id IS NOT NULL), '[]'
    ) as extras
    FROM movie_base m
    LEFT JOIN movie_extras me ON me.movie_id = m.id
),
movie_streams AS (
    SELECT 
        COALESCE(
            json_agg(
                jsonb_build_object(
                    'id', vs.id,
                    'title', vs.title,
                    'index', vs.index,
                    'profile', vs.profile,
                    'aspect_ratio', vs.aspect_ratio,
                    'bit_rate', vs.bit_rate,
                    'bit_depth', vs.bit_depth,
                    'codec', vs.codec,
                    'width', vs.width,
                    'height', vs.height,
                    'coded_width', vs.coded_width,
                    'coded_height', vs.coded_height,
                    'color_transfer', vs.color_transfer,
                    'color_primaries', vs.color_primaries,
                    'color_space', vs.color_space,
                    'color_range', vs.color_range,
                    'frame_rate', vs.frame_rate,
                    'avg_frame_rate', vs.avg_frame_rate,
                    'level', vs.level
                )
            ) FILTER (WHERE vs.id IS NOT NULL), '[]'
        ) as video_streams,
        COALESCE(
            json_agg(
                jsonb_build_object(
                    'id', aus.id,
                    'title', aus.title,
                    'index', aus.index,
                    'codec', aus.codec,
                    'channels', aus.channels,
                    'channel_layout', aus.channel_layout,
                    'language', aus.language
                )
            ) FILTER (WHERE aus.id IS NOT NULL), '[]'
        ) as audio_streams,
        COALESCE(
            json_agg(
                jsonb_build_object(
                    'id', sub.id,
                    'title', sub.title,
                    'index', sub.index,
                    'codec', sub.codec,
                    'language', sub.language
                )
            ) FILTER (WHERE sub.id IS NOT NULL), '[]'
        ) as subtitles
    FROM movie_base m
    LEFT JOIN video_streams vs ON vs.movie_id = m.id
    LEFT JOIN audio_streams aus ON aus.movie_id = m.id
    LEFT JOIN subtitles sub ON sub.movie_id = m.id
),
movie_chapters AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', ch.id,
                'title', ch.title,
                'start_time_ms', ch.start_time_ms,
                'thumb', ch.thumb
            )
            ORDER BY ch.start_time_ms
        ) FILTER (WHERE ch.id IS NOT NULL), '[]'
    ) as chapters
    FROM movie_base m
    LEFT JOIN chapters ch ON ch.movie_id = m.id
)
SELECT 
    m.*,
    g.genres,
    s.studios,
    c.cast,
    cr.crew,
    e.extras,
    ms.video_streams,
    ms.audio_streams,
    ms.subtitles,
    ch.chapters
FROM movie_base m
CROSS JOIN movie_genres g
CROSS JOIN movie_studios s
CROSS JOIN movie_cast c
CROSS JOIN movie_crew cr
CROSS JOIN movie_extras e
CROSS JOIN movie_streams ms
CROSS JOIN movie_chapters ch;

-- name: GetMovieForStreaming :one
SELECT id, file_path, content_type, size FROM movies
WHERE id = $1;

-- name: CreateMovie :one
INSERT INTO movies (
    title,
    file_path,
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
    $21
)
RETURNING *;

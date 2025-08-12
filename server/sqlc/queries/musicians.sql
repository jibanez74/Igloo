-- name: GetOrCreateMusicianBySpotifyID :one
WITH existing_musician AS (
    SELECT m.id, m.created_at, m.updated_at, m.name, m.summary, m.spotify_id, m.spotify_popularity, m.spotify_followers_count 
    FROM musicians m
    WHERE m.spotify_id = $1
    LIMIT 1
), new_musician AS (
    INSERT INTO musicians (
        name,
        summary,
        spotify_id,
        spotify_popularity,
        spotify_followers_count
    )
    SELECT $2, $3, $1, $4, $5
    WHERE NOT EXISTS (SELECT 1 FROM existing_musician)
    RETURNING id, created_at, updated_at, name, summary, spotify_id, spotify_popularity, spotify_followers_count
)
SELECT e.id, e.created_at, e.updated_at, e.name, e.summary, e.spotify_id, e.spotify_popularity, e.spotify_followers_count 
FROM existing_musician e
UNION ALL
SELECT n.id, n.created_at, n.updated_at, n.name, n.summary, n.spotify_id, n.spotify_popularity, n.spotify_followers_count 
FROM new_musician n;

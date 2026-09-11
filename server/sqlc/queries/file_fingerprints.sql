-- name: UpsertTrackFileFingerprint :execrows
INSERT INTO track_file_fingerprints (track_id, mtime_ns, ctime_ns, device, inode, sha256)
SELECT id, sqlc.arg(mtime_ns), sqlc.arg(ctime_ns), sqlc.arg(device), sqlc.arg(inode), sqlc.arg(sha256)
FROM tracks
WHERE file_path = sqlc.arg(file_path)
ON CONFLICT (track_id) DO UPDATE SET
 mtime_ns = excluded.mtime_ns,
 ctime_ns = excluded.ctime_ns,
 device = excluded.device,
 inode = excluded.inode,
 sha256 = excluded.sha256;

-- name: UpsertMovieFileFingerprint :execrows
INSERT INTO movie_file_fingerprints (movie_id, mtime_ns, ctime_ns, device, inode)
SELECT id, sqlc.arg(mtime_ns), sqlc.arg(ctime_ns), sqlc.arg(device), sqlc.arg(inode)
FROM movies
WHERE file_path = sqlc.arg(file_path)
ON CONFLICT (movie_id) DO UPDATE SET
 mtime_ns = excluded.mtime_ns,
 ctime_ns = excluded.ctime_ns,
 device = excluded.device,
 inode = excluded.inode;

-- name: DeleteMovieRemuxSafetyVerdicts :exec
DELETE FROM remux_safety_verdicts WHERE movie_id = ?;

-- name: DeleteMovieKeyframeIndexes :exec
DELETE FROM keyframe_indexes WHERE movie_id = ?;

-- name: GetMovieFileFingerprint :one
SELECT m.size, f.mtime_ns, f.ctime_ns, f.device, f.inode
FROM movie_file_fingerprints f JOIN movies m ON m.id = f.movie_id
WHERE f.movie_id = ?;

-- name: ListLibraryMountPoints :many
SELECT mount_point FROM library_mount_points WHERE root = ? ORDER BY mount_point;

-- name: DeleteLibraryMountPoints :exec
DELETE FROM library_mount_points WHERE root = ?;

-- name: InsertLibraryMountPoint :exec
INSERT INTO library_mount_points (root, mount_point) VALUES (?, ?) ON CONFLICT DO NOTHING;

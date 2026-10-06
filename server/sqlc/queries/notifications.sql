-- name: CreateNotification :exec
INSERT INTO notifications (
  created_by_user_id,
  title,
  message
)
VALUES (?, ?, ?);

-- name: ListNotificationsForUser :many
-- The shared admin request queue, newest first. Visibility is admin-only and
-- enforced by the handlers; user_id here only selects the viewer's read state
-- from notification_reads.
SELECT
  n.id,
  n.title,
  n.message,
  n.created_at,
  creator.name AS created_by_name,
  CAST((nr.notification_id IS NOT NULL) AS BOOLEAN) AS is_read
FROM notifications AS n
INNER JOIN users AS creator
  ON creator.id = n.created_by_user_id
LEFT JOIN notification_reads AS nr
  ON nr.notification_id = n.id
  AND nr.user_id = sqlc.arg(user_id)
ORDER BY n.created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: MarkNotificationReadForUser :exec
-- Idempotent: marking an already-read or nonexistent notification is a no-op.
INSERT INTO notification_reads (notification_id, user_id)
SELECT n.id, sqlc.arg(user_id)
FROM notifications AS n
WHERE n.id = sqlc.arg(notification_id)
ON CONFLICT (notification_id, user_id) DO NOTHING;

-- name: MarkAllNotificationsReadForUser :exec
-- SQLite needs a WHERE clause before an upsert's ON CONFLICT in INSERT ... SELECT,
-- or it parses ON as a join constraint.
INSERT INTO notification_reads (notification_id, user_id)
SELECT n.id, sqlc.arg(user_id)
FROM notifications AS n
WHERE true
ON CONFLICT (notification_id, user_id) DO NOTHING;

-- name: DeleteNotificationForUser :execrows
DELETE FROM notifications
WHERE id = sqlc.arg(notification_id);

-- name: GetNotificationBadgeForUser :one
-- The viewer's admin flag and unread count in one round trip. The client polls
-- the badge endpoint, the list endpoint needs both values too, and the database
-- runs on a single shared connection (InitDB), so the admin check and the count
-- share one statement. The queue is admin-only, so a non-admin short-circuits
-- to 0 without touching notifications at all. No rows means the session
-- outlived its user, which the handlers treat as a stale session.
SELECT
  u.is_admin,
  CASE
    WHEN u.is_admin THEN (
      SELECT COUNT(*)
      FROM notifications AS n
      WHERE NOT EXISTS (
        SELECT 1
        FROM notification_reads AS nr
        WHERE nr.notification_id = n.id
          AND nr.user_id = u.id
      )
    )
    ELSE 0
  END AS unread_count
FROM users AS u
WHERE u.id = sqlc.arg(user_id);

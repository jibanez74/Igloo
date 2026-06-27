-- name: CreateNotification :one
INSERT INTO notifications (
  created_by_user_id,
  user_id,
  title,
  message,
  is_admin
)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListNotificationsForUser :many
-- Notifications visible to the viewer: those targeted at them, plus the admin
-- request queue when the viewer is an admin. Read state comes from a left join
-- against notification_reads for this viewer.
SELECT
  n.id,
  n.created_by_user_id,
  n.user_id,
  n.title,
  n.message,
  n.is_admin,
  n.created_at,
  n.updated_at,
  creator.name AS created_by_name,
  CAST((nr.notification_id IS NOT NULL) AS BOOLEAN) AS is_read
FROM notifications AS n
INNER JOIN users AS creator
  ON creator.id = n.created_by_user_id
LEFT JOIN notification_reads AS nr
  ON nr.notification_id = n.id
  AND nr.user_id = sqlc.arg(user_id)
WHERE n.user_id = sqlc.arg(user_id)
  OR (sqlc.arg(viewer_is_admin) AND n.is_admin = true)
ORDER BY n.created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: CountUnreadNotificationsForUser :one
SELECT
  COUNT(*) AS unread_count
FROM notifications AS n
LEFT JOIN notification_reads AS nr
  ON nr.notification_id = n.id
  AND nr.user_id = sqlc.arg(user_id)
WHERE nr.notification_id IS NULL
  AND (
    n.user_id = sqlc.arg(user_id)
    OR (sqlc.arg(viewer_is_admin) AND n.is_admin = true)
  );

-- name: MarkNotificationReadForUser :exec
-- Idempotent and relevance-gated: only records a read when the notification is
-- actually visible to the viewer.
INSERT INTO notification_reads (notification_id, user_id)
SELECT n.id, sqlc.arg(user_id)
FROM notifications AS n
WHERE n.id = sqlc.arg(notification_id)
  AND (
    n.user_id = sqlc.arg(user_id)
    OR (sqlc.arg(viewer_is_admin) AND n.is_admin = true)
  )
ON CONFLICT (notification_id, user_id) DO NOTHING;

-- name: MarkAllNotificationsReadForUser :exec
INSERT INTO notification_reads (notification_id, user_id)
SELECT n.id, sqlc.arg(user_id)
FROM notifications AS n
WHERE n.user_id = sqlc.arg(user_id)
  OR (sqlc.arg(viewer_is_admin) AND n.is_admin = true)
ON CONFLICT (notification_id, user_id) DO NOTHING;

-- name: DeleteNotificationForUser :execrows
DELETE FROM notifications
WHERE id = sqlc.arg(notification_id)
  AND (
    user_id = sqlc.arg(user_id)
    OR (sqlc.arg(viewer_is_admin) AND is_admin = true)
  );

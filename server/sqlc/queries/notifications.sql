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

-- name: ListVisibleNotifications :many
SELECT
  n.id,
  n.created_by_user_id,
  n.user_id,
  n.title,
  n.message,
  n.is_admin,
  n.created_at,
  n.updated_at,
  EXISTS (
    SELECT 1
    FROM notification_reads AS nr
    WHERE nr.notification_id = n.id
      AND nr.user_id = sqlc.arg(user_id)
  ) AS is_read
FROM notifications AS n
WHERE
  n.user_id = sqlc.arg(user_id)
  OR (n.user_id IS NULL AND n.is_admin = false)
  OR (n.is_admin = true AND n.is_admin = sqlc.arg(is_admin))
ORDER BY n.created_at DESC, n.id DESC
LIMIT sqlc.arg(limit) OFFSET sqlc.arg(offset);

-- name: GetVisibleNotification :one
SELECT
  n.id,
  n.created_by_user_id,
  n.user_id,
  n.title,
  n.message,
  n.is_admin,
  n.created_at,
  n.updated_at,
  EXISTS (
    SELECT 1
    FROM notification_reads AS nr
    WHERE nr.notification_id = n.id
      AND nr.user_id = sqlc.arg(user_id)
  ) AS is_read
FROM notifications AS n
WHERE n.id = sqlc.arg(id)
  AND (
    n.user_id = sqlc.arg(user_id)
    OR (n.user_id IS NULL AND n.is_admin = false)
    OR (n.is_admin = true AND n.is_admin = sqlc.arg(is_admin))
  )
LIMIT 1;

-- name: MarkNotificationRead :exec
INSERT INTO notification_reads (
  notification_id,
  user_id
)
SELECT
  n.id,
  sqlc.arg(user_id)
FROM notifications AS n
WHERE n.id = sqlc.arg(notification_id)
  AND (
    n.user_id = sqlc.arg(user_id)
    OR (n.user_id IS NULL AND n.is_admin = false)
    OR (n.is_admin = true AND n.is_admin = sqlc.arg(is_admin))
  )
ON CONFLICT (notification_id, user_id) DO NOTHING;

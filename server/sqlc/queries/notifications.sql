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

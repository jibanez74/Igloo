-- name: CreateDeviceCode :one
INSERT INTO device_codes (
    device_code,
    user_code,
    expires_at
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetDeviceCode :one
SELECT * FROM device_codes
WHERE device_code = $1;

-- name: GetDeviceCodeByUserCode :one
SELECT * FROM device_codes
WHERE user_code = $1;

-- name: VerifyDeviceCode :exec
UPDATE device_codes
SET is_verified = true,
    user_id = $1
WHERE user_code = $2;

-- name: CleanupExpiredDeviceCodes :exec
DELETE FROM device_codes
WHERE expires_at <= CURRENT_TIMESTAMP; 
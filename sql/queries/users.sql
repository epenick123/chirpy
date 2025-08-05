-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: GetUserByID :one
SELECT id, created_at, updated_at, email, is_chirpy_red
FROM users
WHERE id = $1;

-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES ($1, NOW(), NOW(), $2, $3)
RETURNING *;

-- name: GetChirps :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: GetChirp :one
SELECT * FROM chirps
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token = $1 AND revoked_at IS NULL AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;

-- name: UpdateEmailAndPassword :exec
UPDATE users
SET email = $1,
    hashed_password = $2
WHERE id = $3;

-- name: DeleteAllRefreshTokens :exec
DELETE FROM refresh_tokens;

-- name: DeleteChirpByID :exec
DELETE FROM chirps
WHERE id = $1 AND user_id = $2;

-- name: UpgradeToChirpyRed :exec
UPDATE users
SET is_chirpy_red = TRUE
WHERE id = $1;
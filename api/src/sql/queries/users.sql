-- ─── Admin users ─────────────────────────────────────────────────────────────

-- name: GetUserByEmail :one
SELECT *
FROM "User"
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT *
FROM "User"
WHERE id = $1
LIMIT 1;

-- name: CreateUser :one
INSERT INTO "User" (
    "id", "name", "email", "password", "role", "createdAt", "updatedAt"
) VALUES (
    $1, $2, $3, $4, $5, $6, $6
)
RETURNING *;

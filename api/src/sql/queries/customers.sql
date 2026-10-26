-- ─── Customers ───────────────────────────────────────────────────────────────

-- name: GetCustomerByEmail :one
SELECT *
FROM "Customer"
WHERE email = $1
LIMIT 1;

-- name: GetCustomerByID :one
SELECT *
FROM "Customer"
WHERE id = $1
LIMIT 1;

-- name: GetCustomerByValidResetToken :one
SELECT *
FROM "Customer"
WHERE "resetToken" = $1
  AND "resetTokenExpiresAt" > $2
LIMIT 1;

-- name: CreateCustomer :one
INSERT INTO "Customer" (
    "id", "email", "name", "isActive", "createdAt", "updatedAt"
) VALUES (
    $1, $2, $3, true, $4, $4
)
RETURNING *;

-- name: UpsertCustomerByEmail :one
INSERT INTO "Customer" (
    "id", "email", "name", "isActive", "createdAt", "updatedAt"
) VALUES (
    $1, $2, $3, true, $4, $4
)
ON CONFLICT (email) DO UPDATE
    SET email = EXCLUDED.email
RETURNING *;

-- name: UpdateCustomerPassword :one
UPDATE "Customer"
SET password = $2,
    "updatedAt" = $3
WHERE id = $1
RETURNING *;

-- name: SetCustomerResetToken :one
UPDATE "Customer"
SET "resetToken" = $2,
    "resetTokenExpiresAt" = $3,
    "updatedAt" = $4
WHERE id = $1
RETURNING *;

-- name: ResetCustomerPassword :one
UPDATE "Customer"
SET password = $2,
    "resetToken" = NULL,
    "resetTokenExpiresAt" = NULL,
    "updatedAt" = $3
WHERE id = $1
RETURNING *;

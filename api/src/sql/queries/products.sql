-- ─── Products ────────────────────────────────────────────────────────────────

-- name: ListProducts :many
SELECT *
FROM "Product"
WHERE (sqlc.narg('category')::text IS NULL OR category::text = sqlc.narg('category'))
  AND (sqlc.narg('is_active')::boolean IS NULL OR "isActive" = sqlc.narg('is_active'))
ORDER BY "createdAt" DESC;

-- name: GetProductByID :one
SELECT *
FROM "Product"
WHERE id = $1
LIMIT 1;

-- name: CreateProduct :one
INSERT INTO "Product" (
    "id", "title", "description", "price", "coverUrl", "fileUrl",
    "category", "isActive", "createdAt", "updatedAt"
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $9
)
RETURNING *;

-- name: UpdateProduct :one
UPDATE "Product"
SET "title" = $2,
    "description" = $3,
    "price" = $4,
    "coverUrl" = $5,
    "fileUrl" = $6,
    "category" = $7,
    "isActive" = $8,
    "updatedAt" = $9
WHERE id = $1
RETURNING *;

-- name: ToggleProduct :one
UPDATE "Product"
SET "isActive" = NOT "isActive",
    "updatedAt" = $2
WHERE id = $1
RETURNING *;

-- name: DeleteProduct :one
DELETE FROM "Product"
WHERE id = $1
RETURNING *;

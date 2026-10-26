-- ─── Purchases ───────────────────────────────────────────────────────────────

-- name: GetPurchaseByPaymentID :one
SELECT *
FROM "Purchase"
WHERE "mercadoPagoPaymentId" = $1
LIMIT 1;

-- name: GetPurchaseForCustomer :one
SELECT *
FROM "Purchase"
WHERE "customerId" = $1
  AND "productId" = $2
LIMIT 1;

-- name: CreatePurchase :one
INSERT INTO "Purchase" (
    "id", "mercadoPagoPaymentId", "amountPaid", "purchasedAt",
    "customerId", "productId", "createdAt"
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT ("mercadoPagoPaymentId") DO NOTHING
RETURNING *;

-- name: ListCustomerPurchases :many
SELECT
    p."purchasedAt" AS "purchasedAt",
    pr.id AS "id",
    pr.title AS "title",
    pr.description AS "description",
    pr."coverUrl" AS "coverUrl",
    pr."fileUrl" AS "fileUrl",
    pr.category AS "category"
FROM "Purchase" p
INNER JOIN "Product" pr ON pr.id = p."productId"
WHERE p."customerId" = $1
ORDER BY p."purchasedAt" DESC;

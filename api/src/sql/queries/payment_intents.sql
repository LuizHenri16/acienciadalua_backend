-- ─── Payment intents (PIX) ───────────────────────────────────────────────────

-- name: UpsertPaymentIntent :one
INSERT INTO "PaymentIntent" (
    "id", "mercadoPagoPaymentId", "productId", "customerEmail", "customerName",
    "amount", "status", "statusDetail", "qrCode", "qrCodeBase64", "ticketUrl",
    "expiresAt", "idempotencyKey", "lastCheckedAt", "createdAt", "updatedAt"
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
ON CONFLICT ("idempotencyKey") DO UPDATE SET
    "mercadoPagoPaymentId" = EXCLUDED."mercadoPagoPaymentId",
    "status" = EXCLUDED."status",
    "statusDetail" = EXCLUDED."statusDetail",
    "qrCode" = EXCLUDED."qrCode",
    "qrCodeBase64" = EXCLUDED."qrCodeBase64",
    "ticketUrl" = EXCLUDED."ticketUrl",
    "expiresAt" = EXCLUDED."expiresAt",
    "lastCheckedAt" = EXCLUDED."lastCheckedAt",
    "updatedAt" = EXCLUDED."updatedAt"
RETURNING *;

-- name: GetPaymentIntentByPaymentID :one
SELECT *
FROM "PaymentIntent"
WHERE "mercadoPagoPaymentId" = $1
LIMIT 1;

-- name: GetPaymentIntentByIdempotencyKey :one
SELECT *
FROM "PaymentIntent"
WHERE "idempotencyKey" = $1
LIMIT 1;

-- name: GetPaymentIntentByID :one
SELECT *
FROM "PaymentIntent"
WHERE "id" = $1
LIMIT 1;

-- name: UpdatePaymentIntentStatus :one
UPDATE "PaymentIntent"
SET "status" = $2,
    "statusDetail" = $3,
    "approvedAt" = $4,
    "lastCheckedAt" = $5,
    "updatedAt" = $5
WHERE "id" = $1
RETURNING *;

-- name: LinkPaymentIntentPurchase :one
UPDATE "PaymentIntent"
SET "purchaseId" = $2,
    "status" = 'approved',
    "approvedAt" = COALESCE("approvedAt", $3),
    "lastCheckedAt" = $3,
    "updatedAt" = $3
WHERE "id" = $1
RETURNING *;

-- name: ListPendingPaymentIntents :many
SELECT *
FROM "PaymentIntent"
WHERE "status" = 'pending'
  AND ("lastCheckedAt" IS NULL OR "lastCheckedAt" < $1)
  AND "createdAt" > $2
ORDER BY "createdAt" ASC
LIMIT $3;

-- name: DeleteOldPaymentIntents :exec
DELETE FROM "PaymentIntent"
WHERE "createdAt" < $1;

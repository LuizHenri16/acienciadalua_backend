-- ─── Admin panel (analytics) ─────────────────────────────────────────────────
-- All list queries are bounded: the panel shows at most 200 rows per table.

-- name: ListAdminSales :many
SELECT
    p.id AS "saleId",
    p."mercadoPagoPaymentId" AS "externalPaymentId",
    p."customerId",
    c.name AS "customerName",
    c.email AS "customerEmail",
    p."productId",
    pr.title AS "productTitle",
    pr.category::text AS "productCategory",
    p."amountPaid",
    pi.status AS "intentStatus",
    pi."statusDetail" AS "intentStatusDetail",
    pi."createdAt" AS "intentCreatedAt",
    p."purchasedAt"
FROM "Purchase" p
JOIN "Customer" c ON c.id = p."customerId"
JOIN "Product" pr ON pr.id = p."productId"
LEFT JOIN "PaymentIntent" pi ON pi."purchaseId" = p.id
ORDER BY p."purchasedAt" DESC
LIMIT 200;

-- Unlinked intents are shown as sales without access granted.
-- name: ListAdminPendingIntents :many
SELECT
    pi.id AS "intentId",
    pi."mercadoPagoPaymentId" AS "externalPaymentId",
    COALESCE(c.id, '') AS "customerId",
    COALESCE(c.name, pi."customerName") AS "customerName",
    COALESCE(c.email, pi."customerEmail") AS "customerEmail",
    pi."productId",
    pr.title AS "productTitle",
    pr.category::text AS "productCategory",
    pi.amount,
    pi.status AS "intentStatus",
    pi."statusDetail" AS "intentStatusDetail",
    pi."createdAt" AS "intentCreatedAt"
FROM "PaymentIntent" pi
JOIN "Product" pr ON pr.id = pi."productId"
LEFT JOIN "Customer" c ON c.email = pi."customerEmail"
WHERE pi."purchaseId" IS NULL
ORDER BY pi."createdAt" DESC
LIMIT 200;

-- name: CountAdminPendingSales :one
SELECT COUNT(*)::bigint AS count
FROM "PaymentIntent"
WHERE "purchaseId" IS NULL
  AND status IN ('pending', 'in_process', 'authorized', 'in_mediation');

-- name: ListAdminCustomers :many
SELECT
    c.id,
    c.name,
    c.email,
    c."createdAt",
    COUNT(p.id)::bigint AS "purchasesCount",
    COALESCE(SUM(p."amountPaid") FILTER (WHERE pi.status IS NULL OR pi.status = 'approved'), 0::numeric)::numeric AS "totalSpent",
    MAX(p."purchasedAt")::timestamp AS "lastPurchaseAt"
FROM "Customer" c
LEFT JOIN "Purchase" p ON p."customerId" = c.id
LEFT JOIN "PaymentIntent" pi ON pi."purchaseId" = p.id
GROUP BY c.id, c.name, c.email, c."createdAt"
ORDER BY c."createdAt" DESC
LIMIT 200;

-- name: ListAdminCustomerPurchases :many
SELECT
    p."customerId",
    p.id AS "saleId",
    p."productId",
    pr.title AS "productTitle",
    pr.category::text AS "productCategory",
    p."amountPaid",
    p."purchasedAt",
    pi.status AS "intentStatus"
FROM "Purchase" p
JOIN "Product" pr ON pr.id = p."productId"
LEFT JOIN "PaymentIntent" pi ON pi."purchaseId" = p.id
WHERE p."customerId" = ANY(sqlc.arg('customer_ids')::text[])
ORDER BY p."purchasedAt" DESC;

-- name: CountAdminSalesByProduct :many
SELECT "productId", COUNT(*)::bigint AS count
FROM "Purchase"
GROUP BY "productId"
ORDER BY count DESC, "productId" ASC;

-- name: CountAdminCustomersAll :one
SELECT COUNT(*)::bigint AS count FROM "Customer";

-- name: AdminProductCounts :one
SELECT
    COUNT(*)::bigint AS total,
    COUNT(*) FILTER (WHERE "isActive")::bigint AS active
FROM "Product";

-- Aggregate stats for one date window. Payment statuses are normalized here in
-- the same way as in AdminService; non-PIX statuses are approximated because
-- there is no ledger of rejected/refunded checkout_pro payments.
-- name: AdminPeriodStats :one
SELECT
    COUNT(*)::bigint AS "totalSalesCount",
    COALESCE(SUM(amount) FILTER (WHERE status IN ('approved', 'pending')), 0::numeric)::numeric AS "totalRevenue",
    COUNT(*) FILTER (WHERE status = 'approved')::bigint AS "approvedCount",
    COUNT(*) FILTER (WHERE status = 'pending')::bigint AS "pendingCount",
    COUNT(*) FILTER (WHERE status = 'rejected')::bigint AS "rejectedCount",
    COUNT(*) FILTER (WHERE status = 'cancelled')::bigint AS "cancelledCount",
    COUNT(*) FILTER (WHERE status = 'refunded')::bigint AS "refundsCount",
    (SELECT COUNT(*)::bigint FROM "Customer" c
     WHERE c."createdAt" >= sqlc.arg('from') AND c."createdAt" < sqlc.arg('to')) AS "newCustomersCount"
FROM (
    SELECT
        p."amountPaid" AS amount,
        CASE
            WHEN pi.status IS NULL THEN 'approved'
            WHEN pi.status IN ('pending', 'in_process', 'authorized', 'in_mediation') THEN 'pending'
            WHEN pi.status IN ('rejected', 'cancelled', 'error') THEN 'rejected'
            WHEN pi.status = 'expired' THEN 'cancelled'
            WHEN pi.status IN ('refunded', 'charged_back') THEN 'refunded'
            ELSE 'approved'
        END AS status
    FROM "Purchase" p
    LEFT JOIN "PaymentIntent" pi ON pi."purchaseId" = p.id
    WHERE p."purchasedAt" >= sqlc.arg('from') AND p."purchasedAt" < sqlc.arg('to')
    UNION ALL
    SELECT
        pi.amount,
        CASE
            WHEN pi.status IN ('pending', 'in_process', 'authorized', 'in_mediation') THEN 'pending'
            WHEN pi.status IN ('rejected', 'cancelled', 'error') THEN 'rejected'
            WHEN pi.status = 'expired' THEN 'cancelled'
            WHEN pi.status IN ('refunded', 'charged_back') THEN 'refunded'
            ELSE 'approved'
        END AS status
    FROM "PaymentIntent" pi
    WHERE pi."purchaseId" IS NULL
      AND pi."createdAt" >= sqlc.arg('from') AND pi."createdAt" < sqlc.arg('to')
) sales;

-- name: AdminDailySeries :many
-- Start at the truncated lower bound so the partial first day is present in
-- the chart, matching the summary window (purchasedAt >= from).
WITH buckets AS (
    SELECT generate_series(
        date_trunc('day', sqlc.arg('from')::timestamp),
        date_trunc('day', sqlc.arg('to')::timestamp),
        interval '1 day'
    )::timestamp AS bucket
),
sales AS (
    SELECT
        date_trunc('day', at) AS bucket,
        SUM(amount) AS revenue,
        COUNT(*)::bigint AS sales_count
    FROM (
        SELECT
            p."purchasedAt" AS at,
            p."amountPaid" AS amount,
            CASE
                WHEN pi.status IS NULL THEN 'approved'
                WHEN pi.status IN ('pending', 'in_process', 'authorized', 'in_mediation') THEN 'pending'
                WHEN pi.status IN ('rejected', 'cancelled', 'error', 'expired', 'refunded', 'charged_back') THEN 'ignored'
                ELSE 'approved'
            END::text AS status
        FROM "Purchase" p
        LEFT JOIN "PaymentIntent" pi ON pi."purchaseId" = p.id
        UNION ALL
        SELECT
            pi."createdAt" AS at,
            pi.amount AS amount,
            pi.status::text AS status
        FROM "PaymentIntent" pi
        WHERE pi."purchaseId" IS NULL
    ) entries
    WHERE at >= sqlc.arg('from') AND at < sqlc.arg('to')
      AND status IN ('approved', 'pending')
    GROUP BY 1
),
customers AS (
    SELECT date_trunc('day', "createdAt") AS bucket, COUNT(*)::bigint AS customer_count
    FROM "Customer"
    WHERE "createdAt" >= sqlc.arg('from') AND "createdAt" < sqlc.arg('to')
    GROUP BY 1
)
SELECT
    b.bucket AS "bucket",
    COALESCE(s.revenue, 0::numeric)::numeric AS "revenue",
    COALESCE(s.sales_count, 0)::bigint AS "salesCount",
    COALESCE(c.customer_count, 0)::bigint AS "newCustomers"
FROM buckets b
LEFT JOIN sales s ON s.bucket = b.bucket
LEFT JOIN customers c ON c.bucket = b.bucket
ORDER BY b.bucket;

-- name: AdminMonthlySeries :many
-- Same boundary rule as AdminDailySeries: keep the partial first month.
WITH buckets AS (
    SELECT generate_series(
        date_trunc('month', sqlc.arg('from')::timestamp),
        date_trunc('month', sqlc.arg('to')::timestamp),
        interval '1 month'
    )::timestamp AS bucket
),
sales AS (
    SELECT
        date_trunc('month', at) AS bucket,
        SUM(amount) AS revenue,
        COUNT(*)::bigint AS sales_count
    FROM (
        SELECT
            p."purchasedAt" AS at,
            p."amountPaid" AS amount,
            CASE
                WHEN pi.status IS NULL THEN 'approved'
                WHEN pi.status IN ('pending', 'in_process', 'authorized', 'in_mediation') THEN 'pending'
                WHEN pi.status IN ('rejected', 'cancelled', 'error', 'expired', 'refunded', 'charged_back') THEN 'ignored'
                ELSE 'approved'
            END::text AS status
        FROM "Purchase" p
        LEFT JOIN "PaymentIntent" pi ON pi."purchaseId" = p.id
        UNION ALL
        SELECT
            pi."createdAt" AS at,
            pi.amount AS amount,
            pi.status::text AS status
        FROM "PaymentIntent" pi
        WHERE pi."purchaseId" IS NULL
    ) entries
    WHERE at >= sqlc.arg('from') AND at < sqlc.arg('to')
      AND status IN ('approved', 'pending')
    GROUP BY 1
),
customers AS (
    SELECT date_trunc('month', "createdAt") AS bucket, COUNT(*)::bigint AS customer_count
    FROM "Customer"
    WHERE "createdAt" >= sqlc.arg('from') AND "createdAt" < sqlc.arg('to')
    GROUP BY 1
)
SELECT
    b.bucket AS "bucket",
    COALESCE(s.revenue, 0::numeric)::numeric AS "revenue",
    COALESCE(s.sales_count, 0)::bigint AS "salesCount",
    COALESCE(c.customer_count, 0)::bigint AS "newCustomers"
FROM buckets b
LEFT JOIN sales s ON s.bucket = b.bucket
LEFT JOIN customers c ON c.bucket = b.bucket
ORDER BY b.bucket;

-- name: AdminTopProducts :many
SELECT
    pr.id,
    pr.title,
    pr.category::text AS "productCategory",
    pr.price,
    COUNT(p.id)::bigint AS "salesCount",
    COALESCE(SUM(p."amountPaid"), 0::numeric)::numeric AS "revenue"
FROM "Purchase" p
JOIN "Product" pr ON pr.id = p."productId"
WHERE p."purchasedAt" >= sqlc.arg('from') AND p."purchasedAt" < sqlc.arg('to')
GROUP BY pr.id, pr.title, pr.category, pr.price
ORDER BY "salesCount" DESC, "revenue" DESC, pr.id ASC
LIMIT 5;

-- name: ListAdminActivity :many
SELECT id, type, title, description, "createdAt"
FROM (
    SELECT
        p.id,
        'sale_approved'::text AS type,
        'Nova venda aprovada'::text AS title,
        ('Venda concluída: ' || pr.title)::text AS description,
        p."purchasedAt" AS "createdAt"
    FROM "Purchase" p
    JOIN "Product" pr ON pr.id = p."productId"
    UNION ALL
    SELECT
        pi.id,
        'sale_refunded'::text,
        'Reembolso processado',
        ('Reembolso de ' || pi."customerName" || ' — ' || pr.title)::text,
        COALESCE(pi."approvedAt", pi."createdAt")
    FROM "PaymentIntent" pi
    JOIN "Product" pr ON pr.id = pi."productId"
    WHERE pi.status IN ('refunded', 'charged_back')
    UNION ALL
    SELECT
        pi.id,
        'payment_pending'::text,
        'Pagamento pendente',
        ('PIX de ' || pi."customerName" || ' aguardando confirmação')::text,
        pi."createdAt"
    FROM "PaymentIntent" pi
    WHERE pi."purchaseId" IS NULL
      AND pi.status IN ('pending', 'in_process', 'authorized', 'in_mediation')
    UNION ALL
    SELECT
        c.id,
        'customer_registered'::text,
        'Novo cliente cadastrado',
        (c.name || ' criou uma conta na plataforma')::text,
        c."createdAt"
    FROM "Customer" c
    UNION ALL
    SELECT
        pr.id,
        'product_updated'::text,
        'Produto atualizado',
        ('Produto atualizado: ' || pr.title)::text,
        pr."updatedAt"
    FROM "Product" pr
) feed
ORDER BY "createdAt" DESC
LIMIT 5;

-- name: AdminLastWebhookCheck :one
SELECT MAX("lastCheckedAt")::timestamp AS "lastCheckedAt" FROM "PaymentIntent";

-- name: GetAdminSetting :one
SELECT data FROM "AdminSetting" WHERE key = sqlc.arg('key') LIMIT 1;

-- name: UpsertAdminSetting :one
INSERT INTO "AdminSetting" (key, data)
VALUES (sqlc.arg('key'), sqlc.arg('data'))
ON CONFLICT (key) DO UPDATE SET data = EXCLUDED.data
RETURNING data;

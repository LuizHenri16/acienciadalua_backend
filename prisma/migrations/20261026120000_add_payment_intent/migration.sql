-- Additive table created by the Go API (api/src/migrations/000002_payment_intent).
-- The statement is idempotent so `prisma migrate deploy` is safe on databases
-- where golang-migrate already created the table.

CREATE TABLE IF NOT EXISTS "PaymentIntent" (
    "id" TEXT NOT NULL,
    "mercadoPagoPaymentId" TEXT NOT NULL,
    "productId" TEXT NOT NULL,
    "customerEmail" TEXT NOT NULL,
    "customerName" TEXT NOT NULL,
    "amount" DECIMAL(65,30) NOT NULL,
    "status" TEXT NOT NULL,
    "statusDetail" TEXT NOT NULL DEFAULT '',
    "qrCode" TEXT,
    "qrCodeBase64" TEXT,
    "ticketUrl" TEXT,
    "expiresAt" TIMESTAMP(3),
    "approvedAt" TIMESTAMP(3),
    "purchaseId" TEXT,
    "idempotencyKey" TEXT NOT NULL,
    "lastCheckedAt" TIMESTAMP(3),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "PaymentIntent_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "PaymentIntent_mercadoPagoPaymentId_key"
    ON "PaymentIntent"("mercadoPagoPaymentId");
CREATE UNIQUE INDEX IF NOT EXISTS "PaymentIntent_idempotencyKey_key"
    ON "PaymentIntent"("idempotencyKey");
CREATE UNIQUE INDEX IF NOT EXISTS "PaymentIntent_purchaseId_key"
    ON "PaymentIntent"("purchaseId");
CREATE INDEX IF NOT EXISTS "PaymentIntent_status_lastCheckedAt_idx"
    ON "PaymentIntent"("status", "lastCheckedAt");
CREATE INDEX IF NOT EXISTS "PaymentIntent_customerEmail_idx"
    ON "PaymentIntent"("customerEmail");
CREATE INDEX IF NOT EXISTS "PaymentIntent_productId_idx"
    ON "PaymentIntent"("productId");

DO $$
BEGIN
    ALTER TABLE "PaymentIntent"
        ADD CONSTRAINT "PaymentIntent_productId_fkey"
        FOREIGN KEY ("productId") REFERENCES "Product"("id")
        ON DELETE RESTRICT ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

DO $$
BEGIN
    ALTER TABLE "PaymentIntent"
        ADD CONSTRAINT "PaymentIntent_purchaseId_fkey"
        FOREIGN KEY ("purchaseId") REFERENCES "Purchase"("id")
        ON DELETE SET NULL ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

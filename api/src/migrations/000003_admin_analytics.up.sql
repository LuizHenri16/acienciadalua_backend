-- ─── Admin analytics ─────────────────────────────────────────────────────────
-- Additive migration for the admin dashboard: a single-row settings document
-- plus indexes supporting the analytics date-range scans. No existing table is
-- restructured and no existing row is rewritten.

CREATE TABLE IF NOT EXISTS "AdminSetting" (
    "key" TEXT NOT NULL,
    "data" JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT "AdminSetting_pkey" PRIMARY KEY ("key")
);

INSERT INTO "AdminSetting" ("key", "data")
VALUES ('default', '{
    "storeName": "A Ciência da Lua",
    "supportEmail": "contato@acienciadalua.com.br",
    "storefrontEnabled": true,
    "autoReleaseOnApproval": true,
    "notifyOnNewSale": true,
    "notifyOnPendingPix": true,
    "mercadoPagoMode": "production",
    "webhookUrl": ""
}'::jsonb)
ON CONFLICT ("key") DO NOTHING;

CREATE INDEX IF NOT EXISTS "Purchase_purchasedAt_idx" ON "Purchase"("purchasedAt" DESC);
CREATE INDEX IF NOT EXISTS "Customer_createdAt_idx" ON "Customer"("createdAt" DESC);

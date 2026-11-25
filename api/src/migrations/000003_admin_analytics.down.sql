-- Reverts only the structures introduced by 000003_admin_analytics.up.sql.
DROP INDEX IF EXISTS "Customer_createdAt_idx";
DROP INDEX IF EXISTS "Purchase_purchasedAt_idx";
DROP TABLE IF EXISTS "AdminSetting";

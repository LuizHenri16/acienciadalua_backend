-- Baseline schema equivalent to the final Prisma schema (prisma/schema.prisma).
--
-- This migration is intentionally idempotent so it can be applied on top of an
-- existing database that was previously managed by Prisma (`prisma migrate
-- deploy`). On a fresh database it creates the exact same structures.
--
-- Types/columns match the Prisma-generated DDL (TEXT ids, TIMESTAMP(3),
-- DECIMAL(65,30), the "Category" enum and Prisma relation names).

-- ─── enums ───────────────────────────────────────────────────────────────────

DO $$
BEGIN
    CREATE TYPE "Category" AS ENUM ('STUDENT', 'TEACHER');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

-- ─── Admin ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS "User" (
    "id" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "email" TEXT NOT NULL,
    "password" TEXT NOT NULL,
    "role" TEXT NOT NULL DEFAULT 'collaborator',
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL DEFAULT null,
    "deletedAt" TIMESTAMP(3) DEFAULT null,

    CONSTRAINT "User_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "User_email_key" ON "User"("email");

-- ─── Customers (alunos / professores) ────────────────────────────────────────

CREATE TABLE IF NOT EXISTS "Customer" (
    "id" TEXT NOT NULL,
    "email" TEXT NOT NULL,
    "name" TEXT NOT NULL,
    "isActive" BOOLEAN NOT NULL DEFAULT true,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,
    "password" TEXT,
    "resetToken" TEXT,
    "resetTokenExpiresAt" TIMESTAMP(3),

    CONSTRAINT "Customer_pkey" PRIMARY KEY ("id")
);

-- Columns added by later Prisma migrations. `IF NOT EXISTS` keeps this file
-- safe for databases created before those migrations existed.
ALTER TABLE "Customer" ADD COLUMN IF NOT EXISTS "password" TEXT;
ALTER TABLE "Customer" ADD COLUMN IF NOT EXISTS "resetToken" TEXT;
ALTER TABLE "Customer" ADD COLUMN IF NOT EXISTS "resetTokenExpiresAt" TIMESTAMP(3);

CREATE UNIQUE INDEX IF NOT EXISTS "Customer_email_key" ON "Customer"("email");
CREATE UNIQUE INDEX IF NOT EXISTS "Customer_resetToken_key" ON "Customer"("resetToken");

-- ─── Magic Link ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS "AuthToken" (
    "id" SERIAL NOT NULL,
    "token" TEXT NOT NULL,
    "expiresAt" TIMESTAMP(3) NOT NULL,
    "usedAt" TIMESTAMP(3),

    "customerId" TEXT NOT NULL,

    CONSTRAINT "AuthToken_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "AuthToken_token_key" ON "AuthToken"("token");
CREATE INDEX IF NOT EXISTS "AuthToken_token_idx" ON "AuthToken"("token");

-- ─── Vitrine / Produtos ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS "Product" (
    "id" TEXT NOT NULL,
    "title" TEXT NOT NULL,
    "description" TEXT NOT NULL,
    "price" DECIMAL(65,30) NOT NULL,
    "coverUrl" TEXT NOT NULL,
    "fileUrl" TEXT NOT NULL,
    "category" "Category" NOT NULL,
    "isActive" BOOLEAN NOT NULL DEFAULT true,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "Product_pkey" PRIMARY KEY ("id")
);

-- ─── Compras ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS "Purchase" (
    "id" TEXT NOT NULL,
    "mercadoPagoPaymentId" TEXT NOT NULL,
    "amountPaid" DECIMAL(65,30) NOT NULL,
    "purchasedAt" TIMESTAMP(3) NOT NULL,

    "customerId" TEXT NOT NULL,
    "productId" TEXT NOT NULL,

    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "Purchase_pkey" PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "Purchase_mercadoPagoPaymentId_key" ON "Purchase"("mercadoPagoPaymentId");
CREATE INDEX IF NOT EXISTS "Purchase_customerId_idx" ON "Purchase"("customerId");
CREATE INDEX IF NOT EXISTS "Purchase_productId_idx" ON "Purchase"("productId");

-- ─── Foreign keys (guarded so re-runs/existing DBs are safe) ─────────────────

DO $$
BEGIN
    ALTER TABLE "AuthToken"
        ADD CONSTRAINT "AuthToken_customerId_fkey"
        FOREIGN KEY ("customerId") REFERENCES "Customer"("id")
        ON DELETE RESTRICT ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

DO $$
BEGIN
    ALTER TABLE "Purchase"
        ADD CONSTRAINT "Purchase_customerId_fkey"
        FOREIGN KEY ("customerId") REFERENCES "Customer"("id")
        ON DELETE RESTRICT ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

DO $$
BEGIN
    ALTER TABLE "Purchase"
        ADD CONSTRAINT "Purchase_productId_fkey"
        FOREIGN KEY ("productId") REFERENCES "Product"("id")
        ON DELETE RESTRICT ON UPDATE CASCADE;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

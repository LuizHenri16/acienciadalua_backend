-- Type hints for sqlc only.
--
-- The real migration creates the enum inside a `DO $$ ... $$` block (so it is
-- safe to re-run on databases previously managed by Prisma). sqlc does not
-- inspect the body of DO blocks, so the enum is declared again here. This file
-- is used exclusively as a sqlc schema input and is never executed.
CREATE TYPE "Category" AS ENUM ('STUDENT', 'TEACHER');

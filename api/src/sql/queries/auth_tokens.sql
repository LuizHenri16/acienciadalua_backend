-- ─── Magic link tokens (cleanup job) ─────────────────────────────────────────

-- name: DeleteExpiredOrUsedAuthTokens :execrows
DELETE FROM "AuthToken"
WHERE "expiresAt" < $1
   OR "usedAt" IS NOT NULL;

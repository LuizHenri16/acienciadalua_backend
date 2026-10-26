package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeDatabaseURL(t *testing.T) {
	sanitized, err := SanitizeDatabaseURL("postgresql://user:pass@localhost:5432/db?schema=public&sslmode=require")
	require.NoError(t, err)
	assert.Equal(t, "postgresql://user:pass@localhost:5432/db?sslmode=require", sanitized)

	sanitized, err = SanitizeDatabaseURL("postgresql://user:pass@db:5432/acienciadalua?schema=public&connection_limit=5&pool_timeout=10")
	require.NoError(t, err)
	assert.Equal(t, "postgresql://user:pass@db:5432/acienciadalua", sanitized)
}

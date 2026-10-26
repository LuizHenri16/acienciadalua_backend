package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminTokenRoundTrip(t *testing.T) {
	token, err := SignAdminToken("user-id", "admin@test.com", "collaborator", "secret", time.Hour)
	require.NoError(t, err)

	claims, err := VerifyAdminToken(token, "secret")
	require.NoError(t, err)
	assert.Equal(t, "user-id", claims.ID)
	assert.Equal(t, "admin@test.com", claims.Email)
	assert.Equal(t, "collaborator", claims.Role)
	assert.WithinDuration(t, time.Now().Add(time.Hour), claims.ExpiresAt.Time, time.Minute)
}

func TestAdminTokenRejectsWrongSecret(t *testing.T) {
	token, err := SignAdminToken("user-id", "admin@test.com", "collaborator", "secret", time.Hour)
	require.NoError(t, err)

	_, err = VerifyAdminToken(token, "other-secret")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestAdminTokenRejectsExpired(t *testing.T) {
	token, err := SignAdminToken("user-id", "admin@test.com", "collaborator", "secret", -time.Minute)
	require.NoError(t, err)

	_, err = VerifyAdminToken(token, "secret")
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestCustomerTokenUsesSubject(t *testing.T) {
	token, err := SignCustomerToken("customer-id", "cliente@test.com", "Cliente", "customer-secret", time.Hour)
	require.NoError(t, err)

	claims, err := VerifyCustomerToken(token, "customer-secret")
	require.NoError(t, err)
	assert.Equal(t, "customer-id", claims.Subject)
	assert.Equal(t, "cliente@test.com", claims.Email)
	assert.Equal(t, "Cliente", claims.Name)

	// The guards use different secrets, so a customer token is not accepted by
	// the admin guard unless the secrets are misconfigured to be equal.
	_, err = VerifyAdminToken(token, "another-secret")
	assert.Error(t, err)
}

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("password123")
	require.NoError(t, err)

	assert.True(t, ComparePassword("password123", hash))
	assert.False(t, ComparePassword("wrong-password", hash))
}

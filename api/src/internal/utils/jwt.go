package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AdminClaims is the payload of an admin access/refresh token.
// Mirrors `AdminJwtPayload` from the NestJS backend.
type AdminClaims struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// CustomerClaims is the payload of a customer token.
// Mirrors `CustomerJwtPayload` from the NestJS backend (sub/email/name).
type CustomerClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

var (
	// ErrInvalidToken is returned for malformed tokens and bad signatures.
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when the token is expired.
	ErrExpiredToken = errors.New("token expired")
)

// SignAdminToken signs an access/refresh token with the given secret.
func SignAdminToken(id, email, role, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := AdminClaims{
		ID:    id,
		Email: email,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return sign(claims, secret)
}

// SignCustomerToken signs a customer token. The NestJS implementation signs
// customer tokens with JWT_SECRET_CUSTOMER and the JwtModule default expiry
// (1 hour), so the same behaviour is kept here.
func SignCustomerToken(id, email, name, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := CustomerClaims{
		Email: email,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return sign(claims, secret)
}

func sign(claims jwt.Claims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyAdminToken validates signature, algorithm and expiration.
func VerifyAdminToken(tokenString, secret string) (*AdminClaims, error) {
	claims := &AdminClaims{}
	if err := parse(tokenString, secret, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// VerifyCustomerToken validates signature, algorithm and expiration.
func VerifyCustomerToken(tokenString, secret string) (*CustomerClaims, error) {
	claims := &CustomerClaims{}
	if err := parse(tokenString, secret, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func parse(tokenString, secret string, claims jwt.Claims) error {
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return ErrExpiredToken
		}
		return ErrInvalidToken
	}
	if !token.Valid {
		return ErrInvalidToken
	}
	return nil
}

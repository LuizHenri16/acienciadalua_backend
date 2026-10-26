package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// PaymentStatusAudience scopes a token to the public payment status
	// endpoint, so customer/admin tokens can never be replayed there.
	PaymentStatusAudience = "payment-status"
	// PaymentStatusIssuer identifies tokens issued by this API.
	PaymentStatusIssuer = "acienciadalua-api"
)

// PaymentStatusClaims authorizes reading the status of a single Mercado Pago
// payment. It is a capability token: it does not prove payment, it only lets
// the holder ask the backend about that payment.
type PaymentStatusClaims struct {
	ProductID string `json:"pid"`
	jwt.RegisteredClaims
}

// SignPaymentStatusToken signs a status token for a payment/product pair.
func SignPaymentStatusToken(paymentID, productID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := PaymentStatusClaims{
		ProductID: productID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   paymentID,
			Audience:  jwt.ClaimStrings{PaymentStatusAudience},
			Issuer:    PaymentStatusIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return sign(claims, secret)
}

// VerifyPaymentStatusToken validates signature, algorithm, audience, issuer and
// expiration.
func VerifyPaymentStatusToken(tokenString, secret string) (*PaymentStatusClaims, error) {
	claims := &PaymentStatusClaims{}
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
		jwt.WithAudience(PaymentStatusAudience),
		jwt.WithIssuer(PaymentStatusIssuer),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid || claims.Subject == "" || claims.ProductID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

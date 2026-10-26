// Package middleware contains the HTTP middlewares equivalent to the NestJS
// guards, interceptors and platform middlewares.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/acienciadalua/api/internal/utils"
)

type contextKey string

const (
	adminContextKey    contextKey = "admin"
	customerContextKey contextKey = "customer"
)

// RequireAdmin mirrors JwtAdminGuard: it requires a `Bearer <token>` header
// signed with JWT_SECRET.
func RequireAdmin(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				utils.WriteError(w, r, utils.Unauthorized("Missing or invalid authorization header."))
				return
			}

			claims, err := utils.VerifyAdminToken(token, secret)
			if err != nil {
				utils.WriteError(w, r, utils.Unauthorized("Invalid or expired token."))
				return
			}

			ctx := context.WithValue(r.Context(), adminContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireCustomer mirrors JwtCustomerGuard: it requires a token signed with
// JWT_SECRET_CUSTOMER.
func RequireCustomer(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				utils.WriteError(w, r, utils.Unauthorized("Missing or invalid authorization header."))
				return
			}

			claims, err := utils.VerifyCustomerToken(token, secret)
			if err != nil {
				utils.WriteError(w, r, utils.Unauthorized("Invalid or expired token."))
				return
			}

			ctx := context.WithValue(r.Context(), customerContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminFrom returns the authenticated admin claims.
func AdminFrom(ctx context.Context) (*utils.AdminClaims, bool) {
	claims, ok := ctx.Value(adminContextKey).(*utils.AdminClaims)
	return claims, ok
}

// CustomerFrom returns the authenticated customer claims.
func CustomerFrom(ctx context.Context) (*utils.CustomerClaims, bool) {
	claims, ok := ctx.Value(customerContextKey).(*utils.CustomerClaims)
	return claims, ok
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}

	parts := strings.Split(header, " ")
	if len(parts) < 2 {
		return "", false
	}

	return parts[1], parts[1] != ""
}

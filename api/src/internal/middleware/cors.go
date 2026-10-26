package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// CORS configures the allowed origins, methods and headers.
func CORS(origins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// PrettyOrigins is a small helper used by the startup log.
func PrettyOrigins(origins []string) string { return strings.Join(origins, ", ") }

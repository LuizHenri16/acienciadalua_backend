package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/acienciadalua/api/internal/utils"
	"github.com/go-chi/chi/v5/middleware"
)

// Logger logs one structured line per request.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.Info("http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", wrapped.Status(),
					"duration_ms", time.Since(start).Milliseconds(),
					"ip", r.RemoteAddr,
				)
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}

// Recoverer converts panics into the same JSON 500 response Nest's exception
// filter produces.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"method", r.Method,
						"path", r.URL.Path,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					utils.WriteError(w, r, utils.Internal("Internal server error"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// NotFound reproduces Express' 404 body: {"message":"Cannot GET /x",...}.
func NotFound(w http.ResponseWriter, r *http.Request) {
	utils.WriteError(w, r, utils.NotFound("Cannot "+r.Method+" "+r.URL.Path))
}

// MethodNotAllowed reproduces Express' 405 body.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	utils.WriteError(w, r, utils.NewError(http.StatusMethodNotAllowed, "Cannot "+r.Method+" "+r.URL.Path))
}

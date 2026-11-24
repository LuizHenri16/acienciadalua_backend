package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/docs"
	"github.com/acienciadalua/api/internal/handlers"
	appmiddleware "github.com/acienciadalua/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func buildRouter(cfg *config.Config, h *handlers.Handlers, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()

	router.Use(chimiddleware.RealIP)
	router.Use(appmiddleware.Logger(logger))
	router.Use(appmiddleware.Recoverer(logger))
	router.Use(appmiddleware.CORS(cfg.CORSOrigins))

	// Documentation (Swagger UI + OpenAPI JSON).
	router.Get("/api", docs.UIHandler)
	router.Get("/api-json", docs.JSONHandler)

	// Extra: container healthcheck.
	router.Get("/health", h.Health)

	// ─── Auth ────────────────────────────────────────────────────────────────
	// Rate limits per route (10/60s by default, with stricter auth overrides).
	throttle := func(limit int, window time.Duration) func(http.Handler) http.Handler {
		return appmiddleware.NewRateLimiter(limit, window).Middleware
	}

	router.Route("/auth", func(r chi.Router) {
		r.With(throttle(10, time.Minute)).Post("/admin/signup", h.SignUp)
		r.With(throttle(10, time.Minute)).Post("/admin/signin", h.SignIn)
		r.With(throttle(10, time.Minute)).Post("/admin/refresh", h.Refresh)

		r.With(throttle(10, 15*time.Minute)).Post("/user/signin", h.CustomerSignIn)
		r.With(throttle(10, time.Minute)).Post("/user/password", h.CustomerSetPassword)
		r.With(throttle(3, 15*time.Minute)).Post("/user/forgot-password", h.CustomerForgotPassword)
		r.With(throttle(10, time.Minute)).Post("/user/reset-password", h.CustomerResetPassword)
	})

	// ─── Products ────────────────────────────────────────────────────────────
	router.Route("/products", func(r chi.Router) {
		r.Get("/", h.ListProducts)
		r.Get("/{id}", h.GetProduct)

		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.RequireAdmin(cfg.JWTSecret))
			r.Post("/", h.CreateProduct)
			r.Put("/{id}", h.UpdateProduct)
			r.Patch("/{id}/toggle", h.ToggleProduct)
			r.Delete("/{id}", h.DeleteProduct)
		})
	})

	// ─── Payment ─────────────────────────────────────────────────────────────
	router.Route("/payment", func(r chi.Router) {
		r.Post("/create-preferences", h.CreatePreferences)
		r.Post("/pix", h.CreatePixPayment)
		r.With(throttle(60, time.Minute)).Post("/status", h.GetPaymentStatus)
		r.Post("/webhook", h.Webhook)
	})

	// ─── Contact ─────────────────────────────────────────────────────────────
	router.Post("/contact", h.SendContact)

	// ─── Customers (JWT customer) ────────────────────────────────────────────
	router.Route("/customers", func(r chi.Router) {
		r.Use(appmiddleware.RequireCustomer(cfg.JWTSecretCustomer))
		r.Get("/me", h.GetMe)
		r.Get("/me/purchases", h.GetMyPurchases)
		r.Get("/me/downloads/{productId}", h.DownloadProductFile)
	})

	// ─── Static uploads ──────────────────────────────────────────────────────
	// Same prefix as app.useStaticAssets: /uploads/<filename>.
	router.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

	router.NotFound(appmiddleware.NotFound)
	router.MethodNotAllowed(appmiddleware.MethodNotAllowed)

	return router
}

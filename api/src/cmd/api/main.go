// Command api starts the A Ciência da Lua HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/database"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/docs"
	"github.com/acienciadalua/api/internal/handlers"
	"github.com/acienciadalua/api/internal/jobs"
	appmiddleware "github.com/acienciadalua/api/internal/middleware"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("api stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := utils.EnsureStorageDirs(cfg.UploadDir, cfg.FilesDir); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.RunMigrations {
		if err := database.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil {
			return err
		}
	}

	queries := db.New(pool)

	emailService := services.NewEmailService(cfg)
	authService := services.NewAuthService(cfg, queries, emailService)
	productsService := services.NewProductsService(queries)
	customersService := services.NewCustomersService(cfg, queries)
	mercadoPagoClient := services.NewMercadoPagoClient(cfg)
	paymentService := services.NewPaymentService(cfg, pool, queries, productsService, mercadoPagoClient, emailService)

	handlerSet := handlers.New(cfg, authService, productsService, customersService, paymentService, emailService)

	scheduler, err := jobs.StartTokenCleanup(queries, logger)
	if err != nil {
		return fmt.Errorf("starting token cleanup job: %w", err)
	}
	defer scheduler.Stop()

	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           buildRouter(cfg, handlerSet, logger),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api listening",
			"port", cfg.Port,
			"env", cfg.NodeEnv,
			"cors", appmiddleware.PrettyOrigins(cfg.CORSOrigins),
			"docs", "/api",
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("api stopped")
	return nil
}

func buildRouter(cfg *config.Config, h *handlers.Handlers, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()

	router.Use(chimiddleware.RealIP)
	router.Use(appmiddleware.Logger(logger))
	router.Use(appmiddleware.Recoverer(logger))
	router.Use(appmiddleware.CORS(cfg.CORSOrigins))

	// Documentation (same paths as @nestjs/swagger).
	router.Get("/api", docs.UIHandler)
	router.Get("/api-json", docs.JSONHandler)

	// Extra: container healthcheck.
	router.Get("/health", h.Health)

	// ─── Auth ────────────────────────────────────────────────────────────────
	// Rate limits mirror ThrottlerModule (10/60s) and the @Throttle overrides.
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

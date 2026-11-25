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
	"github.com/acienciadalua/api/internal/handlers"
	"github.com/acienciadalua/api/internal/jobs"
	appmiddleware "github.com/acienciadalua/api/internal/middleware"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/utils"
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
	adminService := services.NewAdminService(cfg, queries)

	handlerSet := handlers.New(cfg, authService, productsService, customersService, paymentService, emailService, adminService)

	scheduler, err := jobs.StartTokenCleanup(queries, logger)
	if err != nil {
		return fmt.Errorf("starting token cleanup job: %w", err)
	}
	defer scheduler.Stop()

	if cfg.PaymentReconcileEnabled {
		reconcileScheduler, err := jobs.StartPaymentReconciliation(paymentService, cfg.PaymentReconcileInterval, logger)
		if err != nil {
			return fmt.Errorf("starting payment reconciliation job: %w", err)
		}
		defer reconcileScheduler.Stop()
	}

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

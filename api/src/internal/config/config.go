// Package config loads and validates the runtime configuration.
//
// Every environment variable keeps the exact same name used by the NestJS
// backend so existing deployments only need to change how the service is
// started, not its configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// DefaultCORSOrigins mirrors the list hardcoded in the NestJS bootstrap.
var DefaultCORSOrigins = []string{
	"http://192.241.253.54:3003",
	"http://localhost:3002",
	"https://acienciadalua.com.br",
}

// Config is the full application configuration.
type Config struct {
	// Database
	DatabaseURL string

	// JWT
	JWTSecret           string
	JWTRefreshSecret    string
	JWTSecretCustomer   string
	JWTExpiresIn        time.Duration
	JWTRefreshExpiresIn time.Duration

	// Environment
	NodeEnv string
	Port    string
	AppURL  string

	// Email (Resend)
	ResendAPIKey       string
	EmailSenderAddress string
	ContactEmail       string
	FrontendURL        string

	// Mercado Pago
	MercadoPagoAccessToken string
	MPWebhookURL           string
	MPSuccessURL           string
	MPFailureURL           string
	MPPendingURL           string

	// Storage
	UploadDir string
	FilesDir  string

	// HTTP
	CORSOrigins   []string
	RunMigrations bool
}

// Load reads the environment (loading .env when present) and returns a
// validated configuration.
func Load() (*Config, error) {
	// godotenv does not override variables already present in the environment,
	// which is the behaviour needed in containers.
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		JWTRefreshSecret:       os.Getenv("JWT_REFRESH_SECRET"),
		JWTSecretCustomer:      os.Getenv("JWT_SECRET_CUSTOMER"),
		NodeEnv:                envOrDefault("NODE_ENV", "development"),
		Port:                   envOrDefault("PORT", "3000"),
		AppURL:                 os.Getenv("APP_URL"),
		ResendAPIKey:           os.Getenv("RESEND_API_KEY"),
		EmailSenderAddress:     envOrDefault("EMAIL_SENDER_ADDRESS", "onboarding@resend.dev"),
		ContactEmail:           envOrDefault("CONTACT_EMAIL", "contato@acienciadalua.com.br"),
		MercadoPagoAccessToken: os.Getenv("MERCADO_PAGO_ACCESS_TOKEN"),
		MPWebhookURL:           os.Getenv("MP_WEBHOOK_URL"),
		MPSuccessURL:           os.Getenv("MP_SUCCESS_URL"),
		MPFailureURL:           os.Getenv("MP_FAILURE_URL"),
		MPPendingURL:           os.Getenv("MP_PENDING_URL"),
		UploadDir:              envOrDefault("UPLOAD_DIR", "uploads"),
		FilesDir:               envOrDefault("FILES_DIR", "files"),
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		// FRONTEND_URL is used by the e-mail templates (the NestJS code relied on
		// it too). Fall back to APP_URL so existing environments keep working.
		frontendURL = cfg.AppURL
	}
	cfg.FrontendURL = frontendURL

	expiresIn, err := ParseDuration(envOrDefault("JWT_EXPIRES_IN", "1h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRES_IN: %w", err)
	}
	cfg.JWTExpiresIn = expiresIn

	refreshExpiresIn, err := ParseDuration(envOrDefault("JWT_REFRESH_EXPIRES_IN", "7d"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRES_IN: %w", err)
	}
	cfg.JWTRefreshExpiresIn = refreshExpiresIn

	if origins := os.Getenv("CORS_ORIGINS"); origins != "" {
		cfg.CORSOrigins = splitAndTrim(origins)
	} else {
		cfg.CORSOrigins = DefaultCORSOrigins
	}

	cfg.RunMigrations = envBool("RUN_MIGRATIONS", true)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if c.JWTRefreshSecret == "" {
		missing = append(missing, "JWT_REFRESH_SECRET")
	}
	if c.JWTSecretCustomer == "" {
		missing = append(missing, "JWT_SECRET_CUSTOMER")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

// IsDevelopment reports whether the service runs in development mode.
func (c *Config) IsDevelopment() bool {
	return c.NodeEnv == "development"
}

// Addr returns the listen address.
func (c *Config) Addr() string {
	return ":" + c.Port
}

// ParseDuration accepts the values supported by jsonwebtoken (`1h`, `7d`,
// `30m`, `3600`) plus Go durations.
func ParseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// jsonwebtoken accepts a plain number of seconds.
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	// Go's time.ParseDuration does not understand days.
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(value, "d"), 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}

	return time.ParseDuration(value)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

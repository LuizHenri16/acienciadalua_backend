// Package handlers contains the HTTP layer (controllers). Handlers stay thin:
// parse/validate input, call a service and write the response.
package handlers

import (
	"log/slog"
	"net/http"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/services"
)

// Handlers aggregates the application services.
type Handlers struct {
	cfg       *config.Config
	auth      *services.AuthService
	products  *services.ProductsService
	customers *services.CustomersService
	payment   *services.PaymentService
	email     *services.EmailService
}

func New(
	cfg *config.Config,
	auth *services.AuthService,
	products *services.ProductsService,
	customers *services.CustomersService,
	payment *services.PaymentService,
	email *services.EmailService,
) *Handlers {
	return &Handlers{
		cfg:       cfg,
		auth:      auth,
		products:  products,
		customers: customers,
		payment:   payment,
		email:     email,
	}
}

// Health is used by the container healthcheck.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func logInfo(message string, args ...any) {
	slog.Info(message, args...)
}

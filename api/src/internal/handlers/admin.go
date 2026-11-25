package handlers

import (
	"net/http"

	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/utils"
)

// GetAdminDashboard returns dashboard analytics for the requested period.
func (h *Handlers) GetAdminDashboard(w http.ResponseWriter, r *http.Request) {
	query := dto.DashboardQuery{Period: r.URL.Query().Get("period")}
	if messages := query.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	dashboard, err := h.admin.GetDashboard(r.Context(), query.Period)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

// ListAdminSales returns a direct array, not an envelope.
func (h *Handlers) ListAdminSales(w http.ResponseWriter, r *http.Request) {
	sales, err := h.admin.ListSales(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, sales)
}

// ListAdminCustomers returns a direct array, not an envelope.
func (h *Handlers) ListAdminCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := h.admin.ListCustomers(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, customers)
}

// GetProductSalesCount returns a map of product ID to all-time sales.
func (h *Handlers) GetProductSalesCount(w http.ResponseWriter, r *http.Request) {
	counts, err := h.admin.SalesCountByProduct(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, counts)
}

func (h *Handlers) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.admin.GetSettings(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

func (h *Handlers) PutAdminSettings(w http.ResponseWriter, r *http.Request) {
	var request dto.UpdateAdminSettingsRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	settings, err := h.admin.UpdateSettings(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

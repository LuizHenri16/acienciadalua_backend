package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/utils"
)

func (h *Handlers) CreatePreferences(w http.ResponseWriter, r *http.Request) {
	var request dto.CreatePreferencesRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	response, err := h.payment.CreatePreferences(r.Context(), request.ProductID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *Handlers) CreatePixPayment(w http.ResponseWriter, r *http.Request) {
	var request dto.CreatePixPaymentRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	response, err := h.payment.CreatePixPayment(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

// Webhook processes Mercado Pago notifications. The body is decoded
// permissively (Mercado Pago sends many extra fields and new ones are added
// over time); query-string parameters are accepted as a fallback because
// Mercado Pago also delivers `?type=payment&data.id=...` notifications.
func (h *Handlers) Webhook(w http.ResponseWriter, r *http.Request) {
	var body services.WebhookRequest

	if r.Body != nil {
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := decoder.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, r, utils.BadRequest("Malformed JSON in request body"))
			return
		}
	}

	query := r.URL.Query()
	notificationType := query.Get("type")
	paymentID := firstNonEmpty(query.Get("data.id"), query.Get("data[id]"))

	if err := h.payment.HandleWebhook(r.Context(), body, notificationType, paymentID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

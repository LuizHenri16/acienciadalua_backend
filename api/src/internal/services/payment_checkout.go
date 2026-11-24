package services

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreatePreferences creates a Mercado Pago checkout preference.
func (s *PaymentService) CreatePreferences(ctx context.Context, productID string) (PreferenceResponse, error) {
	product, err := s.products.FindOne(ctx, productID)
	if err != nil {
		return PreferenceResponse{}, err
	}

	payload := map[string]any{
		"items": []map[string]any{{
			"id":         product.ID,
			"title":      product.Title,
			"quantity":   1,
			"unit_price": product.Price.InexactFloat64(),
		}},
		"external_reference": product.ID,
		"payment_methods": map[string]any{
			"excluded_payment_types": []map[string]string{
				{"id": "ticket"},
				{"id": "atm"},
			},
			"excluded_payment_methods": []map[string]string{
				{"id": "pec"},
			},
			"installments": 6,
		},
		"auto_return": "approved",
	}

	if s.cfg.MPWebhookURL != "" {
		payload["notification_url"] = s.cfg.MPWebhookURL
	}

	backURLs := map[string]string{}
	if s.cfg.MPSuccessURL != "" {
		backURLs["success"] = s.cfg.MPSuccessURL
	}
	if s.cfg.MPFailureURL != "" {
		backURLs["failure"] = s.cfg.MPFailureURL
	}
	if s.cfg.MPPendingURL != "" {
		backURLs["pending"] = s.cfg.MPPendingURL
	}
	if len(backURLs) > 0 {
		payload["back_urls"] = backURLs
	}

	preference, err := s.mercadoPago.CreatePreference(ctx, payload)
	if err != nil {
		slog.Error("Erro ao criar preferência de pagamento", "error", err)
		return PreferenceResponse{}, err
	}

	return PreferenceResponse{
		InitPoint:    preference.InitPoint,
		PreferenceID: preference.ID,
	}, nil
}

// CreatePixPayment creates a PIX payment and returns the QR code data.
//
// The creation is idempotent: when the client sends the same idempotencyKey,
// the stored PaymentIntent is returned instead of creating a second charge.
// The response also carries a signed status token and the expiration date so
// the frontend can poll POST /payment/status.
func (s *PaymentService) CreatePixPayment(ctx context.Context, data dto.CreatePixPaymentRequest) (PixPaymentResponse, error) {
	product, err := s.products.FindOne(ctx, data.ProductID)
	if err != nil {
		return PixPaymentResponse{}, err
	}

	idempotencyKey := uuid.NewString()
	if data.IdempotencyKey != nil && strings.TrimSpace(*data.IdempotencyKey) != "" {
		idempotencyKey = strings.TrimSpace(*data.IdempotencyKey)
	}

	// A retry with the same key must return the original PIX (same QR code and
	// expiration), never a new charge.
	if existing, err := s.q.GetPaymentIntentByIdempotencyKey(ctx, idempotencyKey); err == nil {
		if existing.ProductId != product.ID {
			return PixPaymentResponse{}, utils.BadRequest("idempotencyKey já utilizada para outro produto")
		}
		return s.pixResponseFromIntent(existing)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return PixPaymentResponse{}, err
	}

	payer := map[string]any{"email": data.Payer.Email}
	if data.Payer.Name != nil {
		payer["first_name"] = *data.Payer.Name
	}
	if data.Payer.CPF != nil && *data.Payer.CPF != "" {
		payer["identification"] = map[string]string{
			"type":   "CPF",
			"number": *data.Payer.CPF,
		}
	}

	customerName := ""
	if data.Payer.Name != nil {
		customerName = *data.Payer.Name
	}

	payload := map[string]any{
		"transaction_amount": product.Price.InexactFloat64(),
		"description":        product.Title,
		"payment_method_id":  "pix",
		"payer":              payer,
		"metadata": map[string]string{
			"customer_email": data.Payer.Email,
			"customer_name":  customerName,
		},
		"external_reference": product.ID,
	}

	if s.cfg.MPWebhookURL != "" {
		payload["notification_url"] = s.cfg.MPWebhookURL
	}

	expiresAt := time.Time{}
	if s.cfg.MPPixExpiration > 0 {
		expiresAt = time.Now().Add(s.cfg.MPPixExpiration).UTC()
		// Mercado Pago expects an ISO-8601 timestamp with an explicit offset.
		payload["date_of_expiration"] = expiresAt.In(brtZone).Format("2006-01-02T15:04:05.000-07:00")
	}

	payment, err := s.mercadoPago.CreatePayment(ctx, payload, idempotencyKey)
	if err != nil {
		slog.Error("Erro ao criar pagamento PIX", "error", err)
		return PixPaymentResponse{}, err
	}

	paymentID := payment.ID.String()
	if _, err := strconv.ParseInt(paymentID, 10, 64); err != nil {
		slog.Error("Mercado Pago returned an invalid payment id", "payment_id", paymentID)
		return PixPaymentResponse{}, utils.Internal("Erro inesperado")
	}

	if fromProvider := formatMPDate(payment.DateOfExpiration); !fromProvider.IsZero() {
		expiresAt = fromProvider
	}

	now := types.NewTimestamp(time.Now())
	amount := parseMPAmount(payment.TransactionAmount)
	if amount.IsZero() {
		amount = product.Price
	}

	intent, err := s.q.UpsertPaymentIntent(ctx, db.UpsertPaymentIntentParams{
		ID:                   uuid.NewString(),
		MercadoPagoPaymentId: paymentID,
		ProductId:            product.ID,
		CustomerEmail:        data.Payer.Email,
		CustomerName:         customerName,
		Amount:               amount,
		Status:               PaymentStatusPending,
		StatusDetail:         payment.StatusDetail,
		QrCode:               nonEmptyString(transactionQRCode(payment)),
		QrCodeBase64:         nonEmptyString(transactionQRCodeBase64(payment)),
		TicketUrl:            nonEmptyString(transactionTicketURL(payment)),
		ExpiresAt:            timestampPtr(expiresAt),
		IdempotencyKey:       idempotencyKey,
		LastCheckedAt:        nil,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		// A concurrent request may have stored the same payment under another
		// idempotency key. Reuse that row instead of failing the checkout.
		if existing, getErr := s.q.GetPaymentIntentByPaymentID(ctx, paymentID); getErr == nil {
			intent = existing
		} else {
			return PixPaymentResponse{}, err
		}
	}

	return s.pixResponseFromIntent(intent)
}

func (s *PaymentService) pixResponseFromIntent(intent db.PaymentIntent) (PixPaymentResponse, error) {
	id, err := strconv.ParseInt(intent.MercadoPagoPaymentId, 10, 64)
	if err != nil {
		return PixPaymentResponse{}, utils.Internal("Erro inesperado")
	}

	token, err := utils.SignPaymentStatusToken(
		intent.MercadoPagoPaymentId,
		intent.ProductId,
		s.cfg.PaymentStatusSecret,
		s.statusTokenTTL(),
	)
	if err != nil {
		return PixPaymentResponse{}, err
	}

	return PixPaymentResponse{
		ID:           id,
		Status:       intent.Status,
		QRCode:       intent.QrCode,
		QRCodeBase64: intent.QrCodeBase64,
		TicketURL:    intent.TicketUrl,
		PaymentID:    intent.MercadoPagoPaymentId,
		StatusToken:  token,
		ExpiresAt:    intent.ExpiresAt,
	}, nil
}

func (s *PaymentService) statusTokenTTL() time.Duration {
	if s.cfg.PaymentStatusTTL > 0 {
		return s.cfg.PaymentStatusTTL
	}
	return 24 * time.Hour
}

var brtZone = time.FixedZone("BRT", -3*60*60)

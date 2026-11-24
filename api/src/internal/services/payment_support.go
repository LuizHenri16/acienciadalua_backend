package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func (s *PaymentService) getProviderPayment(ctx context.Context, paymentID string) (*MPPayment, error) {
	if s.cfg.MPStatusTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.MPStatusTimeout)
		defer cancel()
	}
	return s.mercadoPago.GetPayment(ctx, paymentID)
}

func (s *PaymentService) findIntent(ctx context.Context, paymentID string) (*db.PaymentIntent, error) {
	intent, err := s.q.GetPaymentIntentByPaymentID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &intent, nil
}

func (s *PaymentService) findPurchase(ctx context.Context, paymentID string) (*db.Purchase, error) {
	purchase, err := s.q.GetPurchaseByPaymentID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &purchase, nil
}

func normalizePayment(payment *MPPayment, fallback *db.PaymentIntent) normalizedPayment {
	expiresAt := formatMPDate(payment.DateOfExpiration)
	if expiresAt.IsZero() && fallback != nil && fallback.ExpiresAt != nil {
		expiresAt = fallback.ExpiresAt.AsTime()
	}

	status := strings.ToLower(strings.TrimSpace(payment.Status))
	detail := payment.StatusDetail
	normalized := PaymentStatusError
	switch status {
	case "approved":
		normalized = PaymentStatusApproved
	case "pending", "in_process", "authorized", "in_mediation":
		normalized = PaymentStatusPending
		if !expiresAt.IsZero() && time.Now().UTC().After(expiresAt) {
			normalized = PaymentStatusExpired
			if detail == "" {
				detail = "expired"
			}
		}
	case "rejected", "cancelled":
		normalized = PaymentStatusRejected
	case "expired":
		normalized = PaymentStatusExpired
	case "refunded", "charged_back":
		normalized = PaymentStatusRefunded
	}
	if detail == "" {
		detail = status
	}

	return normalizedPayment{
		Status:     normalized,
		Detail:     detail,
		ExpiresAt:  expiresAt,
		ApprovedAt: formatMPDate(payment.DateApproved),
	}
}

func providerStatusError(err error) error {
	var mpErr *MPError
	if errors.As(err, &mpErr) {
		switch {
		case mpErr.StatusCode == http.StatusNotFound:
			return utils.NotFound("Pagamento não encontrado")
		case mpErr.StatusCode == 0 || mpErr.Retryable:
			return utils.NewError(http.StatusServiceUnavailable, "Não foi possível consultar o pagamento. Tente novamente.")
		}
		return utils.Internal("Erro inesperado")
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return utils.NewError(http.StatusServiceUnavailable, "Não foi possível consultar o pagamento. Tente novamente.")
	}

	return err
}

func paymentCustomer(payment *MPPayment) (email, name string) {
	payerEmail := ""
	payerName := ""
	if payment.Payer != nil {
		payerEmail = payment.Payer.Email
		payerName = payment.Payer.FirstName
	}

	metadataEmail := ""
	metadataName := ""
	if payment.Metadata != nil {
		metadataEmail = payment.Metadata.CustomerEmail
		metadataName = payment.Metadata.CustomerName
	}

	email = metadataEmail
	if isEmail(payerEmail) {
		email = payerEmail
	}

	name = metadataName
	if payerName != "" {
		name = payerName
	}

	return email, name
}

func transactionQRCode(payment *MPPayment) string {
	if payment.PointOfInteraction == nil || payment.PointOfInteraction.TransactionData == nil {
		return ""
	}
	return payment.PointOfInteraction.TransactionData.QRCode
}

func transactionQRCodeBase64(payment *MPPayment) string {
	if payment.PointOfInteraction == nil || payment.PointOfInteraction.TransactionData == nil {
		return ""
	}
	return payment.PointOfInteraction.TransactionData.QRCodeBase64
}

func transactionTicketURL(payment *MPPayment) string {
	if payment.PointOfInteraction == nil || payment.PointOfInteraction.TransactionData == nil {
		return ""
	}
	return payment.PointOfInteraction.TransactionData.TicketURL
}

func nonEmptyString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func parseMPAmount(value json.Number) types.Decimal {
	raw := strings.TrimSpace(value.String())
	if raw == "" {
		return types.Decimal{Decimal: decimal.Zero}
	}
	parsed, err := decimal.NewFromString(raw)
	if err != nil {
		return types.Decimal{Decimal: decimal.Zero}
	}
	return types.Decimal{Decimal: parsed}
}

func timestampPtr(t time.Time) *types.Timestamp {
	if t.IsZero() {
		return nil
	}
	value := types.NewTimestamp(t)
	return &value
}

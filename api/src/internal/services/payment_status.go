package services

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/utils"
)

// GetPaymentStatus returns the normalized status of a payment. It is the only
// path through which the frontend may learn that a payment was approved.
//
// Mercado Pago is the source of truth. A local Purchase short-circuits the
// provider call, but "approved" is never returned without a Purchase row.
func (s *PaymentService) GetPaymentStatus(ctx context.Context, data dto.PaymentStatusRequest) (PaymentStatusResponse, error) {
	claims, err := utils.VerifyPaymentStatusToken(data.Token, s.cfg.PaymentStatusSecret)
	if err != nil {
		return PaymentStatusResponse{}, utils.Unauthorized("Token de pagamento inválido ou expirado")
	}
	if claims.Subject != data.PaymentID {
		return PaymentStatusResponse{}, utils.Forbidden("O token não corresponde a este pagamento")
	}

	intent, err := s.findIntent(ctx, data.PaymentID)
	if err != nil {
		return PaymentStatusResponse{}, err
	}
	if intent != nil && intent.ProductId != claims.ProductID {
		return PaymentStatusResponse{}, utils.Forbidden("O token não corresponde a este produto")
	}

	purchase, err := s.findPurchase(ctx, data.PaymentID)
	if err != nil {
		return PaymentStatusResponse{}, err
	}
	if purchase != nil && purchase.ProductId != claims.ProductID {
		return PaymentStatusResponse{}, utils.Forbidden("O token não corresponde a este produto")
	}

	productID := claims.ProductID
	if intent != nil {
		productID = intent.ProductId
	}

	// Refunds are terminal until a webhook/reconciliation says otherwise.
	if intent != nil && intent.Status == PaymentStatusRefunded {
		return s.statusFromIntent(data.PaymentID, intent, purchase, PaymentStatusRefunded), nil
	}

	// Fast path: the purchase already exists and there is no pending refund.
	if purchase != nil && (intent == nil || intent.Status == PaymentStatusApproved) {
		return s.statusFromPurchase(data.PaymentID, productID, purchase, intent), nil
	}

	payment, err := s.getProviderPayment(ctx, data.PaymentID)
	if err != nil {
		return PaymentStatusResponse{}, providerStatusError(err)
	}

	if payment.ExternalReference == "" || payment.ExternalReference != claims.ProductID {
		slog.Warn("status token/product mismatch",
			"payment_id", data.PaymentID,
			"payment_product", payment.ExternalReference,
		)
		return PaymentStatusResponse{}, utils.Forbidden("O pagamento não corresponde a este produto")
	}

	normalized := normalizePayment(payment, intent)
	if normalized.Status == PaymentStatusApproved {
		purchase, _, err := s.ensurePurchase(ctx, payment)
		if err != nil {
			if _, ok := utils.AsError(err); ok {
				return PaymentStatusResponse{}, utils.NewError(http.StatusBadGateway, "Não foi possível registrar o pagamento")
			}
			return PaymentStatusResponse{}, err
		}
		if purchase == nil {
			return PaymentStatusResponse{}, utils.NewError(http.StatusBadGateway, "Não foi possível confirmar o pagamento")
		}
		return s.statusFromProvider(data.PaymentID, productID, payment, normalized, purchase, intent), nil
	}

	if intent != nil {
		if err := s.persistIntentStatus(ctx, *intent, normalized); err != nil {
			slog.Error("failed to persist payment status", "payment_id", data.PaymentID, "error", err)
		}
	}

	return s.statusFromProvider(data.PaymentID, productID, payment, normalized, purchase, intent), nil
}

func (s *PaymentService) statusFromPurchase(paymentID, productID string, purchase *db.Purchase, intent *db.PaymentIntent) PaymentStatusResponse {
	response := PaymentStatusResponse{
		PaymentID:      paymentID,
		Status:         PaymentStatusApproved,
		ProviderStatus: PaymentStatusApproved,
		ProductID:      productID,
		Amount:         purchase.AmountPaid,
		PurchaseID:     &purchase.ID,
		AccessGranted:  true,
		ApprovedAt:     &purchase.PurchasedAt,
	}
	if intent != nil {
		response.StatusDetail = intent.StatusDetail
		response.ExpiresAt = intent.ExpiresAt
	}
	return response
}

func (s *PaymentService) statusFromIntent(paymentID string, intent *db.PaymentIntent, purchase *db.Purchase, status string) PaymentStatusResponse {
	response := PaymentStatusResponse{
		PaymentID:      paymentID,
		Status:         status,
		ProviderStatus: intent.Status,
		StatusDetail:   intent.StatusDetail,
		ProductID:      intent.ProductId,
		Amount:         intent.Amount,
		AccessGranted:  status == PaymentStatusApproved && purchase != nil,
		ApprovedAt:     intent.ApprovedAt,
		ExpiresAt:      intent.ExpiresAt,
		Retryable:      status == PaymentStatusPending || status == PaymentStatusError,
	}
	if purchase != nil {
		response.PurchaseID = &purchase.ID
	}
	return response
}

func (s *PaymentService) statusFromProvider(
	paymentID, productID string,
	payment *MPPayment,
	normalized normalizedPayment,
	purchase *db.Purchase,
	intent *db.PaymentIntent,
) PaymentStatusResponse {
	amount := parseMPAmount(payment.TransactionAmount)
	if purchase != nil {
		amount = purchase.AmountPaid
	} else if intent != nil && intent.Amount.IsPositive() {
		amount = intent.Amount
	}

	response := PaymentStatusResponse{
		PaymentID:      paymentID,
		Status:         normalized.Status,
		ProviderStatus: payment.Status,
		StatusDetail:   normalized.Detail,
		ProductID:      productID,
		Amount:         amount,
		AccessGranted:  normalized.Status == PaymentStatusApproved && purchase != nil,
		Retryable:      normalized.Status == PaymentStatusPending || normalized.Status == PaymentStatusError,
	}
	if purchase != nil {
		response.PurchaseID = &purchase.ID
		response.ApprovedAt = &purchase.PurchasedAt
	} else if intent != nil {
		response.ApprovedAt = intent.ApprovedAt
	}
	if response.ApprovedAt == nil && !normalized.ApprovedAt.IsZero() {
		response.ApprovedAt = timestampPtr(normalized.ApprovedAt)
	}
	if !normalized.ExpiresAt.IsZero() {
		response.ExpiresAt = timestampPtr(normalized.ExpiresAt)
	} else if intent != nil {
		response.ExpiresAt = intent.ExpiresAt
	}
	return response
}

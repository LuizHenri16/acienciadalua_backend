package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ensurePurchase registers the access granted by an approved payment. It is
// shared by the webhook, the status endpoint and the reconciliation job, and
// is idempotent thanks to the unique index on Purchase.mercadoPagoPaymentId.
//
// The returned bool reports whether this call created the purchase (used to
// send the welcome e-mail exactly once).
func (s *PaymentService) ensurePurchase(ctx context.Context, payment *MPPayment) (*db.Purchase, bool, error) {
	paymentID := payment.ID.String()

	if existing, err := s.findPurchase(ctx, paymentID); err != nil {
		return nil, false, err
	} else if existing != nil {
		return existing, false, nil
	}

	if payment.ExternalReference == "" {
		return nil, false, utils.Internal("Pagamento sem produto associado")
	}
	if _, err := s.products.FindOne(ctx, payment.ExternalReference); err != nil {
		return nil, false, err
	}

	intent, err := s.findIntent(ctx, paymentID)
	if err != nil {
		return nil, false, err
	}

	amountPaid := parseMPAmount(payment.TransactionAmount)
	if amountPaid.IsZero() {
		return nil, false, utils.Internal("Pagamento sem valor")
	}
	if intent != nil && intent.Amount.IsPositive() && amountPaid.Decimal.LessThan(intent.Amount.Decimal) {
		return nil, false, utils.Internal("Valor do pagamento não confere")
	}

	payerEmail, payerName := paymentCustomer(payment)
	email := payerEmail
	if !isEmail(email) {
		return nil, false, utils.Internal("Pagamento sem e-mail válido")
	}

	customerName := payerName
	if customerName == "" {
		customerName = email
	}

	purchasedAt := formatMPDate(payment.DateApproved)
	if purchasedAt.IsZero() {
		purchasedAt = formatMPDate(payment.DateCreated)
	}
	if purchasedAt.IsZero() {
		purchasedAt = time.Now().UTC()
	}

	now := types.NewTimestamp(time.Now())

	// Customer upsert + purchase insert are atomic, and the unique index on
	// mercadoPagoPaymentId guarantees idempotency even under concurrent
	// deliveries of the same event.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.q.WithTx(tx)

	customer, err := queries.UpsertCustomerByEmail(ctx, db.UpsertCustomerByEmailParams{
		ID:        uuid.NewString(),
		Email:     email,
		Name:      customerName,
		CreatedAt: now,
	})
	if err != nil {
		return nil, false, err
	}

	purchase, err := queries.CreatePurchase(ctx, db.CreatePurchaseParams{
		ID:                   uuid.NewString(),
		MercadoPagoPaymentId: paymentID,
		AmountPaid:           amountPaid,
		PurchasedAt:          types.NewTimestamp(purchasedAt),
		CustomerId:           customer.ID,
		ProductId:            payment.ExternalReference,
		CreatedAt:            now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING: another delivery already registered it.
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		existing, err := s.findPurchase(ctx, paymentID)
		if err != nil {
			return nil, false, err
		}
		if existing != nil && intent != nil && intent.PurchaseId == nil {
			_, _ = s.q.LinkPaymentIntentPurchase(ctx, db.LinkPaymentIntentPurchaseParams{
				ID:            intent.ID,
				PurchaseId:    &existing.ID,
				LastCheckedAt: timestampPtr(time.Now()),
			})
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if intent != nil {
		approvedAt := timestampPtr(purchasedAt)
		if _, err := queries.LinkPaymentIntentPurchase(ctx, db.LinkPaymentIntentPurchaseParams{
			ID:            intent.ID,
			PurchaseId:    &purchase.ID,
			LastCheckedAt: approvedAt,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}

	if err := s.email.SendWelcomeEmail(ctx, customer.Email, customer.Name); err != nil {
		// E-mail failures are logged and swallowed so the webhook still
		// returns 200.
		slog.Error("[EmailService] Erro ao enviar e-mail de boas-vindas", "error", err)
	}

	return &purchase, true, nil
}

func (s *PaymentService) persistIntentStatus(ctx context.Context, intent db.PaymentIntent, normalized normalizedPayment) error {
	// Never downgrade a payment that already granted access, except for a
	// refund/chargeback, which is a real state transition.
	if intent.Status == PaymentStatusApproved &&
		normalized.Status != PaymentStatusApproved &&
		normalized.Status != PaymentStatusRefunded {
		return nil
	}

	approvedAt := intent.ApprovedAt
	if approvedAt == nil && !normalized.ApprovedAt.IsZero() {
		approvedAt = timestampPtr(normalized.ApprovedAt)
	}

	_, err := s.q.UpdatePaymentIntentStatus(ctx, db.UpdatePaymentIntentStatusParams{
		ID:            intent.ID,
		Status:        normalized.Status,
		StatusDetail:  normalized.Detail,
		ApprovedAt:    approvedAt,
		LastCheckedAt: timestampPtr(time.Now()),
	})
	return err
}

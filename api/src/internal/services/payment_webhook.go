package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
)

// ReconcilePending refreshes pending payment intents against Mercado Pago. It
// compensates for lost or delayed webhooks: an approved payment becomes a
// Purchase even when no notification ever arrives.
func (s *PaymentService) ReconcilePending(ctx context.Context) error {
	now := time.Now().UTC()
	intents, err := s.q.ListPendingPaymentIntents(ctx, db.ListPendingPaymentIntentsParams{
		LastCheckedAt: timestampPtr(now.Add(-time.Minute)),
		CreatedAt:     types.NewTimestamp(now.Add(-48 * time.Hour)),
		Limit:         50,
	})
	if err != nil {
		return err
	}

	for i := range intents {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.reconcileIntent(ctx, intents[i]); err != nil {
			slog.Error("payment reconciliation failed",
				"payment_id", intents[i].MercadoPagoPaymentId,
				"error", err,
			)
		}
	}

	return nil
}

func (s *PaymentService) reconcileIntent(ctx context.Context, intent db.PaymentIntent) error {
	payment, err := s.getProviderPayment(ctx, intent.MercadoPagoPaymentId)
	if err != nil {
		return err
	}

	normalized := normalizePayment(payment, &intent)
	if normalized.Status == PaymentStatusApproved {
		_, _, err := s.ensurePurchase(ctx, payment)
		return err
	}

	return s.persistIntentStatus(ctx, intent, normalized)
}

// HandleWebhook processes Mercado Pago notifications idempotently.
//
// notificationType and queryPaymentID are the query-string fallbacks used by
// Mercado Pago when the notification arrives as
// `?type=payment&data.id=...` or the legacy `?topic=payment&id=...`.
func (s *PaymentService) HandleWebhook(
	ctx context.Context,
	body WebhookRequest,
	notificationType string,
	queryPaymentID string,
	headers WebhookHeaders,
) error {
	if notificationType == "" {
		notificationType = body.Type
	}
	if notificationType != "payment" {
		return nil
	}

	paymentID := queryPaymentID
	if body.Data != nil && body.Data.ID != "" {
		paymentID = string(body.Data.ID)
	}
	if paymentID == "" {
		return nil
	}

	if err := s.verifyWebhookSignature(paymentID, headers); err != nil {
		return err
	}

	payment, err := s.getProviderPayment(ctx, paymentID)
	if err != nil {
		return providerStatusError(err)
	}

	intent, err := s.findIntent(ctx, paymentID)
	if err != nil {
		return err
	}

	normalized := normalizePayment(payment, intent)
	if normalized.Status == PaymentStatusApproved {
		if _, _, err := s.ensurePurchase(ctx, payment); err != nil {
			// Permanent data problems (missing product, invalid e-mail) must
			// not make Mercado Pago retry forever. Log and acknowledge.
			if _, ok := utils.AsError(err); ok {
				slog.Error("[Webhook] pagamento não pôde ser registrado",
					"payment_id", paymentID,
					"error", err,
				)
				return nil
			}
			return err
		}
		return nil
	}

	if intent != nil {
		if err := s.persistIntentStatus(ctx, *intent, normalized); err != nil {
			return err
		}
	}

	return nil
}

// verifyWebhookSignature validates Mercado Pago's X-Signature header:
//
//	manifest = id:{data.id};request-id:{x-request-id};ts:{ts};
//	v1 = HMAC-SHA256(manifest, MP_WEBHOOK_SECRET)
//
// Validation is skipped when MP_WEBHOOK_SECRET is not configured, so existing
// deployments keep working until the secret is added.
func (s *PaymentService) verifyWebhookSignature(paymentID string, headers WebhookHeaders) error {
	secret := strings.TrimSpace(s.cfg.MPWebhookSecret)
	if secret == "" {
		return nil
	}

	ts, signature, ok := parseWebhookSignature(headers.Signature)
	if !ok {
		return utils.Unauthorized("Assinatura de webhook inválida")
	}

	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return utils.Unauthorized("Assinatura de webhook inválida")
	}
	skew := time.Since(time.Unix(seconds, 0))
	if skew > 5*time.Minute || skew < -5*time.Minute {
		return utils.Unauthorized("Assinatura de webhook expirada")
	}

	manifest := fmt.Sprintf("id:%s;request-id:%s;ts:%s;", paymentID, headers.RequestID, ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(manifest))
	expected := mac.Sum(nil)

	received, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(expected, received) {
		return utils.Unauthorized("Assinatura de webhook inválida")
	}

	return nil
}

func parseWebhookSignature(header string) (ts, signature string, ok bool) {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "ts="):
			ts = strings.TrimPrefix(part, "ts=")
		case strings.HasPrefix(part, "v1="):
			signature = strings.TrimPrefix(part, "v1=")
		}
	}
	return ts, signature, ts != "" && signature != ""
}

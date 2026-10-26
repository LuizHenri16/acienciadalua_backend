package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Normalized payment states returned by POST /payment/status. They are the
// only states the frontend needs to know about; Mercado Pago has a few more
// (in_process, in_mediation, charged_back, ...) that are folded into these.
const (
	PaymentStatusPending  = "pending"
	PaymentStatusApproved = "approved"
	PaymentStatusRejected = "rejected"
	PaymentStatusExpired  = "expired"
	PaymentStatusRefunded = "refunded"
	PaymentStatusError    = "error"
)

// PreferenceResponse is returned by POST /payment/create-preferences.
type PreferenceResponse struct {
	InitPoint    string `json:"init_point"`
	PreferenceID string `json:"preference_id"`
}

// PixPaymentResponse is returned by POST /payment/pix. Optional fields are
// omitted when Mercado Pago does not return point_of_interaction data,
// matching Nest's JSON.stringify behaviour for undefined values.
//
// payment_id, status_token and expires_at are additive: existing clients that
// only read id/status/qr_code keep working unchanged.
type PixPaymentResponse struct {
	ID           int64            `json:"id"`
	Status       string           `json:"status"`
	QRCode       *string          `json:"qr_code,omitempty"`
	QRCodeBase64 *string          `json:"qr_code_base64,omitempty"`
	TicketURL    *string          `json:"ticket_url,omitempty"`
	PaymentID    string           `json:"payment_id,omitempty"`
	StatusToken  string           `json:"status_token,omitempty"`
	ExpiresAt    *types.Timestamp `json:"expires_at,omitempty"`
}

// PaymentStatusResponse is returned by POST /payment/status. accessGranted is
// only true when a matching Purchase exists, so the frontend can never infer
// access from provider status alone.
type PaymentStatusResponse struct {
	PaymentID      string           `json:"paymentId"`
	Status         string           `json:"status"`
	ProviderStatus string           `json:"providerStatus"`
	StatusDetail   string           `json:"statusDetail"`
	ProductID      string           `json:"productId"`
	Amount         types.Decimal    `json:"amount"`
	PurchaseID     *string          `json:"purchaseId"`
	AccessGranted  bool             `json:"accessGranted"`
	ApprovedAt     *types.Timestamp `json:"approvedAt"`
	ExpiresAt      *types.Timestamp `json:"expiresAt"`
	Retryable      bool             `json:"retryable"`
}

// WebhookRequest is the notification body sent by Mercado Pago.
type WebhookRequest struct {
	Type   string       `json:"type"`
	Action string       `json:"action"`
	Data   *WebhookData `json:"data"`
}

// WebhookData holds the payment id (Mercado Pago sends a number, but older
// notifications used a string).
type WebhookData struct {
	ID FlexibleID `json:"id"`
}

// WebhookHeaders carries the signature headers used to authenticate a
// Mercado Pago notification. Both are empty in legacy IPN notifications.
type WebhookHeaders struct {
	Signature string
	RequestID string
}

// FlexibleID accepts either a JSON number or a JSON string.
type FlexibleID string

func (f *FlexibleID) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" || raw == "" {
		*f = ""
		return nil
	}
	if len(raw) >= 2 && raw[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*f = FlexibleID(value)
		return nil
	}
	*f = FlexibleID(raw)
	return nil
}

// normalizedPayment is the provider payment mapped to the public state machine.
type normalizedPayment struct {
	Status     string
	Detail     string
	ExpiresAt  time.Time
	ApprovedAt time.Time
}

// PaymentService ports the Nest PaymentService.
type PaymentService struct {
	cfg         *config.Config
	pool        *pgxpool.Pool
	q           *db.Queries
	products    *ProductsService
	mercadoPago *MercadoPagoClient
	email       *EmailService
}

func NewPaymentService(
	cfg *config.Config,
	pool *pgxpool.Pool,
	q *db.Queries,
	products *ProductsService,
	mercadoPago *MercadoPagoClient,
	email *EmailService,
) *PaymentService {
	return &PaymentService{
		cfg:         cfg,
		pool:        pool,
		q:           q,
		products:    products,
		mercadoPago: mercadoPago,
		email:       email,
	}
}

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
		// The Nest implementation logs and swallows e-mail failures so the
		// webhook still returns 200.
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

var brtZone = time.FixedZone("BRT", -3*60*60)

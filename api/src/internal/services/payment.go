package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

// PreferenceResponse is returned by POST /payment/create-preferences.
type PreferenceResponse struct {
	InitPoint    string `json:"init_point"`
	PreferenceID string `json:"preference_id"`
}

// PixPaymentResponse is returned by POST /payment/pix. Optional fields are
// omitted when Mercado Pago does not return point_of_interaction data,
// matching Nest's JSON.stringify behaviour for undefined values.
type PixPaymentResponse struct {
	ID           int64   `json:"id"`
	Status       string  `json:"status"`
	QRCode       *string `json:"qr_code,omitempty"`
	QRCodeBase64 *string `json:"qr_code_base64,omitempty"`
	TicketURL    *string `json:"ticket_url,omitempty"`
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
func (s *PaymentService) CreatePixPayment(ctx context.Context, data dto.CreatePixPaymentRequest) (PixPaymentResponse, error) {
	product, err := s.products.FindOne(ctx, data.ProductID)
	if err != nil {
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

	payment, err := s.mercadoPago.CreatePayment(ctx, payload)
	if err != nil {
		slog.Error("Erro ao criar pagamento PIX", "error", err)
		return PixPaymentResponse{}, err
	}

	response := PixPaymentResponse{
		ID:     parsePaymentID(payment.ID),
		Status: payment.Status,
	}

	if payment.PointOfInteraction != nil && payment.PointOfInteraction.TransactionData != nil {
		transactionData := payment.PointOfInteraction.TransactionData
		response.QRCode = &transactionData.QRCode
		response.QRCodeBase64 = &transactionData.QRCodeBase64
		response.TicketURL = &transactionData.TicketURL
	}

	return response, nil
}

// HandleWebhook processes Mercado Pago notifications idempotently.
//
// notificationType and queryPaymentID are the query-string fallbacks used by
// Mercado Pago when the notification arrives as `?type=payment&data.id=...`.
func (s *PaymentService) HandleWebhook(ctx context.Context, body WebhookRequest, notificationType, queryPaymentID string) error {
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

	payment, err := s.mercadoPago.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}

	if payment.Status != "approved" {
		return nil
	}

	alreadyProcessed, err := s.q.GetPurchaseByPaymentID(ctx, paymentID)
	if err == nil && alreadyProcessed.ID != "" {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	return s.registerApprovedPayment(ctx, payment)
}

func (s *PaymentService) registerApprovedPayment(ctx context.Context, payment *MPPayment) error {
	paymentID := payment.ID.String()

	alreadyProcessed, err := s.q.GetPurchaseByPaymentID(ctx, paymentID)
	if err == nil && alreadyProcessed.ID != "" {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	payerEmail := ""
	if payment.Payer != nil {
		payerEmail = payment.Payer.Email
	}
	metadataEmail := ""
	metadataName := ""
	if payment.Metadata != nil {
		metadataEmail = payment.Metadata.CustomerEmail
		metadataName = payment.Metadata.CustomerName
	}

	email := metadataEmail
	if isEmail(payerEmail) {
		email = payerEmail
	}

	slog.Info("[Webhook] Dados do pagamento",
		"id", paymentID,
		"status", payment.Status,
		"payer_email", payerEmail,
		"metadata_email", metadataEmail,
		"metadata_name", metadataName,
		"email_final", email,
	)

	if !isEmail(email) {
		slog.Error("[Webhook] Nenhum email válido encontrado para o pagamento", "payment_id", paymentID)
		return utils.Internal("Pagamento sem e-mail válido")
	}

	customerName := email
	if payment.Payer != nil && payment.Payer.FirstName != "" {
		customerName = payment.Payer.FirstName
	} else if metadataName != "" {
		customerName = metadataName
	}

	purchasedAt := formatMPDate(payment.DateApproved)
	if purchasedAt.IsZero() {
		purchasedAt = formatMPDate(payment.DateCreated)
	}
	if purchasedAt.IsZero() {
		purchasedAt = time.Now().UTC()
	}

	now := types.NewTimestamp(time.Now())
	amountPaid := types.Decimal{Decimal: decimal.Zero}
	if raw := payment.TransactionAmount.String(); raw != "" {
		if parsed, err := decimal.NewFromString(raw); err == nil {
			amountPaid = types.Decimal{Decimal: parsed}
		}
	}

	// Customer upsert + purchase insert are atomic, and the unique index on
	// mercadoPagoPaymentId guarantees idempotency even under concurrent
	// deliveries of the same notification.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
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
		return err
	}

	_, err = queries.CreatePurchase(ctx, db.CreatePurchaseParams{
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
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if err := s.email.SendWelcomeEmail(ctx, customer.Email, customer.Name); err != nil {
		// The Nest implementation logs and swallows e-mail failures so the
		// webhook still returns 200.
		slog.Error("[EmailService] Erro ao enviar e-mail de boas-vindas", "error", err)
	}

	return nil
}

func parsePaymentID(id json.Number) int64 {
	parsed, err := strconv.ParseInt(id.String(), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

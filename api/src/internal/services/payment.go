package services

import (
	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
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

// PaymentService implements checkout and PIX payments.
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

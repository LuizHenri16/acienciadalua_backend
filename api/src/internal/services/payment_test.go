package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/database"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests run only when TEST_DATABASE_URL points at a disposable
// PostgreSQL database (they truncate the application tables).
func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, err := database.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, database.RunMigrations(ctx, dsn, logger))

	_, err = pool.Exec(ctx, `TRUNCATE "PaymentIntent", "Purchase", "AuthToken", "Product", "Customer", "User" CASCADE`)
	require.NoError(t, err)

	return pool
}

func integrationConfig() *config.Config {
	return &config.Config{
		MPWebhookURL:        "https://example.test/payment/webhook",
		MPSuccessURL:        "https://example.test/success",
		MPFailureURL:        "https://example.test/failure",
		MPPendingURL:        "https://example.test/pending",
		EmailSenderAddress:  "contato@example.test",
		ContactEmail:        "contato@example.test",
		FrontendURL:         "https://example.test",
		PaymentStatusSecret: "test-payment-status-secret",
		PaymentStatusTTL:    time.Hour,
		MPStatusTimeout:     5 * time.Second,
		MPPixExpiration:     30 * time.Minute,
	}
}

func createTestProduct(t *testing.T, pool *pgxpool.Pool, queries *db.Queries) db.Product {
	t.Helper()

	now := types.NewTimestamp(time.Now())
	product, err := queries.CreateProduct(context.Background(), db.CreateProductParams{
		ID:          uuid.NewString(),
		Title:       "Apostila de Química",
		Description: "Material completo",
		Price:       types.MustDecimal("59.5"),
		CoverUrl:    "cover.jpg",
		FileUrl:     "material.pdf",
		Category:    db.CategorySTUDENT,
		IsActive:    true,
		CreatedAt:   now,
	})
	require.NoError(t, err)
	return product
}

type fakeServices struct {
	*httptest.Server
	mu                 sync.Mutex
	emailCount         int
	pixCreateCount     int
	lastPixBody        map[string]any
	lastPreferenceBody map[string]any
	paymentStatus      map[string]string
	lastIdempotencyKey string
}

func newFakeServices(t *testing.T, productID string) *fakeServices {
	t.Helper()

	fake := &fakeServices{paymentStatus: map[string]string{}}

	fake.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/payments/"):
			paymentID := strings.TrimPrefix(r.URL.Path, "/v1/payments/")
			id, _ := strconv.ParseInt(paymentID, 10, 64)

			fake.mu.Lock()
			status := fake.paymentStatus[paymentID]
			fake.mu.Unlock()
			if status == "" {
				status = "approved"
			}

			payload := map[string]any{
				"id":                 id,
				"status":             status,
				"status_detail":      "detail_" + status,
				"external_reference": productID,
				"transaction_amount": 59.5,
				"date_of_expiration": time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
				"payer": map[string]any{
					"email":      "comprador@test.com",
					"first_name": "Comprador",
				},
				"metadata": map[string]any{
					"customer_email": "comprador@test.com",
					"customer_name":  "Comprador",
				},
			}
			if status == "approved" {
				payload["date_approved"] = "2026-10-24T16:33:00.000-04:00"
			}
			_ = json.NewEncoder(w).Encode(payload)

		case r.Method == http.MethodPost && r.URL.Path == "/v1/payments":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			fake.mu.Lock()
			fake.lastPixBody = body
			fake.pixCreateCount++
			fake.lastIdempotencyKey = r.Header.Get("X-Idempotency-Key")
			fake.mu.Unlock()

			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                 987654,
				"status":             "pending",
				"status_detail":      "pending_waiting_transfer",
				"external_reference": productID,
				"transaction_amount": 59.5,
				"date_of_expiration": time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
				"point_of_interaction": map[string]any{
					"transaction_data": map[string]any{
						"qr_code":        "00020126580014BR.GOV.BCB.PIX",
						"qr_code_base64": "iVBORw0KGgo=",
						"ticket_url":     "https://mercadopago.test/ticket",
					},
				},
			})

		case r.Method == http.MethodPost && r.URL.Path == "/checkout/preferences":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			fake.mu.Lock()
			fake.lastPreferenceBody = body
			fake.mu.Unlock()

			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "preference-123",
				"init_point": "https://mercadopago.test/checkout",
			})

		case r.Method == http.MethodPost && r.URL.Path == "/emails":
			fake.mu.Lock()
			fake.emailCount++
			fake.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "email-123"})

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.Close)

	return fake
}

func newIntegrationPaymentService(t *testing.T, pool *pgxpool.Pool, fake *fakeServices) (*PaymentService, *db.Queries, *config.Config) {
	t.Helper()

	cfg := integrationConfig()
	queries := db.New(pool)
	products := NewProductsService(queries)

	mercadoPago := &MercadoPagoClient{
		accessToken: "test-token",
		baseURL:     fake.URL,
		client:      fake.Client(),
	}

	email := NewEmailService(cfg)
	email.baseURL = fake.URL

	return NewPaymentService(cfg, pool, queries, products, mercadoPago, email), queries, cfg
}

func TestWebhookIsIdempotent(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, queries, _ := newIntegrationPaymentService(t, pool, fake)

	body := WebhookRequest{
		Type: "payment",
		Data: &WebhookData{ID: "123456"},
	}

	require.NoError(t, service.HandleWebhook(context.Background(), body, "", "", WebhookHeaders{}))
	require.NoError(t, service.HandleWebhook(context.Background(), body, "", "", WebhookHeaders{}))

	var purchaseCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "Purchase"`).Scan(&purchaseCount))
	assert.Equal(t, 1, purchaseCount, "webhook must be idempotent")

	var customerCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "Customer"`).Scan(&customerCount))
	assert.Equal(t, 1, customerCount)

	purchase, err := queries.GetPurchaseByPaymentID(context.Background(), "123456")
	require.NoError(t, err)
	assert.Equal(t, "59.5", purchase.AmountPaid.String())
	assert.Equal(t, product.ID, purchase.ProductId)

	purchasedAt, err := json.Marshal(purchase.PurchasedAt)
	require.NoError(t, err)
	assert.Equal(t, `"2026-10-24T20:33:00.000Z"`, string(purchasedAt))

	fake.mu.Lock()
	assert.Equal(t, 1, fake.emailCount, "welcome e-mail must be sent once")
	fake.mu.Unlock()
}

func TestWebhookIgnoresNonApprovedAndOtherTypes(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                 123456,
			"status":             "pending",
			"external_reference": product.ID,
		})
	}))
	t.Cleanup(server.Close)

	cfg := integrationConfig()
	mercadoPago := &MercadoPagoClient{accessToken: "token", baseURL: server.URL, client: server.Client()}
	email := NewEmailService(cfg)
	email.baseURL = server.URL
	service := NewPaymentService(cfg, pool, queries, NewProductsService(queries), mercadoPago, email)

	require.NoError(t, service.HandleWebhook(context.Background(), WebhookRequest{
		Type: "payment",
		Data: &WebhookData{ID: "123456"},
	}, "", "", WebhookHeaders{}))
	require.NoError(t, service.HandleWebhook(context.Background(), WebhookRequest{
		Type: "merchant_order",
		Data: &WebhookData{ID: "123456"},
	}, "", "", WebhookHeaders{}))

	var purchaseCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "Purchase"`).Scan(&purchaseCount))
	assert.Zero(t, purchaseCount)
}

func TestCreatePixPaymentContract(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, _ := newIntegrationPaymentService(t, pool, fake)

	name := "Comprador"
	cpf := "12345678909"

	response, err := service.CreatePixPayment(context.Background(), dto.CreatePixPaymentRequest{
		ProductID: product.ID,
		Payer: &dto.PixPayer{
			Email: "comprador@test.com",
			Name:  &name,
			CPF:   &cpf,
		},
	})
	require.NoError(t, err)

	assert.EqualValues(t, 987654, response.ID)
	assert.Equal(t, "pending", response.Status)
	require.NotNil(t, response.QRCode)
	assert.Equal(t, "00020126580014BR.GOV.BCB.PIX", *response.QRCode)
	require.NotNil(t, response.QRCodeBase64)
	require.NotNil(t, response.TicketURL)

	fake.mu.Lock()
	body := fake.lastPixBody
	fake.mu.Unlock()

	assert.EqualValues(t, 59.5, body["transaction_amount"])
	assert.Equal(t, "pix", body["payment_method_id"])
	assert.Equal(t, product.ID, body["external_reference"])
	assert.Equal(t, "https://example.test/payment/webhook", body["notification_url"])

	payer := body["payer"].(map[string]any)
	assert.Equal(t, "comprador@test.com", payer["email"])
	assert.Equal(t, "Comprador", payer["first_name"])
	identification := payer["identification"].(map[string]any)
	assert.Equal(t, "CPF", identification["type"])
	assert.Equal(t, "12345678909", identification["number"])
}

func TestCreatePreferencesContract(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, _ := newIntegrationPaymentService(t, pool, fake)

	response, err := service.CreatePreferences(context.Background(), product.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://mercadopago.test/checkout", response.InitPoint)
	assert.Equal(t, "preference-123", response.PreferenceID)

	fake.mu.Lock()
	body := fake.lastPreferenceBody
	fake.mu.Unlock()

	items := body["items"].([]any)
	item := items[0].(map[string]any)
	assert.Equal(t, product.ID, item["id"])
	assert.EqualValues(t, 59.5, item["unit_price"])
	assert.EqualValues(t, 1, item["quantity"])
	assert.Equal(t, product.ID, body["external_reference"])
	assert.Equal(t, "approved", body["auto_return"])

	paymentMethods := body["payment_methods"].(map[string]any)
	assert.EqualValues(t, 6, paymentMethods["installments"])
}

func (f *fakeServices) setPaymentStatus(paymentID, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paymentStatus[paymentID] = status
}

func createPixForTest(t *testing.T, service *PaymentService, productID, idempotencyKey string) PixPaymentResponse {
	t.Helper()

	name := "Comprador"
	response, err := service.CreatePixPayment(context.Background(), dto.CreatePixPaymentRequest{
		ProductID:      productID,
		Payer:          &dto.PixPayer{Email: "comprador@test.com", Name: &name},
		IdempotencyKey: &idempotencyKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, response.PaymentID)
	require.NotEmpty(t, response.StatusToken)
	require.NotNil(t, response.ExpiresAt)
	return response
}

func TestPaymentStatusPendingThenApproved(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, _ := newIntegrationPaymentService(t, pool, fake)

	pix := createPixForTest(t, service, product.ID, "checkout-status-0001")

	fake.setPaymentStatus(pix.PaymentID, "pending")
	status, err := service.GetPaymentStatus(context.Background(), dto.PaymentStatusRequest{
		PaymentID: pix.PaymentID,
		Token:     pix.StatusToken,
	})
	require.NoError(t, err)
	assert.Equal(t, PaymentStatusPending, status.Status)
	assert.False(t, status.AccessGranted)
	assert.Nil(t, status.PurchaseID)

	var purchaseCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "Purchase"`).Scan(&purchaseCount))
	assert.Zero(t, purchaseCount)

	fake.setPaymentStatus(pix.PaymentID, "approved")
	status, err = service.GetPaymentStatus(context.Background(), dto.PaymentStatusRequest{
		PaymentID: pix.PaymentID,
		Token:     pix.StatusToken,
	})
	require.NoError(t, err)
	assert.Equal(t, PaymentStatusApproved, status.Status)
	assert.True(t, status.AccessGranted)
	require.NotNil(t, status.PurchaseID)
	assert.Equal(t, product.ID, status.ProductID)

	// Repeated polling must not duplicate the purchase or the welcome e-mail.
	_, err = service.GetPaymentStatus(context.Background(), dto.PaymentStatusRequest{
		PaymentID: pix.PaymentID,
		Token:     pix.StatusToken,
	})
	require.NoError(t, err)

	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "Purchase"`).Scan(&purchaseCount))
	assert.Equal(t, 1, purchaseCount)

	fake.mu.Lock()
	assert.Equal(t, 1, fake.emailCount)
	fake.mu.Unlock()
}

func TestCreatePixPaymentIsIdempotent(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, _ := newIntegrationPaymentService(t, pool, fake)

	const key = "checkout-idempotent-0001"
	first := createPixForTest(t, service, product.ID, key)
	second := createPixForTest(t, service, product.ID, key)

	assert.Equal(t, first.PaymentID, second.PaymentID)
	assert.Equal(t, first.StatusToken, second.StatusToken)

	fake.mu.Lock()
	assert.Equal(t, 1, fake.pixCreateCount, "a retry must not create a second PIX")
	fake.mu.Unlock()

	var intentCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "PaymentIntent"`).Scan(&intentCount))
	assert.Equal(t, 1, intentCount)
}

func TestPaymentStatusRejectsInvalidToken(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, _ := newIntegrationPaymentService(t, pool, fake)

	pix := createPixForTest(t, service, product.ID, "checkout-token-0001")

	_, err := service.GetPaymentStatus(context.Background(), dto.PaymentStatusRequest{
		PaymentID: pix.PaymentID,
		Token:     pix.StatusToken + "tampered",
	})
	require.Error(t, err)
	appErr, ok := utils.AsError(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, appErr.StatusCode)

	_, err = service.GetPaymentStatus(context.Background(), dto.PaymentStatusRequest{
		PaymentID: "999999999",
		Token:     pix.StatusToken,
	})
	require.Error(t, err)
	appErr, ok = utils.AsError(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
}

func TestWebhookSignatureValidation(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, cfg := newIntegrationPaymentService(t, pool, fake)
	cfg.MPWebhookSecret = "webhook-secret"

	body := WebhookRequest{Type: "payment", Data: &WebhookData{ID: "123456"}}

	err := service.HandleWebhook(context.Background(), body, "", "", WebhookHeaders{})
	require.Error(t, err, "webhook without signature must be rejected")
	appErr, ok := utils.AsError(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, appErr.StatusCode)

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	manifest := "id:123456;request-id:req-1;ts:" + ts + ";"
	mac := hmac.New(sha256.New, []byte(cfg.MPWebhookSecret))
	mac.Write([]byte(manifest))

	headers := WebhookHeaders{
		Signature: "ts=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil)),
		RequestID: "req-1",
	}
	require.NoError(t, service.HandleWebhook(context.Background(), body, "", "", headers))

	var purchaseCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "Purchase"`).Scan(&purchaseCount))
	assert.Equal(t, 1, purchaseCount)
}

func TestReconcilePendingCreatesPurchase(t *testing.T) {
	pool := integrationPool(t)
	queries := db.New(pool)
	product := createTestProduct(t, pool, queries)
	fake := newFakeServices(t, product.ID)
	service, _, _ := newIntegrationPaymentService(t, pool, fake)

	pix := createPixForTest(t, service, product.ID, "checkout-reconcile-0001")

	fake.setPaymentStatus(pix.PaymentID, "approved")
	require.NoError(t, service.ReconcilePending(context.Background()))

	purchase, err := queries.GetPurchaseByPaymentID(context.Background(), pix.PaymentID)
	require.NoError(t, err)
	assert.Equal(t, product.ID, purchase.ProductId)

	intent, err := queries.GetPaymentIntentByPaymentID(context.Background(), pix.PaymentID)
	require.NoError(t, err)
	assert.Equal(t, PaymentStatusApproved, intent.Status)
	require.NotNil(t, intent.PurchaseId)
}

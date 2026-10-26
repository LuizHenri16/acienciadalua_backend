package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
)

const mercadoPagoBaseURL = "https://api.mercadopago.com"

// MercadoPagoClient is a small REST client for the endpoints used by the
// application. It replaces the official Node SDK.
type MercadoPagoClient struct {
	accessToken string
	baseURL     string
	client      *http.Client
}

func NewMercadoPagoClient(cfg *config.Config) *MercadoPagoClient {
	return &MercadoPagoClient{
		accessToken: cfg.MercadoPagoAccessToken,
		baseURL:     mercadoPagoBaseURL,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// MPPayer is the payer object returned by Mercado Pago.
type MPPayer struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
}

// MPMetadata holds the custom metadata sent when creating a PIX payment.
type MPMetadata struct {
	CustomerEmail string `json:"customer_email"`
	CustomerName  string `json:"customer_name"`
}

// MPTransactionData is the PIX payload returned inside point_of_interaction.
type MPTransactionData struct {
	QRCode       string `json:"qr_code"`
	QRCodeBase64 string `json:"qr_code_base64"`
	TicketURL    string `json:"ticket_url"`
}

// MPPointOfInteraction wraps the PIX transaction data.
type MPPointOfInteraction struct {
	TransactionData *MPTransactionData `json:"transaction_data"`
}

// MPPayment is the subset of the Mercado Pago payment resource used here.
type MPPayment struct {
	ID                        json.Number           `json:"id"`
	Status                    string                `json:"status"`
	StatusDetail              string                `json:"status_detail"`
	ExternalReference         string                `json:"external_reference"`
	TransactionAmount         json.Number           `json:"transaction_amount"`
	TransactionAmountRefunded json.Number           `json:"transaction_amount_refunded"`
	PaymentMethodID           string                `json:"payment_method_id"`
	DateApproved              string                `json:"date_approved"`
	DateCreated               string                `json:"date_created"`
	DateLastUpdated           string                `json:"date_last_updated"`
	DateOfExpiration          string                `json:"date_of_expiration"`
	Payer                     *MPPayer              `json:"payer"`
	Metadata                  *MPMetadata           `json:"metadata"`
	PointOfInteraction        *MPPointOfInteraction `json:"point_of_interaction"`
}

// MPError is a Mercado Pago transport/API error. StatusCode is 0 for network
// failures and timeouts. Retryable tells the status endpoint whether the
// client should try again instead of treating the payment as invalid.
type MPError struct {
	StatusCode int
	Retryable  bool
	Message    string
}

func (e *MPError) Error() string {
	return e.Message
}

// MPPreference is the subset of the checkout preference response used here.
type MPPreference struct {
	ID        string `json:"id"`
	InitPoint string `json:"init_point"`
}

// CreatePreference creates a checkout preference.
func (c *MercadoPagoClient) CreatePreference(ctx context.Context, payload any) (*MPPreference, error) {
	var response MPPreference
	if err := c.post(ctx, "/checkout/preferences", payload, &response, ""); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreatePayment creates a PIX payment. Mercado Pago requires an idempotency key
// for POST /v1/payments; pass the key generated for this checkout so retries
// return the same payment instead of creating a second charge.
func (c *MercadoPagoClient) CreatePayment(ctx context.Context, payload any, idempotencyKey string) (*MPPayment, error) {
	var response MPPayment
	if err := c.post(ctx, "/v1/payments", payload, &response, idempotencyKey); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetPayment fetches a payment by id.
func (c *MercadoPagoClient) GetPayment(ctx context.Context, id string) (*MPPayment, error) {
	if c.accessToken == "" {
		return nil, utils.Internal("Erro inesperado")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/payments/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.accessToken)

	var response MPPayment
	if err := c.do(request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *MercadoPagoClient) post(ctx context.Context, path string, payload any, out any, idempotencyKey string) error {
	if c.accessToken == "" {
		return utils.Internal("Erro inesperado")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.accessToken)
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}
	request.Header.Set("X-Idempotency-Key", idempotencyKey)

	return c.do(request, out)
}

func (c *MercadoPagoClient) do(request *http.Request, out any) error {
	response, err := c.client.Do(request)
	if err != nil {
		return &MPError{
			StatusCode: 0,
			Retryable:  !errors.Is(err, context.Canceled),
			Message:    "Mercado Pago request failed",
		}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return &MPError{StatusCode: 0, Retryable: true, Message: "Mercado Pago response could not be read"}
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &MPError{
			StatusCode: response.StatusCode,
			Retryable:  response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
			Message:    mercadoPagoErrorMessage(body),
		}
	}

	if out == nil || len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, out); err != nil {
		return &MPError{StatusCode: response.StatusCode, Retryable: true, Message: "Mercado Pago returned an invalid response"}
	}

	return nil
}

func mercadoPagoErrorMessage(body []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Cause   []struct {
			Description string `json:"description"`
		} `json:"cause"`
	}

	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Message != "" {
			return parsed.Message
		}
		if parsed.Error != "" {
			return parsed.Error
		}
		for _, cause := range parsed.Cause {
			if cause.Description != "" {
				return cause.Description
			}
		}
	}

	if text := strings.TrimSpace(string(body)); text != "" && len(text) < 500 {
		return text
	}

	return "Erro inesperado"
}

// formatMPDate converts a Mercado Pago ISO-8601 date to UTC.
func formatMPDate(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}

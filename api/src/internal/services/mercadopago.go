package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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

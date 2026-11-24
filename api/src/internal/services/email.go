// Package services holds the business rules.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/acienciadalua/api/internal/config"
)

const resendBaseURL = "https://api.resend.com"

// EmailService sends transactional e-mails through Resend.
type EmailService struct {
	cfg     *config.Config
	client  *http.Client
	baseURL string
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		cfg:     cfg,
		client:  &http.Client{Timeout: 20 * time.Second},
		baseURL: resendBaseURL,
	}
}

type resendEmail struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

type resendResponse struct {
	ID string `json:"id"`
}

type resendError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

func (s *EmailService) send(ctx context.Context, payload resendEmail) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var apiErr resendError
		if err := json.Unmarshal(responseBody, &apiErr); err == nil && apiErr.Message != "" {
			return "", fmt.Errorf("%s", apiErr.Message)
		}
		return "", fmt.Errorf("resend request failed with status %d", response.StatusCode)
	}

	var parsed resendResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", nil // response without a body/id is still a success
	}

	return parsed.ID, nil
}

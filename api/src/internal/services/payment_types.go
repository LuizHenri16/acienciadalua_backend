package services

import (
	"encoding/json"
	"strings"
	"time"
)

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

package services

import (
	"encoding/json"
	"strings"
	"time"
)

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

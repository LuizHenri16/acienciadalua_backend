package services

import (
	"encoding/json"
)

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

package dto

import "github.com/acienciadalua/api/internal/utils"

// CreatePreferencesRequest is the body of POST /payment/create-preferences.
type CreatePreferencesRequest struct {
	ProductID string `json:"productId" validate:"required"`
}

var createPreferencesMessages = utils.Messages{
	"productId.required": "productId should not be empty",
}

func (d CreatePreferencesRequest) Validate() []string {
	return utils.Validate(d, createPreferencesMessages)
}

// PixPayer is the payer block accepted by POST /payment/pix.
type PixPayer struct {
	Email string  `json:"email" validate:"required,email"`
	Name  *string `json:"name"`
	CPF   *string `json:"cpf"`
}

// CreatePixPaymentRequest is the body of POST /payment/pix.
type CreatePixPaymentRequest struct {
	ProductID string    `json:"productId" validate:"required"`
	Payer     *PixPayer `json:"payer" validate:"required"`
	// IdempotencyKey lets the client retry the same checkout without creating
	// a second PIX. Optional for backwards compatibility.
	IdempotencyKey *string `json:"idempotencyKey" validate:"omitempty,min=8,max=128"`
}

var createPixMessages = utils.Messages{
	"productId.required":   "productId should not be empty",
	"payer.required":       "payer should not be empty",
	"payer.email.required": "email should not be empty",
	"payer.email.email":    "email must be an email",
	"idempotencyKey.min":   "idempotencyKey must be longer than or equal to 8 characters",
	"idempotencyKey.max":   "idempotencyKey must be shorter than or equal to 128 characters",
}

func (d CreatePixPaymentRequest) Validate() []string {
	return utils.Validate(d, createPixMessages)
}

// PaymentStatusRequest is the body of POST /payment/status. The token is a
// signed capability, so it is sent in the body instead of a custom header
// (keeps CORS unchanged and avoids leaking it in access logs).
type PaymentStatusRequest struct {
	PaymentID string `json:"paymentId" validate:"required"`
	Token     string `json:"token" validate:"required"`
}

var paymentStatusMessages = utils.Messages{
	"paymentId.required": "paymentId should not be empty",
	"token.required":     "token should not be empty",
}

func (d PaymentStatusRequest) Validate() []string {
	return utils.Validate(d, paymentStatusMessages)
}

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
}

var createPixMessages = utils.Messages{
	"productId.required":   "productId should not be empty",
	"payer.required":       "payer should not be empty",
	"payer.email.required": "email should not be empty",
	"payer.email.email":    "email must be an email",
}

func (d CreatePixPaymentRequest) Validate() []string {
	return utils.Validate(d, createPixMessages)
}

package dto

import (
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
)

// PurchaseProduct is an item of GET /customers/me/purchases. It mirrors the
// Prisma `select` used by the Nest CustomersService (note: no price field)
// plus the purchase date.
type PurchaseProduct struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	CoverUrl    string          `json:"coverUrl"`
	FileUrl     string          `json:"fileUrl"`
	Category    string          `json:"category"`
	PurchasedAt types.Timestamp `json:"purchasedAt"`
}

// MyPurchasesResponse groups purchased materials by category.
type MyPurchasesResponse struct {
	Student []PurchaseProduct `json:"student"`
	Teacher []PurchaseProduct `json:"teacher"`
}

// ContactRequest is the body of POST /contact.
type ContactRequest struct {
	Name    string  `json:"name" validate:"required"`
	Email   string  `json:"email" validate:"required,email"`
	Subject *string `json:"subject"`
	Message string  `json:"message" validate:"required"`
}

var contactMessages = utils.Messages{
	"name.required":    "Nome é obrigatório",
	"email.required":   "Email é obrigatório",
	"email.email":      "Email inválido",
	"message.required": "Mensagem é obrigatória",
}

func (d ContactRequest) Validate() []string { return utils.Validate(d, contactMessages) }

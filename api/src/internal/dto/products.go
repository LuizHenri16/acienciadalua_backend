package dto

import "github.com/acienciadalua/api/internal/utils"

// Category values accepted by the API (Prisma enum Category).
const (
	CategoryStudent = "STUDENT"
	CategoryTeacher = "TEACHER"
)

// CreateProductRequest is the multipart/JSON body for POST /products.
// Pointers are used where the original DTO distinguishes "missing" from a
// zero value (price 0 and isActive false are both valid).
type CreateProductRequest struct {
	Title       string   `json:"title" validate:"required"`
	Description string   `json:"description" validate:"required"`
	Price       *float64 `json:"price" validate:"required,gte=0"`
	IsActive    *bool    `json:"isActive" validate:"required"`
	Category    string   `json:"category" validate:"required,oneof=STUDENT TEACHER"`
}

var createProductMessages = utils.Messages{
	"title.required":       "title should not be empty",
	"description.required": "description should not be empty",
	"price.required":       "price must be a number conforming to the specified constraints",
	"price.gte":            "price must not be less than 0",
	"isActive.required":    "isActive must be a boolean value",
	"category.required":    "category must be one of the following values: STUDENT, TEACHER",
	"category.oneof":       "category must be one of the following values: STUDENT, TEACHER",
}

func (d CreateProductRequest) Validate() []string { return utils.Validate(d, createProductMessages) }

// UpdateProductRequest is the multipart/JSON body for PUT /products/:id.
// Every field is optional; nil means "not provided".
type UpdateProductRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price" validate:"omitempty,gte=0"`
	IsActive    *bool    `json:"isActive"`
	Category    *string  `json:"category" validate:"omitempty,oneof=STUDENT TEACHER"`
}

var updateProductMessages = utils.Messages{
	"price.gte":      "price must not be less than 0",
	"category.oneof": "category must be one of the following values: STUDENT, TEACHER",
}

func (d UpdateProductRequest) Validate() []string { return utils.Validate(d, updateProductMessages) }

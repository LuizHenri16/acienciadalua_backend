package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignUpValidationMessages(t *testing.T) {
	request := SignUpRequest{Name: "Lu", Email: "bad", Password: "123"}

	assert.Equal(t, []string{
		"Name must be at least 3 characters",
		"Invalid email format",
		"Password must be at least 8 characters",
	}, request.Validate())
}

func TestCreateProductValidation(t *testing.T) {
	request := CreateProductRequest{}

	assert.Equal(t, []string{
		"title should not be empty",
		"description should not be empty",
		"price must be a number conforming to the specified constraints",
		"isActive must be a boolean value",
		"category must be one of the following values: STUDENT, TEACHER",
	}, request.Validate())

	price := 0.0
	isActive := false
	valid := CreateProductRequest{
		Title:       "Apostila",
		Description: "Descrição",
		Price:       &price,
		IsActive:    &isActive,
		Category:    "TEACHER",
	}
	assert.Empty(t, valid.Validate())
}

func TestPixValidationIncludesNestedField(t *testing.T) {
	request := CreatePixPaymentRequest{
		ProductID: "product-id",
		Payer:     &PixPayer{Email: "not-an-email"},
	}

	// class-validator reports the nested property name only, like Nest does.
	assert.Equal(t, []string{"email must be an email"}, request.Validate())
}

func TestContactValidationMessages(t *testing.T) {
	request := ContactRequest{}

	assert.Equal(t, []string{
		"Nome é obrigatório",
		"Email é obrigatório",
		"Mensagem é obrigatória",
	}, request.Validate())
}

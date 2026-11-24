package handlers

import (
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/types"
	"github.com/shopspring/decimal"
)

// ─── DTO → service input ─────────────────────────────────────────────────────

func createInput(request dto.CreateProductRequest, coverURL, fileURL string) services.CreateProductInput {
	input := services.CreateProductInput{
		Title:       request.Title,
		Description: request.Description,
		IsActive:    request.IsActive != nil && *request.IsActive,
		Category:    request.Category,
		CoverURL:    coverURL,
		FileURL:     fileURL,
	}

	if request.Price != nil {
		input.Price = types.Decimal{Decimal: decimal.NewFromFloat(*request.Price)}
	}

	return input
}

func updateInput(request dto.UpdateProductRequest, coverURL, fileURL *string) services.UpdateProductInput {
	input := services.UpdateProductInput{
		Title:       request.Title,
		Description: request.Description,
		IsActive:    request.IsActive,
		Category:    request.Category,
		CoverURL:    coverURL,
		FileURL:     fileURL,
	}

	if request.Price != nil {
		price := types.Decimal{Decimal: decimal.NewFromFloat(*request.Price)}
		input.Price = &price
	}

	return input
}

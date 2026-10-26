package services

import (
	"context"
	"errors"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateProductInput carries validated product data plus the stored filenames.
type CreateProductInput struct {
	Title       string
	Description string
	Price       types.Decimal
	IsActive    bool
	Category    string
	CoverURL    string
	FileURL     string
}

// UpdateProductInput uses nil for "field not provided".
type UpdateProductInput struct {
	Title       *string
	Description *string
	Price       *types.Decimal
	IsActive    *bool
	Category    *string
	CoverURL    *string
	FileURL     *string
}

// ProductsService implements the product catalog and admin CRUD.
type ProductsService struct {
	q *db.Queries
}

func NewProductsService(q *db.Queries) *ProductsService {
	return &ProductsService{q: q}
}

func (s *ProductsService) FindAll(ctx context.Context, category *string, isActive *bool) ([]db.Product, error) {
	return s.q.ListProducts(ctx, db.ListProductsParams{
		Category: category,
		IsActive: isActive,
	})
}

func (s *ProductsService) FindOne(ctx context.Context, id string) (db.Product, error) {
	product, err := s.q.GetProductByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Product{}, utils.NotFound("Product not found.")
	}
	if err != nil {
		return db.Product{}, err
	}
	return product, nil
}

func (s *ProductsService) Create(ctx context.Context, input CreateProductInput) (db.Product, error) {
	return s.q.CreateProduct(ctx, db.CreateProductParams{
		ID:          uuid.NewString(),
		Title:       input.Title,
		Description: input.Description,
		Price:       input.Price,
		CoverUrl:    input.CoverURL,
		FileUrl:     input.FileURL,
		Category:    db.Category(input.Category),
		IsActive:    input.IsActive,
		CreatedAt:   types.NewTimestamp(time.Now()),
	})
}

func (s *ProductsService) Update(ctx context.Context, id string, input UpdateProductInput) (db.Product, error) {
	current, err := s.FindOne(ctx, id)
	if err != nil {
		return db.Product{}, err
	}

	title := current.Title
	if input.Title != nil {
		title = *input.Title
	}
	description := current.Description
	if input.Description != nil {
		description = *input.Description
	}
	price := current.Price
	if input.Price != nil {
		price = *input.Price
	}
	isActive := current.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	category := string(current.Category)
	if input.Category != nil {
		category = *input.Category
	}
	coverURL := current.CoverUrl
	if input.CoverURL != nil {
		coverURL = *input.CoverURL
	}
	fileURL := current.FileUrl
	if input.FileURL != nil {
		fileURL = *input.FileURL
	}

	return s.q.UpdateProduct(ctx, db.UpdateProductParams{
		ID:          id,
		Title:       title,
		Description: description,
		Price:       price,
		CoverUrl:    coverURL,
		FileUrl:     fileURL,
		Category:    db.Category(category),
		IsActive:    isActive,
		UpdatedAt:   types.NewTimestamp(time.Now()),
	})
}

func (s *ProductsService) Toggle(ctx context.Context, id string) (db.Product, error) {
	if _, err := s.FindOne(ctx, id); err != nil {
		return db.Product{}, err
	}

	return s.q.ToggleProduct(ctx, db.ToggleProductParams{
		ID:        id,
		UpdatedAt: types.NewTimestamp(time.Now()),
	})
}

func (s *ProductsService) Remove(ctx context.Context, id string) (db.Product, error) {
	if _, err := s.FindOne(ctx, id); err != nil {
		return db.Product{}, err
	}

	product, err := s.q.DeleteProduct(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Product{}, utils.NotFound("Product not found.")
	}
	return product, err
}

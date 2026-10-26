package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/jackc/pgx/v5"
)

// Download describes a purchased file ready to be streamed.
type Download struct {
	Product db.Product
	Path    string
	FileURL string
}

// CustomersService implements customer profile, purchases and downloads.
type CustomersService struct {
	cfg *config.Config
	q   *db.Queries
}

func NewCustomersService(cfg *config.Config, q *db.Queries) *CustomersService {
	return &CustomersService{cfg: cfg, q: q}
}

func (s *CustomersService) GetMe(ctx context.Context, customerID string) (db.Customer, error) {
	customer, err := s.q.GetCustomerByID(ctx, customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Customer{}, utils.NotFound("Customer not found.")
	}
	if err != nil {
		return db.Customer{}, err
	}
	return customer, nil
}

func (s *CustomersService) GetMyPurchases(ctx context.Context, customerID string) (dto.MyPurchasesResponse, error) {
	rows, err := s.q.ListCustomerPurchases(ctx, customerID)
	if err != nil {
		return dto.MyPurchasesResponse{}, err
	}

	response := dto.MyPurchasesResponse{
		Student: []dto.PurchaseProduct{},
		Teacher: []dto.PurchaseProduct{},
	}

	for _, row := range rows {
		item := dto.PurchaseProduct{
			ID:          row.ID,
			Title:       row.Title,
			Description: row.Description,
			CoverUrl:    row.CoverUrl,
			FileUrl:     row.FileUrl,
			Category:    string(row.Category),
			PurchasedAt: row.PurchasedAt,
		}

		switch string(row.Category) {
		case "STUDENT":
			response.Student = append(response.Student, item)
		case "TEACHER":
			response.Teacher = append(response.Teacher, item)
		}
	}

	return response, nil
}

func (s *CustomersService) GetDownload(ctx context.Context, customerID, productID string) (Download, error) {
	purchase, err := s.q.GetPurchaseForCustomer(ctx, db.GetPurchaseForCustomerParams{
		CustomerId: customerID,
		ProductId:  productID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Download{}, utils.NotFound("Compra não encontrada.")
	}
	if err != nil {
		return Download{}, err
	}

	product, err := s.q.GetProductByID(ctx, purchase.ProductId)
	if errors.Is(err, pgx.ErrNoRows) {
		return Download{}, utils.NotFound("Produto sem arquivo.")
	}
	if err != nil {
		return Download{}, err
	}

	if product.FileUrl == "" {
		return Download{}, utils.NotFound("Produto sem arquivo.")
	}

	fileName := filepath.Base(product.FileUrl)
	filePath := filepath.Join(s.cfg.FilesDir, fileName)
	legacyPath := filepath.Join(s.cfg.UploadDir, fileName)

	path := ""
	switch {
	case fileExists(filePath):
		path = filePath
	case fileExists(legacyPath):
		path = legacyPath
	default:
		return Download{}, utils.NotFound("Arquivo não encontrado no servidor.")
	}

	return Download{Product: product, Path: path, FileURL: product.FileUrl}, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

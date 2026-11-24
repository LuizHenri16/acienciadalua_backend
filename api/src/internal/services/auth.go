package services

import (
	"errors"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/jackc/pgx/v5/pgconn"
)

// TokenPair is returned by signup/signin.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// AuthService implements admin and customer authentication.
type AuthService struct {
	cfg   *config.Config
	q     *db.Queries
	email *EmailService
}

func NewAuthService(cfg *config.Config, q *db.Queries, email *EmailService) *AuthService {
	return &AuthService{cfg: cfg, q: q, email: email}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

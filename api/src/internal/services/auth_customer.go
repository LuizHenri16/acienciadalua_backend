package services

import (
	"context"
	"errors"
	"time"

	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// customerTokenTTL is the lifetime of customer access tokens.
const customerTokenTTL = time.Hour

// ResetTokenTTL is the 1 hour validity used by customerForgotPassword.
const ResetTokenTTL = time.Hour

// ─── Customer ────────────────────────────────────────────────────────────────

func (s *AuthService) CustomerSignIn(ctx context.Context, data dto.CustomerSignInRequest) (string, error) {
	customer, err := s.q.GetCustomerByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.Unauthorized("Uma conta com este usuário não foi encontrado")
	}
	if err != nil {
		return "", err
	}

	if customer.Password == nil {
		return "", utils.Unauthorized("NO_PASSWORD_SET")
	}

	if !utils.ComparePassword(data.Password, *customer.Password) {
		return "", utils.Unauthorized("Email ou senha de usuário incorretos.")
	}

	return utils.SignCustomerToken(customer.ID, customer.Email, customer.Name, s.cfg.JWTSecretCustomer, customerTokenTTL)
}

func (s *AuthService) CustomerSetPassword(ctx context.Context, data dto.CustomerSetPasswordRequest) (string, error) {
	customer, err := s.q.GetCustomerByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.NotFound("Uma conta com este usuário não foi encontrado")
	}
	if err != nil {
		return "", err
	}

	if customer.Password != nil {
		return "", utils.BadRequest("Senha já definida. Use a opção 'esqueci minha senha' para redefinir.")
	}

	hashedPassword, err := utils.HashPassword(data.Password)
	if err != nil {
		return "", err
	}

	now := types.NewTimestamp(time.Now())
	if _, err := s.q.UpdateCustomerPassword(ctx, db.UpdateCustomerPasswordParams{
		ID:        customer.ID,
		Password:  &hashedPassword,
		UpdatedAt: now,
	}); err != nil {
		return "", err
	}

	return utils.SignCustomerToken(customer.ID, customer.Email, customer.Name, s.cfg.JWTSecretCustomer, customerTokenTTL)
}

func (s *AuthService) CustomerForgotPassword(ctx context.Context, data dto.ForgotPasswordRequest) error {
	customer, err := s.q.GetCustomerByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		// Don't reveal whether the e-mail exists.
		return nil
	}
	if err != nil {
		return err
	}

	resetToken := uuid.NewString()
	expiresAt := types.NewTimestamp(time.Now().Add(ResetTokenTTL))
	now := types.NewTimestamp(time.Now())

	if _, err := s.q.SetCustomerResetToken(ctx, db.SetCustomerResetTokenParams{
		ID:                  customer.ID,
		ResetToken:          &resetToken,
		ResetTokenExpiresAt: &expiresAt,
		UpdatedAt:           now,
	}); err != nil {
		return err
	}

	return s.email.SendResetPasswordEmail(ctx, customer.Email, resetToken)
}

func (s *AuthService) CustomerResetPassword(ctx context.Context, data dto.ResetPasswordRequest) (string, error) {
	customer, err := s.q.GetCustomerByValidResetToken(ctx, db.GetCustomerByValidResetTokenParams{
		ResetToken:          &data.Token,
		ResetTokenExpiresAt: &types.Timestamp{Time: time.Now()},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.BadRequest("Token de redefinição inválido ou expirado.")
	}
	if err != nil {
		return "", err
	}

	hashedPassword, err := utils.HashPassword(data.Password)
	if err != nil {
		return "", err
	}

	now := types.NewTimestamp(time.Now())
	if _, err := s.q.ResetCustomerPassword(ctx, db.ResetCustomerPasswordParams{
		ID:        customer.ID,
		Password:  &hashedPassword,
		UpdatedAt: now,
	}); err != nil {
		return "", err
	}

	return utils.SignCustomerToken(customer.ID, customer.Email, customer.Name, s.cfg.JWTSecretCustomer, customerTokenTTL)
}

// CustomerDevToken is retained internally for development but is not exposed
// through any route.
func (s *AuthService) CustomerDevToken(ctx context.Context, email string) (string, error) {
	customer, err := s.q.GetCustomerByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.Unauthorized("Cliente não encontrado")
	}
	if err != nil {
		return "", err
	}

	return utils.SignCustomerToken(customer.ID, customer.Email, customer.Name, s.cfg.JWTSecretCustomer, customerTokenTTL)
}

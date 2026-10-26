package services

import (
	"context"
	"errors"
	"time"

	"github.com/acienciadalua/api/internal/config"
	"github.com/acienciadalua/api/internal/db"
	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// customerTokenTTL matches the `expiresIn: '1h'` registered in the Nest
// JwtModule (it also applied to customer tokens, whose secret was overridden
// but whose sign options were merged).
const customerTokenTTL = time.Hour

// ResetTokenTTL is the 1 hour validity used by customerForgotPassword.
const ResetTokenTTL = time.Hour

// TokenPair is returned by signup/signin.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// AuthService ports the Nest AuthService and PasswordService.
type AuthService struct {
	cfg   *config.Config
	q     *db.Queries
	email *EmailService
}

func NewAuthService(cfg *config.Config, q *db.Queries, email *EmailService) *AuthService {
	return &AuthService{cfg: cfg, q: q, email: email}
}

// ─── Admin ───────────────────────────────────────────────────────────────────

func (s *AuthService) SignUp(ctx context.Context, data dto.SignUpRequest) (TokenPair, error) {
	_, err := s.q.GetUserByEmail(ctx, data.Email)
	switch {
	case err == nil:
		return TokenPair{}, utils.Unauthorized("User already exists")
	case !errors.Is(err, pgx.ErrNoRows):
		return TokenPair{}, err
	}

	hashedPassword, err := utils.HashPassword(data.Password)
	if err != nil {
		return TokenPair{}, err
	}

	role := "collaborator"
	if data.Role != nil {
		role = *data.Role
	}

	now := types.NewTimestamp(time.Now())
	user, err := s.q.CreateUser(ctx, db.CreateUserParams{
		ID:        uuid.NewString(),
		Name:      data.Name,
		Email:     data.Email,
		Password:  hashedPassword,
		Role:      role,
		CreatedAt: now,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return TokenPair{}, utils.Unauthorized("User already exists")
		}
		return TokenPair{}, err
	}

	return s.adminTokenPair(user)
}

func (s *AuthService) SignIn(ctx context.Context, data dto.SignInRequest) (TokenPair, error) {
	user, err := s.q.GetUserByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenPair{}, utils.Unauthorized("User email or password wrong.")
	}
	if err != nil {
		return TokenPair{}, err
	}

	if !utils.ComparePassword(data.Password, user.Password) {
		return TokenPair{}, utils.Unauthorized("User email or password wrong.")
	}

	return s.adminTokenPair(user)
}

func (s *AuthService) Refresh(refreshToken string) (string, error) {
	claims, err := utils.VerifyAdminToken(refreshToken, s.cfg.JWTRefreshSecret)
	if err != nil {
		return "", utils.Unauthorized("Invalid or expired refresh token.")
	}

	accessToken, err := utils.SignAdminToken(
		claims.ID,
		claims.Email,
		claims.Role,
		s.cfg.JWTSecret,
		s.cfg.JWTExpiresIn,
	)
	if err != nil {
		return "", err
	}

	return accessToken, nil
}

func (s *AuthService) adminTokenPair(user db.User) (TokenPair, error) {
	accessToken, err := utils.SignAdminToken(
		user.ID,
		user.Email,
		user.Role,
		s.cfg.JWTSecret,
		s.cfg.JWTExpiresIn,
	)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := utils.SignAdminToken(
		user.ID,
		user.Email,
		user.Role,
		s.cfg.JWTRefreshSecret,
		s.cfg.JWTRefreshExpiresIn,
	)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// ─── Customer ────────────────────────────────────────────────────────────────

func (s *AuthService) CustomerSignIn(ctx context.Context, data dto.CustomerSignInRequest) (string, error) {
	customer, err := s.q.GetCustomerByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.Unauthorized("Customer not found")
	}
	if err != nil {
		return "", err
	}

	if customer.Password == nil {
		return "", utils.Unauthorized("NO_PASSWORD_SET")
	}

	if !utils.ComparePassword(data.Password, *customer.Password) {
		return "", utils.Unauthorized("Email or password wrong.")
	}

	return utils.SignCustomerToken(customer.ID, customer.Email, customer.Name, s.cfg.JWTSecretCustomer, customerTokenTTL)
}

func (s *AuthService) CustomerSetPassword(ctx context.Context, data dto.CustomerSetPasswordRequest) (string, error) {
	customer, err := s.q.GetCustomerByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.NotFound("Customer not found")
	}
	if err != nil {
		return "", err
	}

	if customer.Password != nil {
		return "", utils.BadRequest("Password already set. Use forgot password to reset.")
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
		return "", utils.BadRequest("Invalid or expired reset token.")
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

// CustomerDevToken is kept for parity with the Nest service. It is not
// exposed through any route (the Nest controller did not expose it either).
func (s *AuthService) CustomerDevToken(ctx context.Context, email string) (string, error) {
	customer, err := s.q.GetCustomerByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", utils.Unauthorized("Customer not found")
	}
	if err != nil {
		return "", err
	}

	return utils.SignCustomerToken(customer.ID, customer.Email, customer.Name, s.cfg.JWTSecretCustomer, customerTokenTTL)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

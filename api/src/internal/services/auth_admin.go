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
			return TokenPair{}, utils.Unauthorized("Usuário já existe")
		}
		return TokenPair{}, err
	}

	return s.adminTokenPair(user)
}

func (s *AuthService) SignIn(ctx context.Context, data dto.SignInRequest) (TokenPair, error) {
	user, err := s.q.GetUserByEmail(ctx, data.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenPair{}, utils.Unauthorized("Email ou senha de usuário incorretos.")
	}
	if err != nil {
		return TokenPair{}, err
	}

	if !utils.ComparePassword(data.Password, user.Password) {
		return TokenPair{}, utils.Unauthorized("Email ou senha de usuário incorretos.")
	}

	return s.adminTokenPair(user)
}

func (s *AuthService) Refresh(refreshToken string) (string, error) {
	claims, err := utils.VerifyAdminToken(refreshToken, s.cfg.JWTRefreshSecret)
	if err != nil {
		return "", utils.Unauthorized("Token de atualização inválido ou expirado.")
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

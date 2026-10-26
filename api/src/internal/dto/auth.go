package dto

import "github.com/acienciadalua/api/internal/utils"

// ─── Admin auth ──────────────────────────────────────────────────────────────

type SignUpRequest struct {
	Name     string  `json:"name" validate:"required,min=3"`
	Email    string  `json:"email" validate:"required,email"`
	Role     *string `json:"role"`
	Password string  `json:"password" validate:"required,min=8"`
}

var signUpMessages = utils.Messages{
	"name.required":     "Name is required",
	"name.min":          "Name must be at least 3 characters",
	"email.required":    "Email is required",
	"email.email":       "Invalid email format",
	"password.required": "Password is required",
	"password.min":      "Password must be at least 8 characters",
}

func (d SignUpRequest) Validate() []string { return utils.Validate(d, signUpMessages) }

type SignInRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

var signInMessages = utils.Messages{
	"email.required":    "Email is required",
	"email.email":       "Invalid email format",
	"password.required": "Password is required",
	"password.min":      "Password must be at least 8 characters",
}

func (d SignInRequest) Validate() []string { return utils.Validate(d, signInMessages) }

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

var refreshMessages = utils.Messages{
	"refresh_token.required": "Refresh token is required",
}

func (d RefreshRequest) Validate() []string { return utils.Validate(d, refreshMessages) }

// ─── Customer auth ───────────────────────────────────────────────────────────

type CustomerSignInRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

var customerSignInMessages = utils.Messages{
	"email.required":    "Email is required",
	"email.email":       "Invalid email format",
	"password.required": "Password is required",
}

func (d CustomerSignInRequest) Validate() []string {
	return utils.Validate(d, customerSignInMessages)
}

type CustomerSetPasswordRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

var customerSetPasswordMessages = utils.Messages{
	"email.required":    "Email is required",
	"email.email":       "Invalid email format",
	"password.required": "Password is required",
	"password.min":      "Password must be at least 8 characters",
}

func (d CustomerSetPasswordRequest) Validate() []string {
	return utils.Validate(d, customerSetPasswordMessages)
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

var forgotPasswordMessages = utils.Messages{
	"email.required": "Email is required",
	"email.email":    "Invalid email format",
}

func (d ForgotPasswordRequest) Validate() []string {
	return utils.Validate(d, forgotPasswordMessages)
}

type ResetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
}

var resetPasswordMessages = utils.Messages{
	"token.required":    "Token is required",
	"password.required": "Password is required",
	"password.min":      "Password must be at least 8 characters",
}

func (d ResetPasswordRequest) Validate() []string {
	return utils.Validate(d, resetPasswordMessages)
}

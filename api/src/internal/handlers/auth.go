package handlers

import (
	"net/http"

	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/utils"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	utils.WriteJSON(w, status, payload)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	utils.WriteError(w, r, err)
}

func validationError(messages []string) error {
	return utils.NewValidationError(messages)
}

// ─── Admin ───────────────────────────────────────────────────────────────────

func (h *Handlers) SignUp(w http.ResponseWriter, r *http.Request) {
	var request dto.SignUpRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	tokens, err := h.auth.SignUp(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, struct {
		Message string `json:"message"`
		services.TokenPair
	}{
		Message:   "User created successfully",
		TokenPair: tokens,
	})
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	var request dto.SignInRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	tokens, err := h.auth.SignIn(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Message string `json:"message"`
		services.TokenPair
	}{
		Message:   "User signed in successfully",
		TokenPair: tokens,
	})
}

func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var request dto.RefreshRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	accessToken, err := h.auth.Refresh(request.RefreshToken)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Message     string `json:"message"`
		AccessToken string `json:"access_token"`
	}{
		Message:     "Token refreshed successfully",
		AccessToken: accessToken,
	})
}

// ─── Customer ────────────────────────────────────────────────────────────────

func (h *Handlers) CustomerSignIn(w http.ResponseWriter, r *http.Request) {
	var request dto.CustomerSignInRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	accessToken, err := h.auth.CustomerSignIn(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Message     string `json:"message"`
		AccessToken string `json:"access_token"`
	}{
		Message:     "Signed in successfully",
		AccessToken: accessToken,
	})
}

func (h *Handlers) CustomerSetPassword(w http.ResponseWriter, r *http.Request) {
	var request dto.CustomerSetPasswordRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	accessToken, err := h.auth.CustomerSetPassword(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Message     string `json:"message"`
		AccessToken string `json:"access_token"`
	}{
		Message:     "Password set successfully",
		AccessToken: accessToken,
	})
}

func (h *Handlers) CustomerForgotPassword(w http.ResponseWriter, r *http.Request) {
	var request dto.ForgotPasswordRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	if err := h.auth.CustomerForgotPassword(r.Context(), request); err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "If the email exists, a reset link has been sent.",
	})
}

func (h *Handlers) CustomerResetPassword(w http.ResponseWriter, r *http.Request) {
	var request dto.ResetPasswordRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	accessToken, err := h.auth.CustomerResetPassword(r.Context(), request)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Message     string `json:"message"`
		AccessToken string `json:"access_token"`
	}{
		Message:     "Password reset successfully",
		AccessToken: accessToken,
	})
}

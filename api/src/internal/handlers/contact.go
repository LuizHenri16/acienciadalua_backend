package handlers

import (
	"net/http"

	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/utils"
)

func (h *Handlers) SendContact(w http.ResponseWriter, r *http.Request) {
	var request dto.ContactRequest
	if err := utils.BindJSON(w, r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	subject := ""
	if request.Subject != nil {
		subject = *request.Subject
	}

	if err := h.email.SendContactEmail(r.Context(), request.Name, request.Email, subject, request.Message); err != nil {
		writeError(w, r, err)
		return
	}

	logInfo("contact message sent", "email", request.Email)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Mensagem enviada com sucesso!"})
}

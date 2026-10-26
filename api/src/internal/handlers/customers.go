package handlers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/acienciadalua/api/internal/middleware"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/go-chi/chi/v5"
)

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CustomerFrom(r.Context())
	if !ok {
		writeError(w, r, utils.Unauthorized("Missing or invalid authorization header."))
		return
	}

	customer, err := h.customers.GetMe(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

func (h *Handlers) GetMyPurchases(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CustomerFrom(r.Context())
	if !ok {
		writeError(w, r, utils.Unauthorized("Missing or invalid authorization header."))
		return
	}

	purchases, err := h.customers.GetMyPurchases(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, purchases)
}

func (h *Handlers) DownloadProductFile(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.CustomerFrom(r.Context())
	if !ok {
		writeError(w, r, utils.Unauthorized("Missing or invalid authorization header."))
		return
	}

	download, err := h.customers.GetDownload(r.Context(), claims.Subject, chi.URLParam(r, "productId"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	file, err := os.Open(download.Path)
	if err != nil {
		writeError(w, r, utils.NotFound("Arquivo não encontrado no servidor."))
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeError(w, r, utils.Internal("Internal server error"))
		return
	}

	w.Header().Set("Content-Type", utils.DownloadContentType(download.FileURL))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, utils.SafeDownloadName(download.FileURL)))
	w.Header().Set("Cache-Control", "no-store")

	http.ServeContent(w, r, download.FileURL, info.ModTime(), file)
}

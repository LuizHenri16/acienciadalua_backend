package handlers

import (
	"net/http"

	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/go-chi/chi/v5"
)

func (h *Handlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	var category *string
	if value := query.Get("category"); value != "" {
		if value != dto.CategoryStudent && value != dto.CategoryTeacher {
			writeError(w, r, validationError([]string{
				"category must be one of the following values: STUDENT, TEACHER",
			}))
			return
		}
		category = &value
	}

	var isActive *bool
	if query.Has("isActive") {
		active := query.Get("isActive") == "true"
		isActive = &active
	}

	products, err := h.products.FindAll(r.Context(), category, isActive)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, products)
}

func (h *Handlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	product, err := h.products.FindOne(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (h *Handlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateProductRequest
	coverName, fileName := "", ""

	if isMultipart(r) {
		form, err := parseMultipartForm(r)
		if err != nil {
			writeError(w, r, err)
			return
		}

		cover, file, err := h.saveProductFiles(form)
		if err != nil {
			writeError(w, r, err)
			return
		}
		if cover != nil {
			coverName = *cover
		}
		if file != nil {
			fileName = *file
		}

		if err := rejectUnknownFormFields(form, productFormFields); err != nil {
			writeError(w, r, err)
			return
		}

		parsed, err := createRequestFromForm(form)
		if err != nil {
			writeError(w, r, err)
			return
		}
		request = parsed
	} else {
		if err := utils.BindJSON(w, r, &request); err != nil {
			writeError(w, r, err)
			return
		}
	}

	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	product, err := h.products.Create(r.Context(), createInput(request, coverName, fileName))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, product)
}

func (h *Handlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var request dto.UpdateProductRequest
	var coverName, fileName *string

	if isMultipart(r) {
		form, err := parseMultipartForm(r)
		if err != nil {
			writeError(w, r, err)
			return
		}

		coverName, fileName, err = h.saveProductFiles(form)
		if err != nil {
			writeError(w, r, err)
			return
		}

		if err := rejectUnknownFormFields(form, productFormFields); err != nil {
			writeError(w, r, err)
			return
		}

		parsed, err := updateRequestFromForm(form)
		if err != nil {
			writeError(w, r, err)
			return
		}
		request = parsed
	} else {
		if err := utils.BindJSON(w, r, &request); err != nil {
			writeError(w, r, err)
			return
		}
	}

	if messages := request.Validate(); len(messages) > 0 {
		writeError(w, r, validationError(messages))
		return
	}

	product, err := h.products.Update(r.Context(), id, updateInput(request, coverName, fileName))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (h *Handlers) ToggleProduct(w http.ResponseWriter, r *http.Request) {
	product, err := h.products.Toggle(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (h *Handlers) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	product, err := h.products.Remove(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

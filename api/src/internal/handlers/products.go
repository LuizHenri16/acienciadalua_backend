package handlers

import (
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/acienciadalua/api/internal/dto"
	"github.com/acienciadalua/api/internal/services"
	"github.com/acienciadalua/api/internal/types"
	"github.com/acienciadalua/api/internal/utils"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
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

// ─── multipart helpers ───────────────────────────────────────────────────────

var productFormFields = map[string]bool{
	"title":       true,
	"description": true,
	"price":       true,
	"isActive":    true,
	"category":    true,
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
}

func parseMultipartForm(r *http.Request) (*multipart.Form, error) {
	// Keep up to 32 MiB in memory and spool the rest to temporary files
	// (multer uses disk storage for everything; this is only an allocation
	// detail and does not change the response contract).
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, utils.BadRequest("Malformed multipart body")
	}
	return r.MultipartForm, nil
}

func rejectUnknownFormFields(form *multipart.Form, allowed map[string]bool) error {
	for field := range form.Value {
		if !allowed[field] {
			return validationError([]string{"property " + field + " should not exist"})
		}
	}
	return nil
}

func (h *Handlers) saveProductFiles(form *multipart.Form) (*string, *string, error) {
	for field := range form.File {
		if field != "cover" && field != "file" {
			return nil, nil, utils.BadRequest("Unexpected field")
		}
	}

	var coverName, fileName *string

	if headers, ok := form.File["cover"]; ok {
		if len(headers) > 1 {
			return nil, nil, utils.BadRequest("Unexpected field")
		}
		name, err := h.saveUpload(h.cfg.UploadDir, "cover", headers[0])
		if err != nil {
			return nil, nil, err
		}
		coverName = &name
	}

	if headers, ok := form.File["file"]; ok {
		if len(headers) > 1 {
			return nil, nil, utils.BadRequest("Unexpected field")
		}
		name, err := h.saveUpload(h.cfg.FilesDir, "file", headers[0])
		if err != nil {
			return nil, nil, err
		}
		fileName = &name
	}

	return coverName, fileName, nil
}

func (h *Handlers) saveUpload(dir, field string, header *multipart.FileHeader) (string, error) {
	if err := utils.ValidateUploadExtension(field, header.Filename); err != nil {
		return "", err
	}

	file, err := header.Open()
	if err != nil {
		return "", utils.Internal("Failed to read uploaded file")
	}
	defer file.Close()

	name, err := utils.SaveUploadedFile(dir, file, header.Filename)
	if err != nil {
		return "", utils.Internal("Failed to save uploaded file")
	}
	return name, nil
}

func createRequestFromForm(form *multipart.Form) (dto.CreateProductRequest, error) {
	request := dto.CreateProductRequest{
		Title:       firstValue(form, "title"),
		Description: firstValue(form, "description"),
		Category:    firstValue(form, "category"),
	}

	if raw, ok := form.Value["price"]; ok {
		price, err := parseFloat(firstValueFrom(raw, "price"))
		if err != nil {
			return request, validationError([]string{"price must be a number conforming to the specified constraints"})
		}
		request.Price = &price
	}

	if raw, ok := form.Value["isActive"]; ok {
		value := firstValueFrom(raw, "isActive") == "true"
		request.IsActive = &value
	}

	return request, nil
}

func updateRequestFromForm(form *multipart.Form) (dto.UpdateProductRequest, error) {
	request := dto.UpdateProductRequest{}

	if values, ok := form.Value["title"]; ok {
		value := firstValueFrom(values, "title")
		request.Title = &value
	}
	if values, ok := form.Value["description"]; ok {
		value := firstValueFrom(values, "description")
		request.Description = &value
	}
	if values, ok := form.Value["category"]; ok {
		value := firstValueFrom(values, "category")
		request.Category = &value
	}
	if values, ok := form.Value["price"]; ok {
		price, err := parseFloat(firstValueFrom(values, "price"))
		if err != nil {
			return request, validationError([]string{"price must be a number conforming to the specified constraints"})
		}
		request.Price = &price
	}
	if values, ok := form.Value["isActive"]; ok {
		value := firstValueFrom(values, "isActive") == "true"
		request.IsActive = &value
	}

	return request, nil
}

func parseFloat(raw string) (float64, error) {
	// class-transformer's Number("") is 0.
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func firstValue(form *multipart.Form, field string) string {
	values, ok := form.Value[field]
	if !ok || len(values) == 0 {
		return ""
	}
	return values[0]
}

func firstValueFrom(values []string, _ string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// ─── DTO → service input ─────────────────────────────────────────────────────

func createInput(request dto.CreateProductRequest, coverURL, fileURL string) services.CreateProductInput {
	input := services.CreateProductInput{
		Title:       request.Title,
		Description: request.Description,
		IsActive:    request.IsActive != nil && *request.IsActive,
		Category:    request.Category,
		CoverURL:    coverURL,
		FileURL:     fileURL,
	}

	if request.Price != nil {
		input.Price = types.Decimal{Decimal: decimal.NewFromFloat(*request.Price)}
	}

	return input
}

func updateInput(request dto.UpdateProductRequest, coverURL, fileURL *string) services.UpdateProductInput {
	input := services.UpdateProductInput{
		Title:       request.Title,
		Description: request.Description,
		IsActive:    request.IsActive,
		Category:    request.Category,
		CoverURL:    coverURL,
		FileURL:     fileURL,
	}

	if request.Price != nil {
		price := types.Decimal{Decimal: decimal.NewFromFloat(*request.Price)}
		input.Price = &price
	}

	return input
}

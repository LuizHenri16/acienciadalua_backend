package utils

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
)

// MaxJSONBody matches Express' default JSON body limit (100kb) with a little
// headroom for large product descriptions.
const MaxJSONBody = 1 << 20 // 1 MiB

// WriteJSON writes a JSON response. A nil payload produces an empty body,
// which is what Nest does when a controller returns undefined.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	if payload == nil {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// WriteError writes an error using the NestJS exception filter shape:
//
//	{"message": "...", "error": "Unauthorized", "statusCode": 401}
//
// Validation errors have `message` as an array.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := AsError(err)
	if !ok {
		slog.Error("unhandled request error",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)
		appErr = Internal("Internal server error")
	}

	if appErr.StatusCode >= http.StatusInternalServerError {
		slog.Error("request failed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", appErr.StatusCode,
			"error", appErr.Error(),
		)
	}

	body := struct {
		Message    any    `json:"message"`
		Error      string `json:"error,omitempty"`
		StatusCode int    `json:"statusCode"`
	}{
		Message:    appErr.Message,
		Error:      appErr.Label,
		StatusCode: appErr.StatusCode,
	}

	WriteJSON(w, appErr.StatusCode, body)
}

// BindJSON decodes a JSON request body into dst, rejecting unknown fields like
// the Nest ValidationPipe (whitelist + forbidNonWhitelisted).
func BindJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBody)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			// Express delivers an empty object for requests without a body, so
			// DTO validation is responsible for the error message.
			return nil
		}
		return bindError(err)
	}

	// Reject trailing JSON documents.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BadRequest("Unexpected data after JSON body")
	}

	return nil
}

func bindError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return StatusError(http.StatusRequestEntityTooLarge, "request entity too large", http.StatusText(http.StatusRequestEntityTooLarge))
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		return NewValidationError([]string{typeMessage(field, typeErr.Type)})
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return BadRequest("Malformed JSON in request body")
	}

	const unknownPrefix = "json: unknown field "
	if strings.HasPrefix(err.Error(), unknownPrefix) {
		field := strings.Trim(err.Error()[len(unknownPrefix):], `"`)
		return NewValidationError([]string{"property " + field + " should not exist"})
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		return BadRequest("Malformed JSON in request body")
	}

	return BadRequest("Malformed JSON in request body")
}

func typeMessage(field string, typ reflect.Type) string {
	if typ == nil {
		return field + " must be a valid value"
	}

	switch typ.Kind() {
	case reflect.String:
		return field + " must be a string"
	case reflect.Bool:
		return field + " must be a boolean value"
	case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field + " must be a number conforming to the specified constraints"
	case reflect.Slice, reflect.Array:
		return field + " must be an array"
	case reflect.Struct:
		return field + " must be an object"
	default:
		return field + " must be a " + typ.String()
	}
}

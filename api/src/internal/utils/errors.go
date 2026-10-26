// Package utils contains shared helpers: HTTP plumbing, error types,
// validation, JWT and password primitives.
package utils

import (
	"errors"
	"net/http"
)

// Error is an application error that maps directly to an HTTP response with
// the same shape produced by NestJS exception filters.
type Error struct {
	StatusCode int
	// Message may be a string or a []string (validation errors).
	Message any
	// Label is the "error" field. It is empty for exceptions raised without an
	// object body (e.g. the ThrottlerException), matching Nest's behaviour.
	Label string
}

func (e *Error) Error() string {
	switch m := e.Message.(type) {
	case string:
		return m
	default:
		return http.StatusText(e.StatusCode)
	}
}

// NewError builds an application error using the standard HTTP status text as
// the label ("Unauthorized", "Not Found", ...).
func NewError(status int, message string) *Error {
	return &Error{StatusCode: status, Message: message, Label: http.StatusText(status)}
}

// NewValidationError builds a 400 error whose message is a list, like
// class-validator errors returned by the Nest ValidationPipe.
func NewValidationError(messages []string) *Error {
	return &Error{
		StatusCode: http.StatusBadRequest,
		Message:    messages,
		Label:      http.StatusText(http.StatusBadRequest),
	}
}

// StatusError builds an error with an explicit label (empty label omits the
// "error" property from the JSON body).
func StatusError(status int, message, label string) *Error {
	return &Error{StatusCode: status, Message: message, Label: label}
}

func BadRequest(message string) *Error {
	return NewError(http.StatusBadRequest, message)
}

func Unauthorized(message string) *Error {
	return NewError(http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	return NewError(http.StatusForbidden, message)
}

func NotFound(message string) *Error {
	return NewError(http.StatusNotFound, message)
}

func Internal(message string) *Error {
	return NewError(http.StatusInternalServerError, message)
}

// TooManyRequests mirrors @nestjs/throttler's exception, which has no
// "error" property.
func TooManyRequests() *Error {
	return StatusError(http.StatusTooManyRequests, "ThrottlerException: Too Many Requests", "")
}

// AsError unwraps an application error, if any.
func AsError(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

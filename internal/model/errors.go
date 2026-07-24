package model

import "net/http"

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *AppError) Error() string { return e.Message }

var (
	ErrNotFound    = &AppError{Code: "NOT_FOUND", Message: "resource not found", HTTPStatus: http.StatusNotFound}
	ErrUnauthorized = &AppError{Code: "UNAUTHORIZED", Message: "unauthorized", HTTPStatus: http.StatusUnauthorized}
	ErrForbidden   = &AppError{Code: "FORBIDDEN", Message: "forbidden", HTTPStatus: http.StatusForbidden}
	ErrConflict    = &AppError{Code: "CONFLICT", Message: "resource conflict", HTTPStatus: http.StatusConflict}
	ErrValidation  = &AppError{Code: "VALIDATION_ERROR", Message: "validation failed", HTTPStatus: http.StatusUnprocessableEntity}
	ErrInternal    = &AppError{Code: "INTERNAL_ERROR", Message: "internal server error", HTTPStatus: http.StatusInternalServerError}
	ErrRateLimited = &AppError{Code: "RATE_LIMITED", Message: "rate limit exceeded", HTTPStatus: http.StatusTooManyRequests}
)

func NewNotFoundError(resource string) *AppError {
	return &AppError{Code: "NOT_FOUND", Message: resource + " not found", HTTPStatus: http.StatusNotFound}
}

func NewValidationError(message string) *AppError {
	return &AppError{Code: "VALIDATION_ERROR", Message: message, HTTPStatus: http.StatusUnprocessableEntity}
}

func NewConflictError(message string) *AppError {
	return &AppError{Code: "CONFLICT", Message: message, HTTPStatus: http.StatusConflict}
}

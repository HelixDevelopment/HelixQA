package errors

import (
	"errors"
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code string, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

func Wrap(err error, code string, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Details:    err.Error(),
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}

func Wrapf(err error, code string, httpStatus int, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:       code,
		Message:    fmt.Sprintf(format, args...),
		Details:    err.Error(),
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

func StatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

func ErrorCode(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return "INTERNAL_ERROR"
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}

func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

var (
	ErrBadRequest          = New("BAD_REQUEST", "invalid request", http.StatusBadRequest)
	ErrUnauthorized        = New("UNAUTHORIZED", "unauthorized", http.StatusUnauthorized)
	ErrForbidden           = New("FORBIDDEN", "forbidden", http.StatusForbidden)
	ErrNotFound            = New("NOT_FOUND", "resource not found", http.StatusNotFound)
	ErrConflict            = New("CONFLICT", "resource conflict", http.StatusConflict)
	ErrUnprocessableEntity = New("UNPROCESSABLE_ENTITY", "unprocessable entity", http.StatusUnprocessableEntity)
	ErrTooManyRequests     = New("TOO_MANY_REQUESTS", "rate limit exceeded", http.StatusTooManyRequests)
	ErrInternalServerError = New("INTERNAL_ERROR", "internal server error", http.StatusInternalServerError)
	ErrServiceUnavailable  = New("SERVICE_UNAVAILABLE", "service unavailable", http.StatusServiceUnavailable)
	ErrGatewayTimeout      = New("GATEWAY_TIMEOUT", "gateway timeout", http.StatusGatewayTimeout)

	ErrValidationFailed = New("VALIDATION_FAILED", "validation failed", http.StatusUnprocessableEntity)
	ErrInvalidCurrency  = New("INVALID_CURRENCY", "invalid currency code", http.StatusBadRequest)
	ErrInvalidAmount    = New("INVALID_AMOUNT", "invalid amount", http.StatusBadRequest)
	ErrInvalidEmail     = New("INVALID_EMAIL", "invalid email address", http.StatusBadRequest)

	ErrJWTExpired  = New("JWT_EXPIRED", "token has expired", http.StatusUnauthorized)
	ErrJWTInvalid  = New("JWT_INVALID", "invalid token", http.StatusUnauthorized)
	ErrAPIKeyInvalid = New("API_KEY_INVALID", "invalid API key", http.StatusUnauthorized)

	ErrIdempotencyConflict = New("IDEMPOTENCY_CONFLICT", "idempotency key conflict", http.StatusConflict)
	ErrPaymentDeclined     = New("PAYMENT_DECLINED", "payment declined", http.StatusPaymentRequired)
	ErrInsufficientFunds   = New("INSUFFICIENT_FUNDS", "insufficient funds", http.StatusPaymentRequired)
)

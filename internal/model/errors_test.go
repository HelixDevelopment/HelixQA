package model

import (
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := &AppError{Code: "TEST", Message: "test error", HTTPStatus: 400}
	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error")
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantCode   string
		wantStatus int
	}{
		{"NotFound", ErrNotFound, "NOT_FOUND", http.StatusNotFound},
		{"Unauthorized", ErrUnauthorized, "UNAUTHORIZED", http.StatusUnauthorized},
		{"Forbidden", ErrForbidden, "FORBIDDEN", http.StatusForbidden},
		{"Conflict", ErrConflict, "CONFLICT", http.StatusConflict},
		{"Validation", ErrValidation, "VALIDATION_ERROR", http.StatusUnprocessableEntity},
		{"Internal", ErrInternal, "INTERNAL_ERROR", http.StatusInternalServerError},
		{"RateLimited", ErrRateLimited, "RATE_LIMITED", http.StatusTooManyRequests},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			if tt.err.HTTPStatus != tt.wantStatus {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.wantStatus)
			}
		})
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("user")
	if err.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want %q", err.Code, "NOT_FOUND")
	}
	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusNotFound)
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("email is required")
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("Code = %q, want %q", err.Code, "VALIDATION_ERROR")
	}
	if err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnprocessableEntity)
	}
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("email already exists")
	if err.Code != "CONFLICT" {
		t.Errorf("Code = %q, want %q", err.Code, "CONFLICT")
	}
	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusConflict)
	}
}

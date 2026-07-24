package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := New("TEST", "test message", http.StatusBadRequest)
	if err.Error() != "test message" {
		t.Errorf("expected 'test message', got %q", err.Error())
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := Wrap(inner, "CODE", "wrapped")
	if err.Unwrap() != inner {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestNew(t *testing.T) {
	err := New("CODE", "msg", 400)
	if err.Code != "CODE" {
		t.Errorf("expected CODE, got %s", err.Code)
	}
	if err.HTTPStatus != 400 {
		t.Errorf("expected 400, got %d", err.HTTPStatus)
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("inner")
	err := Wrap(inner, "CODE", "wrapped")
	if err.Details != "inner" {
		t.Errorf("expected details 'inner', got %q", err.Details)
	}
	if err.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", err.HTTPStatus)
	}
}

func TestWrapf(t *testing.T) {
	inner := errors.New("inner")
	err := Wrapf(inner, "CODE", 400, "user %s failed", "bob")
	if err.Message != "user bob failed" {
		t.Errorf("expected 'user bob failed', got %q", err.Message)
	}
}

func TestStatusCode_AppError(t *testing.T) {
	err := New("CODE", "msg", 422)
	if StatusCode(err) != 422 {
		t.Errorf("expected 422, got %d", StatusCode(err))
	}
}

func TestStatusCode_RegularError(t *testing.T) {
	err := errors.New("regular")
	if StatusCode(err) != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", StatusCode(err))
	}
}

func TestErrorCode_AppError(t *testing.T) {
	err := New("MY_CODE", "msg", 400)
	if ErrorCode(err) != "MY_CODE" {
		t.Errorf("expected MY_CODE, got %s", ErrorCode(err))
	}
}

func TestErrorCode_RegularError(t *testing.T) {
	err := errors.New("regular")
	if ErrorCode(err) != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR, got %s", ErrorCode(err))
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    *AppError
		code   string
		status int
	}{
		{"BadRequest", ErrBadRequest, "BAD_REQUEST", 400},
		{"Unauthorized", ErrUnauthorized, "UNAUTHORIZED", 401},
		{"Forbidden", ErrForbidden, "FORBIDDEN", 403},
		{"NotFound", ErrNotFound, "NOT_FOUND", 404},
		{"Conflict", ErrConflict, "CONFLICT", 409},
		{"ValidationFailed", ErrValidationFailed, "VALIDATION_FAILED", 422},
		{"PaymentDeclined", ErrPaymentDeclined, "PAYMENT_DECLINED", 402},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, tt.err.Code)
			}
			if tt.err.HTTPStatus != tt.status {
				t.Errorf("expected status %d, got %d", tt.status, tt.err.HTTPStatus)
			}
		})
	}
}

func TestIs(t *testing.T) {
	err := Wrap(ErrNotFound, "CODE", "msg")
	if !Is(err, ErrNotFound) {
		t.Error("expected errors.Is to match")
	}
}

func TestAs(t *testing.T) {
	err := New("CODE", "msg", 400)
	var target *AppError
	if !As(err, &target) {
		t.Error("expected errors.As to match")
	}
	if target.Code != "CODE" {
		t.Errorf("expected CODE, got %s", target.Code)
	}
}

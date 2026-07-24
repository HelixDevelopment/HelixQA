package validator

import (
	"regexp"
	"testing"
)

func TestIsValidCurrency(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"USD", true},
		{"EUR", true},
		{"gbp", false},
		{"US", false},
		{"USDX", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidCurrency(tt.input); got != tt.valid {
			t.Errorf("IsValidCurrency(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"user@example.com", true},
		{"test@test.co", true},
		{"not-an-email", false},
		{"@missing.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsValidEmail(tt.input); got != tt.valid {
			t.Errorf("IsValidEmail(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidAmount(t *testing.T) {
	if !IsValidAmount(100) {
		t.Error("expected 100 to be valid")
	}
	if IsValidAmount(0) {
		t.Error("expected 0 to be invalid")
	}
	if IsValidAmount(-1) {
		t.Error("expected -1 to be invalid")
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  hello  ", "hello"},
		{"hello", "hello"},
		{"  ", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SanitizeString(tt.input); got != tt.want {
			t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	if !IsEmpty("") {
		t.Error("expected empty string to be empty")
	}
	if !IsEmpty("  ") {
		t.Error("expected whitespace to be empty")
	}
	if IsEmpty("hello") {
		t.Error("expected 'hello' to not be empty")
	}
}

func TestMinLength(t *testing.T) {
	if !MinLength("hello", 3) {
		t.Error("expected 'hello' >= 3")
	}
	if MinLength("hi", 3) {
		t.Error("expected 'hi' < 3")
	}
	if !MinLength("", 0) {
		t.Error("expected '' >= 0")
	}
}

func TestMaxLength(t *testing.T) {
	if !MaxLength("hi", 5) {
		t.Error("expected 'hi' <= 5")
	}
	if MaxLength("hello", 3) {
		t.Error("expected 'hello' > 3")
	}
}

func TestInRange(t *testing.T) {
	if !InRange(5, 1, 10) {
		t.Error("expected 5 in [1,10]")
	}
	if InRange(0, 1, 10) {
		t.Error("expected 0 not in [1,10]")
	}
	if InRange(11, 1, 10) {
		t.Error("expected 11 not in [1,10]")
	}
	if !InRange(1, 1, 10) {
		t.Error("expected 1 in [1,10]")
	}
	if !InRange(10, 1, 10) {
		t.Error("expected 10 in [1,10]")
	}
}

func TestContainsOnly(t *testing.T) {
	digits := regexp.MustCompile(`^[0-9]+$`)
	alpha := regexp.MustCompile(`^[a-zA-Z]+$`)
	alphanum := regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	tests := []struct {
		s       string
		charset *regexp.Regexp
		want    bool
	}{
		{"12345", digits, true},
		{"123a45", digits, false},
		{"", digits, false},
		{"hello", alpha, true},
		{"hello123", alpha, false},
		{"abc123", alphanum, true},
		{"abc 123", alphanum, false},
		{"ABC", alpha, true},
	}

	for _, tt := range tests {
		if got := ContainsOnly(tt.s, tt.charset); got != tt.want {
			t.Errorf("ContainsOnly(%q, %v) = %v, want %v", tt.s, tt.charset, got, tt.want)
		}
	}
}

package utils

import (
	"testing"
)

func TestIsValidCurrency(t *testing.T) {
	valid := []string{"USD", "EUR", "GBP", "JPY", "BTC", "ETH"}
	for _, c := range valid {
		if !IsValidCurrency(c) {
			t.Errorf("IsValidCurrency(%q) = false, want true", c)
		}
	}

	invalid := []string{"usd", "US", "X", ""}
	for _, c := range invalid {
		if IsValidCurrency(c) {
			t.Errorf("IsValidCurrency(%q) = true, want false", c)
		}
	}
}

func TestIsMajorCurrency(t *testing.T) {
	major := []string{"USD", "EUR", "GBP", "JPY", "CHF", "CAD", "AUD", "CNY", "INR", "BRL"}
	for _, c := range major {
		if !IsMajorCurrency(c) {
			t.Errorf("IsMajorCurrency(%q) = false, want true", c)
		}
	}

	minor := []string{"BTC", "ETH", "DOGE", "XYZ"}
	for _, c := range minor {
		if IsMajorCurrency(c) {
			t.Errorf("IsMajorCurrency(%q) = true, want false", c)
		}
	}
}

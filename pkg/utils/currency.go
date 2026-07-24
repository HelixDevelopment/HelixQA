package utils

import "regexp"

var validCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

var majorCurrencies = map[string]bool{
	"USD": true,
	"EUR": true,
	"GBP": true,
	"JPY": true,
	"CHF": true,
	"CAD": true,
	"AUD": true,
	"CNY": true,
	"INR": true,
	"BRL": true,
}

func IsValidCurrency(code string) bool {
	return validCurrency.MatchString(code)
}

func IsMajorCurrency(code string) bool {
	return majorCurrencies[code]
}

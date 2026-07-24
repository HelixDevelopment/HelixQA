package validator

import (
	"net/mail"
	"regexp"
	"strings"
)

var currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

func IsValidCurrency(code string) bool {
	return currencyRegex.MatchString(code)
}

func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func IsValidAmount(amount int64) bool {
	return amount > 0
}

func SanitizeString(s string) string {
	return strings.TrimSpace(s)
}

func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

func MinLength(s string, min int) bool {
	return len([]rune(s)) >= min
}

func MaxLength(s string, max int) bool {
	return len([]rune(s)) <= max
}

func InRange(val, min, max int64) bool {
	return val >= min && val <= max
}

func ContainsOnly(s string, charset *regexp.Regexp) bool {
	return charset.MatchString(s)
}

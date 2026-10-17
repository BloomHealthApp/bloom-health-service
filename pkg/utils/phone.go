package utils

import (
	"errors"
	"regexp"
	"strings"
)

// DefaultCountryCode is used when no country code is configured.
const DefaultCountryCode = "+91"

// ErrInvalidPhone is returned by NormalizePhone for input that is not a usable phone number.
var ErrInvalidPhone = errors.New("invalid phone number")

var countryCodeRe = regexp.MustCompile(`^\+[1-9][0-9]{0,2}$`)

// ValidCountryCode reports whether cc looks like "+<1-3 digits>".
func ValidCountryCode(cc string) bool { return countryCodeRe.MatchString(cc) }

// NormalizePhone converts user input to E.164. Spaces, dashes, dots and parentheses are ignored.
// Accepted forms: "+<country><number>", or a bare 10-digit number which gets defaultCC (empty means
// +91). The result must have 8-15 digits and must not start with 0 after the "+".
func NormalizePhone(raw, defaultCC string) (string, error) {
	if defaultCC == "" {
		defaultCC = DefaultCountryCode
	}
	s := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '-', '.', '(', ')':
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
	plus := strings.HasPrefix(s, "+")
	digits := strings.TrimPrefix(s, "+")
	if digits == "" {
		return "", ErrInvalidPhone
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", ErrInvalidPhone
		}
	}
	var e164 string
	switch {
	case plus:
		e164 = "+" + digits
	case len(digits) == 10:
		if !ValidCountryCode(defaultCC) {
			return "", ErrInvalidPhone
		}
		e164 = defaultCC + digits
	default:
		return "", ErrInvalidPhone
	}
	n := len(e164) - 1
	if n < 8 || n > 15 || e164[1] == '0' {
		return "", ErrInvalidPhone
	}
	return e164, nil
}

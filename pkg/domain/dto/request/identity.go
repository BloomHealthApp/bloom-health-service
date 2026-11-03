package request

import (
	"net/mail"
	"strings"
)

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func validEmail(e string) bool {
	_, err := mail.ParseAddress(e)
	return err == nil && len(e) <= 254 && !strings.ContainsAny(e, " <>")
}

const phoneMsg = "phone must be a valid number (8-15 digits, optional +country code)"

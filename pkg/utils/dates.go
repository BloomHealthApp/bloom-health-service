package utils

import (
	"errors"
	"regexp"
	"time"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// DateLayout is the localDate format.
const DateLayout = "2006-01-02"

// ErrInvalidDate is returned for malformed dates.
var ErrInvalidDate = errors.New("date must be YYYY-MM-DD")

// ParseDate validates and parses YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	if !dateRe.MatchString(s) {
		return time.Time{}, ErrInvalidDate
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, errors.New("invalid date")
	}
	return t, nil
}

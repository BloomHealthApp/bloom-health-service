package utils

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// ErrInvalidCursor is returned when a pull cursor cannot be decoded.
var ErrInvalidCursor = errors.New("invalid cursor")

// EncodeCursor turns a server sequence into an opaque cursor.
func EncodeCursor(seq int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("s:" + strconv.FormatInt(seq, 10)))
}

// DecodeCursor parses a cursor; the empty cursor means "from the beginning".
func DecodeCursor(c string) (int64, error) {
	if c == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || !strings.HasPrefix(string(b), "s:") {
		return 0, ErrInvalidCursor
	}
	n, err := strconv.ParseInt(string(b)[2:], 10, 64)
	if err != nil || n < 0 {
		return 0, ErrInvalidCursor
	}
	return n, nil
}

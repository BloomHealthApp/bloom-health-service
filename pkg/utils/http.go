package utils

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/gin-gonic/gin"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// RespondError writes the error envelope (and any extra headers) and aborts the handler chain.
func RespondError(c *gin.Context, e apperrors.IHttpError) {
	for k, v := range e.GetHeaders() {
		c.Header(k, v)
	}
	code, body := e.Get()
	c.AbortWithStatusJSON(code, body)
}

// BindStrictJSON decodes the request body into v, rejecting unknown fields and trailing data.
// Body size is limited by the BodyLimit middleware.
func BindStrictJSON(c *gin.Context, v any) apperrors.IHttpError {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperrors.BadRequestError("invalid JSON body: " + shortErr(err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return apperrors.BadRequestError("unexpected data after JSON body")
	}
	return nil
}

func shortErr(err error) string {
	m := err.Error()
	if len(m) > 120 {
		m = m[:120]
	}
	return m
}

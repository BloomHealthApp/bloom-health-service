// Package request holds the request DTOs. Each exposes Validate() built on ozzo-validation; the
// returned IHttpError is already a 400 validation_failed with the contract's message.
package request

import (
	"errors"
	"sort"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
)

// toHTTPError turns an ozzo validation result into a 400 carrying the first message (ordered by field).
func toHTTPError(err error) apperrors.IHttpError {
	if err == nil {
		return nil
	}
	var verrs validation.Errors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		keys := make([]string, 0, len(verrs))
		for k := range verrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return apperrors.BadRequestError(verrs[keys[0]].Error())
	}
	return apperrors.BadRequestError(err.Error())
}

// rule builds an ozzo rule that fails with msg when ok returns false.
func rule(msg string, ok func(v any) bool) validation.Rule {
	return validation.By(func(v any) error {
		if ok(v) {
			return nil
		}
		return errors.New(msg)
	})
}

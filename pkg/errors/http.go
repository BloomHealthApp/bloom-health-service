// Package errors defines the HTTP-facing error type returned by services and the typed repository errors.
package errors

import "net/http"

// Machine-readable error codes from docs/api-contract.md.
const (
	CodeValidationFailed  = "validation_failed"
	CodeUnauthorized      = "unauthorized"
	CodeTokenExpired      = "token_expired"
	CodeForbidden         = "forbidden"
	CodeNotFound          = "not_found"
	CodeConflict          = "conflict"
	CodeAlreadyRegistered = "already_registered"
	CodeRateLimited       = "rate_limited"
	CodeInternal          = "internal"
)

// IHttpError is an error that knows how to render itself as an HTTP response.
type IHttpError interface {
	error
	// Get returns the status code and the JSON body.
	Get() (code int, obj any)
	GetErrorBody() (obj any)
	// GetHeaders returns extra response headers (for example Retry-After).
	GetHeaders() map[string]string
	GetCode() string
}

// ErrorBody is the public error envelope: {"error":{"code","message","details"}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner object of the error envelope.
type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// HttpError is the default IHttpError implementation.
type HttpError struct {
	StatusCode int
	Code       string
	Message    string
	Details    map[string]any
	Headers    map[string]string
}

func (e *HttpError) Get() (int, any)               { return e.StatusCode, e.GetErrorBody() }
func (e *HttpError) Error() string                 { return e.Message }
func (e *HttpError) GetCode() string               { return e.Code }
func (e *HttpError) GetHeaders() map[string]string { return e.Headers }
func (e *HttpError) GetErrorBody() any {
	return ErrorBody{Error: ErrorDetail{Code: e.Code, Message: e.Message, Details: e.Details}}
}

func newErr(status int, code, msg string) *HttpError {
	return &HttpError{StatusCode: status, Code: code, Message: msg}
}

// BadRequestError is a 400 validation_failed error.
func BadRequestError(message string) IHttpError {
	return newErr(http.StatusBadRequest, CodeValidationFailed, message)
}

// MethodNotAllowedError is a 405 reported with the validation_failed code (as in the v1 contract).
func MethodNotAllowedError(message string) IHttpError {
	return newErr(http.StatusMethodNotAllowed, CodeValidationFailed, message)
}

// UnauthorizedError is a 401 unauthorized error.
func UnauthorizedError(message string) IHttpError {
	return newErr(http.StatusUnauthorized, CodeUnauthorized, message)
}

// TokenExpiredError is a 401 token_expired error.
func TokenExpiredError(message string) IHttpError {
	return newErr(http.StatusUnauthorized, CodeTokenExpired, message)
}

// ForbiddenError is a 403 error.
func ForbiddenError(message string) IHttpError {
	return newErr(http.StatusForbidden, CodeForbidden, message)
}

// NotFoundError is a 404 error.
func NotFoundError(message string) IHttpError {
	return newErr(http.StatusNotFound, CodeNotFound, message)
}

// ConflictError is a 409 error; details usually carry {"server": <current record>}.
func ConflictError(message string, details map[string]any) IHttpError {
	e := newErr(http.StatusConflict, CodeConflict, message)
	e.Details = details
	return e
}

// AlreadyRegisteredError is 409 already_registered (sign-up or profile update with a taken email/phone).
func AlreadyRegisteredError(message string) IHttpError {
	return newErr(http.StatusConflict, CodeAlreadyRegistered, message)
}

// TooManyRequestsError is a 429 error with a Retry-After header (seconds).
func TooManyRequestsError(message, retryAfterSeconds string) IHttpError {
	e := newErr(http.StatusTooManyRequests, CodeRateLimited, message)
	e.Headers = map[string]string{"Retry-After": retryAfterSeconds}
	return e
}

// ServiceUnavailableError is a 503 reported with the internal code.
func ServiceUnavailableError(message string) IHttpError {
	return newErr(http.StatusServiceUnavailable, CodeInternal, message)
}

// InternalServerError is a 500 error. The message must never contain internal details.
func InternalServerError(message string) IHttpError {
	return newErr(http.StatusInternalServerError, CodeInternal, message)
}

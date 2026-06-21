package jira

import "fmt"

// ErrCode represents a Jira client error code.
type ErrCode string

const (
	// ErrAuth is returned when authentication or authorization fails.
	ErrAuth ErrCode = "auth"
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound ErrCode = "not_found"
	// ErrTransport is returned for network-level failures.
	ErrTransport ErrCode = "transport"
	// ErrHTTP is returned when Jira responds with a non-2xx status.
	ErrHTTP ErrCode = "http"
	// ErrUnknown is returned for unclassified errors.
	ErrUnknown ErrCode = "unknown"
)

// ClientError represents a structured Jira client error.
type ClientError struct {
	Code     ErrCode
	Message  string
	HTTPCode int
}

// Error implements the error interface.
func (e *ClientError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("jira: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("jira: %s", e.Code)
}

// IsClientError reports whether err is a *ClientError with the given code.
func IsClientError(err error, code ErrCode) bool {
	var cerr *ClientError
	if err == nil {
		return false
	}
	return isClientError(err, &cerr) && cerr.Code == code
}

// isClientError is a helper that unwraps errors to find a *ClientError.
func isClientError(err error, target **ClientError) bool {
	for err != nil {
		if cerr, ok := err.(*ClientError); ok {
			*target = cerr
			return true
		}
		err = unwrap(err)
	}
	return false
}

// HasCode reports whether err has the given error code at any level of the
// error chain.
func HasCode(err error, code ErrCode) bool {
	if err == nil {
		return false
	}
	var cerr *ClientError
	return isClientError(err, &cerr) && cerr.Code == code
}

// unwrap extracts the wrapped error from standard error types.
func unwrap(err error) error {
	switch e := err.(type) {
	case interface{ Unwrap() error }:
		return e.Unwrap()
	case interface{ Unwrap() []error }:
		unwrapped := e.Unwrap()
		if len(unwrapped) > 0 {
			return unwrapped[0]
		}
	}
	return nil
}

// NewAuthError returns a new authentication error.
func NewAuthError(msg string) *ClientError {
	return &ClientError{Code: ErrAuth, Message: msg}
}

// NewNotFoundError returns a new not-found error.
func NewNotFoundError(key string) *ClientError {
	return &ClientError{Code: ErrNotFound, Message: key}
}

// NewTransportError returns a new transport error.
func NewTransportError(msg string) *ClientError {
	return &ClientError{Code: ErrTransport, Message: msg}
}

// NewHTTPErr returns a new HTTP error with the given status code and message.
func NewHTTPErr(statusCode int, msg string) *ClientError {
	return &ClientError{Code: ErrHTTP, HTTPCode: statusCode, Message: msg}
}

// NewUnknownErr returns a new unknown error.
func NewUnknownErr(msg string) *ClientError {
	return &ClientError{Code: ErrUnknown, Message: msg}
}

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
	URL      string
}

// Error implements the error interface.
func (e *ClientError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("jira: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("jira: %s", e.Code)
}

// IsClientError reports whether err is a *ClientError with the given code.
// Every branch of a joined error (errors.Join) is inspected, as is each
// wrapped error in the chain.
func IsClientError(err error, code ErrCode) bool {
	return hasClientCode(err, code)
}

// hasClientCode walks the error tree depth-first, following both single and
// multi-error Unwrap methods, and reports whether any *ClientError in it has
// the given code.
func hasClientCode(err error, code ErrCode) bool {
	if err == nil {
		return false
	}
	if cerr, ok := err.(*ClientError); ok && cerr.Code == code {
		return true
	}
	switch e := err.(type) {
	case interface{ Unwrap() error }:
		return hasClientCode(e.Unwrap(), code)
	case interface{ Unwrap() []error }:
		for _, inner := range e.Unwrap() {
			if hasClientCode(inner, code) {
				return true
			}
		}
	}
	return false
}

// HasCode reports whether err has the given error code at any level of the
// error chain, including every branch of a joined error.
func HasCode(err error, code ErrCode) bool {
	return hasClientCode(err, code)
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

// NewHTTPErrorWithURL returns a new HTTP error with status and URL context.
func NewHTTPErrorWithURL(statusCode int, url string, msg string) *ClientError {
	return &ClientError{Code: ErrHTTP, HTTPCode: statusCode, URL: url, Message: msg}
}

// NewUnknownErr returns a new unknown error.
func NewUnknownErr(msg string) *ClientError {
	return &ClientError{Code: ErrUnknown, Message: msg}
}

// NewUnknownErrWithURL returns a new unknown error with URL context.
func NewUnknownErrWithURL(url string, msg string) *ClientError {
	return &ClientError{Code: ErrUnknown, URL: url, Message: msg}
}

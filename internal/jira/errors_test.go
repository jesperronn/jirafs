package jira

import (
	"errors"
	"fmt"
	"testing"
)

func TestClientError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantStr string
	}{
		{
			name:    "auth error with message",
			err:     NewAuthError("invalid token"),
			wantStr: "jira: auth: invalid token",
		},
		{
			name:    "not found error with key",
			err:     NewNotFoundError("PROJ-123"),
			wantStr: "jira: not_found: PROJ-123",
		},
		{
			name:    "transport error with message",
			err:     NewTransportError("connection refused"),
			wantStr: "jira: transport: connection refused",
		},
		{
			name:    "http error with status",
			err:     NewHTTPErr(500, "internal server error"),
			wantStr: "jira: http: internal server error",
		},
		{
			name:    "unknown error with message",
			err:     NewUnknownErr("something went wrong"),
			wantStr: "jira: unknown: something went wrong",
		},
		{
			name:    "nil message",
			err:     &ClientError{Code: ErrAuth},
			wantStr: "jira: auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantStr {
				t.Errorf("Error() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestIsClientError(t *testing.T) {
	authErr := NewAuthError("bad token")

	tests := []struct {
		name   string
		err    error
		code   ErrCode
		want   bool
	}{
		{
			name: "matching auth error",
			err:  authErr,
			code: ErrAuth,
			want: true,
		},
		{
			name: "non-matching code",
			err:  authErr,
			code: ErrNotFound,
			want: false,
		},
		{
			name: "wrapped error",
			err:  fmt.Errorf("wrapped: %w", authErr),
			code: ErrAuth,
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			code: ErrAuth,
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("plain error"),
			code: ErrAuth,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsClientError(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("IsClientError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasCode(t *testing.T) {
	authErr := NewAuthError("bad token")

	tests := []struct {
		name string
		err  error
		code ErrCode
		want bool
	}{
		{
			name: "matching error",
			err:  authErr,
			code: ErrAuth,
			want: true,
		},
		{
			name: "non-matching code",
			err:  authErr,
			code: ErrNotFound,
			want: false,
		},
		{
			name: "wrapped error",
			err:  fmt.Errorf("wrapped: %w", authErr),
			code: ErrAuth,
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			code: ErrAuth,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasCode(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("HasCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewErrorConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode ErrCode
	}{
		{"NewAuthError", NewAuthError("msg"), ErrAuth},
		{"NewNotFoundError", NewNotFoundError("key"), ErrNotFound},
		{"NewTransportError", NewTransportError("msg"), ErrTransport},
		{"NewHTTPErr", NewHTTPErr(404, "msg"), ErrHTTP},
		{"NewUnknownErr", NewUnknownErr("msg"), ErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !HasCode(tt.err, tt.wantCode) {
				t.Errorf("expected error code %q, got %v", tt.wantCode, tt.err)
			}
		})
	}
}

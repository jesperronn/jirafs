package jira

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewHTTPErrorWithURL_And_UnknownWithURL(t *testing.T) {
	e := NewHTTPErrorWithURL(502, "https://x/rest", "bad gateway")
	if e.Code != ErrHTTP || e.HTTPCode != 502 || e.URL != "https://x/rest" || e.Message != "bad gateway" {
		t.Errorf("got %+v", e)
	}
	u := NewUnknownErrWithURL("https://y", "boom")
	if u.Code != ErrUnknown || u.URL != "https://y" || u.Message != "boom" {
		t.Errorf("got %+v", u)
	}
}

func TestIsClientError_ThroughJoinedErrors(t *testing.T) {
	inner := NewNotFoundError("A-1")
	if !IsClientError(errors.Join(inner, errors.New("second")), ErrNotFound) {
		t.Error("expected first joined element to be found")
	}
	// Every element of a joined error is inspected, not just the first.
	if !IsClientError(errors.Join(errors.New("first"), inner), ErrNotFound) {
		t.Error("second joined element should be inspected")
	}
	if !HasCode(errors.Join(errors.New("first"), fmt.Errorf("ctx: %w", inner)), ErrNotFound) {
		t.Error("wrapped element inside a later joined branch should be found")
	}
	if IsClientError(errors.Join(errors.New("plain"), NewAuthError("x")), ErrNotFound) {
		t.Error("joined error without the code must not match")
	}
	wrapped := fmt.Errorf("ctx: %w", inner)
	if !IsClientError(wrapped, ErrNotFound) || HasCode(wrapped, ErrAuth) {
		t.Error("wrapped error code detection wrong")
	}
}

func TestUnwrap_Variants(t *testing.T) {
	base := errors.New("base")
	if got := unwrap(fmt.Errorf("w: %w", base)); got != base {
		t.Errorf("single unwrap = %v", got)
	}
	if got := unwrap(errors.Join(base, errors.New("other"))); got != base {
		t.Errorf("multi unwrap = %v, want first", got)
	}
	if got := unwrap(errors.New("plain")); got != nil {
		t.Errorf("plain unwrap = %v, want nil", got)
	}
}

package config

import (
	"fmt"
	"strings"
)

// CredentialRef holds the parsed components of a credential reference string.
// The scheme and target are both non-empty after successful parsing.
type CredentialRef struct {
	Scheme string
	Target string
}

// ParseCredentialRef splits a credential reference string into scheme and target.
// It expects the format "scheme://target" where scheme is non-empty and target
// is non-empty. Empty input or missing scheme separator returns
// ErrInvalidCredentialRef.
func ParseCredentialRef(ref string) (CredentialRef, error) {
	if ref == "" {
		return CredentialRef{}, NewSettingError(
			ErrInvalidCredentialRef,
			"credential ref must not be empty",
			"credential_ref",
			"",
		)
	}

	idx := strings.Index(ref, "://")
	if idx < 0 {
		return CredentialRef{}, NewSettingError(
			ErrInvalidCredentialRef,
			"credential ref must have a scheme (scheme://target)",
			"credential_ref",
			ref,
		)
	}

	scheme := ref[:idx]
	target := ref[idx+3:]

	if scheme == "" {
		return CredentialRef{}, NewSettingError(
			ErrInvalidCredentialRef,
			"credential ref scheme must not be empty",
			"credential_ref",
			ref,
		)
	}

	if target == "" {
		return CredentialRef{}, NewSettingError(
			ErrInvalidCredentialRef,
			fmt.Sprintf("credential ref target must not be empty for scheme %q", scheme),
			"credential_ref",
			ref,
		)
	}

	switch scheme {
	case "env", "file":
		// Supported schemes — allow through.
	default:
		return CredentialRef{}, NewSettingError(
			ErrInvalidCredentialRef,
			fmt.Sprintf("unsupported credential ref scheme %q: only env:// and file:// are allowed", scheme),
			"credential_ref",
			ref,
		)
	}

	return CredentialRef{
		Scheme: scheme,
		Target: target,
	}, nil
}

// ParseCredentialRefs parses a slice of raw credential ref strings into an
// ordered slice of typed CredentialRef values. It preserves the input order
// and returns the first error encountered (from the lowest-index entry).
// An empty input returns an empty slice with no error.
func ParseCredentialRefs(refs []string) ([]CredentialRef, error) {
	out := make([]CredentialRef, 0, len(refs))
	for _, ref := range refs {
		parsed, err := ParseCredentialRef(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

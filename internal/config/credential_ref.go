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

	return CredentialRef{
		Scheme: scheme,
		Target: target,
	}, nil
}

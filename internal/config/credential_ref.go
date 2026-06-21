package config

import (
	"fmt"
	"os"
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

// ResolvedCredential holds a credential reference after its source has been
// resolved into a map of normalized auth fields. The Scheme and Target fields
// preserve the original parsed values for traceability.
type ResolvedCredential struct {
	Scheme string            // "env" or "file"
	Target string            // original target (VAR name or file path)
	Fields map[string]string // normalized auth fields
}

// ResolveEnvCredential reads the environment variable named by Target and
// returns a ResolvedCredential with the variable name as the key and the
// variable value as the value in Fields. If the variable is unset, it
// returns ErrCredentialResolve.
func ResolveEnvCredential(ref CredentialRef) (ResolvedCredential, error) {
	if ref.Scheme != "env" {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("expected env:// scheme, got %q", ref.Scheme),
			"credential_ref", ref.Scheme+"://"+ref.Target,
		)
	}

	value := os.Getenv(ref.Target)
	if value == "" {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("environment variable %q is not set", ref.Target),
			"credential_ref", "env://"+ref.Target,
		)
	}

	return ResolvedCredential{
		Scheme: "env",
		Target: ref.Target,
		Fields: map[string]string{
			ref.Target: value,
		},
	}, nil
}

// AuthTypeRequiredFields maps supported auth types to the set of field keys
// that must be present in a resolved credential for that auth type to be
// considered valid. This is the first implementation of auth field validation.
var AuthTypeRequiredFields = map[string]map[string]struct{}{
	"basic": {
		"username": {},
		"password": {},
	},
	"atlassian_api_token": {
		"api_token": {},
	},
	"oauth1": {
		"oauth_token":      {},
		"oauth_secret":     {},
		"oauth_consumer_key": {},
		"oauth_signature":  {},
	},
}

// ValidateResolvedCredential checks that the resolved credential contains all
// fields required by the given auth_type. It returns ErrMissingAuthField with
// details about which fields are missing. An empty auth_type matches any
// credential (no validation). An unknown auth_type returns ErrMissingAuthField
// listing all keys in the resolved credential as "unknown auth type" fields.
// An empty Fields map with a known auth_type returns ErrMissingAuthField listing
// all required fields.
func ValidateResolvedCredential(authType string, cred ResolvedCredential) error {
	if authType == "" {
		return nil
	}

	required, ok := AuthTypeRequiredFields[authType]
	if !ok {
		// Unknown auth type: list every field key as a missing required field.
		missing := make([]string, 0, len(cred.Fields))
		for k := range cred.Fields {
			missing = append(missing, k)
		}
		return NewSettingError(
			ErrMissingAuthField,
			fmt.Sprintf("unknown auth type %q: no required fields defined, but %d field(s) present", authType, len(cred.Fields)),
			"auth_type",
			authType,
		)
	}

	missing := make([]string, 0, len(required))
	for field := range required {
		if _, present := cred.Fields[field]; !present {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		return NewSettingError(
			ErrMissingAuthField,
			fmt.Sprintf("missing required auth field(s) for auth_type %q: %v", authType, missing),
			"auth_type",
			authType,
		)
	}

	return nil
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

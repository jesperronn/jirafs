package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
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

// ResolveFileCredential reads the file at ref.Target (a file:// path),
// parses it as TOML, and returns a ResolvedCredential with the parsed
// string key-value pairs as Fields. If the file cannot be read or parsed,
// it returns ErrCredentialResolve.
func ResolveFileCredential(ref CredentialRef) (ResolvedCredential, error) {
	if ref.Scheme != "file" {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("expected file:// scheme, got %q", ref.Scheme),
			"credential_ref", ref.Scheme+"://"+ref.Target,
		)
	}

	path := ref.Target
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ResolvedCredential{}, NewSettingError(
				ErrCredentialResolve,
				fmt.Sprintf("cannot resolve home directory for path %q", path),
				"credential_ref", "file://"+path,
			)
		}
		path = home + path[1:]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("cannot read credential file %q: %s", path, err),
			"credential_ref", "file://"+path,
		)
	}

	fields, err := parseTOMLFields(data)
	if err != nil {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("cannot parse credential file %q: %s", path, err),
			"credential_ref", "file://"+path,
		)
	}

	return ResolvedCredential{
		Scheme: "file",
		Target: ref.Target,
		Fields: fields,
	}, nil
}

// parseTOMLFields parses a TOML byte slice and returns a map of top-level
// string key-value pairs. Only scalar string values are included; other
// types are skipped. Returns an error on TOML parse failure.
func parseTOMLFields(data []byte) (map[string]string, error) {
	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			fields[k] = s
		}
	}
	return fields, nil
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

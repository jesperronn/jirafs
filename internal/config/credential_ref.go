package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// ResolvedInstanceCredentials packages the instance-level connection details
// Jira callers need after credential resolution is complete.
type ResolvedInstanceCredentials struct {
	BaseURL    string
	AuthType   string
	Credential ResolvedCredential
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

// ResolveFileCredential reads the file at the credential ref's target path,
// parses it as TOML, and returns a ResolvedCredential with each top-level
// key mapped to its string value in Fields. The path target is expanded
// (tilde → home directory) before reading. If the file cannot be read or
// parsed, it returns ErrCredentialResolve with details.
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
				fmt.Sprintf("cannot resolve home directory for file:// path: %s", ref.Target),
				"credential_ref", "file://"+ref.Target,
			)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("cannot read file %q: %s", path, err.Error()),
			"credential_ref", "file://"+ref.Target,
		)
	}

	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return ResolvedCredential{}, NewSettingError(
			ErrCredentialResolve,
			fmt.Sprintf("cannot parse file %q as TOML: %s", path, err.Error()),
			"credential_ref", "file://"+ref.Target,
		)
	}

	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			fields[k] = val
		case float64:
			fields[k] = fmt.Sprintf("%g", val)
		case bool:
			fields[k] = fmt.Sprintf("%t", val)
		default:
			fields[k] = fmt.Sprintf("%v", val)
		}
	}

	return ResolvedCredential{
		Scheme: "file",
		Target: ref.Target,
		Fields: fields,
	}, nil
}

// MergeCredentials merges an ordered slice of resolved credentials into a
// single ResolvedCredential. The merged result preserves the Scheme and
// Target from the last (highest-index) source. All Fields maps are
// deep-merged with later sources overriding earlier ones for the same key.
// An empty input returns an empty Fields map with no error.
func MergeCredentials(creds []ResolvedCredential) ResolvedCredential {
	merged := ResolvedCredential{
		Fields: make(map[string]string),
	}

	for _, c := range creds {
		merged.Scheme = c.Scheme
		merged.Target = c.Target
		for k, v := range c.Fields {
			merged.Fields[k] = v
		}
	}

	return merged
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

// ResolveInstanceCredentials resolves, merges, and validates credentials for a
// named instance from the loaded settings. It is the path-local API that Jira
// callers should use instead of re-implementing credential resolution.
func (s *Settings) ResolveInstanceCredentials(instanceName string) (ResolvedInstanceCredentials, error) {
	inst, ok := s.Instances[instanceName]
	if !ok {
		return ResolvedInstanceCredentials{}, NewSettingError(
			ErrNoUsableInstance,
			fmt.Sprintf("instance %q not found", instanceName),
			"instance",
			instanceName,
		)
	}

	if len(inst.CredentialRefs) == 0 {
		return ResolvedInstanceCredentials{}, NewSettingError(
			ErrNoUsableInstance,
			fmt.Sprintf("instance %q has no credential_refs", instanceName),
			"instances."+instanceName+".credential_refs",
			"",
		)
	}

	parsedRefs, err := ParseCredentialRefs(inst.CredentialRefs)
	if err != nil {
		return ResolvedInstanceCredentials{}, err
	}

	resolved := make([]ResolvedCredential, 0, len(parsedRefs))
	for _, ref := range parsedRefs {
		switch ref.Scheme {
		case "env":
			cred, err := ResolveEnvCredential(ref)
			if err != nil {
				return ResolvedInstanceCredentials{}, err
			}
			resolved = append(resolved, cred)
		case "file":
			cred, err := ResolveFileCredential(ref)
			if err != nil {
				return ResolvedInstanceCredentials{}, err
			}
			resolved = append(resolved, cred)
		default:
			return ResolvedInstanceCredentials{}, NewSettingError(
				ErrInvalidCredentialRef,
				fmt.Sprintf("unsupported credential ref scheme %q", ref.Scheme),
				"credential_ref",
				ref.Scheme+"://"+ref.Target,
			)
		}
	}

	merged := MergeCredentials(resolved)
	if err := ValidateResolvedCredential(inst.AuthType, merged); err != nil {
		return ResolvedInstanceCredentials{}, err
	}

	return ResolvedInstanceCredentials{
		BaseURL:    inst.BaseURL,
		AuthType:   inst.AuthType,
		Credential: merged,
	}, nil
}

// InstanceCredentialsForPath resolves the project associated with path and
// returns the resolved instance credentials for that project's instance.
// It matches path against project mirror_dir and local_dirs as prefixes,
// using the longest-match heuristic (most-specific project wins).
// It returns ErrNoUsableInstance when no project matches the path.
func (s *Settings) InstanceCredentialsForPath(path string) (ResolvedInstanceCredentials, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ResolvedInstanceCredentials{}, NewSettingError(
			ErrMissingField,
			fmt.Sprintf("cannot resolve absolute path for %q: %s", path, err.Error()),
			"path", path,
		)
	}
	abs = filepath.Clean(abs)

	var bestProject string
	var bestDepth int
	for name, proj := range s.Projects {
		mirror := filepath.Clean(proj.MirrorDir)
		if isPathPrefixOf(mirror, abs) {
			d := pathDepth(mirror)
			if d > bestDepth {
				bestDepth = d
				bestProject = name
			}
		}
		for _, ld := range proj.LocalDirs {
			local := filepath.Clean(ld)
			if isPathPrefixOf(local, abs) {
				d := pathDepth(local)
				if d > bestDepth {
					bestDepth = d
					bestProject = name
				}
			}
		}
	}

	if bestProject == "" {
		return ResolvedInstanceCredentials{}, NewSettingError(
			ErrNoUsableInstance,
			fmt.Sprintf("no project matches path %q", path),
			"path", path,
		)
	}

	inst := s.Projects[bestProject].Instance
	return s.ResolveInstanceCredentials(inst)
}

// isPathPrefixOf reports whether prefix is a directory prefix of target.
func isPathPrefixOf(prefix, target string) bool {
	return target == prefix || len(target) > len(prefix) && target[:len(prefix)+1] == prefix+string(filepath.Separator)
}

// pathDepth returns the number of path components in p.
func pathDepth(p string) int {
	return strings.Count(p, string(filepath.Separator)) + 1
}

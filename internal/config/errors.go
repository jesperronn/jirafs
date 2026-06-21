// Package config provides settings loading and validation for ~/.jirafs/settings.toml.
package config

import "fmt"

// Error codes are stable string identifiers that callers can check
// programmatically. They are not expected to change across releases.
const (
	// ErrMissingField is returned when a required field is absent.
	ErrMissingField = "missing_field"

	// ErrInvalidURL is returned when base_url is invalid or not absolute.
	ErrInvalidURL = "invalid_url"

	// ErrUnknownInstance is returned when a project references a non-existent instance.
	ErrUnknownInstance = "unknown_instance"

	// ErrUnknownProject is returned when state references a non-existent project.
	ErrUnknownProject = "unknown_project"

	// ErrDuplicateLocalDir is returned when two projects share a normalized local_dirs entry.
	ErrDuplicateLocalDir = "duplicate_local_dir"

	// ErrEmptyMirrorDir is returned when mirror_dir is empty after expansion.
	ErrEmptyMirrorDir = "empty_mirror_dir"

	// ErrInvalidCredentialRef is returned when a credential reference scheme is unsupported.
	ErrInvalidCredentialRef = "invalid_credential_ref"

	// ErrNoProjectResolved is returned when no project can be determined for the current context.
	ErrNoProjectResolved = "no_project_resolved"

	// ErrAmbiguousMatch is returned when multiple projects match the current directory equally.
	ErrAmbiguousMatch = "ambiguous_match"

	// ErrCredentialResolve is returned when a credential reference cannot be resolved at runtime.
	ErrCredentialResolve = "credential_resolve"

	// ErrNoUsableInstance is returned when the resolved project has no usable instance config.
	ErrNoUsableInstance = "no_usable_instance"
)

// SettingError wraps a stable code with a human-readable message and optional
// context fields. Callers should prefer checking Code over comparing messages.
type SettingError struct {
	Code    string
	Message string
	Field   string // optional: the setting path that triggered the error
	Value   string // optional: the offending value, if any
}

func (e *SettingError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("jirafs/settings: %s (%s): %s", e.Code, e.Field, e.Message)
	}
	return fmt.Sprintf("jirafs/settings: %s: %s", e.Code, e.Message)
}

// NewSettingError creates a SettingError from a stable code, message, and optional field/value.
func NewSettingError(code, message, field, value string) *SettingError {
	return &SettingError{
		Code:    code,
		Message: message,
		Field:   field,
		Value:   value,
	}
}

// IsSettingError returns true if err is a *SettingError with the given code.
func IsSettingError(err error, code string) bool {
	if err == nil {
		return false
	}
	var se *SettingError
	for err != nil {
		if target, ok := err.(*SettingError); ok {
			se = target
			break
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
			continue
		}
		return false
	}
	return se != nil && se.Code == code
}

// Unwrap implements errors.Unwrap for SettingError so errors.Is works across
// wrapped chains.
func (e *SettingError) Unwrap() error {
	return nil
}

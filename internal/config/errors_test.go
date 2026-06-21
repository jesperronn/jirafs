package config

import (
	"errors"
	"fmt"
	"testing"
)

func TestSettingError_Error(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		field   string
		value   string
		want    string
	}{
		{
			name:    "no field or value",
			code:    ErrMissingField,
			message: "version is required",
			field:   "",
			value:   "",
			want:    "jirafs/settings: missing_field: version is required",
		},
		{
			name:    "with field only",
			code:    ErrInvalidURL,
			message: "must be an absolute URL",
			field:   "instances.work.base_url",
			value:   "",
			want:    "jirafs/settings: invalid_url (instances.work.base_url): must be an absolute URL",
		},
		{
			name:    "with field and value",
			code:    ErrEmptyMirrorDir,
			message: "mirror_dir must not be empty after expansion",
			field:   "projects.platform.mirror_dir",
			value:   "~/",
			want:    "jirafs/settings: empty_mirror_dir (projects.platform.mirror_dir): mirror_dir must not be empty after expansion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			se := NewSettingError(tt.code, tt.message, tt.field, tt.value)
			got := se.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSettingError(t *testing.T) {
	se := NewSettingError(ErrUnknownInstance, "instance %q not found", "instances.foo", "foo")

	tests := []struct {
		name  string
		err   error
		code  string
		want  bool
	}{
		{
			name:  "matching code",
			err:   se,
			code:  ErrUnknownInstance,
			want:  true,
		},
		{
			name:  "different code",
			err:   se,
			code:  ErrMissingField,
			want:  false,
		},
		{
			name:  "nil error",
			err:   nil,
			code:  ErrUnknownInstance,
			want:  false,
		},
		{
			name:  "wrapped error",
			err:   fmt.Errorf("wrapped: %w", se),
			code:  ErrUnknownInstance,
			want:  true,
		},
		{
			name:  "plain error",
			err:   errors.New("plain error"),
			code:  ErrUnknownInstance,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSettingError(tt.err, tt.code)
			if got != tt.want {
				t.Errorf("IsSettingError(%v, %q) = %v, want %v", tt.err, tt.code, got, tt.want)
			}
		})
	}
}

func TestErrorCodesAreStable(t *testing.T) {
	// Verify all expected codes are defined and non-empty.
	codes := []string{
		ErrMissingField,
		ErrInvalidURL,
		ErrUnknownInstance,
		ErrUnknownProject,
		ErrDuplicateLocalDir,
		ErrEmptyMirrorDir,
		ErrInvalidCredentialRef,
		ErrNoProjectResolved,
		ErrAmbiguousMatch,
		ErrCredentialResolve,
		ErrNoUsableInstance,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("error code must not be empty")
		}
	}
}

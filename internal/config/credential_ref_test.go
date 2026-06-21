package config

import (
	"testing"
)

func TestParseCredentialRef(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantScheme  string
		wantTarget  string
		wantErr     bool
		wantCode    string
	}{
		{
			name:       "env scheme",
			input:      "env://MY_API_TOKEN",
			wantScheme: "env",
			wantTarget: "MY_API_TOKEN",
			wantErr:    false,
		},
		{
			name:       "file scheme with tilde path",
			input:      "file://~/.jirafs/credentials/user.toml",
			wantScheme: "file",
			wantTarget: "~/.jirafs/credentials/user.toml",
			wantErr:    false,
		},
		{
			name:       "file scheme with absolute path",
			input:      "file:///etc/jirafs/creds.toml",
			wantScheme: "file",
			wantTarget: "/etc/jirafs/creds.toml",
			wantErr:    false,
		},
		{
			name:      "unsupported scheme vault",
			input:     "vault://secret/jira",
			wantErr:   true,
			wantCode:  ErrInvalidCredentialRef,
		},
		{
			name:      "unsupported scheme ssh",
			input:     "ssh://git@github.com/jirafs/jirafs.git",
			wantErr:   true,
			wantCode:  ErrInvalidCredentialRef,
		},
		{
			name:      "empty string",
			input:     "",
			wantErr:   true,
			wantCode:  ErrInvalidCredentialRef,
		},
		{
			name:      "no scheme separator",
			input:     "just-a-target",
			wantErr:   true,
			wantCode:  ErrInvalidCredentialRef,
		},
		{
			name:      "empty scheme",
			input:     "://target",
			wantErr:   true,
			wantCode:  ErrInvalidCredentialRef,
		},
		{
			name:      "empty target",
			input:     "env://",
			wantErr:   true,
			wantCode:  ErrInvalidCredentialRef,
		},
		{
			name:       "target with colons",
			input:      "file://http://example.com/creds.toml",
			wantScheme: "file",
			wantTarget: "http://example.com/creds.toml",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCredentialRef(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCredentialRef(%q) expected error, got nil", tt.input)
					return
				}
				if tt.wantCode != "" {
					if !IsSettingError(err, tt.wantCode) {
						t.Errorf("ParseCredentialRef(%q) error code = %v, want %v",
							tt.input, err, tt.wantCode)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ParseCredentialRef(%q) unexpected error = %v", tt.input, err)
				return
			}

			if got.Scheme != tt.wantScheme {
				t.Errorf("ParseCredentialRef(%q).Scheme = %q, want %q",
					tt.input, got.Scheme, tt.wantScheme)
			}
			if got.Target != tt.wantTarget {
				t.Errorf("ParseCredentialRef(%q).Target = %q, want %q",
					tt.input, got.Target, tt.wantTarget)
			}
		})
	}
}

func TestResolveEnvCredential(t *testing.T) {
	t.Setenv("TEST_JIRAFS_TOKEN", "secret-token-value")
	t.Setenv("TEST_JIRAFS_USER", "testuser")

	tests := []struct {
		name      string
		ref       CredentialRef
		wantErr   bool
		wantCode  string
		wantKey   string
		wantValue string
	}{
		{
			name:      "set env var resolves",
			ref:       CredentialRef{Scheme: "env", Target: "TEST_JIRAFS_TOKEN"},
			wantErr:   false,
			wantKey:   "TEST_JIRAFS_TOKEN",
			wantValue: "secret-token-value",
		},
		{
			name:      "another set env var resolves",
			ref:       CredentialRef{Scheme: "env", Target: "TEST_JIRAFS_USER"},
			wantErr:   false,
			wantKey:   "TEST_JIRAFS_USER",
			wantValue: "testuser",
		},
		{
			name:    "unset env var returns error",
			ref:     CredentialRef{Scheme: "env", Target: "UNSET_VAR_DOES_NOT_EXIST"},
			wantErr: true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:    "non-env scheme returns error",
			ref:     CredentialRef{Scheme: "file", Target: "/path/to/file"},
			wantErr: true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:    "empty target returns error",
			ref:     CredentialRef{Scheme: "env", Target: ""},
			wantErr: true,
			wantCode: ErrCredentialResolve,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveEnvCredential(tt.ref)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveEnvCredential(%+v) expected error, got nil", tt.ref)
					return
				}
				if tt.wantCode != "" {
					if !IsSettingError(err, tt.wantCode) {
						t.Errorf("ResolveEnvCredential(%+v) error code = %v, want %v",
							tt.ref, err, tt.wantCode)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ResolveEnvCredential(%+v) unexpected error = %v", tt.ref, err)
				return
			}

			if got.Scheme != "env" {
				t.Errorf("ResolveEnvCredential(%+v).Scheme = %q, want %q",
					tt.ref, got.Scheme, "env")
			}
			if got.Target != tt.ref.Target {
				t.Errorf("ResolveEnvCredential(%+v).Target = %q, want %q",
					tt.ref, got.Target, tt.ref.Target)
			}
			if len(got.Fields) != 1 {
				t.Errorf("ResolveEnvCredential(%+v).Fields length = %d, want 1",
					tt.ref, len(got.Fields))
			}
			if got.Fields[tt.wantKey] != tt.wantValue {
				t.Errorf("ResolveEnvCredential(%+v).Fields[%q] = %q, want %q",
					tt.ref, tt.wantKey, got.Fields[tt.wantKey], tt.wantValue)
			}
		})
	}
}

func TestParseCredentialRefs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []CredentialRef
		wantErr bool
	}{
		{
			name:  "empty slice",
			input: []string{},
			want:  []CredentialRef{},
		},
		{
			name:  "nil slice",
			input: nil,
			want:  []CredentialRef{},
		},
		{
			name:  "single env ref",
			input: []string{"env://MY_API_TOKEN"},
			want: []CredentialRef{
				{Scheme: "env", Target: "MY_API_TOKEN"},
			},
		},
		{
			name:  "single file ref",
			input: []string{"file://~/.jirafs/creds.toml"},
			want: []CredentialRef{
				{Scheme: "file", Target: "~/.jirafs/creds.toml"},
			},
		},
		{
			name:  "ordered multi refs",
			input: []string{"env://API_TOKEN", "env://API_SECRET", "file://creds.toml"},
			want: []CredentialRef{
				{Scheme: "env", Target: "API_TOKEN"},
				{Scheme: "env", Target: "API_SECRET"},
				{Scheme: "file", Target: "creds.toml"},
			},
		},
		{
			name:    "first entry invalid",
			input:   []string{"vault://bad", "env://OK"},
			wantErr: true,
		},
		{
			name:    "middle entry invalid",
			input:   []string{"env://OK", "vault://bad", "env://OK2"},
			wantErr: true,
		},
		{
			name:    "last entry invalid",
			input:   []string{"env://OK", "file://ok.toml", "ssh://bad"},
			wantErr: true,
		},
		{
			name:    "empty string in middle",
			input:   []string{"env://OK", "", "env://OK2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCredentialRefs(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCredentialRefs(%v) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseCredentialRefs(%v) unexpected error = %v", tt.input, err)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("ParseCredentialRefs(%v) length = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i].Scheme != tt.want[i].Scheme {
					t.Errorf("ParseCredentialRefs(%v)[%d].Scheme = %q, want %q", tt.input, i, got[i].Scheme, tt.want[i].Scheme)
				}
				if got[i].Target != tt.want[i].Target {
					t.Errorf("ParseCredentialRefs(%v)[%d].Target = %q, want %q", tt.input, i, got[i].Target, tt.want[i].Target)
				}
			}
		})
	}
}

func TestValidateResolvedCredential(t *testing.T) {
	tests := []struct {
		name      string
		authType  string
		cred      ResolvedCredential
		wantErr   bool
		wantCode  string
	}{
		{
			name:     "empty auth_type passes",
			authType: "",
			cred:     ResolvedCredential{Fields: map[string]string{}},
			wantErr:  false,
		},
		{
			name:     "empty auth_type passes with fields",
			authType: "",
			cred:     ResolvedCredential{Fields: map[string]string{"api_token": "tok"}},
			wantErr:  false,
		},
		{
			name:     "basic with all fields passes",
			authType: "basic",
			cred:     ResolvedCredential{Fields: map[string]string{"username": "u", "password": "p"}},
			wantErr:  false,
		},
		{
			name:     "basic missing password fails",
			authType: "basic",
			cred:     ResolvedCredential{Fields: map[string]string{"username": "u"}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "basic missing username fails",
			authType: "basic",
			cred:     ResolvedCredential{Fields: map[string]string{"password": "p"}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "basic missing both fields fails",
			authType: "basic",
			cred:     ResolvedCredential{Fields: map[string]string{}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "atlassian_api_token with api_token passes",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{"api_token": "tok"}},
			wantErr:  false,
		},
		{
			name:     "atlassian_api_token missing api_token fails",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{"email": "test@example.com"}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "atlassian_api_token empty fields fails",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "oauth1 with all fields passes",
			authType: "oauth1",
			cred: ResolvedCredential{Fields: map[string]string{
				"oauth_token":        "ot",
				"oauth_secret":       "os",
				"oauth_consumer_key": "ck",
				"oauth_signature":    "sig",
			}},
			wantErr: false,
		},
		{
			name:     "oauth1 missing oauth_secret fails",
			authType: "oauth1",
			cred: ResolvedCredential{Fields: map[string]string{
				"oauth_token":        "ot",
				"oauth_consumer_key": "ck",
				"oauth_signature":    "sig",
			}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "unknown auth type returns error",
			authType: "unknown_auth",
			cred:     ResolvedCredential{Fields: map[string]string{"api_token": "tok"}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "extra fields do not cause failure",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{"api_token": "tok", "extra": "val"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResolvedCredential(tt.authType, tt.cred)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateResolvedCredential(%q, %+v) expected error, got nil", tt.authType, tt.cred)
					return
				}
				if tt.wantCode != "" {
					if !IsSettingError(err, tt.wantCode) {
						t.Errorf("ValidateResolvedCredential(%q, %+v) error code = %v, want %v",
							tt.authType, tt.cred, err, tt.wantCode)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateResolvedCredential(%q, %+v) unexpected error = %v", tt.authType, tt.cred, err)
			}
		})
	}
}

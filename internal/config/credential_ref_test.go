package config

import (
	"os"
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

func TestResolveFileCredential(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid TOML credential file
	validTOML := `api_token = "file-token-123"
api_secret = "file-secret-456"
`
	validPath := tmpDir + "/valid.toml"
	if err := os.WriteFile(validPath, []byte(validTOML), 0644); err != nil {
		t.Fatalf("write valid TOML: %v", err)
	}

	// Create an empty TOML file
	emptyTOML := ""
	emptyPath := tmpDir + "/empty.toml"
	if err := os.WriteFile(emptyPath, []byte(emptyTOML), 0644); err != nil {
		t.Fatalf("write empty TOML: %v", err)
	}

	// Create a non-TOML file
	badContent := `not = [valid, toml`
	badPath := tmpDir + "/bad.toml"
	if err := os.WriteFile(badPath, []byte(badContent), 0644); err != nil {
		t.Fatalf("write bad TOML: %v", err)
	}

	// Create a TOML with non-string values
	nonStringTOML := `api_token = "string-val"
port = 8080
enabled = true
`
	nonStringPath := tmpDir + "/nonstring.toml"
	if err := os.WriteFile(nonStringPath, []byte(nonStringTOML), 0644); err != nil {
		t.Fatalf("write non-string TOML: %v", err)
	}

	tests := []struct {
		name      string
		ref       CredentialRef
		wantErr   bool
		wantCode  string
		wantKeys  int
		wantField map[string]string
	}{
		{
			name: "valid TOML file resolves",
			ref:  CredentialRef{Scheme: "file", Target: validPath},
			wantErr: false,
			wantKeys: 2,
			wantField: map[string]string{
				"api_token":  "file-token-123",
				"api_secret": "file-secret-456",
			},
		},
		{
			name:    "non-existent file returns error",
			ref:     CredentialRef{Scheme: "file", Target: tmpDir + "/nope.toml"},
			wantErr: true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:    "invalid TOML returns error",
			ref:     CredentialRef{Scheme: "file", Target: badPath},
			wantErr: true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:    "non-file scheme returns error",
			ref:     CredentialRef{Scheme: "env", Target: "SOME_VAR"},
			wantErr: true,
			wantCode: ErrCredentialResolve,
		},
		{
			name: "empty TOML returns empty fields",
			ref:  CredentialRef{Scheme: "file", Target: emptyPath},
			wantErr: false,
			wantKeys: 0,
		},
		{
			name: "TOML with non-string values skips them",
			ref:  CredentialRef{Scheme: "file", Target: nonStringPath},
			wantErr: false,
			wantKeys: 1,
			wantField: map[string]string{
				"api_token": "string-val",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveFileCredential(tt.ref)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveFileCredential(%+v) expected error, got nil", tt.ref)
					return
				}
				if tt.wantCode != "" {
					if !IsSettingError(err, tt.wantCode) {
						t.Errorf("ResolveFileCredential(%+v) error code = %v, want %v",
							tt.ref, err, tt.wantCode)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ResolveFileCredential(%+v) unexpected error = %v", tt.ref, err)
				return
			}

			if got.Scheme != "file" {
				t.Errorf("ResolveFileCredential(%+v).Scheme = %q, want %q",
					tt.ref, got.Scheme, "file")
			}
			if got.Target != tt.ref.Target {
				t.Errorf("ResolveFileCredential(%+v).Target = %q, want %q",
					tt.ref, got.Target, tt.ref.Target)
			}
			if len(got.Fields) != tt.wantKeys {
				t.Errorf("ResolveFileCredential(%+v).Fields length = %d, want %d",
					tt.ref, len(got.Fields), tt.wantKeys)
			}
			for k, wantV := range tt.wantField {
				if got.Fields[k] != wantV {
					t.Errorf("ResolveFileCredential(%+v).Fields[%q] = %q, want %q",
						tt.ref, k, got.Fields[k], wantV)
				}
			}
		})
	}
}

func TestResolveFileCredentialHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tmpDir := t.TempDir()
	credFile := tmpDir + "/creds.toml"
	credContent := `api_token = "home-test-token"
`
	if err := os.WriteFile(credFile, []byte(credContent), 0644); err != nil {
		t.Fatalf("write test credential: %v", err)
	}

	// Build a ~-prefixed path from home + relative segment
	rel := "/creds.toml"
	relPath := "~" + rel
	// Write the file in the real home dir for this test
	realPath := home + rel
	if err := os.WriteFile(realPath, []byte(credContent), 0644); err != nil {
		t.Fatalf("write home credential: %v", err)
	}
	t.Cleanup(func() { os.Remove(realPath) })

	ref := CredentialRef{Scheme: "file", Target: relPath}

	got, err := ResolveFileCredential(ref)
	if err != nil {
		t.Fatalf("ResolveFileCredential(%q) unexpected error = %v", relPath, err)
	}
	if len(got.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(got.Fields))
	}
	if got.Fields["api_token"] != "home-test-token" {
		t.Errorf("api_token = %q, want %q", got.Fields["api_token"], "home-test-token")
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

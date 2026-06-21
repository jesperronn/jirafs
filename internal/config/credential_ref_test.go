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

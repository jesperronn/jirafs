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

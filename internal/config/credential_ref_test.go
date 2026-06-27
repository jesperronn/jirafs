package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCredentialRef(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantScheme string
		wantTarget string
		wantErr    bool
		wantCode   string
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
			name:       "op scheme with item only",
			input:      "op://JIRA_API_TOKEN_NINE_JRJ",
			wantScheme: "op",
			wantTarget: "JIRA_API_TOKEN_NINE_JRJ",
			wantErr:    false,
		},
		{
			name:       "op scheme with item field",
			input:      "op://JIRA_API_TOKEN_NINE_JRJ/email",
			wantScheme: "op",
			wantTarget: "JIRA_API_TOKEN_NINE_JRJ/email",
			wantErr:    false,
		},
		{
			name:     "unsupported scheme vault",
			input:    "vault://secret/jira",
			wantErr:  true,
			wantCode: ErrInvalidCredentialRef,
		},
		{
			name:     "unsupported scheme ssh",
			input:    "ssh://git@github.com/jirafs/jirafs.git",
			wantErr:  true,
			wantCode: ErrInvalidCredentialRef,
		},
		{
			name:     "empty string",
			input:    "",
			wantErr:  true,
			wantCode: ErrInvalidCredentialRef,
		},
		{
			name:     "no scheme separator",
			input:    "just-a-target",
			wantErr:  true,
			wantCode: ErrInvalidCredentialRef,
		},
		{
			name:     "empty scheme",
			input:    "://target",
			wantErr:  true,
			wantCode: ErrInvalidCredentialRef,
		},
		{
			name:     "empty target",
			input:    "env://",
			wantErr:  true,
			wantCode: ErrInvalidCredentialRef,
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
			name:     "unset env var returns error",
			ref:      CredentialRef{Scheme: "env", Target: "UNSET_VAR_DOES_NOT_EXIST"},
			wantErr:  true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:     "non-env scheme returns error",
			ref:      CredentialRef{Scheme: "file", Target: "/path/to/file"},
			wantErr:  true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:     "empty target returns error",
			ref:      CredentialRef{Scheme: "env", Target: ""},
			wantErr:  true,
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
			name:  "single op ref",
			input: []string{"op://JIRA_API_TOKEN_NINE_JRJ/email"},
			want: []CredentialRef{
				{Scheme: "op", Target: "JIRA_API_TOKEN_NINE_JRJ/email"},
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

func TestResolveFileCredential(t *testing.T) {
	// Create a temp TOML credential file.
	tmpDir := t.TempDir()
	credsFile := tmpDir + "/creds.toml"
	tomlContent := `
username = "fileuser"
password = "filepass"
api_token = "file-token-123"
port = 8080
ssl = true
`
	err := os.WriteFile(credsFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("setup: write temp file: %v", err)
	}

	// Create a second file with only one field.
	singleFile := tmpDir + "/single.toml"
	singleContent := `api_token = "single-token"
`
	err = os.WriteFile(singleFile, []byte(singleContent), 0644)
	if err != nil {
		t.Fatalf("setup: write single file: %v", err)
	}

	tests := []struct {
		name       string
		ref        CredentialRef
		wantErr    bool
		wantCode   string
		wantFields map[string]string
	}{
		{
			name:    "file scheme resolves fields",
			ref:     CredentialRef{Scheme: "file", Target: credsFile},
			wantErr: false,
			wantFields: map[string]string{
				"username":  "fileuser",
				"password":  "filepass",
				"api_token": "file-token-123",
				"port":      "8080",
				"ssl":       "true",
			},
		},
		{
			name:    "single field file resolves",
			ref:     CredentialRef{Scheme: "file", Target: singleFile},
			wantErr: false,
			wantFields: map[string]string{
				"api_token": "single-token",
			},
		},
		{
			name:     "non-existent file returns error",
			ref:      CredentialRef{Scheme: "file", Target: "/nonexistent/path/creds.toml"},
			wantErr:  true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:     "non-TOML file returns error",
			ref:      CredentialRef{Scheme: "file", Target: tmpDir + "/not-toml.txt"},
			wantErr:  true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:     "env scheme returns error",
			ref:      CredentialRef{Scheme: "env", Target: "SOME_VAR"},
			wantErr:  true,
			wantCode: ErrCredentialResolve,
		},
		{
			name:     "empty target returns error",
			ref:      CredentialRef{Scheme: "file", Target: ""},
			wantErr:  true,
			wantCode: ErrCredentialResolve,
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
			if len(got.Fields) != len(tt.wantFields) {
				t.Errorf("ResolveFileCredential(%+v).Fields length = %d, want %d",
					tt.ref, len(got.Fields), len(tt.wantFields))
				return
			}
			for k, wantVal := range tt.wantFields {
				if got.Fields[k] != wantVal {
					t.Errorf("ResolveFileCredential(%+v).Fields[%q] = %q, want %q",
						tt.ref, k, got.Fields[k], wantVal)
				}
			}
		})
	}
}

func TestResolveFileCredentialTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	// Create the credential file under the actual home directory
	// so the tilde expansion resolves correctly.
	credsDir := filepath.Join(home, ".jirafs-test-tilde")
	if err := os.MkdirAll(credsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	credsFile := filepath.Join(credsDir, "creds.toml")
	tomlContent := `api_token = "tilde-test-token"
`
	if err := os.WriteFile(credsFile, []byte(tomlContent), 0o644); err != nil {
		t.Fatalf("setup: write temp file: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(credsDir)
	})

	// Construct a tilde-prefixed path that resolves to the home dir file.
	// This exercises the tilde-expansion branch of ResolveFileCredential.
	tildeTarget := "~/.jirafs-test-tilde/creds.toml"
	ref := CredentialRef{Scheme: "file", Target: tildeTarget}

	got, err := ResolveFileCredential(ref)
	if err != nil {
		t.Fatalf("ResolveFileCredential(%+v) unexpected error = %v", ref, err)
	}
	if got.Scheme != "file" {
		t.Errorf("Scheme = %q, want %q", got.Scheme, "file")
	}
	if got.Target != tildeTarget {
		t.Errorf("Target = %q, want %q", got.Target, tildeTarget)
	}
	if got.Fields["api_token"] != "tilde-test-token" {
		t.Errorf("Fields[api_token] = %q, want %q", got.Fields["api_token"], "tilde-test-token")
	}
}

func TestResolveOPCredential(t *testing.T) {
	original := resolveOPCommand
	t.Cleanup(func() {
		resolveOPCommand = original
	})
	t.Setenv("JIRAFS_ALLOW_LIVE_ENDPOINTS", "1")

	tests := []struct {
		name      string
		ref       CredentialRef
		output    []byte
		runErr    error
		wantErr   bool
		wantCode  string
		wantField string
		wantValue string
		wantArgs  []string
	}{
		{
			name:      "default token field maps to api_token",
			ref:       CredentialRef{Scheme: "op", Target: "JIRA_API_TOKEN_NINE_JRJ"},
			output:    []byte("secret-token\n"),
			wantField: "api_token",
			wantValue: "secret-token",
			wantArgs:  []string{"item", "get", "JIRA_API_TOKEN_NINE_JRJ", "--fields", "label=token", "--reveal"},
		},
		{
			name:      "email field is preserved",
			ref:       CredentialRef{Scheme: "op", Target: "JIRA_API_TOKEN_NINE_JRJ/email"},
			output:    []byte("user@example.com\n"),
			wantField: "email",
			wantValue: "user@example.com",
			wantArgs:  []string{"item", "get", "JIRA_API_TOKEN_NINE_JRJ", "--fields", "label=email", "--reveal"},
		},
		{
			name:     "op cli error returns credential resolve",
			ref:      CredentialRef{Scheme: "op", Target: "JIRA_API_TOKEN_NINE_JRJ"},
			runErr:   os.ErrNotExist,
			wantErr:  true,
			wantCode: ErrCredentialResolve,
			wantArgs: []string{"item", "get", "JIRA_API_TOKEN_NINE_JRJ", "--fields", "label=token", "--reveal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ClearOPCredentialCache()
			var gotArgs []string
			resolveOPCommand = func(args ...string) ([]byte, error) {
				gotArgs = append([]string(nil), args...)
				return tt.output, tt.runErr
			}

			got, err := ResolveOPCredential(tt.ref)

			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("op args len = %d, want %d (%v)", len(gotArgs), len(tt.wantArgs), gotArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Fatalf("op args[%d] = %q, want %q", i, gotArgs[i], tt.wantArgs[i])
				}
			}

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveOPCredential(%+v) expected error, got nil", tt.ref)
				}
				if tt.wantCode != "" && !IsSettingError(err, tt.wantCode) {
					t.Fatalf("ResolveOPCredential(%+v) error = %v, want code %q", tt.ref, err, tt.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveOPCredential(%+v) unexpected error = %v", tt.ref, err)
			}
			if got.Fields[tt.wantField] != tt.wantValue {
				t.Fatalf("ResolveOPCredential(%+v) field %q = %q, want %q", tt.ref, tt.wantField, got.Fields[tt.wantField], tt.wantValue)
			}
			if tt.wantField == "api_token" && got.Fields["bearer_token"] != tt.wantValue {
				t.Fatalf("ResolveOPCredential(%+v) field %q = %q, want %q", tt.ref, "bearer_token", got.Fields["bearer_token"], tt.wantValue)
			}
		})
	}
}


// TestResolveOPCredential_Cache verifies that repeated resolution of the
// same op:// ref hits the cache instead of spawning the op CLI again.
// This is the property the sync loop depends on so a many-issue scan
// pays for credential resolution once.
func TestResolveOPCredential_Cache(t *testing.T) {
	original := resolveOPCommand
	t.Cleanup(func() {
		resolveOPCommand = original
		ClearOPCredentialCache()
	})
	t.Setenv("JIRAFS_ALLOW_LIVE_ENDPOINTS", "1")
	ClearOPCredentialCache()

	calls := 0
	resolveOPCommand = func(args ...string) ([]byte, error) {
		calls++
		return []byte("secret\n"), nil
	}

	ref := CredentialRef{Scheme: "op", Target: "JIRA_TOKEN_CACHE_TEST"}
	for i := 0; i < 5; i++ {
		cred, err := ResolveOPCredential(ref)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if cred.Fields["api_token"] != "secret" {
			t.Fatalf("iteration %d: api_token = %q, want %q", i, cred.Fields["api_token"], "secret")
		}
	}
	if calls != 1 {
		t.Fatalf("resolveOPCommand call count = %d, want 1 (cache should suppress repeats)", calls)
	}

	ClearOPCredentialCache()
	if _, err := ResolveOPCredential(ref); err != nil {
		t.Fatalf("after ClearOPCredentialCache: %v", err)
	}
	if calls != 2 {
		t.Fatalf("after clear, call count = %d, want 2", calls)
	}
}

func TestMergeCredentials(t *testing.T) {
	tests := []struct {
		name       string
		input      []ResolvedCredential
		wantScheme string
		wantTarget string
		wantFields map[string]string
	}{
		{
			name:       "empty slice returns empty fields",
			input:      []ResolvedCredential{},
			wantScheme: "",
			wantTarget: "",
			wantFields: map[string]string{},
		},
		{
			name:       "nil slice returns empty fields",
			input:      nil,
			wantScheme: "",
			wantTarget: "",
			wantFields: map[string]string{},
		},
		{
			name: "single credential passes through",
			input: []ResolvedCredential{
				{Scheme: "env", Target: "MY_TOKEN", Fields: map[string]string{"MY_TOKEN": "env-val"}},
			},
			wantScheme: "env",
			wantTarget: "MY_TOKEN",
			wantFields: map[string]string{"MY_TOKEN": "env-val"},
		},
		{
			name: "non-overlapping keys combine",
			input: []ResolvedCredential{
				{Scheme: "env", Target: "TOKEN", Fields: map[string]string{"TOKEN": "env-val"}},
				{Scheme: "file", Target: "creds.toml", Fields: map[string]string{"username": "from-file"}},
			},
			wantScheme: "file",
			wantTarget: "creds.toml",
			wantFields: map[string]string{
				"TOKEN":    "env-val",
				"username": "from-file",
			},
		},
		{
			name: "later source overrides earlier for same key",
			input: []ResolvedCredential{
				{Scheme: "env", Target: "TOKEN", Fields: map[string]string{"TOKEN": "env-val", "username": "envuser"}},
				{Scheme: "file", Target: "creds.toml", Fields: map[string]string{"TOKEN": "file-val", "password": "filepass"}},
			},
			wantScheme: "file",
			wantTarget: "creds.toml",
			wantFields: map[string]string{
				"TOKEN":    "file-val",
				"username": "envuser",
				"password": "filepass",
			},
		},
		{
			name: "three sources merge correctly",
			input: []ResolvedCredential{
				{Scheme: "env", Target: "A", Fields: map[string]string{"A": "1", "B": "1"}},
				{Scheme: "env", Target: "B", Fields: map[string]string{"B": "2", "C": "2"}},
				{Scheme: "file", Target: "f.toml", Fields: map[string]string{"C": "3"}},
			},
			wantScheme: "file",
			wantTarget: "f.toml",
			wantFields: map[string]string{
				"A": "1",
				"B": "2",
				"C": "3",
			},
		},
		{
			name: "empty fields in middle source does not erase",
			input: []ResolvedCredential{
				{Scheme: "env", Target: "A", Fields: map[string]string{"A": "1"}},
				{Scheme: "file", Target: "empty.toml", Fields: map[string]string{}},
				{Scheme: "env", Target: "B", Fields: map[string]string{"B": "2"}},
			},
			wantScheme: "env",
			wantTarget: "B",
			wantFields: map[string]string{
				"A": "1",
				"B": "2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeCredentials(tt.input)

			if got.Scheme != tt.wantScheme {
				t.Errorf("MergeCredentials().Scheme = %q, want %q", got.Scheme, tt.wantScheme)
			}
			if got.Target != tt.wantTarget {
				t.Errorf("MergeCredentials().Target = %q, want %q", got.Target, tt.wantTarget)
			}
			if len(got.Fields) != len(tt.wantFields) {
				t.Errorf("MergeCredentials().Fields length = %d, want %d", len(got.Fields), len(tt.wantFields))
				return
			}
			for k, wantVal := range tt.wantFields {
				if got.Fields[k] != wantVal {
					t.Errorf("MergeCredentials().Fields[%q] = %q, want %q", k, got.Fields[k], wantVal)
				}
			}
		})
	}
}

func TestValidateResolvedCredential(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		cred     ResolvedCredential
		wantErr  bool
		wantCode string
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
			name:     "atlassian_api_token with email and api_token passes",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{"email": "test@example.com", "api_token": "tok"}},
			wantErr:  false,
		},
		{
			name:     "atlassian_api_token missing email fails",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{"api_token": "tok"}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "atlassian_api_token missing api_token fails",
			authType: "atlassian_api_token",
			cred:     ResolvedCredential{Fields: map[string]string{"email": "test@example.com"}},
			wantErr:  true,
			wantCode: ErrMissingAuthField,
		},
		{
			name:     "bearer_token with bearer_token passes",
			authType: "bearer_token",
			cred:     ResolvedCredential{Fields: map[string]string{"bearer_token": "tok"}},
			wantErr:  false,
		},
		{
			name:     "bearer_token missing bearer_token fails",
			authType: "bearer_token",
			cred:     ResolvedCredential{Fields: map[string]string{"api_token": "tok"}},
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
			cred:     ResolvedCredential{Fields: map[string]string{"email": "test@example.com", "api_token": "tok", "extra": "val"}},
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

func TestResolveInstanceCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credsFile := filepath.Join(tmpDir, "creds.toml")
	if err := os.WriteFile(credsFile, []byte("email = \"user@example.com\"\napi_token = \"file-token\"\n"), 0o644); err != nil {
		t.Fatalf("setup: write creds file: %v", err)
	}

	tests := []struct {
		name         string
		settings     *Settings
		instanceName string
		wantErr      bool
		wantCode     string
		wantBaseURL  string
		wantAuthType string
		wantFields   map[string]string
	}{
		{
			name: "resolves and merges instance credentials",
			settings: &Settings{
				Instances: map[string]Instance{
					"work": {
						BaseURL:  "https://jira.example.com",
						AuthType: "atlassian_api_token",
						CredentialRefs: []string{
							"file://" + credsFile,
						},
					},
				},
			},
			instanceName: "work",
			wantBaseURL:  "https://jira.example.com",
			wantAuthType: "atlassian_api_token",
			wantFields: map[string]string{
				"email":     "user@example.com",
				"api_token": "file-token",
			},
		},
		{
			name: "missing instance returns no usable instance",
			settings: &Settings{
				Instances: map[string]Instance{},
			},
			instanceName: "missing",
			wantErr:      true,
			wantCode:     ErrNoUsableInstance,
		},
		{
			name: "missing credential refs returns no usable instance",
			settings: &Settings{
				Instances: map[string]Instance{
					"work": {
						BaseURL:  "https://jira.example.com",
						AuthType: "atlassian_api_token",
					},
				},
			},
			instanceName: "work",
			wantErr:      true,
			wantCode:     ErrNoUsableInstance,
		},
		{
			name: "invalid credential ref returns parse error",
			settings: &Settings{
				Instances: map[string]Instance{
					"work": {
						BaseURL:  "https://jira.example.com",
						AuthType: "atlassian_api_token",
						CredentialRefs: []string{
							"vault://bad",
						},
					},
				},
			},
			instanceName: "work",
			wantErr:      true,
			wantCode:     ErrInvalidCredentialRef,
		},
		{
			name: "missing required auth field returns validation error",
			settings: &Settings{
				Instances: map[string]Instance{
					"work": {
						BaseURL:  "https://jira.example.com",
						AuthType: "basic",
						CredentialRefs: []string{
							"file://" + credsFile,
						},
					},
				},
			},
			instanceName: "work",
			wantErr:      true,
			wantCode:     ErrMissingAuthField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.settings.ResolveInstanceCredentials(tt.instanceName)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveInstanceCredentials(%q) expected error, got nil", tt.instanceName)
				}
				if tt.wantCode != "" && !IsSettingError(err, tt.wantCode) {
					t.Fatalf("ResolveInstanceCredentials(%q) error = %v, want code %q", tt.instanceName, err, tt.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveInstanceCredentials(%q) unexpected error = %v", tt.instanceName, err)
			}
			if got.BaseURL != tt.wantBaseURL {
				t.Fatalf("BaseURL = %q, want %q", got.BaseURL, tt.wantBaseURL)
			}
			if got.AuthType != tt.wantAuthType {
				t.Fatalf("AuthType = %q, want %q", got.AuthType, tt.wantAuthType)
			}
			if len(got.Credential.Fields) != len(tt.wantFields) {
				t.Fatalf("credential field count = %d, want %d", len(got.Credential.Fields), len(tt.wantFields))
			}
			for key, want := range tt.wantFields {
				if got.Credential.Fields[key] != want {
					t.Fatalf("credential field %q = %q, want %q", key, got.Credential.Fields[key], want)
				}
			}
		})
	}
}

func TestInstanceCredentialsForPath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	jirafsDir := filepath.Join(homeDir, settingsDir)
	if err := os.MkdirAll(jirafsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)

	credsFile := filepath.Join(jirafsDir, "creds.toml")
	credsContent := `username = "pathuser"
password = "pathpass"
`
	if err := os.WriteFile(credsFile, []byte(credsContent), 0o644); err != nil {
		t.Fatalf("setup: write creds file: %v", err)
	}

	instBCredsFile := filepath.Join(jirafsDir, "instb.toml")
	instBCredsContent := `email = "instb@example.com"
api_token = "instb-api-token"
`
	if err := os.WriteFile(instBCredsFile, []byte(instBCredsContent), 0o644); err != nil {
		t.Fatalf("setup: write instb creds file: %v", err)
	}

	t.Setenv("PATH_TOKEN", "path-token-val")

	mirrorA := filepath.Join(jirafsDir, "jira", "projA")
	mirrorB := filepath.Join(jirafsDir, "jira", "projB")
	localB := filepath.Join(tmpDir, "work", "projB")
	for _, d := range []string{mirrorA, mirrorB, localB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	settings := `version = 1

[instances.instA]
base_url = "https://a.example.com"
auth_type = "basic"
credential_refs = [
  "file://` + credsFile + `",
  "env://PATH_TOKEN",
]

[instances.instB]
base_url = "https://b.example.com"
auth_type = "atlassian_api_token"
credential_refs = [
  "file://` + instBCredsFile + `",
]

[projects.projA]
key = "PA"
instance = "instA"
mirror_dir = "` + mirrorA + `"

[projects.projB]
key = "PB"
instance = "instB"
mirror_dir = "` + mirrorB + `"
local_dirs = ["` + localB + `"]
`
	if err := os.WriteFile(filepath.Join(jirafsDir, settingsFile), []byte(settings), 0o644); err != nil {
		t.Fatalf("setup: write settings: %v", err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantErr    bool
		wantCode   string
		wantFields int
	}{
		{
			name:       "mirror_dir match returns instance credentials",
			path:       filepath.Join(mirrorA, "sub", "deep"),
			wantErr:    false,
			wantFields: 3,
		},
		{
			name:       "mirror_dir exact match",
			path:       mirrorA,
			wantErr:    false,
			wantFields: 3,
		},
		{
			name:       "local_dirs match returns instance credentials",
			path:       filepath.Join(localB, "src"),
			wantErr:    false,
			wantCode:   "",
			wantFields: 2,
		},
		{
			name:     "no matching project returns ErrNoUsableInstance",
			path:     filepath.Join(tmpDir, "nowhere"),
			wantErr:  true,
			wantCode: ErrNoUsableInstance,
		},
		{
			name:    "empty path returns error",
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.InstanceCredentialsForPath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("InstanceCredentialsForPath(%q) expected error, got nil", tt.path)
					return
				}
				if tt.wantCode != "" {
					if !IsSettingError(err, tt.wantCode) {
						t.Errorf("InstanceCredentialsForPath(%q) error code = %v, want %v",
							tt.path, err, tt.wantCode)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("InstanceCredentialsForPath(%q) unexpected error = %v", tt.path, err)
				return
			}

			if len(got.Credential.Fields) != tt.wantFields {
				t.Errorf("InstanceCredentialsForPath(%q).Credential.Fields length = %d, want %d",
					tt.path, len(got.Credential.Fields), tt.wantFields)
			}
		})
	}
}

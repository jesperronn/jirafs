package cli

import (
	stdctx "context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/context"
)

// TestNewJiraClientRefusesNonLoopback proves a client built by the cli
// factory cannot reach a non-loopback host from tests.
func TestNewJiraClientRefusesNonLoopback(t *testing.T) {
	credsPath := filepath.Join(t.TempDir(), "creds.toml")
	if err := os.WriteFile(credsPath, []byte("email = \"u@example.com\"\napi_token = \"t\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := &config.Settings{Instances: map[string]config.Instance{"default": {
		BaseURL: "https://example.com", AuthType: "atlassian_api_token",
		CredentialRefs: []string{"file://" + credsPath},
	}}}
	client, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchIssue(stdctx.Background(), "ABC-1")
	if err == nil || !strings.Contains(err.Error(), "netguard") {
		t.Fatalf("want netguard error, got %v", err)
	}
}

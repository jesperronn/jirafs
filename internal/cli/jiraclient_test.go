package cli

import (
	stdctx "context"
	"fmt"
	"net/http"
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

func TestJiraAdapterFactorySelection(t *testing.T) {
	credsPath := filepath.Join(t.TempDir(), "creds.toml")
	if err := os.WriteFile(credsPath, []byte("bearer_token = \"offline-token\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := &config.Settings{Instances: map[string]config.Instance{"default": {
		BaseURL: "https://jira.example.test", AuthType: "bearer_token",
		CredentialRefs: []string{"file://" + credsPath},
	}}}
	for _, tt := range []struct{ selector, want string }{
		{"", "*jira.JiraClient"}, {"legacy", "*jira.JiraClient"}, {"jira-connect", "*jiraconnect.Client"},
	} {
		t.Run(tt.selector, func(t *testing.T) {
			t.Setenv("JIRAFS_JIRA_ADAPTER", tt.selector)
			client, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%T", client); got != tt.want {
				t.Fatalf("client=%s, want %s", got, tt.want)
			}
		})
	}
}

func TestJiraAdapterInvalidSelectorPrecedesCredentialResolution(t *testing.T) {
	t.Setenv("JIRAFS_JIRA_ADAPTER", "unknown")
	settings := &config.Settings{Instances: map[string]config.Instance{"default": {
		BaseURL: "https://jira.example.test", AuthType: "bearer_token",
		CredentialRefs: []string{"file:///missing/offline-credentials.toml"},
	}}}
	_, err := newJiraClient(settings, &context.Context{Instance: "default"}, ".")
	if err == nil || !strings.Contains(err.Error(), "JIRAFS_JIRA_ADAPTER") {
		t.Fatalf("want adapter selector error before credentials, got %v", err)
	}
}

func TestJiraAdapterPreservesResolvedCredentialsAndFieldProjection(t *testing.T) {
	t.Setenv("JIRAFS_JIRA_ADAPTER", "jira-connect")
	t.Setenv("JIRAFS_EPIC_LINK_FIELD", "")
	credsPath := filepath.Join(t.TempDir(), "creds.toml")
	if err := os.WriteFile(credsPath, []byte("bearer_token = \"offline-token\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := &config.Settings{Instances: map[string]config.Instance{"default": {
		BaseURL: "https://jira.example.test/jira", AuthType: "bearer_token", CredentialRefs: []string{"file://" + credsPath},
	}}, Projects: map[string]config.Project{"test": {EpicLinkField: "customfield_10", CustomFields: []string{"customfield_11"}}}}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls := 0
	http.DefaultTransport = adapterHTTP(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() == "" || req.URL.Host != "jira.example.test" || req.URL.Path != "/jira/rest/api/2/issue/ABC-1" {
			t.Fatalf("unexpected request: %s", req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer offline-token" {
			t.Fatal("consumer credentials not retained")
		}
		fields := req.URL.Query().Get("fields")
		for _, field := range []string{"customfield_10", "customfield_11"} {
			if !strings.Contains(fields, field) {
				t.Fatalf("field projection omitted %s: %s", field, fields)
			}
		}
		return adapterResponse(200, `{"id":"1","key":"ABC-1","fields":{"summary":"offline","issuetype":{"name":"Story"},"project":{"key":"ABC"},"customfield_10":"ABC-9","customfield_11":"custom"}}`), nil
	})
	client, err := newJiraClient(settings, &context.Context{Instance: "default", Name: "test"}, ".")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := client.FetchIssue(stdctx.Background(), "ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || issue.CustomFields["customfield_11"] != "custom" {
		t.Fatalf("fields=%v calls=%d", issue.CustomFields, calls)
	}
}

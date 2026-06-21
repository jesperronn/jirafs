package jira

import (
	"context"
	"errors"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestFakeTransportFetchIssue(t *testing.T) {
	fake := NewFakeTransport()

	issue := &schema.Issue{
		Identity: schema.IssueIdentity{
			Key: "PROJ-1",
		},
	}
	fake.SetIssue("PROJ-1", issue)

	got, err := fake.FetchIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil issue")
	}
	if got.Identity.Key != "PROJ-1" {
		t.Errorf("got key %q, want %q", got.Identity.Key, "PROJ-1")
	}
}

func TestFakeTransportFetchIssueNotFound(t *testing.T) {
	fake := NewFakeTransport()

	_, err := fake.FetchIssue(context.Background(), "PROJ-999")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFakeTransportFetchIssueWithError(t *testing.T) {
	fake := NewFakeTransport()
	wantErr := errors.New("connection refused")
	fake.SetErr("fetch", wantErr)

	_, err := fake.FetchIssue(context.Background(), "PROJ-1")
	if err != wantErr {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

func TestFakeTransportSearchIssues(t *testing.T) {
	fake := NewFakeTransport()

	issues := []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "PROJ-1"}},
		{Identity: schema.IssueIdentity{Key: "PROJ-2"}},
	}
	fake.SetIssuesByScope("my-issues", issues)

	got, err := fake.SearchIssues(context.Background(), "my-issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(got))
	}
	if got[0].Identity.Key != "PROJ-1" || got[1].Identity.Key != "PROJ-2" {
		t.Errorf("unexpected keys: %s, %s", got[0].Identity.Key, got[1].Identity.Key)
	}
}

func TestFakeTransportSearchIssuesNotFound(t *testing.T) {
	fake := NewFakeTransport()

	_, err := fake.SearchIssues(context.Background(), "unknown-scope")
	if !IsClientError(err, ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestFakeTransportSearchIssuesWithError(t *testing.T) {
	fake := NewFakeTransport()
	wantErr := errors.New("timeout")
	fake.SetErr("search", wantErr)

	_, err := fake.SearchIssues(context.Background(), "my-issues")
	if err != wantErr {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

func TestFakeTransportImplementsClient(t *testing.T) {
	var _ Client = (*FakeTransport)(nil)
}

func TestFakeTransportSearchMyIssuesDeterministic(t *testing.T) {
	fake := NewFakeTransport()

	// First call should return deterministic keys.
	got, err := fake.SearchIssues(context.Background(), "my-issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(got))
	}
	wantKeys := []string{"PROJ-1", "PROJ-2", "PROJ-3"}
	wantTypes := []string{"Story", "Bug", "Task"}
	for i, g := range got {
		if string(g.Identity.Key) != wantKeys[i] {
			t.Errorf("issue %d key = %q, want %q", i, g.Identity.Key, wantKeys[i])
		}
		if string(g.Identity.Type) != wantTypes[i] {
			t.Errorf("issue %d type = %q, want %q", i, g.Identity.Type, wantTypes[i])
		}
	}

	// Second call should return the same deterministic keys.
	got2, err := fake.SearchIssues(context.Background(), "my-issues")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if len(got2) != 3 {
		t.Fatalf("expected 3 issues on second call, got %d", len(got2))
	}
	for i := range got {
		if string(got[i].Identity.Key) != string(got2[i].Identity.Key) {
			t.Errorf("key changed between calls: %q vs %q", got[i].Identity.Key, got2[i].Identity.Key)
		}
	}
}

func TestFakeTransportSearchMyIssuesOverride(t *testing.T) {
	fake := NewFakeTransport()

	// Explicitly set issues for my-issues.
	customIssues := []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "PROJ-99", Type: "Story"}},
	}
	fake.SetIssuesByScope("my-issues", customIssues)

	// Should return the explicit issues, not the deterministic ones.
	got, err := fake.SearchIssues(context.Background(), "my-issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(got))
	}
	if string(got[0].Identity.Key) != "PROJ-99" {
		t.Errorf("key = %q, want %q", got[0].Identity.Key, "PROJ-99")
	}
}

func TestFakeTransportClearsErrorAfterRead(t *testing.T) {
	fake := NewFakeTransport()
	wantErr := errors.New("one-shot error")
	fake.SetErr("fetch", wantErr)

	_, err := fake.FetchIssue(context.Background(), "PROJ-1")
	if err != wantErr {
		t.Fatalf("first call: expected %v, got %v", wantErr, err)
	}

	// After the error is returned, subsequent calls should use normal behavior
	// (not return the same error again unless re-set)
	fake.SetErr("fetch", nil)
	issue := &schema.Issue{Identity: schema.IssueIdentity{Key: "PROJ-1"}}
	fake.SetIssue("PROJ-1", issue)

	got, err := fake.FetchIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil issue on second call")
	}
}

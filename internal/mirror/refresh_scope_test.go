package mirror

import (
	"context"
	"errors"
	"testing"

	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

func TestRefreshScope_success(t *testing.T) {
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "PROJ-1", Type: "Story"}},
		{Identity: schema.IssueIdentity{Key: "PROJ-2", Type: "Bug"}},
		{Identity: schema.IssueIdentity{Key: "PROJ-3", Type: "Task"}},
	})

	mirror := &Mirror{}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}
	ctx := context.Background()

	added, err := RefreshScope(ctx, fake, scope, mirror)
	if err != nil {
		t.Fatalf("RefreshScope: %v", err)
	}

	if len(added) != 3 {
		t.Errorf("expected 3 added keys, got %d", len(added))
	}

	for _, key := range []schema.IssueKey{"PROJ-1", "PROJ-2", "PROJ-3"} {
		if !mirror.HasScopeMember(key) {
			t.Errorf("expected %s to be a scope member", key)
		}
		if got := mirror.ScopeMemberFor(key); got != "my-issues" {
			t.Errorf("ScopeMemberFor(%s) = %q, want %q", key, got, "my-issues")
		}
	}
}

func TestRefreshScope_empty_scope(t *testing.T) {
	fake := jira.NewFakeTransport()
	var mirror Mirror

	added, err := RefreshScope(context.Background(), fake, Scope{}, &mirror)
	if err != nil {
		t.Fatalf("RefreshScope(zero scope): %v", err)
	}
	if added != nil {
		t.Errorf("expected nil added, got %v", added)
	}
}

func TestRefreshScope_search_error(t *testing.T) {
	fake := jira.NewFakeTransport()
	fake.SetErr("search", jira.NewHTTPErr(500, "server error"))

	mirror := &Mirror{}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}

	added, err := RefreshScope(context.Background(), fake, scope, mirror)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !jira.HasCode(err, jira.ErrHTTP) {
		t.Errorf("expected HTTP error, got %v", err)
	}
	if added != nil {
		t.Errorf("expected nil added on error, got %v", added)
	}
	if len(mirror.ScopeMembers) != 0 {
		t.Errorf("expected no scope members on error, got %d", len(mirror.ScopeMembers))
	}
}

func TestRefreshScope_no_scope_match(t *testing.T) {
	fake := jira.NewFakeTransport()

	mirror := &Mirror{}
	scope := Scope{Name: "nonexistent", Type: ScopeTypeJQL, Target: "status=NonExistent"}

	added, err := RefreshScope(context.Background(), fake, scope, mirror)
	if err == nil {
		t.Fatal("expected not_found error, got nil")
	}
	if !jira.HasCode(err, jira.ErrNotFound) {
		t.Errorf("expected not_found error, got %v", err)
	}
	if added != nil {
		t.Errorf("expected nil added on error, got %v", added)
	}
}

func TestRefreshScope_dedup_existing_member(t *testing.T) {
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "PROJ-1", Type: "Story"}},
		{Identity: schema.IssueIdentity{Key: "PROJ-2", Type: "Bug"}},
	})

	mirror := &Mirror{
		ScopeMembers: []ScopeMember{
			{Key: "PROJ-1", Scope: "my-issues"},
		},
	}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}
	ctx := context.Background()

	added, err := RefreshScope(ctx, fake, scope, mirror)
	if err != nil {
		t.Fatalf("RefreshScope: %v", err)
	}

	// Only PROJ-2 should be added (PROJ-1 already a member)
	if len(added) != 1 {
		t.Errorf("expected 1 added key, got %d", len(added))
	}
	if len(added) > 0 && added[0] != "PROJ-2" {
		t.Errorf("expected added[0] = PROJ-2, got %s", added[0])
	}
}

func TestRefreshScope_shallow_linked_issues(t *testing.T) {
	fake := jira.NewFakeTransport()
	issue := &schema.Issue{
		Identity: schema.IssueIdentity{Key: "PROJ-1", Type: "Story"},
		LinkedIssues: []schema.LinkedIssue{
			{Key: "PROJ-2", Type: "blocks"},
			{Key: "PROJ-3", Type: "relates to"},
		},
	}
	fake.SetIssuesByScope("my-issues", []*schema.Issue{issue})

	mirror := &Mirror{}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}
	ctx := context.Background()

	added, err := RefreshScope(ctx, fake, scope, mirror)
	if err != nil {
		t.Fatalf("RefreshScope: %v", err)
	}

	if len(added) != 1 {
		t.Fatalf("expected 1 added key, got %d", len(added))
	}

	// The linked issues should still be shallow (key + type only).
	// Verify the issue was not fully resolved — only the scope member was added.
	if !mirror.HasScopeMember("PROJ-1") {
		t.Error("PROJ-1 should be a scope member")
	}
}

func TestRefreshScope_nil_issue_in_list(t *testing.T) {
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		nil,
		{Identity: schema.IssueIdentity{Key: "PROJ-1", Type: "Story"}},
		nil,
	})

	mirror := &Mirror{}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}
	ctx := context.Background()

	added, err := RefreshScope(ctx, fake, scope, mirror)
	if err != nil {
		t.Fatalf("RefreshScope: %v", err)
	}

	if len(added) != 1 {
		t.Errorf("expected 1 added key (skipping nils), got %d", len(added))
	}
	if len(added) > 0 && added[0] != "PROJ-1" {
		t.Errorf("expected added[0] = PROJ-1, got %s", added[0])
	}
}

func TestRefreshScope_empty_key_issue(t *testing.T) {
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("my-issues", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "", Type: "Story"}},
		{Identity: schema.IssueIdentity{Key: "PROJ-1", Type: "Bug"}},
	})

	mirror := &Mirror{}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}
	ctx := context.Background()

	added, err := RefreshScope(ctx, fake, scope, mirror)
	if err != nil {
		t.Fatalf("RefreshScope: %v", err)
	}

	if len(added) != 1 {
		t.Errorf("expected 1 added key (skipping empty key), got %d", len(added))
	}
	if len(added) > 0 && added[0] != "PROJ-1" {
		t.Errorf("expected added[0] = PROJ-1, got %s", added[0])
	}
}

func TestRefreshScope_currentSprint(t *testing.T) {
	fake := jira.NewFakeTransport()
	fake.SetIssuesByScope("current-sprint", []*schema.Issue{
		{Identity: schema.IssueIdentity{Key: "PROJ-4", Type: "Story"}},
		{Identity: schema.IssueIdentity{Key: "PROJ-5", Type: "Story"}},
	})

	mirror := &Mirror{}
	scope := Scope{Name: "current-sprint", Type: ScopeTypeJQL, Target: "sprint in openSprints()"}
	ctx := context.Background()

	added, err := RefreshScope(ctx, fake, scope, mirror)
	if err != nil {
		t.Fatalf("RefreshScope: %v", err)
	}

	if len(added) != 2 {
		t.Errorf("expected 2 added keys, got %d", len(added))
	}
	for _, key := range []schema.IssueKey{"PROJ-4", "PROJ-5"} {
		if !mirror.HasScopeMember(key) {
			t.Errorf("expected %s to be a scope member", key)
		}
		if got := mirror.ScopeMemberFor(key); got != "current-sprint" {
			t.Errorf("ScopeMemberFor(%s) = %q, want %q", key, got, "current-sprint")
		}
	}
}

func TestRefreshScope_error_type_assertion(t *testing.T) {
	fake := jira.NewFakeTransport()
	expectedErr := errors.New("transport failure")
	fake.SetErr("search", expectedErr)

	mirror := &Mirror{}
	scope := Scope{Name: "my-issues", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}

	_, err := RefreshScope(context.Background(), fake, scope, mirror)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

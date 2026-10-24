package mirror

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
			{Key: "PROJ-2", Relation: "blocks"},
			{Key: "PROJ-3", Relation: "relates to"},
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

func TestRefreshScope_sendsJQLPerScopeType(t *testing.T) {
	tests := []struct {
		name  string
		scope Scope
		want  string
	}{
		{"project", Scope{Name: "plat", Type: ScopeTypeProject, Target: "PLAT"}, `project = "PLAT" ORDER BY key`},
		{"jql verbatim", Scope{Name: "urgent", Type: ScopeTypeJQL, Target: "priority = Highest ORDER BY created"}, "priority = Highest ORDER BY created"},
		{"component", Scope{Name: "api", Type: ScopeTypeComponent, Target: "API", Project: "PLAT"}, `project = "PLAT" AND component = "API" ORDER BY key`},
		{"quotes escaped", Scope{Name: "q", Type: ScopeTypeProject, Target: `A"B`}, `project = "A\"B" ORDER BY key`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := jira.NewFakeTransport()
			fake.SetIssuesByScope(tt.want, []*schema.Issue{})
			if _, err := RefreshScope(context.Background(), fake, tt.scope, &Mirror{}); err != nil {
				t.Fatalf("RefreshScope: %v", err)
			}
			got := fake.Searches()
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("searches = %q, want [%q]", got, tt.want)
			}
		})
	}
}

func TestRefreshScope_componentWithoutProjectIsValidationError(t *testing.T) {
	fake := jira.NewFakeTransport()
	scope := Scope{Name: "api", Type: ScopeTypeComponent, Target: "API"}
	if _, err := RefreshScope(context.Background(), fake, scope, &Mirror{}); err == nil {
		t.Fatal("expected validation error for component scope without project")
	}
	if got := fake.Searches(); len(got) != 0 {
		t.Fatalf("searches = %q, want none", got)
	}
}

func TestScopeJQL_errors(t *testing.T) {
	for _, s := range []Scope{
		{Name: "a", Type: ScopeTypeProject},
		{Name: "b", Type: "bogus", Target: "x"},
	} {
		if _, err := s.JQL(); err == nil {
			t.Errorf("JQL(%+v) = nil error, want error", s)
		}
	}
}

func TestBuiltinScope(t *testing.T) {
	for name, want := range map[string]string{"my-issues": "assignee = currentUser()", "current-sprint": "sprint in openSprints()"} {
		s, ok := BuiltinScope(name)
		if !ok {
			t.Fatalf("BuiltinScope(%q) not found", name)
		}
		if got, err := s.JQL(); err != nil || got != want {
			t.Errorf("JQL(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"backlog", "next-sprint", "other"} {
		if _, ok := BuiltinScope(name); ok {
			t.Errorf("BuiltinScope(%q) should not exist", name)
		}
	}
}

func TestScopeSearchJQL(t *testing.T) {
	cursor := time.Date(2026, 3, 4, 12, 5, 0, 0, time.UTC)
	since := cursor.Add(-cursorOverlap).UTC().Format("2006/01/02 15:04")
	stamp := cursor.Format(time.RFC3339)
	tests := []struct {
		name  string
		scope Scope
		want  string
		fail  bool
	}{
		{"no cursor", Scope{Name: "a", Type: ScopeTypeJQL, Target: "assignee = currentUser()"}, "assignee = currentUser()", false},
		{"bad cursor ignored", Scope{Name: "a", Type: ScopeTypeJQL, Target: "x = 1", LastSynced: "yesterday"}, "x = 1", false},
		{"jql cursor", Scope{Name: "a", Type: ScopeTypeJQL, Target: "x = 1", LastSynced: stamp}, `(x = 1) AND updated >= "` + since + `"`, false},
		{"order by kept outside", Scope{Name: "a", Type: ScopeTypeProject, Target: "PLAT", LastSynced: stamp}, `(project = "PLAT") AND updated >= "` + since + `" ORDER BY key`, false},
		{"lowercase order by", Scope{Name: "a", Type: ScopeTypeJQL, Target: "x = 1 order by created", LastSynced: stamp}, `(x = 1) AND updated >= "` + since + `" order by created`, false},
		{"invalid scope", Scope{Name: "a", Type: ScopeTypeJQL, Target: " ", LastSynced: stamp}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.scope.SearchJQL()
			if (err != nil) != tt.fail {
				t.Fatalf("err = %v, fail = %v", err, tt.fail)
			}
			if got != tt.want {
				t.Errorf("SearchJQL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetScopeLastSynced(t *testing.T) {
	ts := time.Date(2026, 3, 4, 12, 5, 0, 0, time.UTC)
	m := &Mirror{Scopes: []Scope{{Name: "a", Type: ScopeTypeJQL, Target: "x = 1"}}}
	m.SetScopeLastSynced(m.Scopes[0], ts)
	if got := m.Scopes[0].LastSynced; got != "2026-03-04T12:05:00Z" {
		t.Errorf("existing scope LastSynced = %q", got)
	}
	m.SetScopeLastSynced(Scope{Name: "b", Type: ScopeTypeJQL, Target: "y = 2"}, ts)
	if len(m.Scopes) != 2 || m.Scopes[1].LastSynced == "" {
		t.Errorf("new scope not appended: %+v", m.Scopes)
	}
	if c, ok := m.Scopes[1].Cursor(); !ok || !c.Equal(ts) {
		t.Errorf("Cursor = %v, %v", c, ok)
	}
}

func TestSearchJQLWindowCoversAllTimezones(t *testing.T) {
	old := time.Local
	t.Cleanup(func() { time.Local = old })
	cursor := time.Date(2026, 3, 4, 12, 5, 30, 0, time.UTC)
	scope := Scope{Name: "a", Type: ScopeTypeJQL, Target: "x = 1", LastSynced: cursor.Format(time.RFC3339)}
	for machine := -12; machine <= 14; machine++ {
		time.Local = time.FixedZone("machine", machine*3600)
		jql, err := scope.SearchJQL()
		if err != nil {
			t.Fatal(err)
		}
		lit := jql[strings.Index(jql, `"`)+1 : strings.LastIndex(jql, `"`)]
		for jiraZone := -12; jiraZone <= 14; jiraZone++ {
			loc := time.FixedZone("jira", jiraZone*3600)
			start, err := time.ParseInLocation(jqlTimeLayout, lit, loc)
			if err != nil {
				t.Fatal(err)
			}
			if start.After(cursor) {
				t.Errorf("machine UTC%+d jira UTC%+d: window starts %v after cursor %v", machine, jiraZone, start, cursor)
			}
		}
	}
	t.Logf("checked 27x27 machine/jira zone pairs")
}

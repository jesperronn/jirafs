package jira

import (
	"context"
	"sync"

	"github.com/jirafs/jirafs/internal/schema"
)

// FakeTransport is a fake Jira client for testing without a real network.
// Callers set Issues and Err before invoking FetchIssue or SearchIssues.
type FakeTransport struct {
	mu      sync.RWMutex
	issues  map[string]*schema.Issue // key -> issue
	err     error                    // returned when ErrOn is set
	errOn   string                   // "fetch" or "search"
	issuesByScope map[string][]*schema.Issue // scope -> issues
}

// NewFakeTransport creates a fake transport with empty state.
func NewFakeTransport() *FakeTransport {
	return &FakeTransport{
		issues:        make(map[string]*schema.Issue),
		issuesByScope: make(map[string][]*schema.Issue),
	}
}

// SetIssue registers a single issue keyed by its key.
func (f *FakeTransport) SetIssue(key string, issue *schema.Issue) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issues[key] = issue
}

// SetIssuesByScope registers issues for a search scope.
func (f *FakeTransport) SetIssuesByScope(scope string, issues []*schema.Issue) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issuesByScope[scope] = issues
}

// SetErr configures the transport to return an error on the next call
// to "fetch" or "search". Pass an empty string to clear.
func (f *FakeTransport) SetErr(on string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if on == "" || err == nil {
		f.errOn = ""
		f.err = nil
		return
	}
	f.errOn = on
	f.err = err
}

// FetchIssue returns the issue registered for the given key, or the
// configured error if ErrOn == "fetch".
func (f *FakeTransport) FetchIssue(_ context.Context, key string) (*schema.Issue, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.errOn == "fetch" {
		return nil, f.err
	}
	issue := f.issues[key]
	if issue == nil {
		return nil, NewNotFoundError(key)
	}
	return issue, nil
}

// SearchIssues returns the issues registered for the given scope, or the
// configured error if ErrOn == "search". When no explicit issues are set
// for the scope, it falls back to GenerateScopeIssues for known scopes.
func (f *FakeTransport) SearchIssues(_ context.Context, scope string) ([]*schema.Issue, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.errOn == "search" {
		return nil, f.err
	}
	issues := f.issuesByScope[scope]
	if issues == nil {
		issues = generateScopeIssues(scope)
		if issues == nil {
			return nil, NewNotFoundError("scope:" + scope)
		}
	}
	return issues, nil
}

// generateScopeIssues returns deterministic issues for known scopes.
// It is not thread-safe and must be called outside of locks.
func generateScopeIssues(scope string) []*schema.Issue {
	switch scope {
	case "my-issues":
		return []*schema.Issue{
			{Identity: schema.IssueIdentity{Key: "PROJ-1", Type: "Story"}},
			{Identity: schema.IssueIdentity{Key: "PROJ-2", Type: "Bug"}},
			{Identity: schema.IssueIdentity{Key: "PROJ-3", Type: "Task"}},
		}
	case "current-sprint":
		return []*schema.Issue{
			{Identity: schema.IssueIdentity{Key: "PROJ-4", Type: "Story"}},
			{Identity: schema.IssueIdentity{Key: "PROJ-5", Type: "Story"}},
		}
	default:
		return nil
	}
}

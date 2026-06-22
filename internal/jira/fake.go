package jira

import (
	"context"
	"sync"

	"github.com/jirafs/jirafs/internal/schema"
)

// FakeTransport is a fake Jira client for testing without a real network.
// Callers set Issues and Err before invoking FetchIssue or SearchIssues.
type FakeTransport struct {
	mu            sync.RWMutex
	issues        map[string]*schema.Issue // key -> issue
	currentUser   *User                    // set by SetCurrentUser
	err           error                    // returned when ErrOn is set
	errOn         string                   // "fetch", "search", or "user"
	issuesByScope map[string][]*schema.Issue // scope -> issues
	// Registry data for registry refresh tests.
	statuses      []StatusEntry
	sprints       map[string][]SprintEntry // projectKey -> sprints
	fixVersions   map[string][]FixVersionEntry // projectKey -> versions
}

// NewFakeTransport creates a fake transport with empty state.
func NewFakeTransport() *FakeTransport {
	return &FakeTransport{
		issues:        make(map[string]*schema.Issue),
		issuesByScope: make(map[string][]*schema.Issue),
		sprints:       make(map[string][]SprintEntry),
		fixVersions:   make(map[string][]FixVersionEntry),
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
// to "fetch", "search", "user", or "update". Pass an empty string to clear.
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

// SetCurrentUser registers a user that CurrentUser will return.
func (f *FakeTransport) SetCurrentUser(u *User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.currentUser = u
}

// CurrentUser returns the registered current user, or the configured
// error if ErrOn == "user". When no user is registered it returns a
// default user derived from the first registered issue's assignee, or a
// synthetic user with name "jirafs-test" when no issues exist.
func (f *FakeTransport) CurrentUser(_ context.Context) (*User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.errOn == "user" {
		return nil, f.err
	}
	if f.currentUser != nil {
		return f.currentUser, nil
	}
	return &User{
		Name:        "jirafs-test",
		DisplayName: "Jirafs Test User",
		EmailAddress: "jirafs-test@example.com",
		Active:      true,
	}, nil
}

// SetStatuses registers statuses that FetchStatuses will return.
func (f *FakeTransport) SetStatuses(statuses []StatusEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = statuses
}

// SetSprints registers sprints for a project key that FetchSprints will return.
func (f *FakeTransport) SetSprints(projectKey string, sprints []SprintEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sprints[projectKey] = sprints
}

// SetFixVersions registers fix versions for a project key that
// FetchFixVersions will return.
func (f *FakeTransport) SetFixVersions(projectKey string, versions []FixVersionEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fixVersions[projectKey] = versions
}

// FetchStatuses returns the registered statuses.
func (f *FakeTransport) FetchStatuses(_ context.Context) ([]StatusEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.statuses == nil {
		return nil, NewNotFoundError("statuses")
	}
	return f.statuses, nil
}

// FetchSprints returns the registered sprints for the given project key.
func (f *FakeTransport) FetchSprints(_ context.Context, projectKey string) ([]SprintEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for sprints")
	}
	sprints, ok := f.sprints[projectKey]
	if !ok {
		return nil, NewNotFoundError("sprints:" + projectKey)
	}
	return sprints, nil
}

// FetchFixVersions returns the registered fix versions for the given project key.
func (f *FakeTransport) FetchFixVersions(_ context.Context, projectKey string) ([]FixVersionEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for fix versions")
	}
	versions, ok := f.fixVersions[projectKey]
	if !ok {
		return nil, NewNotFoundError("fix-versions:" + projectKey)
	}
	return versions, nil
}

// UpdateIssue updates a registered issue by its key. For the fake transport,
// this replaces the issue in the map and returns the updated copy.
func (f *FakeTransport) UpdateIssue(_ context.Context, key string, issue *schema.Issue) (*schema.Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errOn == "update" {
		return nil, f.err
	}
	if issue == nil {
		return nil, NewNotFoundError(key)
	}
	// Deep copy the issue to avoid mutations.
	updated := *issue
	f.issues[key] = &updated
	return &updated, nil
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

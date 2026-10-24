package jira

import (
	"context"
	"fmt"
	"sync"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/schema"
)

// FakeTransport is a fake Jira client for testing without a real network.
// Callers set Issues and Err before invoking FetchIssue or SearchIssues.
type FakeTransport struct {
	mu            sync.RWMutex
	issues        map[string]*schema.Issue   // key -> issue
	currentUser   *User                      // set by SetCurrentUser
	err           error                      // returned when ErrOn is set
	errOn         string                     // "fetch", "search", or "user"
	issuesByScope map[string][]*schema.Issue // scope -> issues
	searches      []string
	pagesByScope  map[string][][]*schema.Issue // scope -> token-paginated pages
	pageTokens    []string                     // "scope|token" per simulated page request
	// Registry data for registry refresh tests.
	comments    map[string][]schema.Comment
	statuses    []StatusEntry
	sprints     map[string][]SprintEntry          // projectKey -> sprints
	fixVersions map[string][]FixVersionEntry      // projectKey -> versions
	createMeta  map[string][]CreateMetaField      // project/type -> fields
	transitions map[string][]Transition           // key -> available transitions
	done        []string                          // "KEY:transitionID" performed
	doneFields  map[string]map[string]interface{} // "KEY:ID" -> fields posted
	fetchedTr   int                               // FetchTransitions calls
	created     int                               // issues created via CreateIssue
}

// NewFakeTransport creates a fake transport with empty state.
func NewFakeTransport() *FakeTransport {
	return &FakeTransport{
		issues:        make(map[string]*schema.Issue),
		issuesByScope: make(map[string][]*schema.Issue),
		comments:      make(map[string][]schema.Comment),
		sprints:       make(map[string][]SprintEntry),
		fixVersions:   make(map[string][]FixVersionEntry),
		createMeta:    make(map[string][]CreateMetaField),
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

// SetSearchPages registers a multi-page result for the scope, mimicking the
// token-paginated /search/jql endpoint: each slice is one page, and the fake
// chains them with nextPageToken values ("page-2", "page-3", ...). The pages
// are concatenated in order and the total is the number of issues.
func (f *FakeTransport) SetSearchPages(scope string, pages ...[]*schema.Issue) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pagesByScope == nil {
		f.pagesByScope = make(map[string][][]*schema.Issue)
	}
	f.pagesByScope[scope] = pages
}

// SearchPageTokens returns one "scope|token" entry per simulated page request
// made through SetSearchPages data; the first page has an empty token.
func (f *FakeTransport) SearchPageTokens() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]string(nil), f.pageTokens...)
}

// SearchIssues returns the issues registered for the given scope, or the
// configured error if ErrOn == "search". When no explicit issues are set
// for the scope, it falls back to GenerateScopeIssues for known scopes.
func (f *FakeTransport) SearchIssues(_ context.Context, scope string) ([]*schema.Issue, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches = append(f.searches, scope)
	if f.errOn == "search" {
		return nil, 0, f.err
	}
	if pages, ok := f.pagesByScope[scope]; ok {
		var all []*schema.Issue
		for i, page := range pages {
			token := ""
			if i > 0 {
				token = fmt.Sprintf("page-%d", i+1)
			}
			f.pageTokens = append(f.pageTokens, scope+"|"+token)
			all = append(all, page...)
		}
		return all, len(all), nil
	}
	// The query is JQL; the two built-in scope queries map to their names.
	switch scope {
	case "assignee = currentUser()":
		if _, ok := f.issuesByScope[scope]; !ok {
			scope = "my-issues"
		}
	case "sprint in openSprints()":
		if _, ok := f.issuesByScope[scope]; !ok {
			scope = "current-sprint"
		}
	}
	issues := f.issuesByScope[scope]
	if issues == nil {
		issues = generateScopeIssues(scope)
		if issues == nil {
			return nil, 0, NewNotFoundError("scope:" + scope)
		}
	}
	return issues, len(issues), nil
}

// Searches returns every query passed to SearchIssues, in call order.
func (f *FakeTransport) Searches() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]string(nil), f.searches...)
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
		Name:         "jirafs-test",
		DisplayName:  "Jirafs Test User",
		EmailAddress: "jirafs-test@example.com",
		Active:       true,
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

// CreateIssue registers a copy of issue under a generated key
// (<PROJECT>-<n>, starting at 1) and returns it. SetErr("create", err)
// makes it fail.
func (f *FakeTransport) CreateIssue(_ context.Context, issue *schema.Issue) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errOn == "create" {
		return "", f.err
	}
	f.created++
	key := fmt.Sprintf("%s-%d", issue.Identity.Project.Value, f.created)
	c := *issue
	c.Identity.Key = schema.IssueKey(key)
	f.issues[key] = &c
	return key, nil
}

// SetCreateMeta registers the create-meta fields FetchCreateMeta returns for a
// project key and issue type name (e.g. "ABC", "Story").
func (f *FakeTransport) SetCreateMeta(projectKey, issueType string, fields []CreateMetaField) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createMeta[projectKey+"/"+issueType] = fields
}

// FetchCreateMeta returns the registered fields (none when unregistered), or
// the configured error if ErrOn == "createmeta".
func (f *FakeTransport) FetchCreateMeta(_ context.Context, projectKey, issueType string) ([]CreateMetaField, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.errOn == "createmeta" {
		return nil, f.err
	}
	return f.createMeta[projectKey+"/"+issueType], nil
}

// SetCredentials is a no-op for the fake transport.
func (f *FakeTransport) SetCredentials(creds config.ResolvedInstanceCredentials) {}

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

// SetComments registers the comments FetchComments returns for a key, in the
// order given (the fake does not sort).
func (f *FakeTransport) SetComments(key string, comments []schema.Comment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.comments[key] = comments
}

// FetchComments returns up to limit registered comments for key and the
// total registered, or the configured error if ErrOn == "comments".
func (f *FakeTransport) FetchComments(_ context.Context, key string, limit int) ([]schema.Comment, int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.errOn == "comments" {
		return nil, 0, f.err
	}
	all := f.comments[key]
	if limit > 0 && len(all) > limit {
		return all[:limit], len(all), nil
	}
	return all, len(all), nil
}

// SetTransitions registers the transitions available for an issue.
func (f *FakeTransport) SetTransitions(key string, ts []Transition) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.transitions == nil {
		f.transitions = make(map[string][]Transition)
	}
	f.transitions[key] = ts
}

// DoneTransitions returns the performed transitions as "KEY:ID" strings.
func (f *FakeTransport) DoneTransitions() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]string(nil), f.done...)
}

// DoneFields returns the fields posted with the transition "KEY:ID".
func (f *FakeTransport) DoneFields(keyID string) map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.doneFields[keyID]
}

// FetchTransitionsCalls returns how many times FetchTransitions was called.
func (f *FakeTransport) FetchTransitionsCalls() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fetchedTr
}

// FetchTransitions returns the transitions registered for the issue.
func (f *FakeTransport) FetchTransitions(_ context.Context, key string) ([]Transition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetchedTr++
	if f.errOn == "transitions" {
		return nil, f.err
	}
	return append([]Transition(nil), f.transitions[key]...), nil
}

// DoTransition records the transition and moves the issue to its target status.
func (f *FakeTransport) DoTransition(_ context.Context, key, id string, fields map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errOn == "transition" {
		return f.err
	}
	for _, t := range f.transitions[key] {
		if t.ID == id {
			f.done = append(f.done, key+":"+id)
			if f.doneFields == nil {
				f.doneFields = make(map[string]map[string]interface{})
			}
			f.doneFields[key+":"+id] = fields
			if is := f.issues[key]; is != nil {
				is.Status = t.To.Name
			}
			return nil
		}
	}
	return NewNotFoundError("transition " + id)
}

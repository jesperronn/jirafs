// Package jira defines the Jira client interface for fetching and searching
// issues, along with structured error types for request/response failures.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/export"
	"github.com/jirafs/jirafs/internal/schema"
)

// User represents the authenticated Jira user returned by the /myself endpoint.
type User struct {
	Self         string `json:"self"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
	Timezone     string `json:"timeZone"`
	AccountType  string `json:"accountType"`
}

// StatusEntry represents a Jira issue status returned by the status API.
type StatusEntry struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description,omitempty"`
	StatusKey   string `json:"statusKey,omitempty"`
}

// SprintEntry represents a Jira sprint returned by the agile API.
type SprintEntry struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"`
	StartDate    string `json:"startDate,omitempty"`
	EndDate      string `json:"endDate,omitempty"`
	CompleteDate string `json:"completeDate,omitempty"`
}

// FixVersionEntry represents a Jira fix version returned by the version API.
type FixVersionEntry struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Archived    bool   `json:"archived"`
	Released    bool   `json:"released"`
}

// Client is the Jira API client interface for fetching and searching issues.
// Implementations may talk to a real Jira REST API or a fake transport.
type Client interface {
	// FetchIssue retrieves a single issue by its key.
	FetchIssue(ctx context.Context, key string) (*schema.Issue, error)

	// SearchIssues returns issues matching the given JQL and the total
	// number of matching issues reported by Jira.
	SearchIssues(ctx context.Context, jql string) ([]*schema.Issue, int, error)

	// FetchComments returns up to limit comments for the issue, newest
	// first, and the total number of comments Jira reports.
	FetchComments(ctx context.Context, key string, limit int) ([]schema.Comment, int, error)

	// CurrentUser returns the authenticated user identity from the Jira API.
	CurrentUser(ctx context.Context) (*User, error)

	// FetchStatuses returns all available issue statuses from Jira.
	FetchStatuses(ctx context.Context) ([]StatusEntry, error)

	// FetchSprints returns all sprints for the given project key.
	FetchSprints(ctx context.Context, projectKey string) ([]SprintEntry, error)

	// FetchFixVersions returns all fix versions for the given project key.
	FetchFixVersions(ctx context.Context, projectKey string) ([]FixVersionEntry, error)

	// UpdateIssue updates a single issue by its key, returning the updated
	// issue from Jira. It is used by the sync command to push changes back
	// to Jira after validating and applying a plan.
	UpdateIssue(ctx context.Context, key string, issue *schema.Issue) (*schema.Issue, error)

	// FetchTransitions returns the workflow transitions currently available
	// for the issue.
	FetchTransitions(ctx context.Context, key string) ([]Transition, error)

	// DoTransition performs the transition with the given id on the issue.
	DoTransition(ctx context.Context, key, transitionID string, fields map[string]interface{}) error

	// CreateIssue creates a new issue in the project named by
	// issue.Identity.Project and returns its new key.
	CreateIssue(ctx context.Context, issue *schema.Issue) (string, error)

	// FetchCreateMeta returns the fields Jira accepts when creating an issue
	// of the given type in the project, including which are required.
	FetchCreateMeta(ctx context.Context, projectKey, issueType string) ([]CreateMetaField, error)

	// SetCredentials configures the credentials used for authenticating
	// requests made by this client. It is called after client creation
	// to inject resolved credentials before making API calls.
	SetCredentials(creds config.ResolvedInstanceCredentials)
}

// jiraErrorDetails captures the structured error response from Jira.
// Jira returns {"errorMessages": [...], "errors": {...}} on failure.
type jiraErrorDetails struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// jiraIssueResponse is the JSON structure returned by the Jira REST API
// for a single issue GET request.
type jiraIssueResponse struct {
	ID     string                 `json:"id"`
	Key    string                 `json:"key"`
	Fields map[string]interface{} `json:"fields"`
	Names  map[string]string      `json:"names"`
}

// searchResponse is the JSON structure returned by the Jira REST API
// for a search (JQL) request.
type searchResponse struct {
	StartAt    int                 `json:"startAt"`
	MaxResults int                 `json:"maxResults"`
	Total      int                 `json:"total"`
	Issues     []jiraIssueResponse `json:"issues"`
	Names      map[string]string   `json:"names"`
	// NextPageToken and IsLast are returned by /search/jql instead of
	// startAt/total.
	NextPageToken string `json:"nextPageToken"`
	IsLast        bool   `json:"isLast"`
}

// statusesResponse is the JSON structure returned by the status API.
type statusesResponse struct {
	Statuses []StatusEntry `json:"statuses"`
}

// sprintsResponse is the JSON structure returned by the agile sprint API.
type sprintsResponse struct {
	Values []SprintEntry `json:"values"`
}

// fixVersionsResponse is the JSON structure returned by the project versions API.
type fixVersionsResponse struct {
	Values []FixVersionEntry `json:"values"`
}

func decodeJSONResponse(resp *http.Response, target interface{}, contextLabel string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewUnknownErr(fmt.Sprintf("cannot read %s response body: %s", contextLabel, err.Error()))
	}
	if err := json.Unmarshal(body, target); err != nil {
		preview := strings.TrimSpace(string(body))
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		respURL := ""
		if resp != nil && resp.Request != nil && resp.Request.URL != nil {
			respURL = resp.Request.URL.String()
		}
		if preview != "" {
			return NewUnknownErrWithURL(respURL, fmt.Sprintf("cannot parse %s JSON from HTTP %d: %s; body: %q", contextLabel, resp.StatusCode, err.Error(), preview))
		}
		return NewUnknownErrWithURL(respURL, fmt.Sprintf("cannot parse %s JSON from HTTP %d: %s", contextLabel, resp.StatusCode, err.Error()))
	}
	return nil
}

// mapHTTPErr reads the response body and maps the HTTP status to a
// structured ClientError, preferring Jira error details when available.
func mapHTTPErr(resp *http.Response) *ClientError {
	err := mapHTTPErrBody(resp)
	if b, ok := resp.Body.(*attemptsBody); ok {
		err.Message += " (after " + strconv.Itoa(b.attempts) + " attempts)"
	}
	return err
}

func mapHTTPErrBody(resp *http.Response) *ClientError {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	status := fmt.Sprintf("HTTP %d", resp.StatusCode)
	if text := http.StatusText(resp.StatusCode); text != "" {
		status += " " + text
	}

	var details jiraErrorDetails
	if err := json.Unmarshal(raw, &details); err == nil {
		// Build a message from Jira's structured error fields: every general
		// message, then every field error as "field: message" sorted by field.
		parts := append([]string(nil), details.ErrorMessages...)
		fields := make([]string, 0, len(details.Errors))
		for field := range details.Errors {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			parts = append(parts, field+": "+details.Errors[field])
		}
		if len(parts) > 0 {
			return NewHTTPErr(resp.StatusCode, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.Join(parts, "; ")))
		}
	}

	// Body is not JSON, empty, or carries no message: show status and a
	// whitespace-collapsed preview of the raw body.
	preview := strings.Join(strings.Fields(string(raw)), " ")
	if len(preview) > 300 {
		preview = strings.ToValidUTF8(preview[:300], "") + "..."
	}
	if preview != "" {
		return NewHTTPErr(resp.StatusCode, status+": "+preview)
	}
	return NewHTTPErr(resp.StatusCode, status)
}

// JiraClient is a real Jira REST API client that fetches and searches
// issues over HTTP.
type JiraClient struct {
	baseURL    string
	httpClient *http.Client
	credential config.ResolvedInstanceCredentials
	// epicLinkField is the custom field id holding the epic link
	// (e.g. "customfield_10014"); empty when not configured.
	epicLinkField string
	// customFields lists extra custom field ids to fetch and export.
	customFields []string
	// sleep waits between retries; nil means real time.
	sleep SleepFunc
	// searchMode caches which search endpoint this instance supports.
	searchMode atomic.Int32
}

// EpicLinkFieldEnv names the environment variable that configures the
// epic-link custom field id.
const EpicLinkFieldEnv = "JIRAFS_EPIC_LINK_FIELD"

// ResolveEpicLinkField returns the effective epic-link field id: the
// JIRAFS_EPIC_LINK_FIELD environment variable overrides the configured
// per-project setting. Empty means not configured.
func ResolveEpicLinkField(setting string) string {
	if v := strings.TrimSpace(os.Getenv(EpicLinkFieldEnv)); v != "" {
		return v
	}
	return strings.TrimSpace(setting)
}

// issueFields returns the explicit list of fields requested from Jira for
// both single-issue fetches and searches.
func (c *JiraClient) issueFields() []string {
	fields := []string{
		"summary", "description", "labels", "assignee", "status", "issuetype",
		"issuelinks", "priority", "reporter", "created", "updated",
		"components", "parent", "fixVersions", "subtasks", "project",
	}
	if c.epicLinkField != "" {
		fields = append(fields, c.epicLinkField)
	}
	for _, id := range c.customFields {
		if id != c.epicLinkField {
			fields = append(fields, id)
		}
	}
	return fields
}

// SetCredentials configures the credentials used for authenticating
// requests made by this client.
func (c *JiraClient) SetCredentials(creds config.ResolvedInstanceCredentials) {
	c.credential = creds
}

// DefaultTimeout is the overall timeout applied to each Jira request unless
// overridden with WithTimeout.
const DefaultTimeout = 30 * time.Second

// MaxAttempts is the total number of times a retryable request is sent.
const MaxAttempts = 4

// maxRetryWait caps any single wait, including a server supplied Retry-After.
const maxRetryWait = 60 * time.Second

// attemptsBody wraps the body of a final retryable response that was returned
// after more than one attempt, so mapHTTPErr can say how many attempts were
// made without touching the response headers.
type attemptsBody struct {
	io.ReadCloser
	attempts int
}

// SleepFunc waits for d or until ctx is done, returning ctx.Err() in the latter case.
type SleepFunc func(ctx context.Context, d time.Duration) error

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// neverRetried reports whether req must be sent at most once: issue creation
// and workflow transitions are not idempotent.
func neverRetried(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}
	p := strings.TrimRight(req.URL.Path, "/")
	return strings.HasSuffix(p, "/rest/api/2/issue") || strings.HasSuffix(p, "/transitions")
}

// retryWait returns how long to wait before retry number n (1-based) after
// resp. A valid Retry-After (seconds or HTTP-date) wins, capped at 60s;
// otherwise 1s, 2s, 4s plus up to 25% jitter.
func (c *JiraClient) retryWait(resp *http.Response, n int) time.Duration {
	if resp != nil {
		if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
			var d time.Duration
			ok := false
			if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
				d, ok = time.Duration(secs)*time.Second, true
			} else if t, err := http.ParseTime(v); err == nil {
				d, ok = time.Until(t), true
				if d < 0 {
					d = 0
				}
			}
			if ok {
				if d > maxRetryWait {
					d = maxRetryWait
				}
				return d
			}
		}
	}
	base := time.Second << (n - 1)
	return base + time.Duration(rand.Int63n(int64(base)/4+1))
}

func isConnReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) || strings.Contains(err.Error(), "connection reset")
}

// do sends req, retrying 429 and 503 responses (and connection resets for GET
// and PUT) up to MaxAttempts times with backoff. Requests with a body are only
// retried when the body can be replayed. A timeout is reported as an error
// naming the timeout and the request host, so a stalled connection is
// recognisable.
func (c *JiraClient) do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	replayable := !neverRetried(req) && (req.Body == nil || req.Body == http.NoBody || req.GetBody != nil)
	idempotent := req.Method == http.MethodGet || req.Method == http.MethodPut
	sleep := c.sleep
	if sleep == nil {
		sleep = realSleep
	}

	for attempt := 1; ; attempt++ {
		cur := req
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			cur = req.Clone(ctx)
			cur.Body = body
		}
		resp, err := c.httpClient.Do(cur)
		last := attempt >= MaxAttempts || !replayable
		if err != nil {
			var ne net.Error
			if ctx.Err() == nil && errors.As(err, &ne) && ne.Timeout() {
				return nil, fmt.Errorf("timed out after %s waiting for %s", c.httpClient.Timeout, req.URL.Host)
			}
			if ctx.Err() != nil || !idempotent || !isConnReset(err) || !replayable {
				return nil, err
			}
			if last {
				return nil, fmt.Errorf("%w (after %d attempts)", err, attempt)
			}
			if serr := sleep(ctx, c.retryWait(nil, attempt)); serr != nil {
				return nil, serr
			}
			continue
		}
		if (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable) || !replayable {
			return resp, nil
		}
		if last {
			if attempt > 1 {
				resp.Body = &attemptsBody{ReadCloser: resp.Body, attempts: attempt}
			}
			return resp, nil
		}
		wait := c.retryWait(resp, attempt)
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if serr := sleep(ctx, wait); serr != nil {
			return nil, serr
		}
	}
}

// Option configures a JiraClient at construction time.
type Option func(*JiraClient)

// WithTransport sets the HTTP transport used for all requests.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *JiraClient) { c.httpClient.Transport = rt }
}

// WithTimeout sets the overall per-request timeout of the HTTP client.
func WithTimeout(d time.Duration) Option {
	return func(c *JiraClient) { c.httpClient.Timeout = d }
}

// WithSleep replaces the function used to wait between retries; tests use it
// to avoid real delays.
func WithSleep(f SleepFunc) Option {
	return func(c *JiraClient) { c.sleep = f }
}

// WithEpicLinkField sets the custom field id used for the epic link.
func WithEpicLinkField(id string) Option {
	return func(c *JiraClient) { c.epicLinkField = id }
}

// WithCustomFields sets the custom field ids fetched and exported into the
// issue's custom_fields map.
func WithCustomFields(ids []string) Option {
	return func(c *JiraClient) { c.customFields = ids }
}

// defaultTransport returns the transport used when WithTransport is absent.
// Production gets a plain transport that verifies server certificates against
// the system roots. If http.DefaultTransport has been replaced by something
// other than *http.Transport (the test harness wraps it with a loopback-only
// guard), that replacement is used so the guard applies.
func defaultTransport() http.RoundTripper {
	if _, ok := http.DefaultTransport.(*http.Transport); !ok {
		return http.DefaultTransport
	}
	return productionTransport()
}

// productionTransport returns a transport with default TLS verification.
func productionTransport() *http.Transport {
	return &http.Transport{Proxy: http.ProxyFromEnvironment}
}

// NewJiraClient creates a new JiraClient for the given base URL.
func NewJiraClient(baseURL string, opts ...Option) *JiraClient {
	c := &JiraClient{
		baseURL:       baseURL,
		epicLinkField: strings.TrimSpace(os.Getenv(EpicLinkFieldEnv)),
		httpClient:    &http.Client{Transport: defaultTransport(), Timeout: DefaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// FetchIssue retrieves a single issue by its key from the Jira REST API.
func (c *JiraClient) FetchIssue(ctx context.Context, key string) (*schema.Issue, error) {
	if key == "" {
		return nil, NewNotFoundError("empty key")
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s?expand=names&fields=%s", c.baseURL, key, neturl.QueryEscape(strings.Join(c.issueFields(), ",")))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate user request: " + err.Error())
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError(key)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
		}
		return nil, mapHTTPErr(resp)
	}

	if resp.StatusCode >= 500 {
		return nil, mapHTTPErr(resp)
	}

	var jr jiraIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, NewUnknownErr("cannot parse issue JSON: " + err.Error())
	}

	issue := &schema.Issue{
		Identity: schema.IssueIdentity{
			Key:  schema.IssueKey(jr.Key),
			Type: schema.IssueType(""),
		},
	}

	if jr.Fields != nil {
		if issuetype, ok := jr.Fields["issuetype"]; ok {
			if mt, ok := issuetype.(map[string]interface{}); ok {
				if name, ok := mt["name"]; ok {
					if s, ok := name.(string); ok {
						issue.Identity.Type = schema.IssueType(s)
					}
				}
			}
		}
		export.NormalizeIssue(issue, jr.Fields)
		export.NormalizeLinkedIssues(issue, jr.Fields)
		export.NormalizeContext(issue, jr.Fields, c.epicLinkField)
		export.NormalizeCustomFieldValues(issue, jr.Fields, c.customFields, jr.Names)
	}

	return issue, nil
}

// Search endpoint modes cached per client.
const (
	searchModeUnknown int32 = iota
	searchModeJQL
	searchModeLegacy
)

const (
	searchJQLPath    = "/rest/api/3/search/jql"
	searchLegacyPath = "/rest/api/2/search"
	searchPageSize   = 50
)

// SearchIssues runs the JQL query and returns all matching issues. It uses
// the token-paginated POST /rest/api/3/search/jql endpoint (Jira Cloud). When
// that endpoint answers 404 or 410 (Jira Server / Data Center), it falls back
// to the startAt-paginated POST /rest/api/2/search endpoint; the choice is
// made once per client and cached. The int is Jira's total when the endpoint
// reports one, otherwise the number of issues returned.
func (c *JiraClient) SearchIssues(ctx context.Context, jql string) ([]*schema.Issue, int, error) {
	if strings.TrimSpace(jql) == "" {
		return nil, 0, NewNotFoundError("jql: empty query")
	}
	if c.searchMode.Load() != searchModeLegacy {
		issues, err := c.searchTokenPages(ctx, jql)
		if err == nil {
			c.searchMode.Store(searchModeJQL)
			return issues, len(issues), nil
		}
		var fb *searchFallbackError
		if !errors.As(err, &fb) {
			return nil, 0, err
		}
		c.searchMode.Store(searchModeLegacy)
	}
	return c.searchStartAtPages(ctx, jql)
}

// searchFallbackError signals that the token endpoint does not exist.
type searchFallbackError struct{ status int }

func (e *searchFallbackError) Error() string {
	return fmt.Sprintf("search/jql endpoint unavailable: HTTP %d", e.status)
}

func (c *JiraClient) searchTokenPages(ctx context.Context, jql string) ([]*schema.Issue, error) {
	issues := make([]*schema.Issue, 0)
	token := ""
	for {
		body := map[string]interface{}{
			"jql":        jql,
			"maxResults": searchPageSize,
			"fields":     c.issueFields(),
		}
		if len(c.customFields) > 0 {
			body["expand"] = "names"
		}
		if token != "" {
			body["nextPageToken"] = token
		}
		sr, status, err := c.postSearch(ctx, searchJQLPath, jql, body)
		if err != nil {
			if status == http.StatusNotFound || status == http.StatusGone {
				if c.searchMode.Load() == searchModeUnknown && token == "" {
					return nil, &searchFallbackError{status: status}
				}
			}
			return nil, err
		}
		issues = append(issues, c.searchIssues(sr)...)
		if sr.IsLast || sr.NextPageToken == "" || sr.NextPageToken == token {
			return issues, nil
		}
		token = sr.NextPageToken
	}
}

func (c *JiraClient) searchStartAtPages(ctx context.Context, jql string) ([]*schema.Issue, int, error) {
	issues := make([]*schema.Issue, 0)
	total := 0
	startAt := 0
	for {
		body := map[string]interface{}{
			"jql":        jql,
			"maxResults": searchPageSize,
			"fields":     c.issueFields(),
			"startAt":    startAt,
		}
		if len(c.customFields) > 0 {
			body["expand"] = []string{"names"}
		}
		sr, _, err := c.postSearch(ctx, searchLegacyPath, jql, body)
		if err != nil {
			return nil, 0, err
		}
		if sr.Total > 0 {
			total = sr.Total
		}
		issues = append(issues, c.searchIssues(sr)...)
		if len(sr.Issues) == 0 || len(issues) >= total {
			break
		}
		startAt += len(sr.Issues)
	}
	if total == 0 {
		total = len(issues)
	}
	return issues, total, nil
}

// postSearch POSTs one search page. On failure it returns the HTTP status
// (0 for transport errors) alongside the mapped error.
func (c *JiraClient) postSearch(ctx context.Context, path, jql string, body map[string]interface{}) (*searchResponse, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, NewUnknownErr("cannot marshal search request: " + err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, NewTransportError("cannot create search request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, 0, NewUnknownErr("cannot authenticate search request: " + err.Error())
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, 0, newTransportCause("search request failed: ", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, resp.StatusCode, NewNotFoundError("search:" + jql)
	case resp.StatusCode == http.StatusGone:
		return nil, resp.StatusCode, mapHTTPErr(resp)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, resp.StatusCode, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
	case resp.StatusCode >= 400:
		return nil, resp.StatusCode, mapHTTPErr(resp)
	}
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, resp.StatusCode, NewUnknownErr("cannot parse search response: " + err.Error())
	}
	return &sr, resp.StatusCode, nil
}

// searchIssues converts one search response page into schema issues.
func (c *JiraClient) searchIssues(sr *searchResponse) []*schema.Issue {
	out := make([]*schema.Issue, 0, len(sr.Issues))
	for _, ir := range sr.Issues {
		issue := &schema.Issue{
			Identity: schema.IssueIdentity{
				Key:  schema.IssueKey(ir.Key),
				Type: schema.IssueType(""),
			},
		}
		if ir.Fields != nil {
			if mt, ok := ir.Fields["issuetype"].(map[string]interface{}); ok {
				if s, ok := mt["name"].(string); ok {
					issue.Identity.Type = schema.IssueType(s)
				}
			}
			export.NormalizeIssue(issue, ir.Fields)
			export.NormalizeLinkedIssues(issue, ir.Fields)
			export.NormalizeContext(issue, ir.Fields, c.epicLinkField)
			export.NormalizeCustomFieldValues(issue, ir.Fields, c.customFields, sr.Names)
		}
		out = append(out, issue)
	}
	return out
}

// CurrentUser calls the Jira /rest/api/2/myself endpoint to retrieve
// the authenticated user's identity. It is used for scope resolution
// when building JQL queries that depend on the current user.
func (c *JiraClient) CurrentUser(ctx context.Context) (*User, error) {
	url := c.baseURL + "/rest/api/2/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate user request: " + err.Error())
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError("myself")
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
		}
		return nil, mapHTTPErr(resp)
	}

	if resp.StatusCode >= 500 {
		return nil, mapHTTPErr(resp)
	}

	var u User
	if err := decodeJSONResponse(resp, &u, "user"); err != nil {
		return nil, err
	}

	return &u, nil
}

// FetchStatuses calls the Jira /rest/api/2/status endpoint to retrieve
// all available issue statuses. It returns the parsed StatusEntry slice.
func (c *JiraClient) FetchStatuses(ctx context.Context) ([]StatusEntry, error) {
	url := c.baseURL + "/rest/api/2/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create status request: " + err.Error())
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("status request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError("statuses")
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
		}
		return nil, mapHTTPErr(resp)
	}

	if resp.StatusCode >= 500 {
		return nil, mapHTTPErr(resp)
	}

	var sr statusesResponse
	if err := decodeJSONResponse(resp, &sr, "statuses"); err != nil {
		return nil, err
	}

	return sr.Statuses, nil
}

// FetchSprints calls the Jira /rest/api/2/project/{projectKey}/versions
// endpoint to retrieve all sprints for the given project.
func (c *JiraClient) FetchSprints(ctx context.Context, projectKey string) ([]SprintEntry, error) {
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for sprints")
	}

	url := fmt.Sprintf("%s/rest/api/2/project/%s/versions?project=%s", c.baseURL, projectKey, projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create sprint request: " + err.Error())
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("sprint request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError("sprints:" + projectKey)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
		}
		return nil, mapHTTPErr(resp)
	}

	if resp.StatusCode >= 500 {
		return nil, mapHTTPErr(resp)
	}

	var sr sprintsResponse
	if err := decodeJSONResponse(resp, &sr, "sprints"); err != nil {
		return nil, err
	}

	return sr.Values, nil
}

// UpdateIssue updates a single issue by its key via the Jira REST API.
// It PUTs the editable fields to /rest/api/2/issue/{key} (Jira answers 204
// with no body) and then re-fetches the issue to return its current state.
// Fields that Jira rejects on edit (status needs the transitions API; the
// sprint placeholder is not a real field id) are not sent.
func (c *JiraClient) UpdateIssue(ctx context.Context, key string, issue *schema.Issue) (*schema.Issue, error) {
	if key == "" {
		return nil, NewNotFoundError("empty key")
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, key)

	if err := issue.ValidateLinks(); err != nil {
		return nil, NewUnknownErr(err.Error())
	}
	fields := issue.ToFieldsMap()
	delete(fields, "status")
	delete(fields, "customfield_sprint")
	if !issue.PriorityChanged() {
		delete(fields, "priority")
	}
	for k, v := range issue.CustomFields {
		fields[k] = v
	}

	pbody, err := json.Marshal(map[string]interface{}{"fields": fields})
	if err != nil {
		return nil, NewUnknownErr("cannot marshal update request: " + err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(pbody))
	if err != nil {
		return nil, NewTransportError("cannot create update request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate update request: " + err.Error())
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("update request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError(key)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return nil, mapHTTPErr(resp)
	}

	return c.FetchIssue(ctx, key)
}

// FetchFixVersions calls the Jira /rest/api/2/project/{projectKey}/versions
// endpoint to retrieve all fix versions for the given project.
func (c *JiraClient) FetchFixVersions(ctx context.Context, projectKey string) ([]FixVersionEntry, error) {
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for fix versions")
	}

	url := fmt.Sprintf("%s/rest/api/2/project/%s/versions", c.baseURL, projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create version request: " + err.Error())
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, newTransportCause("version request failed: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError("fix-versions:" + projectKey)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
		}
		return nil, mapHTTPErr(resp)
	}

	if resp.StatusCode >= 500 {
		return nil, mapHTTPErr(resp)
	}

	var fr fixVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return nil, NewUnknownErr("cannot parse fix-versions JSON: " + err.Error())
	}

	return fr.Values, nil
}

// CreateFields builds the Jira "fields" object for creating issue: the
// editable fields plus project and issue type. Fields Jira refuses at
// creation time (status, sprint, issue links) are left out.
func CreateFields(issue *schema.Issue) map[string]interface{} {
	fields := issue.ToFieldsMap()
	delete(fields, "status")
	delete(fields, "customfield_sprint")
	delete(fields, "issuelinks")
	for k, v := range issue.CustomFields {
		fields[k] = v
	}
	fields["project"] = map[string]string{"key": issue.Identity.Project.Value}
	fields["issuetype"] = map[string]string{"name": IssueTypeName(string(issue.Identity.Type))}
	return fields
}

// IssueTypeName converts a local lower-case type ("story") to the Jira
// issue type name ("Story").
func IssueTypeName(t string) string {
	if t == "" {
		return t
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

// CreateIssue creates an issue via POST /rest/api/2/issue and returns the
// new issue key.
func (c *JiraClient) CreateIssue(ctx context.Context, issue *schema.Issue) (string, error) {
	pbody, err := json.Marshal(map[string]interface{}{"fields": CreateFields(issue)})
	if err != nil {
		return "", NewUnknownErr("cannot marshal create request: " + err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/api/2/issue", bytes.NewReader(pbody))
	if err != nil {
		return "", NewTransportError("cannot create request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return "", NewUnknownErr("cannot authenticate create request: " + err.Error())
	}
	resp, err := c.do(req)
	if err != nil {
		return "", newTransportCause("create request failed: ", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return "", mapHTTPErr(resp)
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || created.Key == "" {
		return "", NewUnknownErr("cannot parse create response: missing issue key")
	}
	return created.Key, nil
}

// Package jira defines the Jira client interface for fetching and searching
// issues, along with structured error types for request/response failures.
package jira

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"sort"
	"strings"

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

	// SearchIssues returns issues matching the given scope and the total
	// number of matching issues reported by Jira.
	SearchIssues(ctx context.Context, scope string) ([]*schema.Issue, int, error)

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
}

// searchResponse is the JSON structure returned by the Jira REST API
// for a search (JQL) request.
type searchResponse struct {
	StartAt    int                 `json:"startAt"`
	MaxResults int                 `json:"maxResults"`
	Total      int                 `json:"total"`
	Issues     []jiraIssueResponse `json:"issues"`
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

// SetEpicLinkField configures the custom field id used for the epic link.
func (c *JiraClient) SetEpicLinkField(id string) {
	c.epicLinkField = id
}

// issueFields returns the explicit list of fields requested from Jira for
// both single-issue fetches and searches.
func (c *JiraClient) issueFields() []string {
	fields := []string{
		"summary", "description", "labels", "assignee", "status", "issuetype",
		"issuelinks", "priority", "reporter", "created", "updated",
		"components", "parent", "fixVersions", "subtasks",
	}
	if c.epicLinkField != "" {
		fields = append(fields, c.epicLinkField)
	}
	return fields
}

// SetCredentials configures the credentials used for authenticating
// requests made by this client.
func (c *JiraClient) SetCredentials(creds config.ResolvedInstanceCredentials) {
	c.credential = creds
}

// NewJiraClient creates a new JiraClient for the given base URL.
// The client uses a default HTTP transport with TLS configured to accept
// any certificate (useful for self-signed certs in dev environments).
func NewJiraClient(baseURL string) *JiraClient {
	return &JiraClient{
		baseURL:       baseURL,
		epicLinkField: strings.TrimSpace(os.Getenv(EpicLinkFieldEnv)),
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

// FetchIssue retrieves a single issue by its key from the Jira REST API.
func (c *JiraClient) FetchIssue(ctx context.Context, key string) (*schema.Issue, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if key == "" {
		return nil, NewNotFoundError("empty key")
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=%s", c.baseURL, key, neturl.QueryEscape(strings.Join(c.issueFields(), ",")))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate user request: " + err.Error())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("request failed: " + err.Error())
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
	}

	return issue, nil
}

// SearchIssues builds a JQL query for the given scope and POSTs it to the
// Jira /rest/api/2/search endpoint, returning the matching issues.
//
// Supported scopes:
//
//	"my-issues"       -> assignee = currentUser()
//	"current-sprint"  -> sprint in openSprints()
//
// Unsupported scopes return a not_found error.
func (c *JiraClient) SearchIssues(ctx context.Context, scope string) ([]*schema.Issue, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var jql string
	switch scope {
	case "my-issues":
		jql = "assignee = currentUser()"
	case "current-sprint":
		jql = "sprint in openSprints()"
	default:
		return nil, 0, NewNotFoundError("scope:" + scope)
	}

	url := c.baseURL + "/rest/api/2/search"
	fields := c.issueFields()
	const pageSize = 50
	startAt := 0
	issues := make([]*schema.Issue, 0)
	total := 0
	for {
		body := map[string]interface{}{
			"jql":        jql,
			"maxResults": pageSize,
			"fields":     fields,
			"startAt":    startAt,
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, NewUnknownErr("cannot marshal search request: " + err.Error())
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, NewTransportError("cannot create search request: " + err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		req, err = BuildAuthenticatedRequest(req, c.credential)
		if err != nil {
			return nil, 0, NewUnknownErr("cannot authenticate search request: " + err.Error())
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, NewTransportError("search request failed: " + err.Error())
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, 0, NewNotFoundError("search:" + scope)
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				resp.Body.Close()
				return nil, 0, NewAuthError("HTTP " + fmt.Sprintf("%d", resp.StatusCode))
			}
			defer resp.Body.Close()
			return nil, 0, mapHTTPErr(resp)
		}
		if resp.StatusCode >= 500 {
			defer resp.Body.Close()
			return nil, 0, mapHTTPErr(resp)
		}

		var sr searchResponse
		if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
			resp.Body.Close()
			return nil, 0, NewUnknownErr("cannot parse search response: " + err.Error())
		}
		resp.Body.Close()

		if sr.Total > 0 {
			total = sr.Total
		}
		for _, ir := range sr.Issues {
			issue := &schema.Issue{
				Identity: schema.IssueIdentity{
					Key:  schema.IssueKey(ir.Key),
					Type: schema.IssueType(""),
				},
			}
			if ir.Fields != nil {
				if issuetype, ok := ir.Fields["issuetype"]; ok {
					if mt, ok := issuetype.(map[string]interface{}); ok {
						if name, ok := mt["name"]; ok {
							if s, ok := name.(string); ok {
								issue.Identity.Type = schema.IssueType(s)
							}
						}
					}
				}
				export.NormalizeIssue(issue, ir.Fields)
				export.NormalizeLinkedIssues(issue, ir.Fields)
				export.NormalizeContext(issue, ir.Fields, c.epicLinkField)
			}
			issues = append(issues, issue)
		}
		if len(sr.Issues) == 0 || len(issues) >= total {
			break
		}
		startAt += len(sr.Issues)
	}

	return issues, total, nil
}

// CurrentUser calls the Jira /rest/api/2/myself endpoint to retrieve
// the authenticated user's identity. It is used for scope resolution
// when building JQL queries that depend on the current user.
func (c *JiraClient) CurrentUser(ctx context.Context) (*User, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	url := c.baseURL + "/rest/api/2/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate user request: " + err.Error())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("request failed: " + err.Error())
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
	if ctx == nil {
		ctx = context.Background()
	}
	url := c.baseURL + "/rest/api/2/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create status request: " + err.Error())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("status request failed: " + err.Error())
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
	if ctx == nil {
		ctx = context.Background()
	}
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for sprints")
	}

	url := fmt.Sprintf("%s/rest/api/2/project/%s/versions?project=%s", c.baseURL, projectKey, projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create sprint request: " + err.Error())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("sprint request failed: " + err.Error())
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
	if ctx == nil {
		ctx = context.Background()
	}
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("update request failed: " + err.Error())
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
	if ctx == nil {
		ctx = context.Background()
	}
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for fix versions")
	}

	url := fmt.Sprintf("%s/rest/api/2/project/%s/versions", c.baseURL, projectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create version request: " + err.Error())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("version request failed: " + err.Error())
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
	if ctx == nil {
		ctx = context.Background()
	}
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
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", NewTransportError("create request failed: " + err.Error())
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

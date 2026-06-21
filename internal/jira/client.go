// Package jira defines the Jira client interface for fetching and searching
// issues, along with structured error types for request/response failures.
package jira

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

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

	// SearchIssues returns issues matching the given scope.
	SearchIssues(ctx context.Context, scope string) ([]*schema.Issue, error)

	// CurrentUser returns the authenticated user identity from the Jira API.
	CurrentUser(ctx context.Context) (*User, error)

	// FetchStatuses returns all available issue statuses from Jira.
	FetchStatuses(ctx context.Context) ([]StatusEntry, error)

	// FetchSprints returns all sprints for the given project key.
	FetchSprints(ctx context.Context, projectKey string) ([]SprintEntry, error)

	// FetchFixVersions returns all fix versions for the given project key.
	FetchFixVersions(ctx context.Context, projectKey string) ([]FixVersionEntry, error)
}

// jiraErrorDetails captures the structured error response from Jira.
// Jira returns {"errorMessages": [...], "errors": {...}} on failure.
type jiraErrorDetails struct {
	ErrorMessages []string            `json:"errorMessages"`
	Errors        map[string]string   `json:"errors"`
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
	Total    int                `json:"total"`
	Issues   []jiraIssueResponse `json:"issues"`
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

// mapHTTPErr reads the response body and maps the HTTP status to a
// structured ClientError, preferring Jira error details when available.
func mapHTTPErr(resp *http.Response) *ClientError {
	var details jiraErrorDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		// Body is not JSON or empty; fall back to status code.
		return NewHTTPErr(resp.StatusCode, "HTTP error")
	}

	// Build a message from Jira's structured error fields.
	var msg string
	if len(details.ErrorMessages) > 0 {
		msg = details.ErrorMessages[0]
		if len(details.ErrorMessages) > 1 {
			msg += "; " + details.ErrorMessages[1]
		}
	} else if len(details.Errors) > 0 {
		// Collect field-specific error messages.
		fields := make([]string, 0, len(details.Errors))
		for field := range details.Errors {
			fields = append(fields, field)
		}
		msg = fmt.Sprintf("validation error(s) on: %v", fields)
	} else {
		msg = "HTTP error"
	}

	return NewHTTPErr(resp.StatusCode, msg)
}

// JiraClient is a real Jira REST API client that fetches and searches
// issues over HTTP.
type JiraClient struct {
	baseURL    string
	httpClient *http.Client
	credential config.ResolvedInstanceCredentials
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
		baseURL: baseURL,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

// FetchIssue retrieves a single issue by its key from the Jira REST API.
func (c *JiraClient) FetchIssue(ctx context.Context, key string) (*schema.Issue, error) {
	if key == "" {
		return nil, NewNotFoundError("empty key")
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create request: " + err.Error())
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
			Key:   schema.IssueKey(jr.Key),
			Type:  schema.IssueType(""),
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
	}

	return issue, nil
}

// SearchIssues builds a JQL query for the given scope and POSTs it to the
// Jira /rest/api/3/search endpoint, returning the matching issues.
//
// Supported scopes:
//
//	"my-issues"       -> assignee = currentUser()
//	"current-sprint"  -> sprint in openSprints()
//
// Unsupported scopes return a not_found error.
func (c *JiraClient) SearchIssues(ctx context.Context, scope string) ([]*schema.Issue, error) {
	var jql string
	switch scope {
	case "my-issues":
		jql = "assignee = currentUser()"
	case "current-sprint":
		jql = "sprint in openSprints()"
	default:
		return nil, NewNotFoundError("scope:" + scope)
	}

	body := map[string]interface{}{
		"jql":         jql,
		"maxResults":  50,
		"fields":      []string{"summary", "description", "labels", "assignee", "status", "issuetype"},
		"startAt":     0,
		"expand":      "schema,names",
		"properties":  []string{},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, NewUnknownErr("cannot marshal search request: " + err.Error())
	}

	url := c.baseURL + "/rest/api/3/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, NewTransportError("cannot create search request: " + err.Error())
	}

	req.Header.Set("Content-Type", "application/json")

	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, NewUnknownErr("cannot authenticate search request: " + err.Error())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewTransportError("search request failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, NewNotFoundError("search:" + scope)
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

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, NewUnknownErr("cannot parse search response: " + err.Error())
	}

	issues := make([]*schema.Issue, 0, len(sr.Issues))
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
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// CurrentUser calls the Jira /rest/api/3/myself endpoint to retrieve
// the authenticated user's identity. It is used for scope resolution
// when building JQL queries that depend on the current user.
func (c *JiraClient) CurrentUser(ctx context.Context) (*User, error) {
	url := c.baseURL + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, NewTransportError("cannot create request: " + err.Error())
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
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, NewUnknownErr("cannot parse user JSON: " + err.Error())
	}

	return &u, nil
}

// FetchStatuses calls the Jira /rest/api/3/status endpoint to retrieve
// all available issue statuses. It returns the parsed StatusEntry slice.
func (c *JiraClient) FetchStatuses(ctx context.Context) ([]StatusEntry, error) {
	url := c.baseURL + "/rest/api/3/status"
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
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, NewUnknownErr("cannot parse statuses JSON: " + err.Error())
	}

	return sr.Statuses, nil
}

// FetchSprints calls the Jira /rest/agile/1.0/sprint endpoint with a
// project query parameter to retrieve all sprints for the given project.
func (c *JiraClient) FetchSprints(ctx context.Context, projectKey string) ([]SprintEntry, error) {
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for sprints")
	}

	url := fmt.Sprintf("%s/rest/agile/1.0/sprint?project=%s", c.baseURL, projectKey)
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
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, NewUnknownErr("cannot parse sprints JSON: " + err.Error())
	}

	return sr.Values, nil
}

// FetchFixVersions calls the Jira /rest/api/3/project/{projectKey}/versions
// endpoint to retrieve all fix versions for the given project.
func (c *JiraClient) FetchFixVersions(ctx context.Context, projectKey string) ([]FixVersionEntry, error) {
	if projectKey == "" {
		return nil, NewNotFoundError("empty project key for fix versions")
	}

	url := fmt.Sprintf("%s/rest/api/3/project/%s/versions", c.baseURL, projectKey)
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

// Package jira defines the Jira client interface for fetching and searching
// issues, along with structured error types for request/response failures.
package jira

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jirafs/jirafs/internal/export"
	"github.com/jirafs/jirafs/internal/schema"
)

// Client is the Jira API client interface for fetching and searching issues.
// Implementations may talk to a real Jira REST API or a fake transport.
type Client interface {
	// FetchIssue retrieves a single issue by its key.
	FetchIssue(ctx context.Context, key string) (*schema.Issue, error)

	// SearchIssues returns issues matching the given scope.
	SearchIssues(ctx context.Context, scope string) ([]*schema.Issue, error)
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
	}

	return issue, nil
}

// SearchIssues is a stub that returns an unimplemented error.
// Implementation is deferred to later tasks.
func (c *JiraClient) SearchIssues(ctx context.Context, scope string) ([]*schema.Issue, error) {
	return nil, NewUnknownErr("SearchIssues not yet implemented for JiraClient")
}

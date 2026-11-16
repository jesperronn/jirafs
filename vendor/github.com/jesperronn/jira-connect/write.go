package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// IssuePatch contains only fields the caller explicitly wants to change.
// Missing fields are untouched; null and empty arrays are sent unchanged.
type IssuePatch struct {
	Fields map[string]json.RawMessage `json:"fields"`
}

var fieldIdentifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
var linkIdentifier = regexp.MustCompile(`^[1-9][0-9]*$`)

// Validate checks syntax only. Jira owns field editability and allowed values.
func (p IssuePatch) Validate() error {
	if len(p.Fields) == 0 {
		return errors.New("issue patch requires a nonempty fields object")
	}
	for name, value := range p.Fields {
		if !fieldIdentifier.MatchString(name) || !json.Valid(value) {
			return errors.New("issue patch contains an invalid field identifier or JSON value")
		}
	}
	return nil
}

// IssueLinkInput uses Jira's wire directions, without interpreting dependency semantics.
type IssueLinkInput struct {
	TypeName   string
	InwardKey  string
	OutwardKey string
}

// MarshalJSON creates the dedicated Jira issueLink payload and validates inputs.
func (l IssueLinkInput) MarshalJSON() ([]byte, error) {
	if strings.TrimSpace(l.TypeName) == "" || strings.ContainsAny(l.TypeName, "\r\n\x00") {
		return nil, errors.New("link requires a nonempty Jira type name")
	}
	if err := validateIssueKey(l.InwardKey); err != nil {
		return nil, err
	}
	if err := validateIssueKey(l.OutwardKey); err != nil {
		return nil, err
	}
	type keyRef struct {
		Key string `json:"key"`
	}
	return json.Marshal(struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
		Inward  keyRef `json:"inwardIssue"`
		Outward keyRef `json:"outwardIssue"`
	}{Type: struct {
		Name string `json:"name"`
	}{l.TypeName}, Inward: keyRef{Key: l.InwardKey}, Outward: keyRef{Key: l.OutwardKey}})
}

// WriteOutcomeUnknownError means a request was attempted without a reliable
// response. Inspect Jira before retrying; the write may already have applied.
type WriteOutcomeUnknownError struct{ cause error }

func (e *WriteOutcomeUnknownError) Error() string {
	return "Jira write outcome is unknown; inspect current Jira state before retrying"
}
func (e *WriteOutcomeUnknownError) Unwrap() error { return e.cause }

// UpdateIssue sets explicit fields on an existing issue of any type, once.
func (c *Client) UpdateIssue(ctx context.Context, key string, patch IssuePatch) error {
	if err := validateIssueKey(key); err != nil {
		return err
	}
	if err := patch.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return errors.New("cannot encode Jira issue patch")
	}
	return c.write(ctx, http.MethodPut, c.apiURL("/rest/api/2/issue/"+url.PathEscape(key)), body, http.StatusNoContent)
}

// AddIssueLink adds one explicit link, once. It never adds a comment or infers a type.
func (c *Client) AddIssueLink(ctx context.Context, link IssueLinkInput) error {
	body, err := json.Marshal(link)
	if err != nil {
		return errors.New("invalid issue link: type and both issue keys are required")
	}
	return c.write(ctx, http.MethodPost, c.apiURL("/rest/api/2/issueLink"), body, http.StatusCreated)
}

// DeleteIssueLink deletes one link by ID, once. It never deletes an issue.
func (c *Client) DeleteIssueLink(ctx context.Context, id string) error {
	if !linkIdentifier.MatchString(id) {
		return errors.New("link ID must be a positive numeric identifier")
	}
	return c.write(ctx, http.MethodDelete, c.apiURL("/rest/api/2/issueLink/"+id), nil, http.StatusNoContent)
}

// write intentionally has no retry or read-back loop. A successful response
// is sufficient; parsing/refreshing afterward must not hide a successful write.
func (c *Client) write(ctx context.Context, method string, u *url.URL, body []byte, success int) error {
	_, err := c.writeResponse(ctx, method, u, body, success, false)
	return err
}

// writeResponse performs exactly one mutation attempt. When a representation is
// required, a lost or invalid success body leaves its outcome unknown.
func (c *Client) writeResponse(ctx context.Context, method string, u *url.URL, body []byte, success int, representation bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("cannot construct Jira write request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.credentials.authenticate(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		cause := errors.New("request failed")
		if ctx.Err() != nil {
			cause = ctx.Err()
		} else if errors.Is(err, context.DeadlineExceeded) {
			cause = context.DeadlineExceeded
		}
		return nil, &WriteOutcomeUnknownError{cause: cause}
	}
	defer resp.Body.Close()
	if resp.StatusCode != success {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Attempts: 1}
	}
	if !representation {
		return nil, nil
	}
	result, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil || len(result) > maxBodyBytes || !json.Valid(result) {
		return nil, &WriteOutcomeUnknownError{cause: errors.New("cannot decode Jira write response")}
	}
	return json.RawMessage(result), nil
}

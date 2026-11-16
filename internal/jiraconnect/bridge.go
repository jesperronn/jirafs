// Adapted from jira-connect/bridges/jirafs at f32949d6d702; consumer-owned seam.
// Package jiraconnect adapts the generic Data Center client to jirafs' Client.
// Mirror hashing, conflict detection, planning and disk writes stay in jirafs.
package jiraconnect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	connect "github.com/jesperronn/jira-connect"
	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/export"
	legacy "github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// Config is deliberately independent of legacy Option's private client state.
type Config struct {
	BaseURL       string
	Transport     http.RoundTripper
	Timeout       time.Duration
	EpicLinkField string
	CustomFields  []string
}

type Client struct {
	mu        sync.RWMutex
	cfg       Config
	remote    *connect.Client
	configErr error
}

var _ legacy.Client = (*Client)(nil)

// New constructs an uncredentialed adapter. SetCredentials must precede I/O,
// matching jirafs' existing factory/credential-resolution lifecycle.
func New(cfg Config) *Client {
	cfg.CustomFields = append([]string(nil), cfg.CustomFields...)
	cfg.EpicLinkField = legacy.ResolveEpicLinkField(cfg.EpicLinkField)
	return &Client{cfg: cfg, configErr: legacy.NewAuthError("credentials not configured")}
}

func (c *Client) SetCredentials(creds config.ResolvedInstanceCredentials) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := creds.Credential.Fields
	auth := connect.Credentials{Type: connect.AuthType(creds.AuthType)}
	switch creds.AuthType {
	case legacy.AuthTypeBasic:
		auth.Username, auth.Secret = fields["username"], fields["password"]
	case legacy.AuthTypeAtlassianAPI:
		auth.Type, auth.Username, auth.Secret = connect.BasicAuth, fields["email"], fields["api_token"]
	case legacy.AuthTypeBearerToken:
		auth.Secret = fields["bearer_token"]
	default:
		c.remote, c.configErr = nil, legacy.NewAuthError("unsupported or missing authentication type")
		return
	}
	ids := append([]string(nil), c.cfg.CustomFields...)
	if c.cfg.EpicLinkField != "" {
		ids = append(ids, c.cfg.EpicLinkField)
	}
	remote, err := connect.NewClient(connect.Config{BaseURL: c.cfg.BaseURL, Credentials: auth, Transport: c.cfg.Transport, Timeout: c.cfg.Timeout, CustomFields: ids})
	c.remote = remote
	c.configErr = err
	if err != nil {
		c.configErr = legacy.NewAuthError(err.Error())
	}
}

func (c *Client) client() (*connect.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.remote, c.configErr
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var h *connect.HTTPError
	if errors.As(err, &h) {
		code := legacy.ErrHTTP
		if h.StatusCode == 401 || h.StatusCode == 403 {
			code = legacy.ErrAuth
		}
		if h.StatusCode == 404 {
			code = legacy.ErrNotFound
		}
		return &legacy.ClientError{Code: code, HTTPCode: h.StatusCode, Message: h.Error(), Cause: err}
	}
	var unknown *connect.WriteOutcomeUnknownError
	if errors.As(err, &unknown) {
		return &legacy.ClientError{Code: legacy.ErrUnknown, Message: err.Error(), Cause: err}
	}
	var transport *connect.TransportError
	if errors.As(err, &transport) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &legacy.ClientError{Code: legacy.ErrTransport, Message: "Jira read transport failed", Cause: err}
	}
	return &legacy.ClientError{Code: legacy.ErrUnknown, Message: err.Error(), Cause: err}
}

func (c *Client) issue(raw connect.Issue) (*schema.Issue, error) {
	data, err := json.Marshal(raw.Fields)
	if err != nil {
		return nil, mapError(err)
	}
	var fields map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&fields); err != nil {
		return nil, mapError(err)
	}
	converted, conversionErr := exactNumbers(fields)
	if conversionErr != nil {
		return nil, mapError(conversionErr)
	}
	fields = converted.(map[string]interface{})
	// The legacy normalizer calls the inward wire member "issue".
	if links, ok := fields["issuelinks"].([]interface{}); ok {
		for _, v := range links {
			if link, ok := v.(map[string]interface{}); ok {
				if inward, ok := link["inwardIssue"]; ok {
					link["issue"] = inward
				}
			}
		}
	}
	issue := &schema.Issue{Identity: schema.IssueIdentity{Key: schema.IssueKey(raw.Key), Type: schema.IssueType(raw.Type.Name)}}
	export.NormalizeIssue(issue, fields)
	export.NormalizeLinkedIssues(issue, fields)
	export.NormalizeContext(issue, fields, c.cfg.EpicLinkField)
	export.NormalizeCustomFieldValues(issue, fields, c.cfg.CustomFields, raw.FieldNames)
	return issue, nil
}

func (c *Client) FetchIssue(ctx context.Context, key string) (*schema.Issue, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	raw, e := r.GetIssue(ctx, key)
	if e != nil {
		return nil, mapError(e)
	}
	return c.issue(raw)
}
func (c *Client) SearchIssues(ctx context.Context, jql string) ([]*schema.Issue, int, error) {
	r, e := c.client()
	if e != nil {
		return nil, 0, e
	}
	out := make([]*schema.Issue, 0)
	cursor := ""
	total := 0
	seenIDs, seenKeys := map[string]bool{}, map[string]bool{}
	for {
		page, e := r.SearchIssues(ctx, jql, connect.PageRequest{Limit: 100, Cursor: cursor})
		if e != nil {
			return nil, 0, mapError(e)
		}
		if page.Total != nil {
			total = *page.Total
		}
		for _, raw := range page.Issues {
			if seenIDs[raw.ID] || seenKeys[raw.Key] {
				return nil, 0, legacy.NewUnknownErr("duplicate issue across search pages; restart search")
			}
			seenIDs[raw.ID], seenKeys[raw.Key] = true, true
			issue, e := c.issue(raw)
			if e != nil {
				return nil, 0, e
			}
			out = append(out, issue)
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if total < len(out) {
		total = len(out)
	}
	return out, total, nil
}

// RefreshAfterWriteError means the PUT succeeded but its subsequent GET failed.
// A caller must report the accepted write and refresh, rather than retrying it.
type RefreshAfterWriteError struct {
	Key   string
	Cause error
}

func (e *RefreshAfterWriteError) Error() string {
	return fmt.Sprintf("Jira accepted update to %s; refresh failed: %v", e.Key, e.Cause)
}
func (e *RefreshAfterWriteError) Unwrap() error { return e.Cause }

func patch(fields map[string]interface{}) (connect.IssuePatch, error) {
	out := connect.IssuePatch{Fields: make(map[string]json.RawMessage, len(fields))}
	for id, v := range fields {
		raw, e := json.Marshal(v)
		if e != nil {
			return out, e
		}
		out.Fields[id] = raw
	}
	return out, nil
}
func (c *Client) UpdateIssue(ctx context.Context, key string, issue *schema.Issue) (*schema.Issue, error) {
	if issue == nil {
		return nil, legacy.NewUnknownErr("nil issue")
	}
	if e := issue.ValidateLinks(); e != nil {
		return nil, legacy.NewUnknownErr(e.Error())
	}
	fields := issue.ToFieldsMap()
	delete(fields, "status")
	delete(fields, "customfield_sprint")
	if !issue.PriorityChanged() {
		delete(fields, "priority")
	}
	for id, v := range issue.CustomFields {
		fields[id] = v
	}
	p, e := patch(fields)
	if e != nil {
		return nil, mapError(e)
	}
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	if e = r.UpdateIssue(ctx, key, p); e != nil {
		return nil, mapError(e)
	}
	refreshed, e := c.FetchIssue(ctx, key)
	if e != nil {
		return nil, &RefreshAfterWriteError{Key: key, Cause: e}
	}
	return refreshed, nil
}
func (c *Client) CreateIssue(ctx context.Context, issue *schema.Issue) (string, error) {
	if issue == nil {
		return "", legacy.NewUnknownErr("nil issue")
	}
	p, e := patch(legacy.CreateFields(issue))
	if e != nil {
		return "", mapError(e)
	}
	r, e := c.client()
	if e != nil {
		return "", e
	}
	ref, e := r.CreateIssue(ctx, p)
	return ref.Key, mapError(e)
}
func (c *Client) CurrentUser(ctx context.Context) (*legacy.User, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	raw, e := r.CurrentUser(ctx)
	if e != nil {
		return nil, mapError(e)
	}
	var out legacy.User
	e = json.Unmarshal(raw, &out)
	return &out, mapError(e)
}
func (c *Client) FetchFixVersions(ctx context.Context, project string) ([]legacy.FixVersionEntry, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	raw, e := r.ListVersions(ctx, project)
	if e != nil {
		return nil, mapError(e)
	}
	out := make([]legacy.FixVersionEntry, 0, len(raw))
	for _, v := range raw {
		out = append(out, legacy.FixVersionEntry{Name: v.Name, Description: v.Description, Archived: v.Archived, Released: v.Released})
	}
	return out, nil
}
func (c *Client) FetchStatuses(ctx context.Context) ([]legacy.StatusEntry, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	raw, e := r.Statuses(ctx)
	if e != nil {
		return nil, mapError(e)
	}
	out := make([]legacy.StatusEntry, 0, len(raw))
	for _, v := range raw {
		var s struct {
			ID, Name, Description string
			Category              string
			StatusKey             string `json:"statusKey"`
			StatusCategory        struct{ Key, Name string }
		}
		// Jira statusCategory is an object; the consumer's Category is a string.
		if e = json.Unmarshal(v, &s); e != nil {
			return nil, mapError(e)
		}
		category := s.StatusCategory.Key
		if category == "" {
			category = s.StatusCategory.Name
		}
		if category == "" {
			category = s.Category
		}
		out = append(out, legacy.StatusEntry{Name: s.Name, Category: category, Description: s.Description, StatusKey: s.StatusKey})
	}
	return out, nil
}
func (c *Client) FetchSprints(ctx context.Context, project string) ([]legacy.SprintEntry, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	raw, e := r.Sprints(ctx, project)
	if e != nil {
		return nil, mapError(e)
	}
	out := make([]legacy.SprintEntry, 0, len(raw))
	for _, v := range raw {
		var s legacy.SprintEntry
		if e = json.Unmarshal(v, &s); e != nil {
			return nil, mapError(e)
		}
		out = append(out, s)
	}
	return out, nil
}
func (c *Client) FetchTransitions(ctx context.Context, key string) ([]legacy.Transition, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	raw, e := r.Transitions(ctx, key)
	if e != nil {
		return nil, mapError(e)
	}
	out := make([]legacy.Transition, 0, len(raw))
	for _, v := range raw {
		var t legacy.Transition
		if e = json.Unmarshal(v, &t); e != nil {
			return nil, mapError(e)
		}
		out = append(out, t)
	}
	return out, nil
}
func (c *Client) DoTransition(ctx context.Context, key, id string, fields map[string]interface{}) error {
	p, e := patch(fields)
	if e != nil {
		return mapError(e)
	}
	r, e := c.client()
	if e != nil {
		return e
	}
	return mapError(r.TransitionIssue(ctx, key, connect.TransitionInput{ID: id, Fields: p.Fields}))
}
func jiraTime(value string) (string, time.Time) {
	for _, layout := range []string{"2006-01-02T15:04:05.000-0700", time.RFC3339} {
		if t, e := time.Parse(layout, value); e == nil {
			return t.Format(time.RFC3339), t
		}
	}
	return value, time.Time{}
}
func (c *Client) FetchComments(ctx context.Context, key string, limit int) ([]schema.Comment, int, error) {
	r, e := c.client()
	if e != nil {
		return nil, 0, e
	}
	page, e := r.Comments(ctx, key, limit)
	if e != nil {
		return nil, 0, mapError(e)
	}
	type dated struct {
		comment schema.Comment
		time    time.Time
	}
	comments := make([]dated, 0, len(page.Comments))
	for _, v := range page.Comments {
		var raw struct {
			Author        struct{ DisplayName, Name string }
			Body, Created string
		}
		if e = json.Unmarshal(v, &raw); e != nil {
			return nil, 0, mapError(e)
		}
		author := raw.Author.DisplayName
		if author == "" {
			author = raw.Author.Name
		}
		created, t := jiraTime(raw.Created)
		comments = append(comments, dated{schema.Comment{Author: author, Created: created, Body: raw.Body}, t})
	}
	sort.SliceStable(comments, func(i, j int) bool { return comments[i].time.After(comments[j].time) })
	out := make([]schema.Comment, len(comments))
	for i, v := range comments {
		out[i] = v.comment
	}
	return out, max(page.Total, len(out)), nil
}
func (c *Client) FetchCreateMeta(ctx context.Context, project, issueType string) ([]legacy.CreateMetaField, error) {
	r, e := c.client()
	if e != nil {
		return nil, e
	}
	types, e := r.ListIssueTypes(ctx, project)
	if e != nil {
		return nil, mapError(e)
	}
	id := ""
	for _, v := range types {
		var t struct{ ID, Name string }
		if e = json.Unmarshal(v, &t); e != nil {
			return nil, mapError(e)
		}
		if strings.EqualFold(t.Name, issueType) {
			if id != "" && id != t.ID {
				return nil, legacy.NewUnknownErr("ambiguous issue type name; select an unambiguous configured type")
			}
			id = t.ID
		}
	}
	if id == "" {
		return nil, legacy.NewNotFoundError("issue type " + issueType + " in project " + project)
	}
	raw, e := r.CreateMetadata(ctx, project, id)
	if e != nil {
		return nil, mapError(e)
	}
	out := make([]legacy.CreateMetaField, 0, len(raw))
	for _, v := range raw {
		var f struct {
			FieldID       string `json:"fieldId"`
			ID, Name      string
			Required      bool
			HasDefault    bool `json:"hasDefaultValue"`
			AllowedValues []legacy.AllowedValue
		}
		if e = json.Unmarshal(v, &f); e != nil {
			return nil, mapError(e)
		}
		if f.FieldID == "" {
			f.FieldID = f.ID
		}
		if f.FieldID == "" {
			return nil, legacy.NewUnknownErr("create metadata field has no field ID")
		}
		out = append(out, legacy.CreateMetaField{ID: f.FieldID, Name: f.Name, Required: f.Required, HasDefault: f.HasDefault, AllowedValues: f.AllowedValues})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Convert to numeric types the consumer YAML round trip supports. Refuse a
// precision-changing conversion rather than silently rendering numbers as text.
func exactNumbers(v interface{}) (interface{}, error) {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n, nil
		}
		f, err := x.Float64()
		if err != nil {
			return nil, errors.New("numeric value cannot be represented by consumer YAML")
		}
		encoded, err := json.Marshal(f)
		if err != nil {
			return nil, errors.New("numeric value cannot be represented by consumer YAML")
		}
		original, ok := new(big.Rat).SetString(string(x))
		if !ok {
			return nil, errors.New("invalid numeric value")
		}
		converted, ok := new(big.Rat).SetString(string(encoded))
		if !ok || original.Cmp(converted) != 0 {
			return nil, errors.New("numeric precision is unsupported by consumer YAML; use the generic raw connector")
		}
		return f, nil
	case map[string]interface{}:
		for key, value := range x {
			converted, err := exactNumbers(value)
			if err != nil {
				return nil, err
			}
			x[key] = converted
		}
	case []interface{}:
		for i, value := range x {
			converted, err := exactNumbers(value)
			if err != nil {
				return nil, err
			}
			x[i] = converted
		}
	}
	return v, nil
}

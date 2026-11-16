package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// VersionCreateInput is Jira's native, flat version payload. Extra properties
// remain opaque; Jira determines permissions and supported values.
type VersionCreateInput map[string]json.RawMessage

func (v VersionCreateInput) Validate() error {
	if err := validateRawObject(v); err != nil {
		return err
	}
	var name string
	var project json.Number
	if json.Unmarshal(v["name"], &name) != nil || strings.TrimSpace(name) == "" {
		return errors.New("version requires a nonempty name string")
	}
	rawProject := bytes.TrimSpace(v["projectId"])
	if json.Unmarshal(rawProject, &project) != nil || len(rawProject) == 0 || rawProject[0] == '"' {
		return errors.New("version requires a numeric projectId")
	}
	id, err := project.Int64()
	if err != nil || id <= 0 {
		return errors.New("version requires a positive integer projectId")
	}
	return nil
}

// CommentInput preserves Jira's native body and optional visibility properties.
type CommentInput map[string]json.RawMessage

func (c CommentInput) Validate() error {
	if err := validateRawObject(c); err != nil {
		return err
	}
	var body string
	rawBody := bytes.TrimSpace(c["body"])
	if len(rawBody) == 0 || rawBody[0] != '"' || json.Unmarshal(rawBody, &body) != nil {
		return errors.New("comment requires a body string")
	}
	return nil
}

func validateRawObject(values map[string]json.RawMessage) error {
	for key, value := range values {
		if key == "" || !json.Valid(value) {
			return errors.New("invalid Jira payload property or JSON value")
		}
	}
	return nil
}

// TransitionInput selects one Jira transition and optional explicit field edits.
type TransitionInput struct {
	ID     string
	Fields map[string]json.RawMessage
}

func (t TransitionInput) MarshalJSON() ([]byte, error) {
	if !linkIdentifier.MatchString(t.ID) {
		return nil, errors.New("transition requires a positive numeric ID")
	}
	if len(t.Fields) != 0 {
		if err := (IssuePatch{Fields: t.Fields}).Validate(); err != nil {
			return nil, err
		}
	}
	return json.Marshal(struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
		Fields map[string]json.RawMessage `json:"fields,omitempty"`
	}{Transition: struct {
		ID string `json:"id"`
	}{t.ID}, Fields: t.Fields})
}

// CreateIssue submits explicit fields once. Jira owns required-field validation.
func (c *Client) CreateIssue(ctx context.Context, patch IssuePatch) (IssueRef, error) {
	if err := patch.Validate(); err != nil {
		return IssueRef{}, err
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return IssueRef{}, errors.New("cannot encode Jira issue fields")
	}
	result, err := c.writeResponse(ctx, http.MethodPost, c.apiURL("/rest/api/2/issue"), body, http.StatusCreated, true)
	if err != nil {
		return IssueRef{}, err
	}
	var ref IssueRef
	if json.Unmarshal(result, &ref) != nil || strings.TrimSpace(ref.ID) == "" || strings.TrimSpace(ref.Key) == "" {
		return IssueRef{}, unknownIdentity()
	}
	return ref, nil
}

func unknownIdentity() error {
	return &WriteOutcomeUnknownError{cause: errors.New("missing Jira write response identity")}
}

// CreateVersion submits a native version payload once, returning the raw result.
func (c *Client) CreateVersion(ctx context.Context, input VersionCreateInput) (json.RawMessage, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return c.createEntity(ctx, "/rest/api/2/version", input)
}

// AddComment submits an explicit native comment once, returning the raw result.
func (c *Client) AddComment(ctx context.Context, key string, input CommentInput) (json.RawMessage, error) {
	if err := validateIssueKey(key); err != nil {
		return nil, err
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return c.createEntity(ctx, "/rest/api/2/issue/"+url.PathEscape(key)+"/comment", input)
}

func (c *Client) createEntity(ctx context.Context, path string, input any) (json.RawMessage, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.New("cannot encode Jira creation payload")
	}
	result, err := c.writeResponse(ctx, http.MethodPost, c.apiURL(path), body, http.StatusCreated, true)
	if err != nil {
		return nil, err
	}
	var identity struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(result, &identity) != nil || strings.TrimSpace(identity.ID) == "" {
		return nil, unknownIdentity()
	}
	return result, nil
}

// TransitionIssue applies one explicit workflow transition without retry/readback.
func (c *Client) TransitionIssue(ctx context.Context, key string, input TransitionInput) error {
	if err := validateIssueKey(key); err != nil {
		return err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return errors.New("invalid Jira transition payload")
	}
	return c.write(ctx, http.MethodPost, c.apiURL("/rest/api/2/issue/"+url.PathEscape(key)+"/transitions"), body, http.StatusNoContent)
}

// Statuses reads the instance's status definitions without interpreting them.
func (c *Client) Statuses(ctx context.Context) ([]json.RawMessage, error) {
	body, err := c.get(ctx, c.apiURL("/rest/api/2/status"))
	if err != nil {
		return nil, err
	}
	var statuses []json.RawMessage
	if json.Unmarshal(body, &statuses) != nil || statuses == nil {
		return nil, errors.New("invalid Jira statuses response")
	}
	return statuses, nil
}

// Transitions returns available transitions including their editable field metadata.
func (c *Client) Transitions(ctx context.Context, key string) ([]json.RawMessage, error) {
	if err := validateIssueKey(key); err != nil {
		return nil, err
	}
	u := c.apiURL("/rest/api/2/issue/" + url.PathEscape(key) + "/transitions")
	u.RawQuery = url.Values{"expand": {"transitions.fields"}}.Encode()
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var result struct {
		Transitions []json.RawMessage `json:"transitions"`
	}
	if json.Unmarshal(body, &result) != nil || result.Transitions == nil {
		return nil, errors.New("invalid Jira transitions response")
	}
	return result.Transitions, nil
}

// CommentPage contains at most the requested newest comments and Jira's total.
type CommentPage struct {
	Comments []json.RawMessage `json:"comments"`
	Total    int               `json:"total"`
}

// Comments pages newest-first until limit is reached or Jira has no more results.
// Limits from 1 through 1000 are supported; errors never return partial results.
func (c *Client) Comments(ctx context.Context, key string, limit int) (CommentPage, error) {
	if err := validateIssueKey(key); err != nil {
		return CommentPage{}, err
	}
	if limit < 1 || limit > 1000 {
		return CommentPage{}, errors.New("comment limit must be between 1 and 1000")
	}
	result := CommentPage{Comments: []json.RawMessage{}}
	seen := make(map[string]bool)
	for offset := 0; len(result.Comments) < limit; {
		u := c.apiURL("/rest/api/2/issue/" + url.PathEscape(key) + "/comment")
		u.RawQuery = url.Values{"orderBy": {"-created"}, "startAt": {strconv.Itoa(offset)}, "maxResults": {strconv.Itoa(min(100, limit-len(result.Comments)))}}.Encode()
		body, err := c.get(ctx, u)
		if err != nil {
			return CommentPage{}, err
		}
		var page struct {
			StartAt  *int              `json:"startAt"`
			Total    *int              `json:"total"`
			Comments []json.RawMessage `json:"comments"`
		}
		if json.Unmarshal(body, &page) != nil || page.StartAt == nil || *page.StartAt != offset || page.Total == nil || *page.Total < 0 || page.Comments == nil {
			return CommentPage{}, errors.New("invalid Jira comments page")
		}
		if offset+len(page.Comments) > *page.Total || len(page.Comments) > min(100, limit-len(result.Comments)) {
			return CommentPage{}, errors.New("inconsistent Jira comments count")
		}
		result.Total = *page.Total
		for _, comment := range page.Comments {
			var ref struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(comment, &ref) != nil || ref.ID == "" || seen[ref.ID] {
				return CommentPage{}, errors.New("invalid or repeated Jira comment identity")
			}
			seen[ref.ID] = true
		}
		count := min(len(page.Comments), limit-len(result.Comments))
		result.Comments = append(result.Comments, page.Comments[:count]...)
		offset += len(page.Comments)
		if offset >= result.Total {
			return result, nil
		}
		if len(page.Comments) == 0 {
			return CommentPage{}, errors.New("Jira comments pagination made no progress")
		}
	}
	return result, nil
}

// Sprints discovers the project's Scrum boards and reads their actual Agile
// sprint pages. Shared sprints appear once, in first-seen server order.
func (c *Client) Sprints(ctx context.Context, projectKey string) ([]json.RawMessage, error) {
	if err := validateIssueKey(projectKey); err != nil {
		return nil, err
	}
	boards, err := c.agileValues(ctx, "/rest/agile/1.0/board", url.Values{"projectKeyOrId": {projectKey}, "type": {"scrum"}})
	if err != nil {
		return nil, err
	}
	result := []json.RawMessage{}
	seen := make(map[string]bool)
	seenBoards := make(map[string]bool)
	for _, board := range boards {
		id, err := agileID(board)
		if err != nil {
			return nil, err
		}
		if seenBoards[id] {
			continue
		}
		seenBoards[id] = true
		sprints, err := c.agileValues(ctx, "/rest/agile/1.0/board/"+id+"/sprint", nil)
		if err != nil {
			return nil, err
		}
		for _, sprint := range sprints {
			id, err := agileID(sprint)
			if err != nil {
				return nil, err
			}
			if !seen[id] {
				result = append(result, sprint)
				seen[id] = true
			}
		}
	}
	return result, nil
}

func agileID(value json.RawMessage) (string, error) {
	var raw struct {
		ID json.Number `json:"id"`
	}
	if json.Unmarshal(value, &raw) == nil {
		if id, err := raw.ID.Int64(); err == nil && id > 0 {
			return strconv.FormatInt(id, 10), nil
		}
	}
	return "", errors.New("invalid Jira Agile identity")
}

func (c *Client) agileValues(ctx context.Context, path string, filter url.Values) ([]json.RawMessage, error) {
	result := []json.RawMessage{}
	seen := make(map[string]bool)
	// Explicit bounds keep malformed, never-ending server pagination finite.
	for offset, pages := 0, 0; pages < 10000; pages++ {
		u := c.apiURL(path)
		q := url.Values{"startAt": {strconv.Itoa(offset)}, "maxResults": {"100"}}
		for key, values := range filter {
			q[key] = values
		}
		u.RawQuery = q.Encode()
		body, err := c.get(ctx, u)
		if err != nil {
			return nil, err
		}
		var page struct {
			StartAt *int              `json:"startAt"`
			IsLast  *bool             `json:"isLast"`
			Total   *int              `json:"total"`
			Values  []json.RawMessage `json:"values"`
		}
		if json.Unmarshal(body, &page) != nil || page.StartAt == nil || *page.StartAt != offset || page.Values == nil || (page.IsLast == nil && page.Total == nil) || (page.Total != nil && *page.Total < 0) {
			return nil, errors.New("invalid Jira Agile page")
		}
		next := offset + len(page.Values)
		if len(page.Values) > 100 || (page.Total != nil && (next > *page.Total || (page.IsLast != nil && *page.IsLast != (next == *page.Total)))) {
			return nil, errors.New("inconsistent Jira Agile termination")
		}
		for _, value := range page.Values {
			id, err := agileID(value)
			if err != nil {
				return nil, err
			}
			if seen[id] {
				return nil, errors.New("repeated Jira Agile identity")
			}
			seen[id] = true
		}
		result = append(result, page.Values...)
		offset += len(page.Values)
		if (page.IsLast != nil && *page.IsLast) || (page.IsLast == nil && offset >= *page.Total) {
			return result, nil
		}
		if len(page.Values) == 0 {
			return nil, errors.New("Jira Agile pagination made no progress")
		}
	}
	return nil, errors.New("Jira Agile pagination exceeds page limit")
}

package jira

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// PageRequest fetches one page of 1–100 issues. Cursor is opaque to callers.
type PageRequest struct {
	Limit  int
	Cursor string
}

// IssuePage includes Jira's total only when supplied. Follow NextCursor until
// empty; search results can change between calls and are not a snapshot.
type IssuePage struct {
	Issues     []Issue `json:"issues"`
	NextCursor string  `json:"next_cursor,omitempty"`
	Total      *int    `json:"total,omitempty"`
}

// searchCursor is continuation state, not an authorization token. It contains
// hashes instead of raw JQL and is bound to instance, route, query and fields.
type searchCursor struct {
	Version  int    `json:"v"`
	Scope    string `json:"scope"`
	StartAt  int    `json:"start_at"`
	Previous string `json:"previous"`
}

// SearchIssues reads one page via POST /rest/api/2/search on Data Center.
// Use stable JQL ordering (for example ORDER BY key ASC) while paging.
func (c *Client) SearchIssues(ctx context.Context, jql string, page PageRequest) (IssuePage, error) {
	if strings.TrimSpace(jql) == "" || page.Limit < 1 || page.Limit > 100 {
		return IssuePage{}, errors.New("Jira search requires nonempty JQL and a page limit from 1 to 100")
	}
	u := c.apiURL("/rest/api/2/search")
	scope := hash(u.String() + "\x00" + jql + "\x00" + strings.Join(c.issueFields(), ",") + "\x00" + strconv.Itoa(page.Limit))
	cursor, err := decodeCursor(page.Cursor, scope)
	if err != nil {
		return IssuePage{}, err
	}
	payload, err := json.Marshal(struct {
		JQL        string   `json:"jql"`
		StartAt    int      `json:"startAt"`
		MaxResults int      `json:"maxResults"`
		Fields     []string `json:"fields"`
		Expand     []string `json:"expand"`
	}{jql, cursor.StartAt, page.Limit, c.issueFields(), []string{"names"}})
	if err != nil {
		return IssuePage{}, errors.New("cannot encode Jira search request")
	}
	body, err := c.read(ctx, http.MethodPost, u, payload)
	if err != nil {
		return IssuePage{}, err
	}
	var raw struct {
		StartAt    *int              `json:"startAt"`
		MaxResults *int              `json:"maxResults"`
		Total      *int              `json:"total"`
		Issues     []json.RawMessage `json:"issues"`
		Names      map[string]string `json:"names"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || raw.StartAt == nil || raw.MaxResults == nil || raw.Issues == nil {
		return IssuePage{}, errors.New("invalid Jira search page response")
	}
	count := len(raw.Issues)
	if *raw.StartAt != cursor.StartAt || *raw.MaxResults < 1 || count > page.Limit || count > *raw.MaxResults {
		return IssuePage{}, errors.New("Jira search returned an unexpected offset or page size")
	}
	next := cursor.StartAt + count
	if raw.Total != nil && (*raw.Total < 0 || (count > 0 && next > *raw.Total) || (count == 0 && cursor.StartAt < *raw.Total)) {
		return IssuePage{}, errors.New("Jira search returned inconsistent pagination totals")
	}
	result := IssuePage{Issues: make([]Issue, 0, count), Total: raw.Total}
	ids := make([]string, 0, count)
	seen := make(map[string]bool, count)
	for _, data := range raw.Issues {
		issue, err := decodeIssue(data, c.customFields)
		if err != nil {
			return IssuePage{}, err
		}
		if issue.FieldNames == nil {
			issue.FieldNames = maps.Clone(raw.Names)
		}
		if seen[issue.ID] {
			return IssuePage{}, errors.New("Jira search returned duplicate issue IDs in one page")
		}
		seen[issue.ID] = true
		ids = append(ids, issue.ID)
		result.Issues = append(result.Issues, issue)
	}
	sort.Strings(ids)
	fingerprint := hash(strings.Join(ids, "\x00"))
	if count > 0 && fingerprint == cursor.Previous {
		return IssuePage{}, errors.New("Jira search repeated the previous page")
	}
	if count > 0 && (raw.Total == nil || next < *raw.Total) {
		encoded, err := json.Marshal(searchCursor{Version: 1, Scope: scope, StartAt: next, Previous: fingerprint})
		if err != nil {
			return IssuePage{}, errors.New("cannot encode Jira search cursor")
		}
		result.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return result, nil
}

func decodeCursor(value, scope string) (searchCursor, error) {
	if value == "" {
		return searchCursor{}, nil
	}
	invalid := errors.New("invalid Jira search cursor for this query, instance, fields or limit")
	if len(value) > 2048 {
		return searchCursor{}, invalid
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return searchCursor{}, invalid
	}
	var cursor searchCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Version != 1 || cursor.Scope != scope || cursor.StartAt < 1 || cursor.StartAt > math.MaxInt-100 || len(cursor.Previous) != 64 {
		return searchCursor{}, invalid
	}
	if _, err := hex.DecodeString(cursor.Previous); err != nil {
		return searchCursor{}, invalid
	}
	return cursor, nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

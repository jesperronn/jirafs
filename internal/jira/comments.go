package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sort"
	"time"

	"github.com/jirafs/jirafs/internal/schema"
)

// CommentLimit is the number of most recent comments exported per issue.
const CommentLimit = 20

type jiraCommentsResponse struct {
	Total    int `json:"total"`
	Comments []struct {
		Author struct {
			DisplayName string `json:"displayName"`
			Name        string `json:"name"`
		} `json:"author"`
		Body    string `json:"body"`
		Created string `json:"created"`
	} `json:"comments"`
}

// FetchComments retrieves the most recent comments of an issue from
// /rest/api/2/issue/KEY/comment, newest first, plus Jira's total count.
func (c *JiraClient) FetchComments(ctx context.Context, key string, limit int) ([]schema.Comment, int, error) {
	if key == "" {
		return nil, 0, NewNotFoundError("empty key")
	}
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/comment?orderBy=-created&maxResults=%d", c.baseURL, neturl.PathEscape(key), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, NewTransportError("cannot create request: " + err.Error())
	}
	req, err = BuildAuthenticatedRequest(req, c.credential)
	if err != nil {
		return nil, 0, NewUnknownErr("cannot authenticate user request: " + err.Error())
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, 0, newTransportCause("request failed: ", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, 0, NewNotFoundError(key)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, 0, NewAuthError(fmt.Sprintf("HTTP %d", resp.StatusCode))
	case resp.StatusCode >= 400:
		return nil, 0, mapHTTPErr(resp)
	}

	var jr jiraCommentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&jr); err != nil {
		return nil, 0, NewUnknownErr("cannot parse comments JSON: " + err.Error())
	}
	comments := make([]schema.Comment, 0, len(jr.Comments))
	times := make(map[int]time.Time, len(jr.Comments))
	for idx, jc := range jr.Comments {
		author := jc.Author.DisplayName
		if author == "" {
			author = jc.Author.Name
		}
		created, t := normalizeJiraTime(jc.Created)
		times[idx] = t
		comments = append(comments, schema.Comment{Author: author, Created: created, Body: jc.Body})
	}
	// Enforce newest-first even if the server ignores orderBy.
	order := make([]int, len(comments))
	for idx := range order {
		order[idx] = idx
	}
	sort.SliceStable(order, func(a, b int) bool { return times[order[a]].After(times[order[b]]) })
	sorted := make([]schema.Comment, len(comments))
	for idx, o := range order {
		sorted[idx] = comments[o]
	}
	total := jr.Total
	if total < len(sorted) {
		total = len(sorted)
	}
	return sorted, total, nil
}

// normalizeJiraTime converts Jira's "2006-01-02T15:04:05.000-0700" timestamps
// to RFC3339. Unparseable input is returned unchanged with a zero time.
func normalizeJiraTime(s string) (string, time.Time) {
	for _, layout := range []string{"2006-01-02T15:04:05.000-0700", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(time.RFC3339), t
		}
	}
	return s, time.Time{}
}

// AttachComments fetches the most recent comments for the issue and stores
// them on it. A failed fetch is reported as a warning to warn and leaves the
// issue without comments; it is never an error.
func AttachComments(ctx context.Context, c Client, issue *schema.Issue, warn io.Writer) {
	if issue == nil || issue.Identity.Key == "" {
		return
	}
	comments, total, err := c.FetchComments(ctx, string(issue.Identity.Key), CommentLimit)
	if err != nil {
		fmt.Fprintf(warn, "warning: cannot fetch comments for %s: %v\n", issue.Identity.Key, err)
		return
	}
	issue.Comments = comments
	issue.CommentsTotal = total
}

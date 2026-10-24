package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

type endpointCall struct {
	name string
	call func(c *JiraClient) error
}

func endpointCalls() []endpointCall {
	return []endpointCall{
		{"FetchIssue", func(c *JiraClient) error { _, err := c.FetchIssue(nil, "A-1"); return err }},
		{"SearchIssues", func(c *JiraClient) error { _, _, err := c.SearchIssues(nil, "assignee = currentUser()"); return err }},
		{"CurrentUser", func(c *JiraClient) error { _, err := c.CurrentUser(nil); return err }},
		{"FetchStatuses", func(c *JiraClient) error { _, err := c.FetchStatuses(nil); return err }},
		{"FetchSprints", func(c *JiraClient) error { _, err := c.FetchSprints(nil, "P"); return err }},
		{"FetchFixVersions", func(c *JiraClient) error { _, err := c.FetchFixVersions(nil, "P"); return err }},
		{"UpdateIssue", func(c *JiraClient) error {
			_, err := c.UpdateIssue(nil, "A-1", &schema.Issue{Identity: schema.IssueIdentity{Key: "A-1"}})
			return err
		}},
	}
}

func TestClientEndpoints_StatusMapping(t *testing.T) {
	statuses := []struct {
		status int
		body   string
		code   ErrCode
	}{
		{http.StatusNotFound, `{}`, ErrNotFound},
		{http.StatusUnauthorized, `{}`, ErrAuth},
		{http.StatusForbidden, `{}`, ErrAuth},
		{http.StatusBadRequest, `{"errorMessages":["bad"]}`, ErrHTTP},
		{http.StatusInternalServerError, `{"errorMessages":["boom"]}`, ErrHTTP},
		{http.StatusOK, `not json at all`, ErrUnknown},
	}
	for _, ep := range endpointCalls() {
		for _, st := range statuses {
			t.Run(ep.name+"/"+http.StatusText(st.status), func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(st.status)
					_, _ = w.Write([]byte(st.body))
				}))
				defer srv.Close()
				err := ep.call(NewJiraClient(srv.URL))
				if !IsClientError(err, st.code) {
					t.Errorf("%s with HTTP %d: err = %v, want code %s", ep.name, st.status, err, st.code)
				}
			})
		}
	}
}

func TestClientEndpoints_TransportError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	for _, ep := range endpointCalls() {
		t.Run(ep.name, func(t *testing.T) {
			if err := ep.call(NewJiraClient(url)); !IsClientError(err, ErrTransport) {
				t.Errorf("err = %v, want transport error", err)
			}
		})
	}
}

func TestClientEndpoints_EmptyArgs(t *testing.T) {
	c := NewJiraClient("http://127.0.0.1:1")
	ctx := context.Background()
	if _, err := c.FetchSprints(ctx, ""); !IsClientError(err, ErrNotFound) {
		t.Errorf("FetchSprints empty = %v", err)
	}
	if _, err := c.FetchFixVersions(ctx, ""); !IsClientError(err, ErrNotFound) {
		t.Errorf("FetchFixVersions empty = %v", err)
	}
	if _, err := c.UpdateIssue(ctx, "", &schema.Issue{}); !IsClientError(err, ErrNotFound) {
		t.Errorf("UpdateIssue empty = %v", err)
	}
	if _, _, err := c.SearchIssues(ctx, ""); !IsClientError(err, ErrNotFound) {
		t.Errorf("SearchIssues empty jql = %v", err)
	}
}

func TestDecodeJSONResponse_ParseErrorPreview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 500)))
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/p")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v map[string]any
	derr := decodeJSONResponse(resp, &v, "thing")
	ce, ok := derr.(*ClientError)
	if !ok || ce.Code != ErrUnknown {
		t.Fatalf("err = %v", derr)
	}
	if !strings.Contains(ce.Message, "...") || strings.Count(ce.Message, "x") > 305 {
		t.Errorf("preview not truncated: %d chars", len(ce.Message))
	}
	if !strings.HasSuffix(ce.URL, "/p") {
		t.Errorf("URL = %q, want request URL", ce.URL)
	}
}

func TestDecodeJSONResponse_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v map[string]any
	derr := decodeJSONResponse(resp, &v, "thing")
	if derr == nil || strings.Contains(derr.Error(), "body:") {
		t.Errorf("err = %v, want parse error without body preview", derr)
	}
}

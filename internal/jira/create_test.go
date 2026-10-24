package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/config"
	"github.com/jirafs/jirafs/internal/schema"
)

func draftIssue() *schema.Issue {
	return &schema.Issue{
		Identity:     schema.IssueIdentity{Type: "bug", Project: schema.TypedRef{Type: "project", Value: "ABC"}},
		Summary:      "Crash",
		Description:  "boom",
		Status:       "Open",
		Sprint:       "S1",
		LinkedIssues: []schema.LinkedIssue{{Key: "ABC-1", LinkType: "Blocks"}},
	}
}

func TestCreateFields(t *testing.T) {
	f := CreateFields(draftIssue())
	if f["project"].(map[string]string)["key"] != "ABC" || f["issuetype"].(map[string]string)["name"] != "Bug" {
		t.Errorf("fields=%v", f)
	}
	for _, k := range []string{"status", "customfield_sprint", "issuelinks"} {
		if _, ok := f[k]; ok {
			t.Errorf("%s must be omitted on create", k)
		}
	}
	if IssueTypeName("") != "" {
		t.Error("empty type")
	}
}

func newCreateClient(url string) *JiraClient {
	c := NewJiraClient(url)
	c.SetCredentials(config.ResolvedInstanceCredentials{BaseURL: url})
	return c
}

func TestCreateIssueSuccess(t *testing.T) {
	var gotPath string
	var gotBody map[string]map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"1","key":"ABC-9"}`))
	}))
	defer srv.Close()
	key, err := newCreateClient(srv.URL).CreateIssue(context.Background(), draftIssue())
	if err != nil || key != "ABC-9" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if gotPath != "POST /rest/api/2/issue" || gotBody["fields"]["summary"] != "Crash" {
		t.Errorf("path=%s body=%v", gotPath, gotBody)
	}
}

func TestCreateIssueErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
	}{"auth": {401, ""}, "forbidden": {403, ""}, "bad request": {400, `{"errorMessages":["bad"]}`}, "server": {500, ""}, "no key": {201, `{}`}, "bad json": {201, `x`}} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			if _, err := newCreateClient(srv.URL).CreateIssue(context.Background(), draftIssue()); err == nil {
				t.Error("expected error")
			}
		})
	}
	if _, err := newCreateClient("http://127.0.0.1:1").CreateIssue(context.Background(), draftIssue()); err == nil {
		t.Error("expected transport error")
	}
}

func TestFieldErrorsSurfaceOnCreateAndUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorMessages":["General trouble."],"errors":{"customfield_12601":"Team is required.","assignee":"bad user"}}`))
	}))
	defer srv.Close()
	want := "General trouble.; assignee: bad user; customfield_12601: Team is required."
	c := newCreateClient(srv.URL)
	_, err := c.CreateIssue(context.Background(), draftIssue())
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("create err=%v", err)
	}
	_, err = c.UpdateIssue(context.Background(), "ABC-1", draftIssue())
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("update err=%v", err)
	}
}

func TestCreateFieldsIncludeCustomFields(t *testing.T) {
	i := draftIssue()
	i.CustomFields = map[string]interface{}{
		"customfield_12601": map[string]interface{}{"value": "Team A"},
		"customfield_10002": 5,
	}
	b, _ := json.Marshal(CreateFields(i))
	for _, want := range []string{`"customfield_12601":{"value":"Team A"}`, `"customfield_10002":5`, `"summary":"Crash"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
}

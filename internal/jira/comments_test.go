package jira

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestFetchComments_RequestAndOrdering(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		// Deliberately returned oldest-first to prove client-side ordering.
		w.Write([]byte(`{"total":3,"comments":[
			{"author":{"displayName":"Old"},"body":"first","created":"2024-01-01T09:00:00.000+0000"},
			{"author":{"name":"mid"},"body":"second","created":"2024-02-01T09:00:00.000+0100"},
			{"author":{"displayName":"New"},"body":"third","created":"2024-03-01T09:00:00.000+0000"}]}`))
	}))
	defer server.Close()

	comments, total, err := NewJiraClient(server.URL).FetchComments(context.Background(), "PROJ-1", 20)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/2/issue/PROJ-1/comment" || gotQuery != "orderBy=-created&maxResults=20" {
		t.Errorf("request = %s?%s", gotPath, gotQuery)
	}
	if total != 3 || len(comments) != 3 {
		t.Fatalf("total=%d len=%d", total, len(comments))
	}
	want := []string{"New", "mid", "Old"}
	for i, w := range want {
		if comments[i].Author != w {
			t.Errorf("comments[%d].Author = %q, want %q", i, comments[i].Author, w)
		}
	}
	if comments[1].Created != "2024-02-01T09:00:00+01:00" {
		t.Errorf("Created = %q", comments[1].Created)
	}
}

func TestFetchComments_TotalExceedsPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"total":45,"comments":[{"author":{"displayName":"A"},"body":"b","created":"bad-time"}]}`))
	}))
	defer server.Close()
	comments, total, err := NewJiraClient(server.URL).FetchComments(context.Background(), "PROJ-1", 20)
	if err != nil || total != 45 || len(comments) != 1 || comments[0].Created != "bad-time" {
		t.Fatalf("comments=%v total=%d err=%v", comments, total, err)
	}
}

func TestFetchComments_Errors(t *testing.T) {
	c := NewJiraClient("http://127.0.0.1:1")
	if _, _, err := c.FetchComments(context.Background(), "", 20); err == nil {
		t.Error("empty key: want error")
	}
	if _, _, err := c.FetchComments(context.Background(), "PROJ-1", 20); err == nil {
		t.Error("transport failure: want error")
	}
	for _, code := range []int{404, 401, 403, 400, 500} {
		code := code
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		if _, _, err := NewJiraClient(server.URL).FetchComments(context.Background(), "PROJ-1", 20); err == nil {
			t.Errorf("status %d: want error", code)
		}
		server.Close()
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer bad.Close()
	if _, _, err := NewJiraClient(bad.URL).FetchComments(context.Background(), "PROJ-1", 20); err == nil {
		t.Error("bad JSON: want error")
	}
}

func TestAttachComments_FailureIsWarning(t *testing.T) {
	fake := NewFakeTransport()
	fake.SetErr("comments", errors.New("boom"))
	issue := &schema.Issue{Identity: schema.IssueIdentity{Key: "PROJ-1"}}
	var warn bytes.Buffer
	AttachComments(context.Background(), fake, issue, &warn)
	if len(issue.Comments) != 0 {
		t.Errorf("comments = %v, want none", issue.Comments)
	}
	if !strings.Contains(warn.String(), "warning: cannot fetch comments for PROJ-1: boom") {
		t.Errorf("warn = %q", warn.String())
	}
}

func TestAttachComments_SetsCommentsAndCapsAtLimit(t *testing.T) {
	fake := NewFakeTransport()
	var cs []schema.Comment
	for i := 0; i < 25; i++ {
		cs = append(cs, schema.Comment{Author: "a", Created: "2024-01-01T00:00:00Z", Body: "b"})
	}
	fake.SetComments("PROJ-1", cs)
	issue := &schema.Issue{Identity: schema.IssueIdentity{Key: "PROJ-1"}}
	AttachComments(nil, fake, issue, &bytes.Buffer{})
	if len(issue.Comments) != CommentLimit || issue.CommentsTotal != 25 {
		t.Errorf("len=%d total=%d", len(issue.Comments), issue.CommentsTotal)
	}
	AttachComments(nil, fake, nil, &bytes.Buffer{})
	AttachComments(nil, fake, &schema.Issue{}, &bytes.Buffer{})
	small := &schema.Issue{Identity: schema.IssueIdentity{Key: "PROJ-2"}}
	AttachComments(nil, fake, small, &bytes.Buffer{})
	if len(small.Comments) != 0 {
		t.Errorf("want none, got %v", small.Comments)
	}
}

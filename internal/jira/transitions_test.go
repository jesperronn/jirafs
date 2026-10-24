package jira

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestFetchTransitions(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Write([]byte(`{"transitions":[{"id":"11","name":"Start","to":{"name":"In Progress"}}]}`))
	}))
	defer server.Close()

	ts, err := NewJiraClient(server.URL).FetchTransitions(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "GET" || gotPath != "/rest/api/2/issue/PROJ-1/transitions" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if len(ts) != 1 || ts[0].ID != "11" || ts[0].Name != "Start" || ts[0].To.Name != "In Progress" {
		t.Errorf("transitions = %+v", ts)
	}
}

func TestDoTransition(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := NewJiraClient(server.URL).DoTransition(context.Background(), "PROJ-1", "11", nil); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/rest/api/2/issue/PROJ-1/transitions" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody != `{"transition":{"id":"11"}}` {
		t.Errorf("body = %s", gotBody)
	}
}

func TestTransitions_Errors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["nope"]}`))
	}))
	defer server.Close()
	c := NewJiraClient(server.URL)
	if _, err := c.FetchTransitions(context.Background(), "PROJ-1"); err == nil {
		t.Error("FetchTransitions: want error")
	}
	if err := c.DoTransition(context.Background(), "PROJ-1", "11", nil); err == nil {
		t.Error("DoTransition: want error")
	}
	if _, err := c.FetchTransitions(context.Background(), ""); err == nil {
		t.Error("empty key: want error")
	}
}

func TestTransitions_StatusMapping(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusUnauthorized} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		c := NewJiraClient(server.URL)
		if _, err := c.FetchTransitions(context.Background(), "PROJ-1"); err == nil {
			t.Errorf("FetchTransitions %d: want error", code)
		}
		if err := c.DoTransition(context.Background(), "PROJ-1", "1", nil); err == nil {
			t.Errorf("DoTransition %d: want error", code)
		}
		server.Close()
	}
	if err := NewJiraClient("http://x").DoTransition(context.Background(), "", "1", nil); err == nil {
		t.Error("empty key: want error")
	}
}

func TestFakeTransitions(t *testing.T) {
	f := NewFakeTransport()
	tr := Transition{ID: "1", Name: "Go"}
	tr.To.Name = "Done"
	f.SetTransitions("K-1", []Transition{tr})
	if err := f.DoTransition(context.Background(), "K-1", "9", nil); err == nil {
		t.Error("unknown id: want error")
	}
	f.SetErr("transitions", NewUnknownErr("boom"))
	if _, err := f.FetchTransitions(context.Background(), "K-1"); err == nil {
		t.Error("want fetch error")
	}
	f.SetErr("transition", NewUnknownErr("boom"))
	if err := f.DoTransition(context.Background(), "K-1", "1", nil); err == nil {
		t.Error("want do error")
	}
}

func TestFakeTransitions_Success(t *testing.T) {
	f := NewFakeTransport()
	f.SetIssue("K-1", &schema.Issue{Status: "Open"})
	tr := Transition{ID: "1", Name: "Go"}
	tr.To.Name = "Done"
	f.SetTransitions("K-1", []Transition{tr})
	ts, err := f.FetchTransitions(context.Background(), "K-1")
	if err != nil || len(ts) != 1 {
		t.Fatalf("ts=%v err=%v", ts, err)
	}
	if err := f.DoTransition(context.Background(), "K-1", "1", nil); err != nil {
		t.Fatal(err)
	}
	if got := f.DoneTransitions(); len(got) != 1 || got[0] != "K-1:1" {
		t.Errorf("done = %v", got)
	}
}

func TestTransitions_TransportError(t *testing.T) {
	c := NewJiraClient("http://127.0.0.1:1")
	if _, err := c.FetchTransitions(context.Background(), "K-1"); err == nil {
		t.Error("want error")
	}
	if err := c.DoTransition(context.Background(), "K-1", "1", nil); err == nil {
		t.Error("want error")
	}
}

func TestTransitionFieldsAndBody(t *testing.T) {
	var gotQuery, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.Method == "POST" {
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Write([]byte(`{"transitions":[{"id":"5","name":"Close","to":{"name":"Done"},"fields":{"resolution":{"required":true,"name":"Resolution"},"x":{"required":false,"name":"X"}}}]}`))
	}))
	defer server.Close()
	c := NewJiraClient(server.URL)
	ts, err := c.FetchTransitions(context.Background(), "P-1")
	if err != nil || gotQuery != "expand=transitions.fields" {
		t.Fatalf("err=%v query=%q", err, gotQuery)
	}
	if rf := ts[0].RequiredFields(); len(rf) != 1 || rf[0].ID != "resolution" || rf[0].Name != "Resolution" {
		t.Errorf("required = %+v", rf)
	}
	if err := c.DoTransition(context.Background(), "P-1", "5", map[string]interface{}{"resolution": "Fixed"}); err != nil {
		t.Fatal(err)
	}
	if gotBody != `{"fields":{"resolution":"Fixed"},"transition":{"id":"5"}}` {
		t.Errorf("body = %s", gotBody)
	}
}

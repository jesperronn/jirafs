package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const legacyMeta = `{"projects":[{"issuetypes":[{"name":"Story","fields":{
 "summary":{"required":true,"name":"Summary"},
 "customfield_12601":{"required":true,"name":"Team","hasDefaultValue":false,"allowedValues":[{"id":"1","value":"Team A"},{"id":"2","value":"Team B"}]}}}]}]}`

func TestFetchCreateMetaLegacy(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(legacyMeta))
	}))
	defer srv.Close()
	fields, err := newCreateClient(srv.URL).FetchCreateMeta(context.Background(), "ABC", "Story")
	if err != nil || len(fields) != 2 {
		t.Fatalf("fields=%v err=%v", fields, err)
	}
	for _, want := range []string{"projectKeys=ABC", "issuetypeNames=Story", "expand=projects.issuetypes.fields"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q lacks %s", gotQuery, want)
		}
	}
	f := fields[0]
	if f.ID != "customfield_12601" || !f.Required || f.Name != "Team" || len(f.AllowedValues) != 2 || f.AllowedValues[1].Label() != "Team B" {
		t.Errorf("field=%+v", f)
	}
	if (AllowedValue{Name: "n"}).Label() != "n" || (AllowedValue{ID: "i"}).Label() != "i" {
		t.Error("label fallbacks")
	}
}

func TestFetchCreateMetaJira9Fallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issue/createmeta":
			w.WriteHeader(404)
		case "/rest/api/2/issue/createmeta/ABC/issuetypes":
			_, _ = w.Write([]byte(`{"values":[{"id":"10","name":"Bug"},{"id":"11","name":"Story"}]}`))
		case "/rest/api/2/issue/createmeta/ABC/issuetypes/11":
			_, _ = w.Write([]byte(`{"values":[{"fieldId":"customfield_12601","name":"Team","required":true,"allowedValues":[{"id":"1","name":"Team A"}]},{"fieldId":"summary","name":"Summary","required":true}]}`))
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	c := newCreateClient(srv.URL)
	fields, err := c.FetchCreateMeta(context.Background(), "ABC", "Story")
	if err != nil || len(fields) != 2 || fields[0].ID != "customfield_12601" || fields[0].AllowedValues[0].Name != "Team A" {
		t.Fatalf("fields=%+v err=%v", fields, err)
	}
	if _, err := c.FetchCreateMeta(context.Background(), "ABC", "Epic"); !IsClientError(err, ErrNotFound) {
		t.Errorf("unknown type err=%v", err)
	}
}

func TestFetchCreateMetaErrors(t *testing.T) {
	for _, code := range []int{401, 400, 500} {
		code := code
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		if _, err := newCreateClient(srv.URL).FetchCreateMeta(context.Background(), "ABC", "Story"); err == nil {
			t.Errorf("code %d: want error", code)
		}
		srv.Close()
	}
	// Fallback list failing and bad JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/issue/createmeta" && r.URL.RawQuery != "" && r.Header.Get("X-Bad") == "" {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()
	if _, err := newCreateClient(srv.URL).FetchCreateMeta(context.Background(), "ABC", "Story"); err == nil {
		t.Error("want parse error")
	}
	if _, err := NewJiraClient("http://127.0.0.1:1").FetchCreateMeta(context.Background(), "ABC", "Story"); err == nil {
		t.Error("want transport error")
	}
}

func TestFakeCreateMeta(t *testing.T) {
	f := NewFakeTransport()
	if got, err := f.FetchCreateMeta(context.Background(), "A", "Story"); got != nil || err != nil {
		t.Fatal("empty fake")
	}
	f.SetCreateMeta("A", "Story", []CreateMetaField{{ID: "x"}})
	if got, _ := f.FetchCreateMeta(context.Background(), "A", "Story"); len(got) != 1 {
		t.Fatal("registered")
	}
	f.SetErr("createmeta", NewUnknownErr("boom"))
	if _, err := f.FetchCreateMeta(context.Background(), "A", "Story"); err == nil {
		t.Fatal("want err")
	}
}

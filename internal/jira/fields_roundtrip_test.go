package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jirafs/jirafs/internal/plan"
	"github.com/jirafs/jirafs/internal/schema"
	"github.com/jirafs/jirafs/internal/sync"
)

const fieldsIssueJSON = `{"key":"ABC-1","fields":{
 "summary":"S","labels":["a","b"],"fixVersions":[{"name":"1.0"}],
 "priority":{"name":"High"},"project":{"key":"ABC"},"issuetype":{"name":"Story"},
 "customfield_12601":"criteria text",
 "customfield_12602":{"value":"Feature","id":"1"},
 "customfield_12603":null,
 "customfield_12604":[{"value":"X","id":"2"}],
 "customfield_12605":3.0,
 "customfield_12606":[]
},"names":{"customfield_12601":"Acceptkriterie","customfield_12602":"Story Type"}}`

func fieldsServer(t *testing.T, bodies *[]map[string]interface{}, urls *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body)
			var m map[string]interface{}
			_ = json.Unmarshal(b, &m)
			*bodies = append(*bodies, m)
			w.WriteHeader(204)
			return
		}
		*urls = append(*urls, r.URL.String())
		_, _ = w.Write([]byte(fieldsIssueJSON))
	}))
}

func TestFetchExportsCustomFieldsProjectAndNames(t *testing.T) {
	var bodies []map[string]interface{}
	var urls []string
	srv := fieldsServer(t, &bodies, &urls)
	defer srv.Close()
	c := newCreateClient(srv.URL)
	ids := []string{"customfield_12601", "customfield_12602", "customfield_12603", "customfield_12604", "customfield_12605", "customfield_12606"}
	c.SetCustomFields(ids)
	issue, err := c.FetchIssue(context.Background(), "ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(urls[0], "expand=names") || !strings.Contains(urls[0], "customfield_12601") {
		t.Errorf("url %s", urls[0])
	}
	out := schema.RenderIssue(*issue)
	t.Log("\n" + out)
	for _, want := range []string{
		"project: 'project:ABC'", "priority: 'High'",
		"custom_field_names:\n  customfield_12601: 'Acceptkriterie'\n  customfield_12602: 'Story Type'\n",
		"custom_fields:\n  customfield_12601: criteria text\n  customfield_12602:\n      value: Feature\n  customfield_12604:\n      - value: X\n  customfield_12605: 3\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "12603") || strings.Contains(out, "12606") {
		t.Errorf("null/empty custom fields rendered:\n%s", out)
	}

	// Round trip: parse, edit, plan, sync, PUT body.
	local, perr := schema.ParseIssue(out)
	if perr != nil {
		t.Fatal(perr)
	}
	if ops, _, _ := plan.BuildPlan(local, *issue); len(ops) != 0 {
		t.Fatalf("unedited round trip has ops: %v", ops)
	}
	local.CustomFields["customfield_12601"] = "new criteria"
	local.Context.Priority = "Low"
	local.Labels = nil
	local.FixVersions = nil
	ops, _, _ := plan.BuildPlan(local, *issue)
	res := sync.Sync(local, *issue, ops, t.TempDir())
	if len(res.Conflicts) > 0 {
		t.Fatal(res.Conflicts)
	}
	if _, err := c.UpdateIssue(context.Background(), "ABC-1", res.Remote); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(bodies[0]["fields"])
	for _, want := range []string{`"labels":[]`, `"fixVersions":[]`, `"priority":{"name":"Low"}`, `"customfield_12601":"new criteria"`, `"customfield_12602":{"value":"Feature"}`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
}

func TestUpdateOmitsEmptyListsUnlessCleared(t *testing.T) {
	var bodies []map[string]interface{}
	var urls []string
	srv := fieldsServer(t, &bodies, &urls)
	defer srv.Close()
	c := newCreateClient(srv.URL)
	if _, err := c.UpdateIssue(context.Background(), "ABC-1", &schema.Issue{Summary: "s"}); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(bodies[0]["fields"])
	if strings.Contains(string(b), "labels") || strings.Contains(string(b), "fixVersions") || strings.Contains(string(b), "priority") {
		t.Errorf("unexpected fields %s", b)
	}
}

func TestCreateFieldsIncludePriority(t *testing.T) {
	i := draftIssue()
	i.Context.Priority = "Medium"
	b, _ := json.Marshal(CreateFields(i))
	if !strings.Contains(string(b), `"priority":{"name":"Medium"}`) {
		t.Errorf("%s", b)
	}
}

func TestSearchExpandsNamesAndExportsCustomFields(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"total":1,"issues":[{"key":"ABC-1","fields":{"customfield_12601":"x"}}],"names":{"customfield_12601":"Acceptkriterie"}}`))
	}))
	defer srv.Close()
	c := newCreateClient(srv.URL)
	c.SetCustomFields([]string{"customfield_12601"})
	issues, _, err := c.SearchIssues(context.Background(), "my-issues")
	if err != nil || len(issues) != 1 {
		t.Fatal(err)
	}
	if issues[0].CustomFields["customfield_12601"] != "x" || issues[0].Context.CustomFieldNames["customfield_12601"] != "Acceptkriterie" {
		t.Errorf("%+v", issues[0])
	}
	if got["expand"] == nil {
		t.Error("expand missing")
	}
}

func TestUpdateSendsPriorityOnlyWhenChanged(t *testing.T) {
	var bodies []map[string]interface{}
	var urls []string
	srv := fieldsServer(t, &bodies, &urls)
	defer srv.Close()
	c := newCreateClient(srv.URL)
	i := &schema.Issue{Summary: "s", Context: schema.IssueContext{Priority: "High"}}
	_, _ = c.UpdateIssue(context.Background(), "ABC-1", i)
	i.ClearFields = []string{"priority"}
	_, _ = c.UpdateIssue(context.Background(), "ABC-1", i)
	if _, ok := bodies[0]["fields"].(map[string]interface{})["priority"]; ok {
		t.Error("unchanged priority sent")
	}
	if _, ok := bodies[1]["fields"].(map[string]interface{})["priority"]; !ok {
		t.Error("changed priority not sent")
	}
}

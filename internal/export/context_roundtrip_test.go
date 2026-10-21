package export

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/jirafs/jirafs/internal/schema"
)

func loadFixtureIssue(t *testing.T, epicField string) *schema.Issue {
	t.Helper()
	data, err := os.ReadFile("testdata/issue_with_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var jr struct {
		Key    string                 `json:"key"`
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.Unmarshal(data, &jr); err != nil {
		t.Fatal(err)
	}
	issue := &schema.Issue{Identity: schema.IssueIdentity{
		Key:     schema.IssueKey(jr.Key),
		Type:    "Story",
		Project: schema.TypedRef{Type: schema.RefProject, Value: "SKOLELOGIN"},
	}}
	NormalizeIssue(issue, jr.Fields)
	NormalizeLinkedIssues(issue, jr.Fields)
	NormalizeContext(issue, jr.Fields, epicField)
	return issue
}

func TestExportIssue_ContextAndDescriptionRoundTrip(t *testing.T) {
	issue := loadFixtureIssue(t, "customfield_10014")
	out := ExportIssue(issue)
	t.Logf("rendered:\n%s", out)

	parsed, err := schema.ParseIssue(out)
	if err != nil {
		t.Fatalf("ParseIssue: %v", err)
	}
	if parsed.Description != issue.Description || parsed.Description == "" {
		t.Errorf("Description = %q, want %q", parsed.Description, issue.Description)
	}
	want := schema.IssueContext{
		Status: "In Progress", Priority: "High", Reporter: "Mette Hansen",
		Created: "2026-03-02T09:15:30.000+0100", Updated: "2026-04-11T16:42:05.000+0200",
		Components: []string{"Login", "Backend"}, Parent: "SKOLELOGIN-18000",
		EpicLink: "SKOLELOGIN-17000",
	}
	if !reflect.DeepEqual(issue.Context, want) {
		t.Errorf("normalized Context = %+v, want %+v", issue.Context, want)
	}
	if !reflect.DeepEqual(parsed.Context, want) {
		t.Errorf("parsed Context = %+v, want %+v", parsed.Context, want)
	}
	if parsed.Status != "" {
		t.Errorf("editable Status must stay empty (read-only context), got %q", parsed.Status)
	}
}

func TestNormalizeContext_EpicLinkOnlyWhenConfigured(t *testing.T) {
	issue := loadFixtureIssue(t, "")
	if issue.Context.EpicLink != "" {
		t.Errorf("EpicLink = %q, want empty when field unconfigured", issue.Context.EpicLink)
	}
}

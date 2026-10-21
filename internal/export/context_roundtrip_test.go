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

func TestExportIssue_LinksFixVersionsSubtasksRoundTrip(t *testing.T) {
	issue := loadFixtureIssue(t, "customfield_10014")
	out := ExportIssue(issue)
	t.Logf("rendered:\n%s", out)

	wantLinks := []schema.LinkedIssue{
		{Key: "SKOLELOGIN-18400", Relation: "blocks", Summary: "Release gate", Status: "Open"},
		{Key: "SKOLELOGIN-18100", Relation: "is blocked by", Summary: "Upgrade IdP", Status: "In Review"},
	}
	wantSubs := []schema.Subtask{
		{Key: "SKOLELOGIN-18313", Summary: "Fix surname parser", Status: "Done"},
		{Key: "SKOLELOGIN-18314", Summary: "Add regression test", Status: "To Do"},
	}
	wantVersions := []string{"2026.1", "2026.2"}

	parsed, err := schema.ParseIssue(out)
	if err != nil {
		t.Fatalf("ParseIssue: %v", err)
	}
	for name, got := range map[string]*schema.Issue{"normalized": issue, "parsed": &parsed} {
		if !reflect.DeepEqual(got.LinkedIssues, wantLinks) {
			t.Errorf("%s LinkedIssues = %+v, want %+v", name, got.LinkedIssues, wantLinks)
		}
		if !reflect.DeepEqual(got.Subtasks, wantSubs) {
			t.Errorf("%s Subtasks = %+v, want %+v", name, got.Subtasks, wantSubs)
		}
		if !reflect.DeepEqual(got.FixVersions, wantVersions) {
			t.Errorf("%s FixVersions = %v, want %v", name, got.FixVersions, wantVersions)
		}
	}
}

func TestNormalizeLinkedIssues_SameKeyDifferentRelationKept(t *testing.T) {
	fields := map[string]interface{}{"issuelinks": []interface{}{
		map[string]interface{}{
			"type":         map[string]interface{}{"name": "Blocks", "outward": "blocks", "inward": "is blocked by"},
			"outwardIssue": map[string]interface{}{"key": "P-1"},
		},
		map[string]interface{}{
			"type":  map[string]interface{}{"name": "Blocks", "outward": "blocks", "inward": "is blocked by"},
			"issue": map[string]interface{}{"key": "P-1"},
		},
		map[string]interface{}{
			"type":         map[string]interface{}{"name": "Blocks", "outward": "blocks", "inward": "is blocked by"},
			"outwardIssue": map[string]interface{}{"key": "P-1"},
		},
	}}
	issue := &schema.Issue{}
	NormalizeLinkedIssues(issue, fields)
	want := []schema.LinkedIssue{
		{Key: "P-1", Relation: "blocks"},
		{Key: "P-1", Relation: "is blocked by"},
	}
	if !reflect.DeepEqual(issue.LinkedIssues, want) {
		t.Errorf("LinkedIssues = %+v, want %+v", issue.LinkedIssues, want)
	}
}

func TestNormalizeIssue_SubtasksAndFixVersionsEdgeCases(t *testing.T) {
	issue := &schema.Issue{}
	NormalizeIssue(issue, map[string]interface{}{
		"fixVersions": []interface{}{map[string]interface{}{"name": "1.0"}, map[string]interface{}{}},
		"subtasks":    []interface{}{"junk", map[string]interface{}{}, map[string]interface{}{"key": "P-9"}},
	})
	if !reflect.DeepEqual(issue.FixVersions, []string{"1.0"}) {
		t.Errorf("FixVersions = %v", issue.FixVersions)
	}
	if !reflect.DeepEqual(issue.Subtasks, []schema.Subtask{{Key: "P-9"}}) {
		t.Errorf("Subtasks = %+v", issue.Subtasks)
	}
}

package export

import (
	"testing"
	"time"

	"github.com/jirafs/jirafs/internal/schema"
)

func TestNormalizeIssue(t *testing.T) {
	type args struct {
		fields map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    *schema.Issue
	}{
		{
			name: "all fields present",
			args: args{
				fields: map[string]interface{}{
					"summary":     "Test issue summary",
					"description": "Test issue description",
					"labels":      []interface{}{"bug", "urgent"},
					"assignee": map[string]interface{}{
						"name": "alice",
					},
				},
			},
			want: &schema.Issue{
				Summary:     "Test issue summary",
				Description: "Test issue description",
				Labels:      []string{"bug", "urgent"},
				Assignee:    ptrString("alice"),
			},
		},
		{
			name: "missing fields",
			args: args{
				fields: map[string]interface{}{},
			},
			want: &schema.Issue{},
		},
		{
			name: "nil fields",
			args: args{
				fields: nil,
			},
			want: &schema.Issue{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &schema.Issue{}
			NormalizeIssue(issue, tt.args.fields)
			if issue.Summary != tt.want.Summary {
				t.Errorf("Summary = %q, want %q", issue.Summary, tt.want.Summary)
			}
			if issue.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", issue.Description, tt.want.Description)
			}
			if len(issue.Labels) != len(tt.want.Labels) {
				t.Errorf("Labels len = %d, want %d", len(issue.Labels), len(tt.want.Labels))
				return
			}
			for i := range issue.Labels {
				if issue.Labels[i] != tt.want.Labels[i] {
					t.Errorf("Labels[%d] = %q, want %q", i, issue.Labels[i], tt.want.Labels[i])
				}
			}
			if (issue.Assignee == nil) != (tt.want.Assignee == nil) {
				t.Errorf("Assignee nil = %v, want %v", issue.Assignee == nil, tt.want.Assignee == nil)
				return
			}
			if issue.Assignee != nil && tt.want.Assignee != nil && *issue.Assignee != *tt.want.Assignee {
				t.Errorf("Assignee = %q, want %q", *issue.Assignee, *tt.want.Assignee)
			}
		})
	}
}

func ptrString(s string) *string {
	return &s
}

func TestExportIssue(t *testing.T) {
	tests := []struct {
		name    string
		issue   schema.Issue
		wantLen int
	}{
		{
			name: "synced issue with all fields",
			issue: schema.Issue{
				Identity: schema.IssueIdentity{
					Key:     "PROJ-42",
					Type:    "story",
					Project: schema.TypedRef{Type: schema.RefProject, Value: "PROJ"},
				},
				MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
				RemoteMetadata: schema.RemoteMetadata{
					StateFile:     "synced",
					RemoteVersion: "7",
					ContentHash:   "abc123",
					SyncTime:      mustParseTime("2026-06-21T14:30:00Z"),
				},
				Summary:    "Test summary",
				Labels:     []string{"bug", "urgent"},
				Assignee:   ptrString("alice"),
				LinkedIssues: []schema.LinkedIssue{
					{Key: "PROJ-1", Type: "blocks"},
					{Key: "PROJ-2", Type: "relates to"},
				},
			},
			wantLen: 1,
		},
		{
			name: "draft issue minimal",
			issue: schema.Issue{
				Identity: schema.IssueIdentity{
					Key:     "DRF-1",
					Type:    "task",
					Project: schema.TypedRef{Type: schema.RefProject, Value: "DRF"},
				},
				MachineOwned: schema.MachineOwned{SchemaVersion: "1"},
			},
			wantLen: 1,
		},
		{
			name: "empty issue produces frontmatter with key and type",
			issue: schema.Issue{
				Identity: schema.IssueIdentity{
					Key:     "EMPTY-1",
					Type:    "task",
					Project: schema.TypedRef{Type: schema.RefProject, Value: "EMPTY"},
				},
			},
			wantLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportIssue(&tt.issue)
			if got == "" {
				t.Fatal("ExportIssue returned empty string")
			}
			// Verify round-trip: parse the rendered output back.
			parsed, pe := schema.ParseIssue(got)
			if pe != nil {
				t.Fatalf("ParseIssue failed on exported output: %s", pe.Error())
			}
			// Identity must survive round-trip.
			if parsed.Identity.Key != tt.issue.Identity.Key {
				t.Errorf("key = %q, want %q", parsed.Identity.Key, tt.issue.Identity.Key)
			}
			if parsed.Identity.Type != tt.issue.Identity.Type {
				t.Errorf("type = %q, want %q", parsed.Identity.Type, tt.issue.Identity.Type)
			}
			if parsed.Identity.Project != tt.issue.Identity.Project {
				t.Errorf("project = %v, want %v", parsed.Identity.Project, tt.issue.Identity.Project)
			}
			if parsed.MachineOwned.SchemaVersion != tt.issue.MachineOwned.SchemaVersion {
				t.Errorf("schema_version = %q, want %q",
					parsed.MachineOwned.SchemaVersion, tt.issue.MachineOwned.SchemaVersion)
			}
			if parsed.Summary != tt.issue.Summary {
				t.Errorf("summary = %q, want %q", parsed.Summary, tt.issue.Summary)
			}
			if len(parsed.Labels) != len(tt.issue.Labels) {
				t.Errorf("labels len = %d, want %d", len(parsed.Labels), len(tt.issue.Labels))
			}
			for i := range parsed.Labels {
				if parsed.Labels[i] != tt.issue.Labels[i] {
					t.Errorf("labels[%d] = %q, want %q", i, parsed.Labels[i], tt.issue.Labels[i])
				}
			}
			if (parsed.Assignee == nil) != (tt.issue.Assignee == nil) ||
				(parsed.Assignee != nil && tt.issue.Assignee != nil && *parsed.Assignee != *tt.issue.Assignee) {
				t.Errorf("assignee = %v, want %v", parsed.Assignee, tt.issue.Assignee)
			}
			if len(parsed.LinkedIssues) != len(tt.issue.LinkedIssues) {
				t.Errorf("linked_issues len = %d, want %d", len(parsed.LinkedIssues), len(tt.issue.LinkedIssues))
			}
			for i := range parsed.LinkedIssues {
				if parsed.LinkedIssues[i].Key != tt.issue.LinkedIssues[i].Key {
					t.Errorf("linked_issues[%d].key = %q, want %q",
						i, parsed.LinkedIssues[i].Key, tt.issue.LinkedIssues[i].Key)
				}
				if parsed.LinkedIssues[i].Type != tt.issue.LinkedIssues[i].Type {
					t.Errorf("linked_issues[%d].type = %q, want %q",
						i, parsed.LinkedIssues[i].Type, tt.issue.LinkedIssues[i].Type)
				}
			}
			// Remote metadata round-trip.
			if parsed.RemoteMetadata.StateFile != tt.issue.RemoteMetadata.StateFile {
				t.Errorf("state = %q, want %q", parsed.RemoteMetadata.StateFile, tt.issue.RemoteMetadata.StateFile)
			}
			if parsed.RemoteMetadata.RemoteVersion != tt.issue.RemoteMetadata.RemoteVersion {
				t.Errorf("remote_version = %q, want %q",
					parsed.RemoteMetadata.RemoteVersion, tt.issue.RemoteMetadata.RemoteVersion)
			}
			if parsed.RemoteMetadata.ContentHash != tt.issue.RemoteMetadata.ContentHash {
				t.Errorf("content_hash = %q, want %q",
					parsed.RemoteMetadata.ContentHash, tt.issue.RemoteMetadata.ContentHash)
			}
		})
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNormalizeLinkedIssues(t *testing.T) {
	type args struct {
		fields map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantLen int
		wantKeys []string
		wantTypes []string
	}{
		{
			name: "nil fields",
			args: args{fields: nil},
			wantLen: 0,
		},
		{
			name: "no issuelinks",
			args: args{fields: map[string]interface{}{}},
			wantLen: 0,
		},
		{
			name: "empty issuelinks",
			args: args{fields: map[string]interface{}{"issuelinks": []interface{}{}}},
			wantLen: 0,
		},
		{
			name: "outward link",
			args: args{fields: map[string]interface{}{
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"name": "blocks"},
						"outwardIssue": map[string]interface{}{"key": "PROJ-456"},
					},
				},
			}},
			wantLen:   1,
			wantKeys:  []string{"PROJ-456"},
			wantTypes: []string{"blocks"},
		},
		{
			name: "inward link",
			args: args{fields: map[string]interface{}{
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"name": "is blocked by"},
						"issue": map[string]interface{}{"key": "PROJ-789"},
					},
				},
			}},
			wantLen:   1,
			wantKeys:  []string{"PROJ-789"},
			wantTypes: []string{"is blocked by"},
		},
		{
			name: "multiple links dedup by key",
			args: args{fields: map[string]interface{}{
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"name": "blocks"},
						"outwardIssue": map[string]interface{}{"key": "PROJ-456"},
					},
					map[string]interface{}{
						"type": map[string]interface{}{"name": "is blocked by"},
						"issue": map[string]interface{}{"key": "PROJ-456"},
					},
				},
			}},
			wantLen:   1,
			wantKeys:  []string{"PROJ-456"},
			wantTypes: []string{"blocks"},
		},
		{
			name: "multiple distinct links",
			args: args{fields: map[string]interface{}{
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"name": "relates to"},
						"outwardIssue": map[string]interface{}{"key": "PROJ-100"},
					},
					map[string]interface{}{
						"type": map[string]interface{}{"name": "blocks"},
						"outwardIssue": map[string]interface{}{"key": "PROJ-200"},
					},
				},
			}},
			wantLen:   2,
			wantKeys:  []string{"PROJ-100", "PROJ-200"},
			wantTypes: []string{"relates to", "blocks"},
		},
		{
			name: "missing key is skipped",
			args: args{fields: map[string]interface{}{
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"name": "relates to"},
					},
				},
			}},
			wantLen: 0,
		},
		{
			name: "missing link type defaults to empty",
			args: args{fields: map[string]interface{}{
				"issuelinks": []interface{}{
					map[string]interface{}{
						"outwardIssue": map[string]interface{}{"key": "PROJ-999"},
					},
				},
			}},
			wantLen:   1,
			wantKeys:  []string{"PROJ-999"},
			wantTypes: []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &schema.Issue{}
			NormalizeLinkedIssues(issue, tt.args.fields)
			if len(issue.LinkedIssues) != tt.wantLen {
				t.Errorf("LinkedIssues len = %d, want %d; got %#v", len(issue.LinkedIssues), tt.wantLen, issue.LinkedIssues)
				return
			}
			for i := range tt.wantKeys {
				if string(issue.LinkedIssues[i].Key) != tt.wantKeys[i] {
					t.Errorf("LinkedIssues[%d].Key = %q, want %q", i, issue.LinkedIssues[i].Key, tt.wantKeys[i])
				}
				if issue.LinkedIssues[i].Type != tt.wantTypes[i] {
					t.Errorf("LinkedIssues[%d].Type = %q, want %q", i, issue.LinkedIssues[i].Type, tt.wantTypes[i])
				}
			}
		})
	}
}

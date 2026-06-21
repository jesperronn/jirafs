package export

import (
	"testing"

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

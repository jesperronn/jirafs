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

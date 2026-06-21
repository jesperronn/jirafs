package export

import "github.com/jirafs/jirafs/internal/schema"

// NormalizeIssue extracts summary, description, labels, and assignee from a
// Jira fields map and writes them into the issue model.
func NormalizeIssue(issue *schema.Issue, fields map[string]interface{}) {
	if fields == nil {
		return
	}

	if s, ok := fields["summary"].(string); ok {
		issue.Summary = s
	}

	if d, ok := fields["description"].(string); ok {
		issue.Description = d
	}

	if labels, ok := fields["labels"].([]interface{}); ok {
		ls := make([]string, 0, len(labels))
		for _, l := range labels {
			if s, ok := l.(string); ok {
				ls = append(ls, s)
			}
		}
		issue.Labels = ls
	}

	if assignee, ok := fields["assignee"].(map[string]interface{}); ok {
		if name, ok := assignee["name"].(string); ok {
			issue.Assignee = &name
		}
	}
}

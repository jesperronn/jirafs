package export

import (
	"github.com/jirafs/jirafs/internal/schema"
)

// NormalizeLinkedIssues extracts linked issue references from a Jira fields
// map and writes them into issue.LinkedIssues.
//
// Jira returns linked issues under the "issuelinks" key as an array of link
// objects. Each link may have an "outwardIssue" or "issue" field pointing to
// the target, plus a "type" object with the link name.
func NormalizeLinkedIssues(issue *schema.Issue, fields map[string]interface{}) {
	if fields == nil {
		return
	}

	links, ok := fields["issuelinks"].([]interface{})
	if !ok || len(links) == 0 {
		issue.LinkedIssues = []schema.LinkedIssue{}
		return
	}

	// Deduplicate by target key to avoid counting the same link twice
	// (one outward + one inward entry per relationship).
	seen := make(map[string]bool)
	result := make([]schema.LinkedIssue, 0)

	for _, raw := range links {
		link, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		// Determine the target issue from outward or inward direction.
		var targetKey string
		if outward, ok := link["outwardIssue"].(map[string]interface{}); ok {
			if key, ok := outward["key"].(string); ok {
				targetKey = key
			}
		} else if inward, ok := link["issue"].(map[string]interface{}); ok {
			if key, ok := inward["key"].(string); ok {
				targetKey = key
			}
		}
		if targetKey == "" {
			continue
		}

		// Skip duplicates (same target key = same relationship).
		if seen[targetKey] {
			continue
		}
		seen[targetKey] = true

		// Extract the link type name.
		var linkType string
		if lt, ok := link["type"].(map[string]interface{}); ok {
			if name, ok := lt["name"].(string); ok {
				linkType = name
			}
		}

		result = append(result, schema.LinkedIssue{
			Key:  schema.IssueKey(targetKey),
			Type: linkType,
		})
	}

	issue.LinkedIssues = result
}

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

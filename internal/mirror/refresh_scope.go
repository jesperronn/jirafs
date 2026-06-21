package mirror

import (
	"context"

	"github.com/jirafs/jirafs/internal/jira"
	"github.com/jirafs/jirafs/internal/schema"
)

// RefreshScope searches Jira for issues matching the given scope and adds
// each result as a ScopeMember in the mirror.
//
// The function is idempotent: if a key is already a scope member it is
// skipped. Linked issues are kept shallow (key + type only).
//
// Returns the keys of newly added scope members.
func RefreshScope(ctx context.Context, c jira.Client, scope Scope, mirror *Mirror) ([]schema.IssueKey, error) {
	if scope.IsZero() {
		return nil, nil
	}

	issues, err := c.SearchIssues(ctx, scope.Name)
	if err != nil {
		return nil, err
	}

	added := make([]schema.IssueKey, 0, len(issues))
	for _, iss := range issues {
		if iss == nil {
			continue
		}
		if iss.Identity.Key == "" {
			continue
		}
		// Keep linked issues shallow: only key + type, no resolution.
		_ = iss.LinkedIssues // shallow by design

		if mirror.AddScopeMember(ScopeMember{
			Key:   iss.Identity.Key,
			Scope: scope.Name,
		}) {
			added = append(added, iss.Identity.Key)
		}
	}

	return added, nil
}

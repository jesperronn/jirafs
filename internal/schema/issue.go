package schema

// IssueKey is the machine-issued identifier for a Jira issue
// (e.g. "ABC-123").
type IssueKey string

// IssueType represents the type of Jira issue (e.g. "story", "bug", "task").
type IssueType string

// IssueIdentity holds the immutable identity fields of a Jira issue.
type IssueIdentity struct {
	Key     IssueKey `yaml:"key"`
	Type    IssueType `yaml:"type"`
	Project TypedRef `yaml:"project"`
}

// IsZero reports whether i has no identity set.
func (i IssueIdentity) IsZero() bool {
	return i.Key == "" && i.Type == "" && i.Project.IsZero()
}

// MachineOwned holds fields owned by the jirafs system in issue frontmatter.
type MachineOwned struct {
	SchemaVersion string `yaml:"schema_version"`
}

// IsZero reports whether m has no machine-owned fields set.
func (m MachineOwned) IsZero() bool {
	return m.SchemaVersion == ""
}

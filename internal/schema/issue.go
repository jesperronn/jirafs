package schema

// EditableField represents a field that can be edited by the user in an
// issue file.
type EditableField string

const (
	EditableFieldSummary     EditableField = "summary"
	EditableFieldDescription EditableField = "description"
	EditableFieldLabels      EditableField = "labels"
	EditableFieldAssignee    EditableField = "assignee"
	EditableFieldParent      EditableField = "parent"
	EditableFieldEpic        EditableField = "epic"
	EditableFieldFixVersions EditableField = "fix_versions"
	EditableFieldSprint      EditableField = "sprint"
)

// PermissionCategory represents a permission category for issue fields.
type PermissionCategory string

const (
	PermissionEditable   PermissionCategory = "editable"
	PermissionAppendOnly PermissionCategory = "append_only"
	PermissionReadOnly   PermissionCategory = "read_only"
)

// PermissionModel defines which fields are editable, append-only, or read-only.
type PermissionModel struct {
	Editable   []EditableField
	AppendOnly []string
	ReadOnly   []string
}

// FixedSectionName represents a fixed section name in an issue file.
type FixedSectionName string

const (
	SecDescription        FixedSectionName = "Description"
	SecAcceptanceCriteria FixedSectionName = "Acceptance Criteria"
	SecDefinitionOfReady  FixedSectionName = "Definition of Ready"
	SecNotes              FixedSectionName = "Notes"
	SecCommentsToAdd      FixedSectionName = "Comments To Add"
	SecRemoteComments     FixedSectionName = "Remote Comments"
)

// AllFixedSections returns all known fixed section names in canonical order.
func AllFixedSections() []FixedSectionName {
	return []FixedSectionName{
		SecDescription,
		SecAcceptanceCriteria,
		SecDefinitionOfReady,
		SecNotes,
		SecCommentsToAdd,
		SecRemoteComments,
	}
}

// IsKnown reports whether s is a recognized fixed section name.
func (s FixedSectionName) IsKnown() bool {
	for _, known := range AllFixedSections() {
		if s == known {
			return true
		}
	}
	return false
}

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

// LinkedIssue is a shallow reference to a linked Jira issue.
type LinkedIssue struct {
	// Key is the linked issue's key (e.g. "PROJ-456").
	Key IssueKey `yaml:"key"`
	// Type is the Jira link type (e.g. "blocks", "is blocked by", "relates to").
	Type string `yaml:"type"`
}

// Issue represents a parsed issue file with identity, machine-owned fields,
// remote metadata, and section content.
type Issue struct {
	Identity       IssueIdentity
	MachineOwned   MachineOwned
	RemoteMetadata RemoteMetadata
	Summary        string
	Description    string
	Labels         []string
	Assignee       *string
	LinkedIssues   []LinkedIssue `yaml:"linked_issues,omitempty"`
	// Sections holds the body sections keyed by their fixed section name.
	// Only populated by ParseIssue when section parsing is enabled.
	Sections map[FixedSectionName]string
}

// IsZero reports whether i has no identity set.
func (i Issue) IsZero() bool {
	return i.Identity.IsZero() && i.MachineOwned.IsZero()
}

// Package mirror defines mirror membership models and archive eligibility rules.
package mirror

import (
	"github.com/jirafs/jirafs/internal/schema"
)

// ImportReason represents why an issue was explicitly imported into a mirror.
type ImportReason string

const (
	// ImportReasonManual means the user added the issue by hand.
	ImportReasonManual ImportReason = "manual"
	// ImportReasonDependency means the issue is a dependency of an in-scope
	// issue.
	ImportReasonDependency ImportReason = "dependency"
	// ImportReasonEpicMember means the issue is linked to an epic that is
	// in scope.
	ImportReasonEpicMember ImportReason = "epic_member"
	// ImportReasonSprint means the issue is assigned to a sprint that is
	// in scope.
	ImportReasonSprint ImportReason = "sprint"
)

// ValidImportReasons returns the set of all recognized import reasons.
var ValidImportReasons = []ImportReason{
	ImportReasonManual,
	ImportReasonDependency,
	ImportReasonEpicMember,
	ImportReasonSprint,
}

// IsValidImportReason reports whether r is a known import reason.
func IsValidImportReason(r ImportReason) bool {
	for _, v := range ValidImportReasons {
		if r == v {
			return true
		}
	}
	return false
}

// ImportedIssue represents an issue that has been explicitly imported into
// a mirror, along with the explainable reason for membership.
type ImportedIssue struct {
	// Key is the issue key (e.g. "PROJ-123").
	Key schema.IssueKey `yaml:"key"`
	// Reason is why this issue was imported into the mirror.
	Reason ImportReason `yaml:"reason"`
}

// IsZero reports whether i has no identity set.
func (i ImportedIssue) IsZero() bool {
	return i.Key == "" && i.Reason == ""
}

// String renders the imported issue as "key (reason)".
func (i ImportedIssue) String() string {
	return string(i.Key) + " (" + string(i.Reason) + ")"
}

// Mirror represents a set of explicitly imported issues for a project.
type Mirror struct {
	// Project is the project key this mirror belongs to.
	Project schema.TypedRef `yaml:"project"`
	// Issues are the explicitly imported issues with their membership reasons.
	Issues []ImportedIssue `yaml:"issues,omitempty"`
}

// IsZero reports whether m has no identity set.
func (m Mirror) IsZero() bool {
	return m.Project.IsZero() && len(m.Issues) == 0
}

// HasIssue reports whether the mirror explicitly imports the given issue key.
func (m Mirror) HasIssue(key schema.IssueKey) bool {
	for _, imp := range m.Issues {
		if imp.Key == key {
			return true
		}
	}
	return false
}

// ImportReasonFor returns the import reason for the given issue key, or an
// empty string if the key is not imported.
func (m Mirror) ImportReasonFor(key schema.IssueKey) ImportReason {
	for _, imp := range m.Issues {
		if imp.Key == key {
			return imp.Reason
		}
	}
	return ""
}

// Package mirror defines mirror membership models and archive eligibility rules.
package mirror

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jirafs/jirafs/internal/schema"
)

// UnmarshalMirror parses YAML data into a Mirror.
func UnmarshalMirror(data []byte) (*Mirror, error) {
	var m Mirror
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

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

// ScopeType represents the kind of scope filter.
type ScopeType string

const (
	// ScopeTypeProject means the scope is a project-level filter.
	ScopeTypeProject ScopeType = "project"
	// ScopeTypeComponent means the scope is a component-level filter.
	ScopeTypeComponent ScopeType = "component"
	// ScopeTypeJQL means the scope is a JQL query.
	ScopeTypeJQL ScopeType = "jql"
)

// ValidScopeTypes returns the set of all recognized scope types.
var ValidScopeTypes = []ScopeType{
	ScopeTypeProject,
	ScopeTypeComponent,
	ScopeTypeJQL,
}

// IsValidScopeType reports whether t is a known scope type.
func IsValidScopeType(t ScopeType) bool {
	for _, v := range ValidScopeTypes {
		if t == v {
			return true
		}
	}
	return false
}

// Scope represents a named filter that defines a set of issues.
type Scope struct {
	// Name is the human-readable scope name.
	Name string `yaml:"name"`
	// Type is the kind of filter this scope uses.
	Type ScopeType `yaml:"type"`
	// Target is the filter target: a project key, component name, or JQL
	// string depending on the scope type.
	Target string `yaml:"target"`
	// Project is the project key a component scope belongs to. It is only
	// used by component scopes.
	Project string `yaml:"project,omitempty"`
	// LastSynced is the RFC3339 time a refresh of this scope last finished
	// without failures. It is the cursor for incremental refreshes.
	LastSynced string `yaml:"last_synced,omitempty"`
}

// cursorOverlap is subtracted from the cursor to tolerate clock skew between
// this machine and Jira.
const cursorOverlap = time.Minute

// jqlTimeLayout is the date format JQL accepts for "updated >= ...".
const jqlTimeLayout = "2006/01/02 15:04"

// Cursor returns the parsed LastSynced time. ok is false when no valid
// cursor is recorded.
func (s Scope) Cursor() (time.Time, bool) {
	if s.LastSynced == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s.LastSynced)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// SearchJQL returns the JQL a refresh sends: the scope JQL, narrowed to
// issues updated since the cursor (minus a one minute overlap) when the scope
// has one. Any trailing ORDER BY clause is kept outside the wrapper.
func (s Scope) SearchJQL() (string, error) {
	jql, err := s.JQL()
	if err != nil {
		return "", err
	}
	cursor, ok := s.Cursor()
	if !ok {
		return jql, nil
	}
	cond, order := splitOrderBy(jql)
	since := cursor.Add(-cursorOverlap).Local().Format(jqlTimeLayout)
	out := "(" + cond + ") AND updated >= \"" + since + "\""
	if order != "" {
		out += " " + order
	}
	return out, nil
}

// splitOrderBy separates a trailing ORDER BY clause from the JQL condition.
func splitOrderBy(jql string) (cond, order string) {
	lower := strings.ToLower(jql)
	idx := strings.LastIndex(lower, " order by ")
	if idx < 0 {
		return strings.TrimSpace(jql), ""
	}
	return strings.TrimSpace(jql[:idx]), strings.TrimSpace(jql[idx:])
}

// BuiltinScopeJQL returns the JQL for a built-in scope name that works
// without a matching entry in mirror.yml.
func BuiltinScopeJQL(name string) (string, bool) {
	switch name {
	case "my-issues":
		return "assignee = currentUser()", true
	case "current-sprint":
		return "sprint in openSprints()", true
	default:
		return "", false
	}
}

// BuiltinScope returns the implicit scope for a built-in name.
func BuiltinScope(name string) (Scope, bool) {
	jql, ok := BuiltinScopeJQL(name)
	if !ok {
		return Scope{}, false
	}
	return Scope{Name: name, Type: ScopeTypeJQL, Target: jql}, true
}

func jqlQuote(v string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
}

// JQL builds the JQL query that selects the issues of the scope.
func (s Scope) JQL() (string, error) {
	if strings.TrimSpace(s.Target) == "" {
		return "", fmt.Errorf("scope %q: target must not be empty", s.Name)
	}
	switch s.Type {
	case ScopeTypeProject:
		return "project = " + jqlQuote(s.Target) + " ORDER BY key", nil
	case ScopeTypeJQL:
		return s.Target, nil
	case ScopeTypeComponent:
		if strings.TrimSpace(s.Project) == "" {
			return "", fmt.Errorf("scope %q: component scopes need a project (add \"project: KEY\")", s.Name)
		}
		return "project = " + jqlQuote(s.Project) + " AND component = " + jqlQuote(s.Target) + " ORDER BY key", nil
	default:
		return "", fmt.Errorf("scope %q: unsupported type %q", s.Name, s.Type)
	}
}

// IsZero reports whether s has no identity set.
func (s Scope) IsZero() bool {
	return s.Name == "" && s.Type == "" && s.Target == ""
}

// ScopeMember represents an issue that is a member of a named scope.
type ScopeMember struct {
	// Key is the issue key (e.g. "PROJ-123").
	Key schema.IssueKey `yaml:"key"`
	// Scope is the name of the scope this issue belongs to.
	Scope string `yaml:"scope"`
}

// IsZero reports whether s has no identity set.
func (s ScopeMember) IsZero() bool {
	return s.Key == "" && s.Scope == ""
}

// String renders the scope member as "key (@scope)".
func (s ScopeMember) String() string {
	return string(s.Key) + " (@" + s.Scope + ")"
}

// Mirror represents a set of explicitly imported issues and named scope
// memberships for a project.
type Mirror struct {
	// Project is the project key this mirror belongs to.
	Project schema.TypedRef `yaml:"project"`
	// Issues are the explicitly imported issues with their membership reasons.
	Issues []ImportedIssue `yaml:"issues,omitempty"`
	// Scopes are the named scope filters for this mirror.
	Scopes []Scope `yaml:"scopes,omitempty"`
	// ScopeMembers are the issues that are members of named scopes.
	ScopeMembers []ScopeMember `yaml:"scope_members,omitempty"`
	// RemoteUpdated records, per issue key, the Jira "updated" timestamp seen
	// at the last export, refresh or sync. status compares it with the local
	// file to detect that Jira moved on, without any network access.
	RemoteUpdated map[schema.IssueKey]string `yaml:"remote_updated,omitempty"`
	// Git marks the mirror as git-backed (`git: true` in mirror.yml). It
	// defaults to false and is written by `jirafs init --git`.
	Git bool `yaml:"git,omitempty"`
}

// SetScopeLastSynced records t as the cursor of scope. A scope that is not in
// the mirror yet (a built-in one) is appended so its cursor persists. The
// cursor is stored in UTC.
func (m *Mirror) SetScopeLastSynced(scope Scope, t time.Time) {
	stamp := t.UTC().Format(time.RFC3339)
	for i := range m.Scopes {
		if m.Scopes[i].Name == scope.Name {
			m.Scopes[i].LastSynced = stamp
			return
		}
	}
	scope.LastSynced = stamp
	m.Scopes = append(m.Scopes, scope)
}

// RemoteUpdatedFor returns the stored remote updated timestamp for key, or "".
func (m Mirror) RemoteUpdatedFor(key schema.IssueKey) string {
	return m.RemoteUpdated[key]
}

// SetRemoteUpdated records the remote updated timestamp for key. An empty
// timestamp is ignored.
func (m *Mirror) SetRemoteUpdated(key schema.IssueKey, updated string) {
	if key == "" || updated == "" {
		return
	}
	if m.RemoteUpdated == nil {
		m.RemoteUpdated = map[schema.IssueKey]string{}
	}
	m.RemoteUpdated[key] = updated
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

// HasScope reports whether the mirror defines a scope with the given name.
func (m Mirror) HasScope(name string) bool {
	for _, s := range m.Scopes {
		if s.Name == name {
			return true
		}
	}
	return false
}

// ScopeFor returns the scope with the given name, or a zero Scope if not found.
func (m Mirror) ScopeFor(name string) Scope {
	for _, s := range m.Scopes {
		if s.Name == name {
			return s
		}
	}
	return Scope{}
}

// ScopeTypeFor returns the type of the scope with the given name, or an empty
// string if not found.
func (m Mirror) ScopeTypeFor(name string) ScopeType {
	for _, s := range m.Scopes {
		if s.Name == name {
			return s.Type
		}
	}
	return ""
}

// HasScopeMember reports whether the given issue key is a member of any scope.
func (m Mirror) HasScopeMember(key schema.IssueKey) bool {
	for _, mem := range m.ScopeMembers {
		if mem.Key == key {
			return true
		}
	}
	return false
}

// ScopeMemberFor returns the scope name for the given issue key, or an empty
// string if the key is not a scope member.
func (m Mirror) ScopeMemberFor(key schema.IssueKey) string {
	for _, mem := range m.ScopeMembers {
		if mem.Key == key {
			return mem.Scope
		}
	}
	return ""
}

// AddScope adds a scope to the mirror if it does not already exist.
// It returns true if the scope was added, false if it already existed.
func (m *Mirror) AddScope(s Scope) bool {
	if s.IsZero() {
		return false
	}
	if m.HasScope(s.Name) {
		return false
	}
	m.Scopes = append(m.Scopes, s)
	return true
}

// ResolvedStatus represents the resolved state of a Jira issue.
type ResolvedStatus string

const (
	// ResolvedStatusResolved means the issue has been resolved in Jira.
	ResolvedStatusResolved ResolvedStatus = "resolved"
	// ResolvedStatusOpen means the issue is still open in Jira.
	ResolvedStatusOpen ResolvedStatus = "open"
)

// ValidResolvedStatuses returns the set of all recognized resolved statuses.
var ValidResolvedStatuses = []ResolvedStatus{
	ResolvedStatusResolved,
	ResolvedStatusOpen,
}

// IsValidResolvedStatus reports whether s is a known resolved status.
func IsValidResolvedStatus(s ResolvedStatus) bool {
	for _, v := range ValidResolvedStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// ArchiveEligible represents an issue that is eligible for archiving.
type ArchiveEligible struct {
	// Key is the issue key (e.g. "PROJ-123").
	Key schema.IssueKey `yaml:"key"`
	// ResolvedStatus is the Jira resolved status that made this issue eligible.
	ResolvedStatus ResolvedStatus `yaml:"resolved_status"`
}

// IsZero reports whether a has no identity set.
func (a ArchiveEligible) IsZero() bool {
	return a.Key == "" && a.ResolvedStatus == ""
}

// String renders the archive-eligible issue as "key (resolved_status)".
func (a ArchiveEligible) String() string {
	return string(a.Key) + " (" + string(a.ResolvedStatus) + ")"
}

// IsArchiveEligible reports whether the given issue key is eligible for
// archiving: it must be out of scope (not explicitly imported and not a scope
// member), its resolved status must be ResolvedStatusResolved, it must not be
// pinned, and it must have remote metadata (not unsynced).
func (m Mirror) IsArchiveEligible(key schema.IssueKey, resolved ResolvedStatus, remote schema.RemoteMetadata) bool {
	if m.HasIssue(key) {
		return false
	}
	if m.HasScopeMember(key) {
		return false
	}
	if remote.Pinned {
		return false
	}
	if remote.IsZero() {
		return false
	}
	return resolved == ResolvedStatusResolved
}

// RemoveScopeMember removes any scope membership of key. It reports whether
// a member was removed.
func (m *Mirror) RemoveScopeMember(key schema.IssueKey) bool {
	kept := m.ScopeMembers[:0]
	removed := false
	for _, mem := range m.ScopeMembers {
		if mem.Key == key {
			removed = true
			continue
		}
		kept = append(kept, mem)
	}
	m.ScopeMembers = kept
	return removed
}

// AddScopeMember adds a scope member to the mirror.
// It returns true if the member was added, false if already present.
func (m *Mirror) AddScopeMember(mem ScopeMember) bool {
	if mem.IsZero() {
		return false
	}
	if m.HasScopeMember(mem.Key) {
		return false
	}
	m.ScopeMembers = append(m.ScopeMembers, mem)
	return true
}

// RenameIssue rekeys every membership record of oldKey (imported issue, scope
// member, remote timestamp) to newKey. It reports whether anything changed.
func (m *Mirror) RenameIssue(oldKey, newKey schema.IssueKey) bool {
	changed := false
	for i := range m.Issues {
		if m.Issues[i].Key == oldKey {
			m.Issues[i].Key = newKey
			changed = true
		}
	}
	for i := range m.ScopeMembers {
		if m.ScopeMembers[i].Key == oldKey {
			m.ScopeMembers[i].Key = newKey
			changed = true
		}
	}
	if u, ok := m.RemoteUpdated[oldKey]; ok {
		delete(m.RemoteUpdated, oldKey)
		if _, has := m.RemoteUpdated[newKey]; !has {
			m.RemoteUpdated[newKey] = u
		}
		changed = true
	}
	return changed
}

// ForgetIssue drops every membership record of key. It reports whether
// anything changed.
func (m *Mirror) ForgetIssue(key schema.IssueKey) bool {
	changed := m.RemoveScopeMember(key)
	kept := m.Issues[:0]
	for _, i := range m.Issues {
		if i.Key == key {
			changed = true
			continue
		}
		kept = append(kept, i)
	}
	m.Issues = kept
	if _, ok := m.RemoteUpdated[key]; ok {
		delete(m.RemoteUpdated, key)
		changed = true
	}
	return changed
}
